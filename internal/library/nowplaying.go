package library

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

type Session struct {
	AppID string `json:"appId"`
	Since int64  `json:"since"`
}

func appIDFromArgs(args []string) (string, bool) {
	launch, id := false, ""
	for _, a := range args {
		if a == "SteamLaunch" {
			launch = true
		}
		if v, ok := strings.CutPrefix(a, "AppId="); ok && appIDRe.MatchString(v) {
			id = v
		}
	}
	return id, launch && id != ""
}

type Monitor struct {
	// Extra reports ids of games started by Shelf itself, which Steam's
	// process markers can't see. Set it before calling Run.
	Extra func() []string

	mu      sync.RWMutex
	current map[string]int64
}

func NewMonitor() *Monitor {
	return &Monitor{current: map[string]int64{}}
}

func (m *Monitor) Snapshot() []Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return snapshot(m.current)
}

func snapshot(cur map[string]int64) []Session {
	out := make([]Session, 0, len(cur))
	for id, since := range cur {
		out = append(out, Session{AppID: id, Since: since})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Since != out[j].Since {
			return out[i].Since < out[j].Since
		}
		return out[i].AppID < out[j].AppID
	})
	return out
}

func (m *Monitor) Run(ctx context.Context, every time.Duration, onChange func(now, stopped []Session)) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		m.poll(onChange)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (m *Monitor) poll(onChange func(now, stopped []Session)) {
	ids := runningAppIDs()
	if m.Extra != nil {
		ids = append(ids, m.Extra()...)
	}
	now := time.Now().UnixMilli()

	m.mu.Lock()
	next := make(map[string]int64, len(ids))
	changed := false
	for _, id := range ids {
		if since, ok := m.current[id]; ok {
			next[id] = since
		} else {
			next[id] = now
			changed = true
		}
	}
	var stopped []Session
	for id, since := range m.current {
		if _, ok := next[id]; !ok {
			stopped = append(stopped, Session{AppID: id, Since: since})
			changed = true
		}
	}
	m.current = next
	snap := snapshot(next)
	m.mu.Unlock()

	if changed {
		onChange(snap, stopped)
	}
}