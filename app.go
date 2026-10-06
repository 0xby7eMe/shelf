package main

import (
	"context"
	"fmt"
	"sync"

	"shelf/internal/library"
)

type App struct {
	ctx context.Context
	lib *library.Library

	mu    sync.RWMutex
	paths map[string]string
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

func NewApp() *App {
	return &App{lib: library.New(library.NewSteam())}
}

func (a *App) Launch(appID string) error {
	return library.Launch(appID)
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}