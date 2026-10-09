package epic

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Settings are the user's Epic-related preferences.
type Settings struct {
	// InstallDir is where games are installed. Each game gets its own folder.
	InstallDir string `json:"installDir"`
	// ProtonPath is the Proton directory to run games with. Empty picks the best one found.
	ProtonPath string `json:"protonPath"`
	// AutoCheckUpdates looks for game updates at startup and every few hours.
	AutoCheckUpdates bool `json:"autoCheckUpdates"`
	// AutoUpdate installs found updates by itself, unless a game opts out.
	AutoUpdate bool `json:"autoUpdate"`
	// CloudSaves syncs saves before a game starts and after it closes, unless a game opts out.
	CloudSaves bool `json:"cloudSaves"`
	// UbisoftSoftwareRendering draws Ubisoft Connect's own window in software.
	// On some GPUs its window stays black otherwise. Games are never affected.
	UbisoftSoftwareRendering bool `json:"ubisoftSoftwareRendering"`
}

type settingsStore struct {
	mu   sync.Mutex
	path string
	cur  Settings
}

func configDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "shelf")
}

func dataDir() string {
	if macOS {
		return configDir() // ~/Library/Application Support/shelf
	}
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "shelf")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "shelf")
}

func defaultInstallDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Games", "Shelf")
}

func newSettingsStore() *settingsStore {
	s := &settingsStore{cur: Settings{InstallDir: defaultInstallDir(), AutoCheckUpdates: true, UbisoftSoftwareRendering: true}}
	if dir := configDir(); dir != "" {
		s.path = filepath.Join(dir, "epic.json")
	}
	if s.path == "" {
		return s
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		return s
	}
	// Unmarshal over the defaults so settings missing from older files keep them.
	saved := s.cur
	if json.Unmarshal(data, &saved) == nil {
		if saved.InstallDir == "" {
			saved.InstallDir = s.cur.InstallDir
		}
		s.cur = saved
	}
	return s
}

func (s *settingsStore) get() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cur
}

func (s *settingsStore) set(next Settings) error {
	if !filepath.IsAbs(next.InstallDir) {
		return fmt.Errorf("install folder must be an absolute path")
	}
	next.InstallDir = filepath.Clean(next.InstallDir)

	s.mu.Lock()
	defer s.mu.Unlock()
	s.cur = next
	return writeJSON(s.path, next)
}

// writeJSON saves v atomically. An empty path means "keep in memory only".
func writeJSON(path string, v any) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Settings returns the current preferences.
func (m *Manager) Settings() Settings { return m.settings.get() }

// SetSettings saves preferences after checking the Proton choice is real.
func (m *Manager) SetSettings(s Settings) error {
	if s.ProtonPath != "" {
		if _, ok := resolveProtonExact(s.ProtonPath); !ok {
			return fmt.Errorf("that Proton build was not found")
		}
	}
	return m.settings.set(s)
}
