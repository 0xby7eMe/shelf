package main

import goruntime "runtime"

// Platform says which parts of Shelf work on this system, so the interface
// can leave out what can't. Linux and Windows have everything, though Windows
// runs games without Proton; macOS has no Proton, no /proc and no
// freedesktop menus.
type Platform struct {
	OS string `json:"os"` // "linux", "darwin" or "windows"
	// WindowsGames is whether Windows programs run here, natively or through
	// Proton: Ubisoft Connect and GOG's installers.
	WindowsGames bool `json:"windowsGames"`
	// Proton runs Windows games on Linux: prefixes, Proton builds, GE-Proton,
	// the BattlEye runtime, MangoHud and GameMode.
	Proton bool `json:"proton"`
	// HardwareMonitor reads /proc and /sys, or Windows' own counters.
	HardwareMonitor bool `json:"hardwareMonitor"`
	// Desktop is menu entries (Start menu shortcuts on Windows), shelf://
	// links and the tray icon.
	Desktop bool `json:"desktop"`
	// CloudSaves syncs Epic saves: found inside a game's prefix on Linux, and
	// where the game keeps them on Windows.
	CloudSaves bool `json:"cloudSaves"`
}

var platform = func() Platform {
	switch goruntime.GOOS {
	case "darwin":
		return Platform{OS: "darwin"}
	case "windows":
		return Platform{OS: "windows", WindowsGames: true, HardwareMonitor: true, Desktop: true, CloudSaves: true}
	}
	return Platform{OS: goruntime.GOOS, WindowsGames: true, Proton: true, HardwareMonitor: true, Desktop: true, CloudSaves: true}
}()

func (a *App) GetPlatform() Platform { return platform }
