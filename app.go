package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"shelf/internal/achievements"
	"shelf/internal/applog"
	"shelf/internal/desktop"
	"shelf/internal/epic"
	"shelf/internal/friends"
	"shelf/internal/library"
	"shelf/internal/steamapi"
	"shelf/internal/sysmon"
	"shelf/internal/update"
)

type App struct {
	ctx     context.Context
	lib     *library.Library
	favs    *library.Favorites
	org     *library.Organizer
	monitor *library.Monitor
	hist    *library.History
	epic    *epic.Manager
	sys     *sysmon.Sampler
	hw      *sysmon.Monitor
	covers  *library.Covers
	friends *friends.Hub
	steam   *steamapi.Client
	ach     *achievements.Hub
	updater *update.Updater

	desk *desktop.Store
	tray *desktop.Tray

	winMu    sync.Mutex
	hidden   bool // the window is hidden in the tray
	quitting bool
	syncMu   sync.Mutex // one desktop sync at a time

	mu    sync.RWMutex
	paths map[string]string
	known map[string]library.Game // by external id, for the tray
}

func NewApp() *App {
	hist := library.NewHistory()
	ep := epic.New(hist, nil)
	monitor := library.NewMonitor()
	monitor.Extra = ep.Running
	sys := sysmon.New()

	steam := steamapi.New(library.SteamUserID)

	a := &App{
		steam:   steam,
		lib:     library.New(library.NewSteam(steam), ep.Provider(), ep.UbisoftProvider(), ep.GogProvider()),
		favs:    library.NewFavorites(),
		org:     library.NewOrganizer(),
		monitor: monitor,
		hist:    hist,
		epic:    ep,
		sys:     sys,

		friends: friends.NewHub(friends.NewStore(),
			friends.NewSteam(steam),
			friends.NewUnsupported("epic", "Epic Games", "Epic only shares who is online over a private channel that other apps can't use, and legendary has no friends feature."),
			friends.NewUnsupported("ubisoft", "Ubisoft", "Ubisoft Connect has no way for other apps to read your friends."),
			friends.NewGOG(ep),
		),

		desk:  desktop.NewStore(),
		known: map[string]library.Game{},

		updater: update.New(version, "0xby7eMe/shelf", update.NewStore()),
	}
	a.ach = achievements.NewHub(achievements.DefaultCachePath(), a.emitEvent,
		achievements.NewSteam(steam),
		achievements.NewUnsupported("epic", "Epic Games", "Epic only serves achievements to a game's own developer, with credentials issued per game, so other apps can't read them."),
		achievements.NewUnsupported("ubisoft", "Ubisoft", "Ubisoft Connect has no public way for other apps to read your achievements."),
		achievements.NewGOG(ep),
	)
	return a
}

// emitEvent sends an event to the interface, once there is one.
func (a *App) emitEvent(name string, data any) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, name, data)
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.epic.SetEmitter(func(event string, data any) {
		runtime.EventsEmit(ctx, event, data)
	})
	a.epic.Log().SetEmitter(func(lines []applog.Line) {
		runtime.EventsEmit(ctx, "log:lines", lines)
	})

	a.epic.Start(ctx)
	a.startDesktop()
	go a.watchUpdates(ctx)

	go func() {
		err := library.Watch(ctx, func() {
			runtime.EventsEmit(ctx, "library:changed")
		})
		if err != nil {
			log.Printf("watch: %v", err)
		}
	}()

	go a.monitor.Run(ctx, 2*time.Second, func(now, stopped []library.Session) {
		runtime.EventsEmit(ctx, "nowplaying:changed", now)
		if len(stopped) == 0 {
			return
		}

		end := time.Now().UnixMilli()
		recorded := false
		for _, s := range stopped {
			a.epic.GameStopped(s.AppID)
			ok, err := a.hist.Add(s.AppID, s.Since, end)
			if err != nil {
				log.Printf("history: %v", err)
			}
			recorded = recorded || ok
		}
		if recorded {
			runtime.EventsEmit(ctx, "history:changed")
		}

		time.AfterFunc(5*time.Second, func() {
			runtime.EventsEmit(ctx, "library:changed")
		})
	})
}

// --- hardware monitor ---

func (a *App) hardware() *sysmon.Monitor {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.hw == nil {
		a.hw = sysmon.NewMonitor(a.sys, func(s sysmon.Sample) {
			runtime.EventsEmit(a.ctx, "hardware:sample", s)
		})
	}
	return a.hw
}

// HardwareInfo is what doesn't change: names, sizes and capabilities.
func (a *App) HardwareInfo() sysmon.Info { return a.sys.Info() }

// HardwareStart starts the samples that arrive as "hardware:sample" events, and
// keeps them coming. The view calls it again every few seconds; if it stops
// calling, so do the samples.
func (a *App) HardwareStart() { a.hardware().Start() }

func (a *App) HardwareStop() { a.hardware().Stop() }

// HardwareHistory is the samples taken so far, oldest first.
func (a *App) HardwareHistory() []sysmon.Sample { return a.hardware().History() }

func (a *App) GetNowPlaying() []library.Session {
	return a.monitor.Snapshot()
}

// --- collections and tags ---

func (a *App) GetOrganization() library.Organization { return a.org.Get() }

func (a *App) CreateCollection(name string) (library.Organization, error) {
	return a.org.CreateCollection(name)
}

func (a *App) RenameCollection(id, name string) (library.Organization, error) {
	return a.org.RenameCollection(id, name)
}

func (a *App) DeleteCollection(id string) (library.Organization, error) {
	return a.org.DeleteCollection(id)
}

// SetGameCollections files a game under exactly these collections.
func (a *App) SetGameCollections(gameID string, collectionIDs []string) (library.Organization, error) {
	return a.org.SetGameCollections(gameID, collectionIDs)
}

func (a *App) SetGameTags(gameID string, tags []string) (library.Organization, error) {
	return a.org.SetTags(gameID, tags)
}

// DeleteTag removes a tag from every game.
func (a *App) DeleteTag(tag string) (library.Organization, error) { return a.org.DeleteTag(tag) }

// GetCardCover is a game's cover as a data URL, for drawing the share card.
// It is empty when the game has none.
func (a *App) GetCardCover(id string) string {
	_, ext := splitID(id)
	g, ok := a.gameByExternalID(ext)
	if !ok || a.covers == nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
	defer cancel()
	url, err := a.covers.DataURL(ctx, g)
	if err != nil {
		log.Printf("share card: %v", err)
		return ""
	}
	return url
}

// SaveShareCard asks where to put the finished card and writes it there. It
// returns the path, or "" if the dialog was cancelled.
func (a *App) SaveShareCard(dataURL string) (string, error) {
	data, err := library.DecodePNGDataURL(dataURL)
	if err != nil {
		return "", err
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save your Shelf card",
		DefaultFilename: "shelf-card.png",
		Filters:         []runtime.FileFilter{{DisplayName: "PNG image", Pattern: "*.png"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	if !strings.HasSuffix(strings.ToLower(path), ".png") {
		path += ".png"
	}
	return path, os.WriteFile(path, data, 0o644)
}

// GetFriends is who is online and what they play, from every launcher that can
// say. Results are cached for a short while unless force is set.
func (a *App) GetFriends(force bool) friends.Snapshot {
	return a.friends.Snapshot(a.ctx, force)
}

// SetFriendsConfig saves what the user entered for a launcher, such as an API key.
func (a *App) SetFriendsConfig(source string, values map[string]string) (friends.Snapshot, error) {
	if err := a.friends.Configure(source, values); err != nil {
		return friends.Snapshot{}, err
	}
	return a.friends.Snapshot(a.ctx, true), nil
}

// DisconnectFriends forgets what was saved for a launcher.
func (a *App) DisconnectFriends(source string) (friends.Snapshot, error) {
	if err := a.friends.Disconnect(source); err != nil {
		return friends.Snapshot{}, err
	}
	return a.friends.Snapshot(a.ctx, true), nil
}

// SteamAPIStatus is the Steam integration page: whether a key is saved, and
// the account card when Steam answers.
type SteamAPIStatus struct {
	Configured bool               `json:"configured"`
	Message    string             `json:"message,omitempty"` // what is missing or went wrong
	Overview   *steamapi.Overview `json:"overview,omitempty"`
}

// steamStatus describes the integration. The error is why the account card is
// missing, if it is; it is also in Message for the interface.
func (a *App) steamStatus() (SteamAPIStatus, error) {
	st := SteamAPIStatus{Configured: a.steam.Configured()}
	if !st.Configured {
		return st, nil
	}
	if ok, why := a.steam.Ready(); !ok {
		st.Message = why
		return st, errors.New(why)
	}
	ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
	defer cancel()
	o, err := a.steam.Overview(ctx)
	if err != nil {
		st.Message = err.Error()
		return st, err
	}
	st.Overview = &o
	return st, nil
}

func (a *App) GetSteamAPI() SteamAPIStatus {
	st, _ := a.steamStatus()
	return st
}

// SetSteamAPIKey saves a key once Steam accepts it, then refreshes the library
// so the games that aren't installed show up.
func (a *App) SetSteamAPIKey(key string) (SteamAPIStatus, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return SteamAPIStatus{}, fmt.Errorf("paste your Web API key first")
	}
	prev := a.steam.Key()
	if err := a.steam.SetKey(key); err != nil {
		return SteamAPIStatus{}, err
	}
	st, err := a.steamStatus()
	if errors.Is(err, steamapi.ErrBadKey) {
		// A key Steam turns down isn't kept. Other trouble, such as being offline, is not the key's fault.
		a.steam.SetKey(prev)
		return SteamAPIStatus{}, err
	}
	runtime.EventsEmit(a.ctx, "library:changed")
	return st, nil
}

// DisconnectSteamAPI forgets the key. Games that aren't installed leave the library again.
func (a *App) DisconnectSteamAPI() (SteamAPIStatus, error) {
	if err := a.steam.SetKey(""); err != nil {
		return SteamAPIStatus{}, err
	}
	runtime.EventsEmit(a.ctx, "library:changed")
	st, _ := a.steamStatus()
	return st, nil
}

// GetAchievementOverview is what is known of your progress, without asking anyone.
func (a *App) GetAchievementOverview() achievements.Overview { return a.ach.Overview() }

// ScanAchievements looks your played games up in the background and returns
// what is known so far. Progress comes as "achievements:changed" events.
func (a *App) ScanAchievements(force bool) achievements.Overview {
	a.mu.RLock()
	refs := make([]achievements.GameRef, 0, len(a.known))
	for _, g := range a.known {
		// GOG keeps achievements of games played anywhere, so installed GOG games count too.
		if g.PlaytimeMinutes > 0 || (g.Source == library.SourceGog && g.Installed) {
			refs = append(refs, achievements.GameRef{Source: string(g.Source), GameID: g.ExternalID, Name: g.Name})
		}
	}
	a.mu.RUnlock()
	a.ach.Scan(a.ctx, refs, force)
	return a.ach.Overview()
}

// GetGameAchievements lists one game's achievements, for the game's sheet.
func (a *App) GetGameAchievements(id string) (achievements.Detail, error) {
	store, ext := splitID(id)
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	return a.ach.Detail(ctx, string(store), ext)
}

// Version is the release this copy was built from, or "dev".
func (a *App) Version() string { return version }

func (a *App) GetUpdateStatus() update.Status { return a.updater.Status() }

func (a *App) CheckForUpdates() (update.Status, error) {
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	return a.updater.Check(ctx)
}

func (a *App) SetAutoUpdateCheck(on bool) (update.Status, error) {
	err := a.updater.SetAutoCheck(on)
	return a.updater.Status(), err
}

func (a *App) SkipUpdate(tag string) (update.Status, error) {
	err := a.updater.Skip(tag)
	return a.updater.Status(), err
}

// InstallUpdate downloads the newest release and swaps it in. Progress comes
// as "update:progress" events; the new version runs after RestartApp.
func (a *App) InstallUpdate() (string, error) {
	var last time.Time
	return a.updater.Install(a.ctx, func(done, total int64) {
		if time.Since(last) < 200*time.Millisecond && done != total {
			return
		}
		last = time.Now()
		runtime.EventsEmit(a.ctx, "update:progress", map[string]int64{"done": done, "total": total})
	})
}

// RestartApp starts the installed copy again once this one has quit.
func (a *App) RestartApp() error {
	path, err := a.updater.RelaunchPath()
	if err != nil {
		return err
	}
	// The new copy has to wait for this one to let go of the single-instance lock.
	cmd := exec.Command("sh", "-c", `sleep 2; exec "$0"`, path)
	cmd.Env = library.ChildEnv()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	a.quit()
	return nil
}

// watchUpdates checks for a new release shortly after start and then daily,
// and tells the interface once per release.
func (a *App) watchUpdates(ctx context.Context) {
	notified := ""
	notify := func(st update.Status) {
		if st.Available && !st.Skipped && st.Latest != notified {
			notified = st.Latest
			runtime.EventsEmit(ctx, "update:available", st)
		}
	}

	delay := 20 * time.Second
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		delay = 24 * time.Hour

		// What the last run found counts even if no check is due.
		if st := a.updater.Status(); st.AutoCheck {
			notify(st)
		}
		if !a.updater.DueForCheck(6 * time.Hour) {
			continue
		}
		cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		st, err := a.updater.Check(cctx)
		cancel()
		if err != nil {
			log.Printf("updates: %v", err)
			continue
		}
		notify(st)
	}
}

func (a *App) GetFavorites() []string { return a.favs.List() }

func (a *App) ToggleFavorite(appID string) ([]string, error) {
	return a.favs.Toggle(appID)
}

func (a *App) GetGames() []library.Game {
	games := a.lib.Scan()

	paths := make(map[string]string, len(games))
	for _, g := range games {
		if g.Installed && g.InstallPath != "" {
			paths[g.ID] = g.InstallPath
		}
	}
	known := make(map[string]library.Game, len(games))
	for _, g := range games {
		known[g.ExternalID] = g
	}
	a.mu.Lock()
	a.paths = paths
	a.known = known
	a.mu.Unlock()

	go a.syncDesktop(games)

	return games
}

// splitID breaks a game id such as "epic:Fortnite" into its store and external id.
func splitID(id string) (library.Source, string) {
	store, ext, ok := strings.Cut(id, ":")
	if !ok {
		return "", id
	}
	return library.Source(store), ext
}

func (a *App) OpenInstallFolder(id string) error {
	a.mu.RLock()
	p, ok := a.paths[id]
	a.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown game")
	}
	return library.OpenFolder(p)
}

func (a *App) OpenStorePage(id string) error {
	switch store, ext := splitID(id); store {
	case library.SourceSteam:
		return library.OpenStore(ext)
	case library.SourceEpic:
		u, err := a.epic.StoreURL(ext)
		if err != nil {
			return err
		}
		return library.OpenURL(u)
	case library.SourceUbisoft:
		u, err := a.epic.UbisoftStoreURL(ext)
		if err != nil {
			return err
		}
		return library.OpenURL(u)
	case library.SourceGog:
		u, err := a.epic.GogStoreURL(ext)
		if err != nil {
			return err
		}
		return library.OpenURL(u)
	}
	return fmt.Errorf("unknown game")
}

func (a *App) shutdown(ctx context.Context) {
	if a.tray != nil {
		a.tray.Stop()
	}
	end := time.Now().UnixMilli()
	for _, s := range a.monitor.Snapshot() {
		if _, err := a.hist.Add(s.AppID, s.Since, end); err != nil {
			log.Printf("history: %v", err)
		}
	}
}

func (a *App) GetStats(weeks int) library.Stats {
	return a.hist.Stats(weeks)
}

func (a *App) Launch(id string) error {
	switch store, ext := splitID(id); store {
	case library.SourceSteam:
		// A game that isn't installed is one the Web API listed: open Steam's install dialog.
		a.mu.RLock()
		g, known := a.known[ext]
		a.mu.RUnlock()
		if known && g.Source == library.SourceSteam && !g.Installed {
			return library.InstallSteam(ext)
		}
		return library.Launch(ext)
	case library.SourceEpic:
		return a.epic.Launch(ext)
	case library.SourceUbisoft:
		return a.epic.UbisoftLaunch(ext)
	case library.SourceGog:
		return a.epic.GogLaunch(ext)
	}
	return fmt.Errorf("unknown game")
}

func (a *App) UbisoftSync() error { return a.epic.UbisoftSync() }

func (a *App) GetUbisoftStatus() epic.UbisoftStatus { return a.epic.UbisoftStatus() }

func (a *App) GetUbisoftSetup() epic.UbisoftSetupState { return a.epic.UbisoftSetupState() }

func (a *App) UbisoftSetup() error { return a.epic.UbisoftSetup() }

func (a *App) UbisoftOpenConnect() error { return a.epic.UbisoftOpenConnect() }

func (a *App) UbisoftCloseConnect() error { return a.epic.UbisoftCloseConnect() }

func (a *App) UbisoftReset() error { return a.epic.UbisoftReset() }

func (a *App) UbisoftInstall(id string) error { return a.epic.UbisoftInstall(id) }

func (a *App) UbisoftUninstall(id string) error { return a.epic.UbisoftUninstall(id) }

func (a *App) UbisoftInstalling() []epic.UbisoftInstalling { return a.epic.UbisoftInstallStates() }

func (a *App) GetGogAccount() epic.GogAccount { return a.epic.GogAccount() }

func (a *App) GogOpenLogin() error { return library.OpenURL(epic.GogLoginURL) }

func (a *App) GogLogin(code string) error { return a.epic.GogLogin(code) }

func (a *App) GogLogout() error { return a.epic.GogLogout() }

func (a *App) GogSync() error { return a.epic.GogSync() }

// GogInstall queues a GOG game's download; it shows in the same queue as Epic's.
// Cancel it with EpicCancelInstall.
func (a *App) GogInstall(key string) error { return a.epic.GogInstall(key) }

func (a *App) GogUninstall(key string) error { return a.epic.GogUninstall(key) }

// GogUpdate queues a GOG game's newer installer, run over the installed game.
func (a *App) GogUpdate(key string) error { return a.epic.GogUpdate(key) }

// GogUpdates lists installed GOG games with a newer version, as last checked.
func (a *App) GogUpdates() []epic.UpdateInfo { return a.epic.GogUpdates() }

// GogCheckUpdates asks GOG for the current versions and returns the games that are behind.
func (a *App) GogCheckUpdates() ([]epic.UpdateInfo, error) { return a.epic.GogCheckUpdates() }

func (a *App) GetEpicAccount() epic.Account { return a.epic.Account() }

// InstallLegendary downloads legendary's standalone build for this system.
// Progress comes as "legendary:install" events.
func (a *App) InstallLegendary() error { return a.epic.InstallLegendary() }

func (a *App) GetLegendaryInstall() epic.ProtonInstallState { return a.epic.LegendaryInstallState() }

func (a *App) EpicLoginURL() string { return epic.LoginURL }

func (a *App) EpicOpenLogin() error { return library.OpenURL(epic.LoginURL) }

func (a *App) EpicLogin(code string) error { return a.epic.Login(code) }

func (a *App) EpicLogout() error { return a.epic.Logout() }

func (a *App) EpicSync() error { return a.epic.SyncLibrary() }

func (a *App) EpicInstall(appName string) error { return a.epic.Install(appName) }

func (a *App) EpicCancelInstall(appName string) { a.epic.CancelInstall(appName) }

func (a *App) EpicUninstall(appName string) error { return a.epic.Uninstall(appName) }

func (a *App) EpicInstallStates() []epic.Progress { return a.epic.InstallStates() }

func (a *App) EpicLaunchInfo(appName string) (epic.LaunchInfo, error) {
	return a.epic.LaunchInfo(appName)
}

func (a *App) GetEpicSettings() epic.Settings { return a.epic.Settings() }

func (a *App) SetEpicSettings(s epic.Settings) error { return a.epic.SetSettings(s) }

func (a *App) EpicUpdate(appName string) error { return a.epic.Update(appName) }

func (a *App) EpicVerify(appName string) error { return a.epic.Verify(appName) }

func (a *App) EpicRepair(appName string) error { return a.epic.Repair(appName) }

func (a *App) EpicUpdates() []epic.UpdateInfo { return a.epic.Updates() }

func (a *App) EpicCheckUpdates() ([]epic.UpdateInfo, error) { return a.epic.CheckUpdates() }

func (a *App) EpicSyncSaves(appName string) error { return a.epic.SyncSaves(appName) }

func (a *App) GetEpicGameSettings(appName string) (epic.GameSettings, error) {
	return a.epic.GameSettings(appName)
}

func (a *App) SetEpicGameSettings(appName string, s epic.GameSettings) error {
	return a.epic.SetGameSettings(appName, s)
}

func (a *App) GetEpicTools() epic.Tools { return a.epic.Tools() }

func (a *App) EpicFindImportable() ([]epic.Importable, error) { return a.epic.FindImportable() }

func (a *App) EpicImport(appName, path string) error { return a.epic.Import(appName, path) }

func (a *App) EpicImportAll() (int, error) { return a.epic.ImportAll() }

func (a *App) EpicQueue() epic.QueueState { return a.epic.QueueState() }

func (a *App) EpicQueueMove(appName string, delta int) error { return a.epic.QueueMove(appName, delta) }

func (a *App) EpicSetQueuePaused(paused bool) { a.epic.SetQueuePaused(paused) }

// Storage is what the storage view needs beyond the game list: free space and prefixes.
type Storage struct {
	Volumes  []library.Volume   `json:"volumes"`
	Prefixes []epic.PrefixUsage `json:"prefixes"`
}

func (a *App) GetStorage() Storage {
	var paths []string
	if root, err := library.SteamRoot(); err == nil {
		for _, lib := range library.SteamLibraryDirs(root) {
			paths = append(paths, filepath.Join(lib, "steamapps"))
		}
	}
	paths = append(paths, a.epic.InstallRoots()...)
	return Storage{Volumes: library.Volumes(paths), Prefixes: a.epic.Prefixes()}
}

func (a *App) EpicDeletePrefix(appName string) error { return a.epic.DeletePrefix(appName) }

func (a *App) GetLogs() []applog.Line { return a.epic.Log().Snapshot() }

func (a *App) ClearLogs() { a.epic.Log().Clear() }

func (a *App) OpenLogsFolder() error {
	dir := epic.LogsDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return library.OpenFolder(dir)
}

func (a *App) GetBattlEyeRuntime() epic.BattlEyeRuntime { return a.epic.BattlEyeRuntime() }

func (a *App) InstallBattlEyeRuntime() error { return a.epic.InstallBattlEyeRuntime() }

func (a *App) GetBattlEyeInstall() epic.ProtonInstallState { return a.epic.BattlEyeInstallState() }

func (a *App) InstallProtonGE() error { return a.epic.InstallProtonGE() }

func (a *App) GetProtonInstall() epic.ProtonInstallState { return a.epic.ProtonInstallState() }

func (a *App) GetProtonBuilds() []epic.ProtonBuild { return epic.FindProton() }

func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}
