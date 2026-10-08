// Package steamapi talks to Steam's Web API with the key the user provided.
//
// One Client is shared by everything that needs it: the library (games that
// aren't installed), the friends list and the integration page.
package steamapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultBase = "https://api.steampowered.com"

var (
	// ErrNoKey means no Web API key has been entered.
	ErrNoKey = errors.New("Add a Steam Web API key under Settings, Integrations, Steam.")
	// ErrNoAccount means Steam isn't signed in on this computer.
	ErrNoAccount = errors.New("Sign in to Steam on this computer first.")
	// ErrBadKey means Steam turned the key down.
	ErrBadKey = errors.New("Steam rejected the Web API key. Check that you copied all of it.")
	// ErrPrivate means the account's privacy settings hide what was asked for.
	ErrPrivate = errors.New("Steam says that is private. Set your friends list and game details to public in Steam's privacy settings.")
)

type Client struct {
	BaseURL string // the API's address; tests point it elsewhere
	http    *http.Client
	userID  func() (string, error)
	path    string // where the key is saved; empty keeps it in memory

	mu    sync.Mutex
	key   string
	owned ownedCache
}

// New makes a client that signs requests for the account userID names, loading
// the saved key if there is one.
func New(userID func() (string, error)) *Client {
	c := &Client{BaseURL: defaultBase, http: &http.Client{Timeout: 15 * time.Second}, userID: userID}
	if base, err := os.UserConfigDir(); err == nil {
		dir := filepath.Join(base, "shelf")
		if os.MkdirAll(dir, 0o755) == nil {
			c.path = filepath.Join(dir, "steam.json")
			c.key = loadKey(c.path)
		}
	}
	return c
}

func loadKey(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var saved struct {
		APIKey string `json:"apiKey"`
	}
	if json.Unmarshal(raw, &saved) != nil {
		return ""
	}
	return saved.APIKey
}

// Key is the saved key. It is for the Go side only and never goes to the interface.
func (c *Client) Key() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.key
}

// Configured reports whether a key has been entered.
func (c *Client) Configured() bool { return c.Key() != "" }

// SetKey saves the key. An empty key forgets it.
func (c *Client) SetKey(key string) error {
	key = strings.TrimSpace(key)
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.path != "" {
		if key == "" {
			if err := os.Remove(c.path); err != nil && !os.IsNotExist(err) {
				return err
			}
		} else {
			raw, _ := json.Marshal(map[string]string{"apiKey": key})
			tmp := c.path + ".tmp"
			// It is a secret, so only the user may read it.
			if err := os.WriteFile(tmp, raw, 0o600); err != nil {
				return err
			}
			if err := os.Rename(tmp, c.path); err != nil {
				return err
			}
		}
	}
	c.key = key
	c.owned = ownedCache{}
	return nil
}

// Ready reports whether requests can be made, and if not, why in words for the user.
func (c *Client) Ready() (bool, string) {
	if !c.Configured() {
		return false, ErrNoKey.Error()
	}
	if _, err := c.userID(); err != nil {
		return false, ErrNoAccount.Error()
	}
	return true, ""
}

func (c *Client) account() (string, error) {
	if !c.Configured() {
		return "", ErrNoKey
	}
	id, err := c.userID()
	if err != nil {
		return "", ErrNoAccount
	}
	return id, nil
}

// get calls one API method and decodes its answer.
func (c *Client) get(ctx context.Context, path string, q url.Values, out any) error {
	q.Set("key", c.Key())
	q.Set("format", "json")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path+"?"+q.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
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
		return ErrPrivate
	case http.StatusForbidden:
		return ErrBadKey
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

// Player is a profile as Steam describes it, with what the person is doing.
type Player struct {
	SteamID      string `json:"steamid"`
	Name         string `json:"personaname"`
	ProfileURL   string `json:"profileurl"`
	Avatar       string `json:"avatarmedium"`
	PersonaState int    `json:"personastate"` // 0 offline, 1 online, 2 busy, 3 away, 4 snooze, 5 trade, 6 play
	Visibility   int    `json:"communityvisibilitystate"`
	GameID       string `json:"gameid"`
	GameName     string `json:"gameextrainfo"`
}

// Players looks profiles up, up to 100 ids per request.
func (c *Client) Players(ctx context.Context, ids []string) ([]Player, error) {
	out := make([]Player, 0, len(ids))
	for len(ids) > 0 {
		n := min(100, len(ids))
		var res struct {
			Response struct {
				Players []Player `json:"players"`
			} `json:"response"`
		}
		if err := c.get(ctx, "/ISteamUser/GetPlayerSummaries/v2/", url.Values{"steamids": {strings.Join(ids[:n], ",")}}, &res); err != nil {
			return nil, err
		}
		out = append(out, res.Response.Players...)
		ids = ids[n:]
	}
	return out, nil
}

// Friends are the signed-in account's friends, with their presence.
func (c *Client) Friends(ctx context.Context) ([]Player, error) {
	me, err := c.account()
	if err != nil {
		return nil, err
	}
	var list struct {
		FriendsList struct {
			Friends []struct {
				SteamID string `json:"steamid"`
			} `json:"friends"`
		} `json:"friendslist"`
	}
	if err := c.get(ctx, "/ISteamUser/GetFriendList/v1/", url.Values{"steamid": {me}, "relationship": {"friend"}}, &list); err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(list.FriendsList.Friends))
	for _, f := range list.FriendsList.Friends {
		ids = append(ids, f.SteamID)
	}
	return c.Players(ctx, ids)
}

// OwnedGame is one game of the account's library, installed or not.
type OwnedGame struct {
	AppID           int    `json:"appid"`
	Name            string `json:"name"`
	PlaytimeMinutes int    `json:"playtime_forever"`
	LastPlayed      int64  `json:"rtime_last_played"`
}

type ownedCache struct {
	at    time.Time
	games []OwnedGame
}

const ownedFor = 10 * time.Minute

// OwnedGames is every game the account owns. The library asks on every scan,
// so the answer is kept for a while, and an old one is better than none when
// Steam can't be reached.
func (c *Client) OwnedGames(ctx context.Context) ([]OwnedGame, error) {
	c.mu.Lock()
	cached := c.owned
	c.mu.Unlock()
	if cached.games != nil && time.Since(cached.at) < ownedFor {
		return cached.games, nil
	}

	games, err := c.fetchOwned(ctx)
	if err != nil {
		if cached.games != nil {
			return cached.games, nil
		}
		return nil, err
	}
	c.mu.Lock()
	c.owned = ownedCache{at: time.Now(), games: games}
	c.mu.Unlock()
	return games, nil
}

func (c *Client) fetchOwned(ctx context.Context) ([]OwnedGame, error) {
	me, err := c.account()
	if err != nil {
		return nil, err
	}
	var res struct {
		Response struct {
			GameCount *int        `json:"game_count"`
			Games     []OwnedGame `json:"games"`
		} `json:"response"`
	}
	q := url.Values{"steamid": {me}, "include_appinfo": {"1"}, "include_played_free_games": {"1"}}
	if err := c.get(ctx, "/IPlayerService/GetOwnedGames/v1/", q, &res); err != nil {
		return nil, err
	}
	// Steam leaves the count out when game details are private.
	if res.Response.GameCount == nil {
		return nil, ErrPrivate
	}
	if res.Response.Games == nil {
		res.Response.Games = []OwnedGame{}
	}
	return res.Response.Games, nil
}

// Overview is the account card of the integration page.
type Overview struct {
	SteamID        string `json:"steamId"`
	Name           string `json:"name"`
	Avatar         string `json:"avatar"`
	ProfileURL     string `json:"profileUrl"`
	Level          int    `json:"level"` // 0 when Steam didn't say
	Owned          int    `json:"owned"`
	Played         int    `json:"played"`
	TotalMinutes   int    `json:"totalMinutes"`
	TwoWeekMinutes int    `json:"twoWeekMinutes"`
	GamesPrivate   bool   `json:"gamesPrivate"` // game details are hidden, so no library numbers
}

// Overview describes the account. The profile is required; the rest is left
// out when Steam won't say, rather than failing the whole card.
func (c *Client) Overview(ctx context.Context) (Overview, error) {
	me, err := c.account()
	if err != nil {
		return Overview{}, err
	}
	players, err := c.Players(ctx, []string{me})
	if err != nil {
		return Overview{}, err
	}
	if len(players) == 0 {
		return Overview{}, errors.New("Steam doesn't know this account")
	}
	p := players[0]
	o := Overview{SteamID: me, Name: p.Name, Avatar: p.Avatar, ProfileURL: p.ProfileURL}

	var lvl struct {
		Response struct {
			Level int `json:"player_level"`
		} `json:"response"`
	}
	if c.get(ctx, "/IPlayerService/GetSteamLevel/v1/", url.Values{"steamid": {me}}, &lvl) == nil {
		o.Level = lvl.Response.Level
	}

	var recent struct {
		Response struct {
			Games []struct {
				TwoWeeks int `json:"playtime_2weeks"`
			} `json:"games"`
		} `json:"response"`
	}
	if c.get(ctx, "/IPlayerService/GetRecentlyPlayedGames/v1/", url.Values{"steamid": {me}}, &recent) == nil {
		for _, g := range recent.Response.Games {
			o.TwoWeekMinutes += g.TwoWeeks
		}
	}

	games, err := c.OwnedGames(ctx)
	switch {
	case errors.Is(err, ErrPrivate):
		o.GamesPrivate = true
	case err != nil:
		return Overview{}, err
	default:
		o.Owned = len(games)
		for _, g := range games {
			o.TotalMinutes += g.PlaytimeMinutes
			if g.PlaytimeMinutes > 0 {
				o.Played++
			}
		}
	}
	return o, nil
}

// AppID is the id as the text Shelf uses everywhere else.
func (g OwnedGame) ID() string { return strconv.Itoa(g.AppID) }
