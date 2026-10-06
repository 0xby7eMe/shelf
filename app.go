package main

import (
	"context"
	"fmt"
	"sync"
	"log"
	"time"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"shelf/internal/library"
)

type App struct {
	ctx     context.Context
	lib     *library.Library
	favs    *library.Favorites
	monitor *library.Monitor
	hist    *library.History

	mu    sync.RWMutex
	paths map[string]string
}

func NewApp() *App {
	return &App{
		lib:     library.New(library.NewSteam()),
		favs:    library.NewFavorites(),
		monitor: library.NewMonitor(),
		hist:    library.NewHistory(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

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
			paths[g.ExternalID] = g.InstallPath
		}
	}
	a.mu.Lock()
	a.paths = paths
	a.mu.Unlock()

	return games
}

func (a *App) OpenInstallFolder(appID string) error {
	a.mu.RLock()
	p, ok := a.paths[appID]
	a.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown game")
	}
	return library.OpenFolder(p)
}

func (a *App) OpenStorePage(appID string) error {
	return library.OpenStore(appID)
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

func (a *App) Launch(appID string) error {
	return library.Launch(appID)
}

func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}