package friends

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"shelf/internal/library"
)

const steamAPI = "https://api.steampowered.com"

// Steam reads friends and their presence from Steam's Web API. That needs a
// key, which Steam hands out for free, and the id of the account Steam is
// signed in with, which Shelf finds in Steam's own files.
type Steam struct {
	apiBase string
	client  *http.Client
	userID  func() (string, error)

	mu  sync.Mutex
	key string
}

func NewSteam() *Steam {
	return &Steam{
		apiBase: steamAPI,
		client:  &http.Client{Timeout: 12 * time.Second},
		userID:  library.SteamUserID,
	}
}

func (s *Steam) Source() string { return "steam" }
func (s *Steam) Name() string   { return "Steam" }

func (s *Steam) Fields() []Field {
	return []Field{{
		Key:     "apiKey",
		Label:   "Web API key",
		Help:    "A free key from Steam. Any domain name works when it asks, for example localhost. Your friends list must be visible to you in Steam's privacy settings (it is by default).",
		HelpURL: "https://steamcommunity.com/dev/apikey",
		Secret:  true,
	}}
}

func (s *Steam) Configure(values map[string]string) {
	s.mu.Lock()
	s.key = values["apiKey"]
	s.mu.Unlock()
}

func (s *Steam) apiKey() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.key
}

func (s *Steam) Ready() (bool, string) {
	if s.apiKey() == "" {
		return false, "Add a Steam Web API key to see your Steam friends."
	}
	if _, err := s.userID(); err != nil {
		return false, "Sign in to Steam on this computer first."
	}
	return true, ""
}

type steamPlayer struct {
	SteamID      string `json:"steamid"`
	Name         string `json:"personaname"`
	ProfileURL   string `json:"profileurl"`
	Avatar       string `json:"avatarmedium"`
	PersonaState int    `json:"personastate"`
	GameID       string `json:"gameid"`
	GameName     string `json:"gameextrainfo"`
}

func (s *Steam) Friends(ctx context.Context) ([]Friend, error) {
	me, err := s.userID()
	if err != nil {
		return nil, errors.New("Sign in to Steam on this computer first.")
	}

	var list struct {
		FriendsList struct {
			Friends []struct {
				SteamID string `json:"steamid"`
			} `json:"friends"`
		} `json:"friendslist"`
	}
	if err := s.get(ctx, "/ISteamUser/GetFriendList/v1/", url.Values{"steamid": {me}, "relationship": {"friend"}}, &list); err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(list.FriendsList.Friends))
	for _, f := range list.FriendsList.Friends {
		ids = append(ids, f.SteamID)
	}

	friends := make([]Friend, 0, len(ids))
	for len(ids) > 0 { // the API takes up to 100 ids at a time
		n := min(100, len(ids))
		var sum struct {
			Response struct {
				Players []steamPlayer `json:"players"`
			} `json:"response"`
		}
		if err := s.get(ctx, "/ISteamUser/GetPlayerSummaries/v2/", url.Values{"steamids": {strings.Join(ids[:n], ",")}}, &sum); err != nil {
			return nil, err
		}
		for _, p := range sum.Response.Players {
			friends = append(friends, p.friend())
		}
		ids = ids[n:]
	}
	return friends, nil
}

func (p steamPlayer) friend() Friend {
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

func (s *Steam) get(ctx context.Context, path string, q url.Values, out any) error {
	q.Set("key", s.apiKey())
	q.Set("format", "json")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.apiBase+path+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		// The error text holds the request URL, and that holds the key.
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return fmt.Errorf("couldn't reach Steam: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return errors.New("Steam says your friends list is private. Make it visible in Steam's privacy settings.")
	case http.StatusForbidden:
		return errors.New("Steam rejected the Web API key. Check that you copied all of it.")
	case http.StatusTooManyRequests:
		return errors.New("Steam asked us to slow down. Try again in a minute.")
	default:
		return fmt.Errorf("Steam answered %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("Steam sent something unexpected: %w", err)
	}
	return nil
}
