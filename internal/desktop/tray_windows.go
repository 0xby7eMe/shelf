package desktop

import (
	"runtime"

	"fyne.io/systray"
)

// runTray runs the tray on a thread of its own: Windows delivers the icon's
// messages to the thread that made its window, so that thread has to run the
// message loop too. It returns what ends it.
func runTray(onReady func()) (end func()) {
	go func() {
		runtime.LockOSThread()
		systray.Run(onReady, func() {})
	}()
	return systray.Quit
}
