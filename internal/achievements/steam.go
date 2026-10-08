package achievements

import (
	"context"
	"errors"
	"sort"

	"shelf/internal/steamapi"
)

// Steam reads achievements through the shared Steam Web API client.
type Steam struct {
	api *steamapi.Client
}

func NewSteam(api *steamapi.Client) *Steam { return &Steam{api: api} }

func (s *Steam) Source() string        { return "steam" }
func (s *Steam) Name() string          { return "Steam" }
func (s *Steam) Ready() (bool, string) { return s.api.Ready() }

func steamErr(err error) error {
	switch {
	case errors.Is(err, steamapi.ErrNoAchievements):
		return ErrNone
	case errors.Is(err, steamapi.ErrPrivate):
		return ErrPrivate
	}
	return err
}

func (s *Steam) Progress(ctx context.Context, gameID string) (Progress, error) {
	list, err := s.api.PlayerAchievements(ctx, gameID)
	if err != nil {
		return Progress{}, steamErr(err)
	}
	p := Progress{Total: len(list)}
	for _, a := range list {
		if a.Achieved == 1 {
			p.Unlocked++
		}
	}
	return p, nil
}

func (s *Steam) Detail(ctx context.Context, gameID string) (Detail, error) {
	list, err := s.api.PlayerAchievements(ctx, gameID)
	if err != nil {
		return Detail{}, steamErr(err)
	}

	// Icons, hidden flags and rarity are extras: without them the list is still useful.
	icons := map[string]steamapi.SchemaAchievement{}
	if sch, err := s.api.Schema(ctx, gameID); err == nil {
		for _, a := range sch {
			icons[a.Name] = a
		}
	}
	rarity, err := s.api.GlobalPercentages(ctx, gameID)
	if err != nil {
		rarity = nil
	}

	d := Detail{Source: "steam", GameID: gameID, Total: len(list), Achievements: make([]Achievement, 0, len(list))}
	for _, a := range list {
		sch := icons[a.APIName]
		out := Achievement{
			ID:          a.APIName,
			Name:        firstNonEmpty(a.Name, sch.DisplayName, a.APIName),
			Description: firstNonEmpty(a.Description, sch.Description),
			Unlocked:    a.Achieved == 1,
			UnlockedAt:  a.UnlockTime,
			Hidden:      sch.Hidden == 1,
			Percent:     -1,
		}
		if out.Unlocked {
			d.Unlocked++
			out.Icon = sch.Icon
		} else {
			out.Icon = firstNonEmpty(sch.IconGray, sch.Icon)
		}
		if p, ok := rarity[a.APIName]; ok {
			out.Percent = p
		}
		d.Achievements = append(d.Achievements, out)
	}

	// Newest unlocks first, then what is left, easiest first.
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

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
