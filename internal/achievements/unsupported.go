package achievements

import (
	"context"
	"errors"
)

// Unsupported stands in for a launcher that offers no way to read
// achievements, so the interface says so instead of leaving it out.
type Unsupported struct {
	source, name, reason string
}

func NewUnsupported(source, name, reason string) *Unsupported {
	return &Unsupported{source, name, reason}
}

func (u *Unsupported) Source() string        { return u.source }
func (u *Unsupported) Name() string          { return u.name }
func (u *Unsupported) Ready() (bool, string) { return false, u.reason }

func (u *Unsupported) Progress(context.Context, string) (Progress, error) {
	return Progress{}, errors.New(u.reason)
}

func (u *Unsupported) Detail(context.Context, string) (Detail, error) {
	return Detail{}, errors.New(u.reason)
}
