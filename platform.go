package main

import goruntime "runtime"

// Platform says which parts of Shelf work on this system, so the interface
// can leave out what can't. Linux has everything; macOS has no Proton, no
// /proc and no freedesktop menus.
type Platform struct {
	OS string `json:"os"` // "linux" or "darwin"
	// Proton runs Windows games: Ubisoft, GOG installs, prefixes, GE-Proton, BattlEye.
	Proton bool `json:"proton"`
	// HardwareMonitor reads /proc and /sys.
	HardwareMonitor bool `json:"hardwareMonitor"`
	// Desktop is application menu entries, shelf:// links and the tray icon.
	Desktop bool `json:"desktop"`
	// CloudSaves syncs Epic saves found inside a game's prefix.
	CloudSaves bool `json:"cloudSaves"`
}

var platform = func() Platform {
	linux := goruntime.GOOS != "darwin"
	return Platform{OS: goruntime.GOOS, Proton: linux, HardwareMonitor: linux, Desktop: linux, CloudSaves: linux}
}()

func (a *App) GetPlatform() Platform { return platform }
