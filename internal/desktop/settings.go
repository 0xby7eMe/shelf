// Package desktop connects Shelf to the rest of the desktop: application menu entries for games, the shelf:// link scheme and a
// tray icon.
package desktop

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// Settings are the switches on the desktop integration page. Everything is off
// until the user turns it on.
type Settings struct {
	MenuEntries bool `json:"menuEntries"` // an application menu entry per installed game
	URLHandler  bool `json:"urlHandler"`  // open shelf:// links
	Tray        bool `json:"tray"`        // a tray icon; applied at the next start
	CloseToTray bool `json:"closeToTray"` // closing the window leaves Shelf in the tray; applied at the next start
}

// Store keeps Settings in desktop.json in the config folder.
type Store struct {
	mu   sync.Mutex
	path string
	s    Settings
}

func NewStore() *Store {
	st := &Store{}
	base, err := os.UserConfigDir()
	if err != nil {
		log.Printf("desktop: no config dir, keeping settings in memory: %v", err)
		return st
	}
	dir := filepath.Join(base, "shelf")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("desktop: %v", err)
		return st
	}
	st.path = filepath.Join(dir, "desktop.json")
	if raw, err := os.ReadFile(st.path); err == nil {
		var saved Settings
		if json.Unmarshal(raw, &saved) == nil {
			st.s = saved
		}
	}
	return st
}

func (st *Store) Get() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.s
}

func (st *Store) Set(s Settings) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	if st.path != "" {
		raw, err := json.MarshalIndent(s, "", "  ")
		if err != nil {
			return err
		}
		tmp := st.path + ".tmp"
		if err := os.WriteFile(tmp, raw, 0o644); err != nil {
			return err
		}
		if err := os.Rename(tmp, st.path); err != nil {
			return err
		}
	}
	st.s = s
	return nil
}
