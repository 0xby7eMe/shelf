package library

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

type Favorites struct {
	mu   sync.Mutex
	path string
	set  map[string]bool
}

func NewFavorites() *Favorites {
	f := &Favorites{set: map[string]bool{}}

	base, err := os.UserConfigDir()
	if err != nil {
		log.Printf("favorites: no config dir, keeping in memory: %v", err)
		return f
	}
	dir := filepath.Join(base, "shelf")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("favorites: %v", err)
		return f
	}
	f.path = filepath.Join(dir, "favorites.json")

	if data, err := os.ReadFile(f.path); err == nil {
		var ids []string
		if json.Unmarshal(data, &ids) == nil {
			for _, id := range ids {
				if entryIDRe.MatchString(id) {
					f.set[id] = true
				}
			}
		}
	}
	return f
}

func (f *Favorites) List() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.listLocked()
}

func (f *Favorites) listLocked() []string {
	ids := make([]string, 0, len(f.set))
	for id := range f.set {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (f *Favorites) Toggle(appID string) ([]string, error) {
	if !entryIDRe.MatchString(appID) {
		return nil, fmt.Errorf("invalid app id")
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.set[appID] {
		delete(f.set, appID)
	} else {
		f.set[appID] = true
	}

	if err := f.saveLocked(); err != nil {
		if f.set[appID] {
			delete(f.set, appID)
		} else {
			f.set[appID] = true
		}
		return nil, err
	}
	return f.listLocked(), nil
}

func (f *Favorites) saveLocked() error {
	if f.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(f.listLocked(), "", "  ")
	if err != nil {
		return err
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, f.path)
}