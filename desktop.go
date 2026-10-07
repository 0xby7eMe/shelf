package main

import (
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"shelf/internal/desktop"
	"shelf/internal/library"
)

// DesktopStatus is the desktop integration page: the settings and what is
// actually going on.
type DesktopStatus struct {
	Settings         desktop.Settings `json:"settings"`
	DiscordConnected bool             `json:"discordConnected"`
	TrayRunning      bool             `json:"trayRunning"`
	URLHandler       bool             `json:"urlHandler"` // the shelf:// handler file is in place
}

func (a *App) GetDesktopStatus() DesktopStatus {
	return DesktopStatus{
		Settings:         a.desk.Get(),
		DiscordConnected: a.presence.Connected(),
		TrayRunning:      a.tray != nil && a.tray.Running(),
		URLHandler:       desktop.URLHandlerRegistered(desktop.ApplicationsDir()),
	}
}

// SetDesktopSettings saves the settings and applies them. The tray is the one
// thing that waits for the next start.
func (a *App) SetDesktopSettings(s desktop.Settings) (DesktopStatus, error) {
	if err := a.desk.Set(s); err != nil {
		return DesktopStatus{}, err
	}
	a.applyDesktop(s)
	return a.GetDesktopStatus(), nil
}

func (a *App) applyDesktop(s desktop.Settings) {
	if s.DiscordEnabled {
		a.presence.Enable(s.DiscordClientID)
		a.updatePresence(a.monitor.Snapshot())
	} else {
		a.presence.Disable()
	}

	dir := desktop.ApplicationsDir()
	if s.URLHandler {
		exe, err := desktop.Executable()
		if err == nil {
			err = desktop.RegisterURLHandler(dir, exe, library.ChildEnv())
		}
		if err != nil {
			log.Printf("desktop: url handler: %v", err)
		}
	} else if err := desktop.UnregisterURLHandler(dir, library.ChildEnv()); err != nil {
		log.Printf("desktop: url handler: %v", err)
	}

	if !s.MenuEntries {
		if _, err := desktop.RemoveEntries(dir); err != nil {
			log.Printf("desktop: menu entries: %v", err)
		}
	}
	go a.syncDesktop(nil)
}

// startDesktop applies the saved settings and starts the tray at startup.
func (a *App) startDesktop() {
	s := a.desk.Get()
	a.applyDesktop(s)

	if s.Tray {
		a.tray = &desktop.Tray{
			Icon:     icon,
			OnToggle: a.showWindow,
			OnShow:   a.showWindow,
			OnQuit:   func() { runtime.Quit(a.ctx) },
			OnLaunch: a.launchFromLink,
		}
		a.tray.Start()
	}

	if id, ok := desktop.LaunchFromArgs(osArgs()); ok {
		go a.launchFromLink(id)
	}
}

func (a *App) showWindow() {
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}

// onSecondInstance runs when Shelf is started again, e.g. by a shelf:// link or
// a menu entry. The running copy handles it instead.
func (a *App) onSecondInstance(args []string) {
	if id, ok := desktop.LaunchFromArgs(args); ok {
		a.launchFromLink(id)
		return
	}
	a.showWindow()
}

// launchFromLink starts a game the way the Play button does.
// Links can come from anywhere, so they only start games that are installed.
func (a *App) launchFromLink(id string) {
	store, ext := splitID(id)
	g, ok := a.gameByExternalID(ext)
	var err error
	switch {
	case !ok || g.Source != store:
		err = fmt.Errorf("it isn't in your library")
	case !g.Installed:
		err = fmt.Errorf("it isn't installed")
	default:
		err = a.Launch(id)
	}
	if err != nil {
		log.Printf("launch %s: %v", id, err)
		runtime.EventsEmit(a.ctx, "desktop:error", fmt.Sprintf("Couldn't start %s: %v", nameOr(g, id), err))
	}
}

// gameByExternalID finds a game by the id sessions use, rescanning once if it is new.
func (a *App) gameByExternalID(ext string) (library.Game, bool) {
	a.mu.RLock()
	g, ok := a.known[ext]
	a.mu.RUnlock()
	if ok {
		return g, true
	}
	a.GetGames()
	a.mu.RLock()
	defer a.mu.RUnlock()
	g, ok = a.known[ext]
	return g, ok
}

var storeNames = map[library.Source]string{
	library.SourceSteam:   "Steam",
	library.SourceEpic:    "Epic Games",
	library.SourceUbisoft: "Ubisoft",
}

// updatePresence shows the game started last on Discord, or clears it.
func (a *App) updatePresence(now []library.Session) {
	if !a.desk.Get().DiscordEnabled {
		return
	}
	if len(now) == 0 {
		a.presence.Set(nil)
		return
	}
	latest := now[len(now)-1] // sessions are sorted oldest first
	g, ok := a.gameByExternalID(latest.AppID)
	if !ok {
		a.presence.Set(nil)
		return
	}
	a.presence.Set(&desktop.Activity{
		Details: g.Name,
		State:   "on " + storeNames[g.Source],
		Start:   latest.Since / int64(time.Second/time.Millisecond),
	})
}

// syncDesktop brings the menu entries and the tray's recent games up to date.
// With nil games it uses the last scan.
func (a *App) syncDesktop(games []library.Game) {
	a.syncMu.Lock()
	defer a.syncMu.Unlock()

	if games == nil {
		a.mu.RLock()
		for _, g := range a.known {
			games = append(games, g)
		}
		a.mu.RUnlock()
	}

	s := a.desk.Get()
	if s.MenuEntries {
		entries := make([]desktop.Entry, 0, len(games))
		for _, g := range games {
			if g.Installed {
				entries = append(entries, desktop.Entry{ID: g.ID, Name: g.Name})
			}
		}
		exe, err := desktop.Executable()
		if err == nil {
			_, _, err = desktop.SyncEntries(desktop.ApplicationsDir(), exe, entries)
		}
		if err != nil {
			log.Printf("desktop: menu entries: %v", err)
		}
	}

	if a.tray != nil {
		recent := make([]library.Game, 0, len(games))
		for _, g := range games {
			if g.Installed && g.LastPlayed > 0 {
				recent = append(recent, g)
			}
		}
		sort.Slice(recent, func(i, j int) bool { return recent[i].LastPlayed > recent[j].LastPlayed })
		entries := make([]desktop.Entry, 0, len(recent))
		for _, g := range recent {
			entries = append(entries, desktop.Entry{ID: g.ID, Name: g.Name})
		}
		a.tray.SetRecent(entries)
	}
}

func nameOr(g library.Game, fallback string) string {
	if g.Name != "" {
		return g.Name
	}
	return fallback
}
