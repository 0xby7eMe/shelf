package achievements

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	freshFor    = 12 * time.Hour
	scanWorkers = 4
	emitEvery   = 500 * time.Millisecond
)

// GameRef is a game of the library, as far as the hub needs to know it.
type GameRef struct {
	Source string
	GameID string // the store's own id
	Name   string
}

func (g GameRef) key() string { return g.Source + ":" + g.GameID }

type GameProgress struct {
	Source   string `json:"source"`
	GameID   string `json:"gameId"`
	Name     string `json:"name"`
	Unlocked int    `json:"unlocked"`
	Total    int    `json:"total"`
}

type ProviderInfo struct {
	Source  string `json:"source"`
	Name    string `json:"name"`
	Ready   bool   `json:"ready"`
	Message string `json:"message,omitempty"`
	Games   int    `json:"games"` // games with achievements
}

type Overview struct {
	Providers []ProviderInfo `json:"providers"`
	Games     []GameProgress `json:"games"` // only games that have achievements
	Unlocked  int            `json:"unlocked"`
	Total     int            `json:"total"`
	Perfect   int            `json:"perfect"` // games at 100%
	Scanning  bool           `json:"scanning"`
	Done      int            `json:"done"`
	Pending   int            `json:"pending"` // how many the running scan will have looked at
}

type entry struct {
	Unlocked int   `json:"unlocked"`
	Total    int   `json:"total"` // 0: the game has none
	At       int64 `json:"at"`
}

// Hub scans the library for achievements and remembers what it found.
type Hub struct {
	providers []Provider
	path      string
	emit      func(event string, data any)
	now       func() time.Time

	mu       sync.Mutex
	cache    map[string]entry
	refs     map[string]GameRef
	messages map[string]string // per source: why the last scan stopped
	scanning bool
	done     int
	pending  int
}

// NewHub keeps its results at cachePath (empty keeps them in memory) and calls
// emit with "achievements:changed" as results come in.
func NewHub(cachePath string, emit func(string, any), providers ...Provider) *Hub {
	h := &Hub{
		providers: providers,
		path:      cachePath,
		emit:      emit,
		now:       time.Now,
		cache:     map[string]entry{},
		refs:      map[string]GameRef{},
		messages:  map[string]string{},
	}
	if emit == nil {
		h.emit = func(string, any) {}
	}
	if cachePath != "" {
		if raw, err := os.ReadFile(cachePath); err == nil {
			if json.Unmarshal(raw, &h.cache) != nil || h.cache == nil {
				h.cache = map[string]entry{}
			}
		}
	}
	return h
}

// DefaultCachePath is where the results go on a normal install.
func DefaultCachePath() string {
	base, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(base, "shelf")
	if os.MkdirAll(dir, 0o755) != nil {
		return ""
	}
	return filepath.Join(dir, "achievements.json")
}

func (h *Hub) provider(source string) Provider {
	for _, p := range h.providers {
		if p.Source() == source {
			return p
		}
	}
	return nil
}

// Overview sums up what is known, restricted to the games of the last scan.
func (h *Hub) Overview() Overview {
	h.mu.Lock()
	defer h.mu.Unlock()

	o := Overview{Providers: []ProviderInfo{}, Games: []GameProgress{}, Scanning: h.scanning, Done: h.done, Pending: h.pending}
	perSource := map[string]int{}
	for key, ref := range h.refs {
		e, ok := h.cache[key]
		if !ok || e.Total == 0 {
			continue
		}
		o.Games = append(o.Games, GameProgress{Source: ref.Source, GameID: ref.GameID, Name: ref.Name, Unlocked: e.Unlocked, Total: e.Total})
		o.Unlocked += e.Unlocked
		o.Total += e.Total
		perSource[ref.Source]++
		if e.Unlocked == e.Total {
			o.Perfect++
		}
	}
	sort.Slice(o.Games, func(i, j int) bool {
		a, b := o.Games[i], o.Games[j]
		pa, pb := float64(a.Unlocked)/float64(a.Total), float64(b.Unlocked)/float64(b.Total)
		if pa != pb {
			return pa > pb
		}
		if a.Total != b.Total {
			return a.Total > b.Total
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})

	for _, p := range h.providers {
		info := ProviderInfo{Source: p.Source(), Name: p.Name(), Games: perSource[p.Source()]}
		info.Ready, info.Message = p.Ready()
		if info.Ready && h.messages[p.Source()] != "" {
			info.Message = h.messages[p.Source()]
		}
		o.Providers = append(o.Providers, info)
	}
	return o
}

// Scan looks the given games up in the background. Games seen recently are
// skipped unless force is set. Asking while a scan runs only updates the list.
func (h *Hub) Scan(ctx context.Context, games []GameRef, force bool) {
	h.mu.Lock()
	h.refs = make(map[string]GameRef, len(games))
	for _, g := range games {
		h.refs[g.key()] = g
	}
	if h.scanning {
		h.mu.Unlock()
		return
	}

	var todo []GameRef
	for _, g := range games {
		p := h.provider(g.Source)
		if p == nil {
			continue
		}
		if ok, _ := p.Ready(); !ok {
			continue
		}
		e, seen := h.cache[g.key()]
		if force || !seen || h.now().Sub(time.Unix(e.At, 0)) > freshFor {
			todo = append(todo, g)
		}
	}
	for _, p := range h.providers {
		delete(h.messages, p.Source())
	}
	if len(todo) == 0 {
		h.mu.Unlock()
		return
	}
	h.scanning, h.done, h.pending = true, 0, len(todo)
	h.mu.Unlock()

	go h.run(ctx, todo)
}

func (h *Hub) run(ctx context.Context, todo []GameRef) {
	jobs := make(chan GameRef)
	stopped := map[string]bool{} // launchers that said the data is private
	var stopMu sync.Mutex

	var wg sync.WaitGroup
	var lastEmit time.Time
	var emitMu sync.Mutex
	tick := func(force bool) {
		emitMu.Lock()
		defer emitMu.Unlock()
		if force || time.Since(lastEmit) >= emitEvery {
			lastEmit = time.Now()
			h.emit("achievements:changed", nil)
		}
	}

	for i := 0; i < scanWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for g := range jobs {
				stopMu.Lock()
				skip := stopped[g.Source]
				stopMu.Unlock()

				if !skip {
					p := h.provider(g.Source)
					pr, err := p.Progress(ctx, g.GameID)
					switch {
					case err == nil:
						h.store(g, entry{Unlocked: pr.Unlocked, Total: pr.Total})
					case errors.Is(err, ErrNone):
						h.store(g, entry{})
					case errors.Is(err, ErrPrivate):
						stopMu.Lock()
						stopped[g.Source] = true
						stopMu.Unlock()
						h.setMessage(g.Source, "Your "+p.Name()+" privacy settings hide your achievements. Make them public, then scan again.")
					default:
						log.Printf("achievements: %s %s: %v", g.Source, g.GameID, err)
					}
				}
				h.mu.Lock()
				h.done++
				h.mu.Unlock()
				tick(false)
			}
		}()
	}
	for _, g := range todo {
		if ctx.Err() != nil {
			break
		}
		jobs <- g
	}
	close(jobs)
	wg.Wait()

	// Saved before the scan counts as over, so whoever waits for it finds
	// the results on disk too.
	h.save()
	h.mu.Lock()
	h.scanning = false
	h.mu.Unlock()
	tick(true)
}

func (h *Hub) store(g GameRef, e entry) {
	e.At = h.now().Unix()
	h.mu.Lock()
	h.cache[g.key()] = e
	h.mu.Unlock()
}

func (h *Hub) setMessage(source, msg string) {
	h.mu.Lock()
	h.messages[source] = msg
	h.mu.Unlock()
}

// Detail lists one game's achievements and refreshes what the overview knows of it.
func (h *Hub) Detail(ctx context.Context, source, gameID string) (Detail, error) {
	p := h.provider(source)
	if p == nil {
		return Detail{}, errors.New("unknown launcher")
	}
	if ok, why := p.Ready(); !ok {
		return Detail{}, errors.New(why)
	}
	d, err := p.Detail(ctx, gameID)
	ref := GameRef{Source: source, GameID: gameID}
	switch {
	case err == nil:
		h.store(ref, entry{Unlocked: d.Unlocked, Total: d.Total})
	case errors.Is(err, ErrNone):
		h.store(ref, entry{})
		d, err = Detail{Source: source, GameID: gameID, Achievements: []Achievement{}}, nil
	case errors.Is(err, ErrPrivate):
		return Detail{}, errors.New("your " + p.Name() + " privacy settings hide your achievements")
	default:
		return Detail{}, err
	}
	h.save()
	h.emit("achievements:changed", nil)
	return d, nil
}

func (h *Hub) save() {
	if h.path == "" {
		return
	}
	h.mu.Lock()
	raw, err := json.Marshal(h.cache)
	h.mu.Unlock()
	if err != nil {
		return
	}
	tmp := h.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		log.Printf("achievements: %v", err)
		return
	}
	if err := os.Rename(tmp, h.path); err != nil {
		log.Printf("achievements: %v", err)
	}
}
