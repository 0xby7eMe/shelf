//go:build darwin

package desktop

// Tray does nothing on macOS. The menu bar icon library runs its own Cocoa
// event loop, which can't share the process with the app window's.
type Tray struct {
	Icon     []byte
	OnToggle func()
	OnShow   func()
	OnQuit   func()
	OnLaunch func(id string)
}

func (t *Tray) Start()              {}
func (t *Tray) Stop()               {}
func (t *Tray) Running() bool       { return false }
func (t *Tray) SetRecent(_ []Entry) {}
