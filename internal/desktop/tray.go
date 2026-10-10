//go:build !darwin

package desktop

import (
	"sync"

	"fyne.io/systray"
)

const recentSlots = 5

// Tray is the tray icon: click to show or hide the window, and a menu with the
// games played last. On Linux it talks to the desktop over D-Bus
// (StatusNotifierItem), so it needs a desktop that shows those; on one that
// doesn't it does nothing. On Windows the icon has to be an .ico.
type Tray struct {
	Icon     []byte
	OnToggle func() // the icon was clicked
	OnShow   func()
	OnQuit   func()
	OnLaunch func(id string)

	mu      sync.Mutex
	ready   bool
	items   [recentSlots]*systray.MenuItem
	ids     [recentSlots]string
	pending []Entry
	started bool
	end     func()
}

// Start puts the icon in the tray. It returns at once.
func (t *Tray) Start() {
	t.mu.Lock()
	if t.started {
		t.mu.Unlock()
		return
	}
	t.started = true
	t.mu.Unlock()

	end := runTray(t.onReady)
	t.mu.Lock()
	t.end = end
	t.mu.Unlock()
}

// Stop removes the icon.
func (t *Tray) Stop() {
	t.mu.Lock()
	end := t.end
	t.started, t.ready, t.end = false, false, nil
	t.mu.Unlock()
	if end != nil {
		end()
	}
}

// Running reports whether the icon is up.
func (t *Tray) Running() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.ready
}

func (t *Tray) onReady() {
	if len(t.Icon) > 0 {
		systray.SetIcon(t.Icon)
	}
	systray.SetTitle("Shelf")
	systray.SetTooltip("Shelf")
	systray.SetOnTapped(func() {
		if t.OnToggle != nil {
			t.OnToggle()
		}
	})

	show := systray.AddMenuItem("Show Shelf", "")
	systray.AddSeparator()
	header := systray.AddMenuItem("Recently played", "")
	header.Disable()

	t.mu.Lock()
	for i := range t.items {
		item := systray.AddMenuItem("", "")
		item.Hide()
		t.items[i] = item
		go t.watchRecent(i, item)
	}
	t.mu.Unlock()

	systray.AddSeparator()
	quit := systray.AddMenuItem("Quit", "")

	go func() {
		for range show.ClickedCh {
			if t.OnShow != nil {
				t.OnShow()
			}
		}
	}()
	go func() {
		for range quit.ClickedCh {
			if t.OnQuit != nil {
				t.OnQuit()
			}
		}
	}()

	t.mu.Lock()
	t.ready = true
	pending := t.pending
	t.mu.Unlock()
	t.SetRecent(pending)
}

func (t *Tray) watchRecent(i int, item *systray.MenuItem) {
	for range item.ClickedCh {
		t.mu.Lock()
		id := t.ids[i]
		t.mu.Unlock()
		if id != "" && t.OnLaunch != nil {
			t.OnLaunch(id)
		}
	}
}

// SetRecent lists the games in the menu, most recent first. Only the first few fit.
func (t *Tray) SetRecent(games []Entry) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pending = games
	if !t.ready {
		return
	}
	for i, item := range t.items {
		if item == nil {
			continue
		}
		if i < len(games) {
			t.ids[i] = games[i].ID
			item.SetTitle(games[i].Name)
			item.Show()
		} else {
			t.ids[i] = ""
			item.Hide()
		}
	}
}
