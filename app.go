package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

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
		lib:     library.New(library.NewSteam(), ep.Provider()),
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
	}
	return fmt.Errorf("unknown game")
}

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

func (a *App) GetProtonBuilds() []epic.ProtonBuild { return epic.FindProton() }

func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}