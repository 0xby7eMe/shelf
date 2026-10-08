package friends

import (
	"context"
	"errors"
)

// Unsupported stands in for a launcher that offers no way to read friends, so
// the interface can say so instead of leaving the store out without a word.
type Unsupported struct {
	source, name, reason string
}

func NewUnsupported(source, name, reason string) *Unsupported {
	return &Unsupported{source, name, reason}
}

func (u *Unsupported) Source() string              { return u.source }
func (u *Unsupported) Name() string                { return u.name }
func (u *Unsupported) Fields() []Field             { return nil }
func (u *Unsupported) Configure(map[string]string) {}
func (u *Unsupported) Ready() (bool, string)       { return false, u.reason }

func (u *Unsupported) Friends(context.Context) ([]Friend, error) {
	return nil, errors.New(u.reason)
}
