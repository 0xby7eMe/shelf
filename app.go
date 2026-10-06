package main

import (
	"context"
	"fmt"

	"shelf/internal/library"
)

type App struct {
	ctx context.Context
	lib *library.Library
}

func NewApp() *App {
	return &App{lib: library.New()}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) GetGames() []library.Game {
	return a.lib.Scan()
}

func (a *App) Greet(name string) string {
	return fmt.Sprintf("Hello %s, It's show time!", name)
}