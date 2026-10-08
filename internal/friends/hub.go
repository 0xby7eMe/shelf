package friends

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	cacheFor = 30 * time.Second
	fetchFor = 15 * time.Second
)

// FieldInfo is a Field plus whether the user has filled it in. A saved secret
// never goes back to the interface.
type FieldInfo struct {
	Field
	Set bool `json:"set"`
}

// ProviderInfo is what the interface knows about one launcher.
type ProviderInfo struct {
	Source  string      `json:"source"`
	Name    string      `json:"name"`
	Ready   bool        `json:"ready"`
	Message string      `json:"message,omitempty"` // why it isn't ready, or what went wrong
	Fields  []FieldInfo `json:"fields"`
	Count   int         `json:"count"`
}

type Snapshot struct {
	Providers []ProviderInfo `json:"providers"`
	Friends   []Friend       `json:"friends"`
	At        int64          `json:"at"` // unix seconds
}

type result struct {
	at      time.Time
	friends []Friend
	err     error
}

// Hub merges the providers' friends.
type Hub struct {
	store     *Store
	providers []Provider

	mu    sync.Mutex
	cache map[string]result
	now   func() time.Time
}

func NewHub(store *Store, providers ...Provider) *Hub {
	h := &Hub{store: store, providers: providers, cache: map[string]result{}, now: time.Now}
	for _, p := range providers {
		p.Configure(store.Get(p.Source()))
	}
	return h
}

func (h *Hub) provider(source string) Provider {
	for _, p := range h.providers {
		if p.Source() == source {
			return p
		}
	}
	return nil
}

// Snapshot is the current friends list. Results are reused for a short while;
// force asks every provider again.
func (h *Hub) Snapshot(ctx context.Context, force bool) Snapshot {
	results := make([]result, len(h.providers))
	var wg sync.WaitGroup
	for i, p := range h.providers {
		if ok, _ := p.Ready(); !ok {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = h.fetch(ctx, p, force)
		}()
	}
	wg.Wait()

	snap := Snapshot{Providers: make([]ProviderInfo, 0, len(h.providers)), Friends: []Friend{}, At: h.now().Unix()}
	for i, p := range h.providers {
		info := ProviderInfo{Source: p.Source(), Name: p.Name(), Fields: h.fields(p)}
		info.Ready, info.Message = p.Ready()
		if info.Ready {
			if err := results[i].err; err != nil {
				info.Message = err.Error()
			} else {
				info.Count = len(results[i].friends)
				snap.Friends = append(snap.Friends, results[i].friends...)
			}
		}
		snap.Providers = append(snap.Providers, info)
	}
	sortFriends(snap.Friends)
	return snap
}

func (h *Hub) fetch(ctx context.Context, p Provider, force bool) result {
	h.mu.Lock()
	cached, ok := h.cache[p.Source()]
	h.mu.Unlock()
	if ok && !force && h.now().Sub(cached.at) < cacheFor {
		return cached
	}

	ctx, cancel := context.WithTimeout(ctx, fetchFor)
	defer cancel()
	friends, err := p.Friends(ctx)
	r := result{at: h.now(), friends: friends, err: err}
	for i := range r.friends {
		r.friends[i].Source = p.Source()
	}

	h.mu.Lock()
	h.cache[p.Source()] = r
	h.mu.Unlock()
	return r
}

func (h *Hub) fields(p Provider) []FieldInfo {
	saved := h.store.Get(p.Source())
	out := make([]FieldInfo, 0, len(p.Fields()))
	for _, f := range p.Fields() {
		out = append(out, FieldInfo{Field: f, Set: saved[f.Key] != ""})
	}
	return out
}

// Configure saves what the user entered for a provider. A blank secret keeps
// the saved one, so a form can be sent back without retyping the key.
func (h *Hub) Configure(source string, values map[string]string) error {
	p := h.provider(source)
	if p == nil {
		return fmt.Errorf("unknown launcher %q", source)
	}
	saved := h.store.Get(source)
	next := map[string]string{}
	for _, f := range p.Fields() {
		v := strings.TrimSpace(values[f.Key])
		if v == "" && f.Secret {
			v = saved[f.Key]
		}
		if v != "" {
			next[f.Key] = v
		}
	}
	if err := h.store.Set(source, next); err != nil {
		return err
	}
	p.Configure(next)
	h.forget(source)
	return nil
}

// Disconnect forgets everything saved for a provider.
func (h *Hub) Disconnect(source string) error {
	p := h.provider(source)
	if p == nil {
		return fmt.Errorf("unknown launcher %q", source)
	}
	if err := h.store.Set(source, nil); err != nil {
		return err
	}
	p.Configure(nil)
	h.forget(source)
	return nil
}

func (h *Hub) forget(source string) {
	h.mu.Lock()
	delete(h.cache, source)
	h.mu.Unlock()
}

var statusOrder = map[Status]int{StatusPlaying: 0, StatusOnline: 1, StatusAway: 2, StatusOffline: 3}

func sortFriends(fs []Friend) {
	sort.SliceStable(fs, func(i, j int) bool {
		a, b := fs[i], fs[j]
		if statusOrder[a.Status] != statusOrder[b.Status] {
			return statusOrder[a.Status] < statusOrder[b.Status]
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
}
