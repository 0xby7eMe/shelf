package friends

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"
)

// Store keeps what the user entered for each provider (API keys and so on).
type Store struct {
	mu   sync.Mutex
	path string
	data map[string]map[string]string // source -> field key -> value
}

func NewStore() *Store {
	s := &Store{data: map[string]map[string]string{}}
	base, err := os.UserConfigDir()
	if err != nil {
		log.Printf("friends: no config dir, keeping in memory: %v", err)
		return s
	}
	dir := filepath.Join(base, "shelf")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("friends: %v", err)
		return s
	}
	s.path = filepath.Join(dir, "friends.json")
	if raw, err := os.ReadFile(s.path); err == nil {
		if err := json.Unmarshal(raw, &s.data); err != nil || s.data == nil {
			log.Printf("friends: ignoring unreadable %s", s.path)
			s.data = map[string]map[string]string{}
		}
	}
	return s
}

// Get returns a copy of a provider's saved values.
func (s *Store) Get(source string) map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]string, len(s.data[source]))
	for k, v := range s.data[source] {
		out[k] = v
	}
	return out
}

// Set replaces a provider's values. No values forgets the provider.
func (s *Store) Set(source string, values map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	prev, had := s.data[source]
	if len(values) == 0 {
		delete(s.data, source)
	} else {
		s.data[source] = values
	}
	if err := s.saveLocked(); err != nil {
		if had {
			s.data[source] = prev
		} else {
			delete(s.data, source)
		}
		return err
	}
	return nil
}

func (s *Store) saveLocked() error {
	if s.path == "" {
		return nil
	}
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	// Holds API keys, so only the user can read it.
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}
