// Package friends shows who of your friends is online and what they play.
//
// Each launcher is a Provider. The Hub asks every provider that is ready, and
// merges what they return into one list. A new launcher is one more Provider;
// nothing else changes.
package friends

import "context"

type Status string

const (
	StatusPlaying Status = "playing"
	StatusOnline  Status = "online"
	StatusAway    Status = "away"
	StatusOffline Status = "offline"
)

// Playing is the game a friend is in. Source and ID are the same pair a game
// of Shelf's own library is known by ("steam", "620"), so it can be matched.
type Playing struct {
	Source string `json:"source"`
	ID     string `json:"id"`
	Name   string `json:"name"`
}

type Friend struct {
	ID         string   `json:"id"` // unique within the source
	Source     string   `json:"source"`
	Name       string   `json:"name"`
	Avatar     string   `json:"avatar,omitempty"`
	ProfileURL string   `json:"profileUrl,omitempty"`
	Status     Status   `json:"status"`
	Playing    *Playing `json:"playing,omitempty"`
}

// Field is one thing the user has to give a provider, such as an API key. The
// interface draws the form from these, so providers need no interface code.
type Field struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Help    string `json:"help,omitempty"`
	HelpURL string `json:"helpUrl,omitempty"`
	Secret  bool   `json:"secret,omitempty"`
}

// Provider is one launcher's friends list.
type Provider interface {
	// Source is the store's id, as in library.Source ("steam").
	Source() string
	Name() string
	// Fields lists what the user has to enter. None means nothing is needed.
	Fields() []Field
	// Configure hands over the saved values, keyed by Field.Key.
	Configure(values map[string]string)
	// Ready reports whether Friends can work. If not, why not, in words for the user.
	Ready() (ok bool, why string)
	Friends(ctx context.Context) ([]Friend, error)
}
