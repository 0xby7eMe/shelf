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
	"shelf/internal/epic"
	"shelf/internal/library"
)

type App struct {
	ctx     context.Context
	lib     *library.Library
	favs    *library.Favorites
	monitor *library.Monitor
	hist    *library.History
	epic    *epic.Manager

	mu    sync.RWMutex
	paths map[string]string
}

func NewApp() *App {
	hist := library.NewHistory()
	ep := epic.New(hist, nil)
	monitor := library.NewMonitor()
	monitor.Extra = ep.Running

	return &App{
		lib:     library.New(library.NewSteam(), ep.Provider(), ep.UbisoftProvider()),
		favs:    library.NewFavorites(),
		monitor: monitor,
		hist:    hist,
		epic:    ep,
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

func (a *App) GetNowPlaying() []library.Session {
	return a.monitor.Snapshot()
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
	a.mu.Lock()
	a.paths = paths
	a.mu.Unlock()

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

func (a *App) InstallProtonGE() error { return a.epic.InstallProtonGE() }

func (a *App) GetProtonInstall() epic.ProtonInstallState { return a.epic.ProtonInstallState() }

func (a *App) GetProtonBuilds() []epic.ProtonBuild { return epic.FindProton() }

func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}
