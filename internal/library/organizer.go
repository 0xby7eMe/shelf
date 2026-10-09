package library

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

const (
	maxCollectionName = 40
	maxTagLength      = 24
	maxTagsPerGame    = 20
	maxCollections    = 200
)

// Collection is a named group of games the user made. Games are full ids such as "steam:620".
type Collection struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Games []string `json:"games"`
}

// Organization is everything the user filed their games under.
type Organization struct {
	Collections []Collection        `json:"collections"`
	Tags        map[string][]string `json:"tags"` // game id -> tags
}

// Organizer keeps collections and tags, saved to organizer.json next to the favorites.
type Organizer struct {
	mu   sync.Mutex
	path string
	data Organization
}

func NewOrganizer() *Organizer {
	o := &Organizer{data: emptyOrganization()}

	base, err := os.UserConfigDir()
	if err != nil {
		log.Printf("organizer: no config dir, keeping in memory: %v", err)
		return o
	}
	dir := filepath.Join(base, "shelf")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("organizer: %v", err)
		return o
	}
	o.path = filepath.Join(dir, "organizer.json")
	o.load()
	return o
}

func emptyOrganization() Organization {
	return Organization{Collections: []Collection{}, Tags: map[string][]string{}}
}

func (o *Organizer) load() {
	raw, err := os.ReadFile(o.path)
	if err != nil {
		return
	}
	var saved Organization
	if err := json.Unmarshal(raw, &saved); err != nil {
		log.Printf("organizer: ignoring unreadable file: %v", err)
		return
	}

	seen := map[string]bool{}
	for _, c := range saved.Collections {
		name, err := cleanCollectionName(c.Name)
		if err != nil || c.ID == "" || seen[c.ID] {
			continue
		}
		seen[c.ID] = true
		games := []string{}
		for _, g := range c.Games {
			if ValidGameID(g) {
				games = append(games, g)
			}
		}
		o.data.Collections = append(o.data.Collections, Collection{ID: c.ID, Name: name, Games: dedupe(games)})
	}
	for id, tags := range saved.Tags {
		if !ValidGameID(id) {
			continue
		}
		if clean := cleanTags(tags); len(clean) > 0 {
			o.data.Tags[id] = clean
		}
	}
}

// ValidGameID accepts the ids Shelf hands out: a store, a colon and the store's own id.
func ValidGameID(id string) bool {
	store, ext, ok := strings.Cut(id, ":")
	if !ok {
		return false
	}
	switch Source(store) {
	case SourceSteam, SourceEpic, SourceGog, SourceUbisoft:
	default:
		return false
	}
	return entryIDRe.MatchString(ext)
}

func cleanCollectionName(name string) (string, error) {
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		return "", fmt.Errorf("a collection needs a name")
	}
	if utf8.RuneCountInString(name) > maxCollectionName {
		return "", fmt.Errorf("collection names can be at most %d characters", maxCollectionName)
	}
	return name, nil
}

// normalizeTag collapses whitespace and lowercases a tag.
func normalizeTag(t string) string {
	return strings.ToLower(strings.Join(strings.Fields(t), " "))
}

// cleanTags trims and lowercases tags, drops empty, overlong and repeated ones, and sorts the rest.
func cleanTags(tags []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, t := range tags {
		t = normalizeTag(t)
		if t == "" || utf8.RuneCountInString(t) > maxTagLength || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	sort.Strings(out)
	if len(out) > maxTagsPerGame {
		out = out[:maxTagsPerGame]
	}
	return out
}

func dedupe(ids []string) []string {
	seen := map[string]bool{}
	out := ids[:0]
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func newCollectionID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

// Get returns a copy of the current state.
func (o *Organizer) Get() Organization {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.snapshotLocked()
}

func (o *Organizer) snapshotLocked() Organization {
	out := Organization{
		Collections: make([]Collection, len(o.data.Collections)),
		Tags:        make(map[string][]string, len(o.data.Tags)),
	}
	for i, c := range o.data.Collections {
		out.Collections[i] = Collection{ID: c.ID, Name: c.Name, Games: append([]string{}, c.Games...)}
	}
	for id, tags := range o.data.Tags {
		out.Tags[id] = append([]string{}, tags...)
	}
	return out
}

// change applies fn to the state and saves it. If saving fails the state is rolled back.
func (o *Organizer) change(fn func(d *Organization) error) (Organization, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	before := o.snapshotLocked()
	if err := fn(&o.data); err != nil {
		o.data = before
		return Organization{}, err
	}
	if err := o.saveLocked(); err != nil {
		o.data = before
		return Organization{}, err
	}
	return o.snapshotLocked(), nil
}

func (o *Organizer) CreateCollection(name string) (Organization, error) {
	name, err := cleanCollectionName(name)
	if err != nil {
		return Organization{}, err
	}
	return o.change(func(d *Organization) error {
		if len(d.Collections) >= maxCollections {
			return fmt.Errorf("too many collections")
		}
		if indexOfName(d.Collections, name, "") >= 0 {
			return fmt.Errorf("there is already a collection called %q", name)
		}
		d.Collections = append(d.Collections, Collection{ID: newCollectionID(), Name: name, Games: []string{}})
		return nil
	})
}

func (o *Organizer) RenameCollection(id, name string) (Organization, error) {
	name, err := cleanCollectionName(name)
	if err != nil {
		return Organization{}, err
	}
	return o.change(func(d *Organization) error {
		i := indexOfID(d.Collections, id)
		if i < 0 {
			return fmt.Errorf("unknown collection")
		}
		if indexOfName(d.Collections, name, id) >= 0 {
			return fmt.Errorf("there is already a collection called %q", name)
		}
		d.Collections[i].Name = name
		return nil
	})
}

func (o *Organizer) DeleteCollection(id string) (Organization, error) {
	return o.change(func(d *Organization) error {
		i := indexOfID(d.Collections, id)
		if i < 0 {
			return fmt.Errorf("unknown collection")
		}
		d.Collections = append(d.Collections[:i], d.Collections[i+1:]...)
		return nil
	})
}

// SetGameCollections files one game under exactly the given collections.
func (o *Organizer) SetGameCollections(gameID string, collectionIDs []string) (Organization, error) {
	if !ValidGameID(gameID) {
		return Organization{}, fmt.Errorf("invalid game id")
	}
	want := map[string]bool{}
	for _, id := range collectionIDs {
		want[id] = true
	}
	return o.change(func(d *Organization) error {
		for id := range want {
			if indexOfID(d.Collections, id) < 0 {
				return fmt.Errorf("unknown collection")
			}
		}
		for i := range d.Collections {
			c := &d.Collections[i]
			has := false
			games := c.Games[:0]
			for _, g := range c.Games {
				if g == gameID {
					has = true
					if !want[c.ID] {
						continue
					}
				}
				games = append(games, g)
			}
			c.Games = games
			if want[c.ID] && !has {
				c.Games = append(c.Games, gameID)
			}
		}
		return nil
	})
}

// SetTags replaces a game's tags.
func (o *Organizer) SetTags(gameID string, tags []string) (Organization, error) {
	if !ValidGameID(gameID) {
		return Organization{}, fmt.Errorf("invalid game id")
	}
	clean := cleanTags(tags)
	return o.change(func(d *Organization) error {
		if len(clean) == 0 {
			delete(d.Tags, gameID)
		} else {
			d.Tags[gameID] = clean
		}
		return nil
	})
}

// DeleteTag removes a tag from every game that has it.
func (o *Organizer) DeleteTag(tag string) (Organization, error) {
	tag = normalizeTag(tag)
	return o.change(func(d *Organization) error {
		for id, tags := range d.Tags {
			kept := tags[:0]
			for _, t := range tags {
				if t != tag {
					kept = append(kept, t)
				}
			}
			if len(kept) == 0 {
				delete(d.Tags, id)
			} else {
				d.Tags[id] = kept
			}
		}
		return nil
	})
}

func indexOfID(cs []Collection, id string) int {
	for i, c := range cs {
		if c.ID == id {
			return i
		}
	}
	return -1
}

// indexOfName finds a collection by name, ignoring case and the one with id except.
func indexOfName(cs []Collection, name, except string) int {
	for i, c := range cs {
		if c.ID != except && strings.EqualFold(c.Name, name) {
			return i
		}
	}
	return -1
}

func (o *Organizer) saveLocked() error {
	if o.path == "" {
		return nil
	}
	data, err := json.MarshalIndent(o.data, "", "  ")
	if err != nil {
		return err
	}
	tmp := o.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, o.path)
}
