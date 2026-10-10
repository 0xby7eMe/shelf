package achievements

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"time"
)

// GogSession is what the GOG provider needs from the GOG login, which the
// Epic manager keeps (Settings, Integrations, GOG).
type GogSession interface {
	// GogUserID is the account's user id, or "" when signed out.
	GogUserID() string
	// GogGameGet fetches a JSON document from a game's own Galaxy services.
	// address gets the game's Galaxy client id. A game that doesn't use Galaxy
	// fails with an error that has a NoGalaxy method returning true.
	GogGameGet(ctx context.Context, key string, address func(clientID string) string, out any) error
}

// Where GOG Galaxy keeps achievements. A variable so tests can use a local server.
var gogGameplayBase = "https://gameplay.gog.com"

// GOG reads the achievements GOG Galaxy records, with the account Shelf signed in to GOG with.
type GOG struct {
	s GogSession
}

func NewGOG(s GogSession) *GOG { return &GOG{s: s} }

func (g *GOG) Source() string { return "gog" }
func (g *GOG) Name() string   { return "GOG" }

func (g *GOG) Ready() (bool, string) {
	if g.s.GogUserID() == "" {
		return false, "Sign in to GOG under Settings, Integrations, GOG."
	}
	return true, ""
}

type gogAchievements struct {
	Items []struct {
		Key          string   `json:"achievement_key"`
		ID           string   `json:"achievement_id"`
		Name         string   `json:"name"`
		Description  string   `json:"description"`
		Visible      *bool    `json:"visible"`
		ImageLocked  string   `json:"image_url_locked"`
		ImageOpen    string   `json:"image_url_unlocked"`
		DateUnlocked *string  `json:"date_unlocked"`
		Rarity       *float64 `json:"rarity"`
	} `json:"items"`
}

func (g *GOG) fetch(ctx context.Context, gameID string) (gogAchievements, error) {
	me := g.s.GogUserID()
	if me == "" {
		return gogAchievements{}, fmt.Errorf("not signed in to GOG")
	}
	var out gogAchievements
	err := g.s.GogGameGet(ctx, gameID, func(clientID string) string {
		return gogGameplayBase + "/clients/" + url.PathEscape(clientID) + "/users/" + url.PathEscape(me) + "/achievements"
	}, &out)
	var ng interface{ NoGalaxy() bool }
	if errors.As(err, &ng) && ng.NoGalaxy() {
		return gogAchievements{}, ErrNone
	}
	if err != nil {
		return gogAchievements{}, err
	}
	if len(out.Items) == 0 {
		return gogAchievements{}, ErrNone
	}
	return out, nil
}

func (g *GOG) Progress(ctx context.Context, gameID string) (Progress, error) {
	list, err := g.fetch(ctx, gameID)
	if err != nil {
		return Progress{}, err
	}
	p := Progress{Total: len(list.Items)}
	for _, a := range list.Items {
		if gogUnlockTime(a.DateUnlocked) != 0 {
			p.Unlocked++
		}
	}
	return p, nil
}

func (g *GOG) Detail(ctx context.Context, gameID string) (Detail, error) {
	list, err := g.fetch(ctx, gameID)
	if err != nil {
		return Detail{}, err
	}
	d := Detail{Source: "gog", GameID: gameID, Total: len(list.Items), Achievements: make([]Achievement, 0, len(list.Items))}
	for _, a := range list.Items {
		at := gogUnlockTime(a.DateUnlocked)
		out := Achievement{
			ID:          firstNonEmpty(a.Key, a.ID),
			Name:        firstNonEmpty(a.Name, a.Key),
			Description: a.Description,
			Unlocked:    at != 0,
			UnlockedAt:  max(at, 0),
			Hidden:      a.Visible != nil && !*a.Visible,
			Percent:     -1,
		}
		if out.Unlocked {
			d.Unlocked++
			out.Icon = firstNonEmpty(a.ImageOpen, a.ImageLocked)
		} else {
			out.Icon = firstNonEmpty(a.ImageLocked, a.ImageOpen)
		}
		if a.Rarity != nil && *a.Rarity >= 0 {
			out.Percent = *a.Rarity
		}
		d.Achievements = append(d.Achievements, out)
	}

	// The same order as Steam: newest unlocks first, then the rest, easiest first.
	sort.SliceStable(d.Achievements, func(i, j int) bool {
		a, b := d.Achievements[i], d.Achievements[j]
		if a.Unlocked != b.Unlocked {
			return a.Unlocked
		}
		if a.Unlocked {
			return a.UnlockedAt > b.UnlockedAt
		}
		return a.Percent > b.Percent
	})
	return d, nil
}

// gogUnlockTime reads when an achievement was unlocked, in unix seconds, or 0
// when it is still locked. A date Shelf can't read still counts as unlocked.
func gogUnlockTime(date *string) int64 {
	if date == nil || *date == "" {
		return 0
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05-0700", "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, *date); err == nil {
			return t.Unix()
		}
	}
	return -1
}
