package library

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const minSession = time.Minute

type Record struct {
	AppID string `json:"appId"`
	Start int64  `json:"start"`
	End   int64  `json:"end"`
}

type DayStat struct {
	Date    string `json:"date"`
	Minutes int    `json:"minutes"`
}

type GameStat struct {
	AppID          string `json:"appId"`
	WeekMinutes    int    `json:"weekMinutes"`
	TrackedMinutes int    `json:"trackedMinutes"`
	Sessions       int    `json:"sessions"`
}

type Stats struct {
	Days            []DayStat  `json:"days"`
	WeekMinutes     int        `json:"weekMinutes"`
	PrevWeekMinutes int        `json:"prevWeekMinutes"`
	TrackedMinutes  int        `json:"trackedMinutes"`
	Sessions        int        `json:"sessions"`
	Since           int64      `json:"since"`
	Games           []GameStat `json:"games"`
}

type History struct {
	mu      sync.Mutex
	path    string
	records []Record
}

func NewHistory() *History {
	h := &History{}

	base, err := os.UserConfigDir()
	if err != nil {
		log.Printf("history: no config dir, keeping in memory: %v", err)
		return h
	}
	dir := filepath.Join(base, "shelf")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("history: %v", err)
		return h
	}
	h.path = filepath.Join(dir, "sessions.json")

	data, err := os.ReadFile(h.path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("history: %v", err)
			h.path = ""
		}
		return h
	}

	var recs []Record
	if err := json.Unmarshal(data, &recs); err != nil {
		bad := h.path + ".corrupt"
		log.Printf("history: unreadable, moved to %s: %v", bad, err)
		_ = os.Rename(h.path, bad)
		return h
	}
	for _, r := range recs {
		if appIDRe.MatchString(r.AppID) && r.End > r.Start {
			h.records = append(h.records, r)
		}
	}
	return h
}

func (h *History) Add(appID string, startMs, endMs int64) (added bool, err error) {
	if !appIDRe.MatchString(appID) {
		return false, fmt.Errorf("invalid app id")
	}
	if endMs-startMs < minSession.Milliseconds() {
		return false, nil
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, Record{AppID: appID, Start: startMs, End: endMs})
	return true, h.saveLocked()
}

func (h *History) saveLocked() error {
	if h.path == "" {
		return nil
	}
	data, err := json.Marshal(h.records)
	if err != nil {
		return err
	}
	tmp := h.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, h.path)
}

func (h *History) Stats(weeks int) Stats {
	h.mu.Lock()
	recs := append([]Record(nil), h.records...)
	h.mu.Unlock()
	return computeStats(recs, time.Now(), weeks)
}

func minutesOf(d time.Duration) int { return int((d + 30*time.Second) / time.Minute) }

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func startOfWeek(t time.Time) time.Time {
	y, m, d := t.Date()
	off := (int(t.Weekday()) + 6) % 7
	return time.Date(y, m, d-off, 0, 0, 0, 0, t.Location())
}

type gameAcc struct {
	week, total time.Duration
	sessions    int
}

func computeStats(recs []Record, now time.Time, weeks int) Stats {
	if weeks < 1 {
		weeks = 1
	}
	if weeks > 53 {
		weeks = 53
	}
	loc := now.Location()
	thisWeek := startOfWeek(now)
	prevWeek := thisWeek.AddDate(0, 0, -7)
	gridStart := thisWeek.AddDate(0, 0, -7*(weeks-1))

	perDay := map[string]time.Duration{}
	acc := map[string]*gameAcc{}
	var week, prev, total time.Duration
	var first int64
	sessions := 0

	for _, r := range recs {
		s := time.UnixMilli(r.Start).In(loc)
		e := time.UnixMilli(r.End).In(loc)
		if !e.After(s) {
			continue
		}
		sessions++
		if first == 0 || r.Start < first {
			first = r.Start
		}
		g := acc[r.AppID]
		if g == nil {
			g = &gameAcc{}
			acc[r.AppID] = g
		}
		g.sessions++
		g.total += e.Sub(s)
		total += e.Sub(s)

		for cur := s; cur.Before(e); {
			seg := startOfDay(cur).AddDate(0, 0, 1)
			if seg.After(e) {
				seg = e
			}
			d := seg.Sub(cur)
			perDay[cur.Format("2006-01-02")] += d
			switch {
			case !cur.Before(thisWeek):
				week += d
				g.week += d
			case !cur.Before(prevWeek):
				prev += d
			}
			cur = seg
		}
	}

	days := []DayStat{}
	for d := gridStart; !d.After(now); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		days = append(days, DayStat{Date: key, Minutes: minutesOf(perDay[key])})
	}

	games := make([]GameStat, 0, len(acc))
	for id, g := range acc {
		games = append(games, GameStat{
			AppID:          id,
			WeekMinutes:    minutesOf(g.week),
			TrackedMinutes: minutesOf(g.total),
			Sessions:       g.sessions,
		})
	}
	sort.Slice(games, func(i, j int) bool {
		if games[i].TrackedMinutes != games[j].TrackedMinutes {
			return games[i].TrackedMinutes > games[j].TrackedMinutes
		}
		return games[i].AppID < games[j].AppID
	})

	return Stats{
		Days:            days,
		WeekMinutes:     minutesOf(week),
		PrevWeekMinutes: minutesOf(prev),
		TrackedMinutes:  minutesOf(total),
		Sessions:        sessions,
		Since:           first,
		Games:           games,
	}
}