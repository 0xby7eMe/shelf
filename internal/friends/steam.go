package friends

import (
	"context"

	"shelf/internal/steamapi"
)

// Steam reads friends and their presence through the shared Steam Web API
// client, so the key is entered once, on the Steam integration page.
type Steam struct {
	api *steamapi.Client
}

func NewSteam(api *steamapi.Client) *Steam { return &Steam{api: api} }

func (s *Steam) Source() string              { return "steam" }
func (s *Steam) Name() string                { return "Steam" }
func (s *Steam) Fields() []Field             { return nil }
func (s *Steam) Configure(map[string]string) {}

func (s *Steam) Ready() (bool, string) { return s.api.Ready() }

func (s *Steam) Friends(ctx context.Context) ([]Friend, error) {
	players, err := s.api.Friends(ctx)
	if err != nil {
		return nil, err
	}
	friends := make([]Friend, 0, len(players))
	for _, p := range players {
		friends = append(friends, steamFriend(p))
	}
	return friends, nil
}

func steamFriend(p steamapi.Player) Friend {
	f := Friend{
		ID:         p.SteamID,
		Name:       p.Name,
		Avatar:     p.Avatar,
		ProfileURL: p.ProfileURL,
		Status:     steamStatus(p.PersonaState),
	}
	if p.GameID != "" {
		f.Status = StatusPlaying
		name := p.GameName
		if name == "" {
			name = "a game"
		}
		f.Playing = &Playing{Source: "steam", ID: p.GameID, Name: name}
	}
	return f
}

// steamStatus maps Steam's persona states: 0 offline, 1 online, 2 busy,
// 3 away, 4 snooze, 5 looking to trade, 6 looking to play.
func steamStatus(state int) Status {
	switch state {
	case 0:
		return StatusOffline
	case 3, 4:
		return StatusAway
	default:
		return StatusOnline
	}
}
