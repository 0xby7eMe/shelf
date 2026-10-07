package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"shelf/internal/applog"
	"shelf/internal/desktop"
	"shelf/internal/epic"
	"shelf/internal/library"
	"shelf/internal/sysmon"
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

	desk     *desktop.Store
	presence *desktop.Presence
	tray     *desktop.Tray
	syncMu   sync.Mutex // one desktop sync at a time

	mu    sync.RWMutex
	paths map[string]string
	known map[string]library.Game // by external id, for Discord and the tray
}

func NewApp() *App {
	hist := library.NewHistory()
	ep := epic.New(hist, nil)
	monitor := library.NewMonitor()
	monitor.Extra = ep.Running
	sys := sysmon.New()

	return &App{
		lib:     library.New(library.NewSteam(), ep.Provider(), ep.UbisoftProvider()),
		favs:    library.NewFavorites(),
		org:     library.NewOrganizer(),
		monitor: monitor,
		hist:    hist,
		epic:    ep,
		sys:     sys,

		desk:     desktop.NewStore(),
		presence: desktop.NewPresence(),
		known:    map[string]library.Game{},
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
		a.updatePresence(now)
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
	}
	return fmt.Errorf("unknown game")
}

func (a *App) shutdown(ctx context.Context) {
	a.presence.Disable()
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
		return library.Launch(ext)
	case library.SourceEpic:
		return a.epic.Launch(ext)
	case library.SourceUbisoft:
		return a.epic.UbisoftLaunch(ext)
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

func (a *App) GetEpicAccount() epic.Account { return a.epic.Account() }

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
