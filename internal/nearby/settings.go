package nearby

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"
)

const maxNameLen = 40

// Settings are the switches in the Nearby dialog.
type Settings struct {
	Enabled bool   `json:"enabled"` // announce this Shelf and look for others
	Name    string `json:"name"`    // how others see you; empty means the account's name
}

type saved struct {
	Settings
	ID string `json:"id"`
}

// Store keeps Settings and this Shelf's id in nearby.json in the config folder.
type Store struct {
	mu   sync.Mutex
	path string
	s    saved
}

// DefaultPath is nearby.json in Shelf's config folder, or "" if there is none.
func DefaultPath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		log.Printf("nearby: no config dir, keeping settings in memory: %v", err)
		return ""
	}
	return filepath.Join(base, "shelf", "nearby.json")
}

// NewStore reads path, or starts with the defaults: switched on, with a new id.
// An empty path keeps everything in memory.
func NewStore(path string) *Store {
	st := &Store{path: path, s: saved{Settings: Settings{Enabled: true}}}
	if path != "" {
		if raw, err := os.ReadFile(path); err == nil {
			var s saved
			if json.Unmarshal(raw, &s) == nil {
				st.s = s
			}
		}
	}
	if st.s.ID == "" {
		st.s.ID = newID()
		if err := st.save(st.s); err != nil {
			log.Printf("nearby: %v", err)
		}
	}
	return st
}

func (st *Store) Get() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.s.Settings
}

// ID tells this Shelf apart from the others, also after a restart.
func (st *Store) ID() string {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.s.ID
}

func (st *Store) Set(s Settings) error {
	s.Name = cleanName(s.Name)
	st.mu.Lock()
	defer st.mu.Unlock()
	next := saved{Settings: s, ID: st.s.ID}
	if err := st.save(next); err != nil {
		return err
	}
	st.s = next
	return nil
}

func (st *Store) save(s saved) error {
	if st.path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(st.path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := st.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, st.path)
}

// DisplayName is the name to show others: the one set, or else the account's
// full name, or its login.
func (st *Store) DisplayName() string {
	if n := st.Get().Name; n != "" {
		return n
	}
	return accountName()
}

func accountName() string {
	if u, err := user.Current(); err == nil {
		// The full name field can carry more after a comma (room, phone).
		full, _, _ := strings.Cut(u.Name, ",")
		if n := cleanName(full); n != "" {
			return n
		}
		if n := cleanName(u.Username); n != "" {
			return n
		}
	}
	if h, err := os.Hostname(); err == nil {
		return cleanName(h)
	}
	return "Shelf"
}

// cleanName trims a name and cuts it to a length that fits the interface.
func cleanName(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	for utf8.RuneCountInString(s) > maxNameLen {
		_, size := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-size]
	}
	return strings.TrimSpace(s)
}

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
