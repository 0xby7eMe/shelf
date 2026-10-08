// Package achievements shows how far you are in your games.
//
// Each launcher is a Provider. The Hub scans the library through the providers
// that are ready, remembers the results for a while and sums them up. A new
// launcher is one more Provider; nothing else changes.
package achievements

import (
	"context"
	"errors"
)

// ErrNone means the game has no achievements. It is an answer, not a failure,
// and is remembered like one.
var ErrNone = errors.New("no achievements")

// Progress is how many of a game's achievements are unlocked.
type Progress struct {
	Unlocked int
	Total    int
}

type Achievement struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	Icon        string  `json:"icon,omitempty"`
	Unlocked    bool    `json:"unlocked"`
	UnlockedAt  int64   `json:"unlockedAt,omitempty"` // unix seconds
	Hidden      bool    `json:"hidden,omitempty"`
	Percent     float64 `json:"percent"` // share of players who have it; -1 when unknown
}

// Detail is one game's achievements.
type Detail struct {
	Source       string        `json:"source"`
	GameID       string        `json:"gameId"`
	Unlocked     int           `json:"unlocked"`
	Total        int           `json:"total"`
	Achievements []Achievement `json:"achievements"`
}

// Provider is one launcher's achievements. Game ids are the store's own ids,
// the same ones as library.Game.ExternalID.
type Provider interface {
	// Source is the store's id, as in library.Source ("steam").
	Source() string
	Name() string
	// Ready reports whether the provider can answer. If not, why not, in words for the user.
	Ready() (ok bool, why string)
	// Progress is cheap: one request at most. It returns ErrNone for a game without achievements.
	Progress(ctx context.Context, gameID string) (Progress, error)
	// Detail lists every achievement. It may take several requests.
	Detail(ctx context.Context, gameID string) (Detail, error)
}

// ErrPrivate is returned by a provider when the account's privacy settings
// hide the data. The hub stops scanning that launcher and says so.
var ErrPrivate = errors.New("private")
