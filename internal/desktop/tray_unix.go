//go:build !darwin && !windows

package desktop

import "fyne.io/systray"

// runTray runs the tray beside the window's own event loop: on Linux the icon
// talks to the desktop over D-Bus, so it needs no loop of its own. It returns
// what ends it.
func runTray(onReady func()) (end func()) {
	start, end := systray.RunWithExternalLoop(onReady, func() {})
	go start()
	return end
}
