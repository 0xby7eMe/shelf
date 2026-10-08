package steamapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

// ErrNoAchievements means the game has none, which is common and not a failure.
var ErrNoAchievements = errors.New("this game has no achievements")

// Achievement is the account's standing on one achievement.
type Achievement struct {
	APIName     string `json:"apiname"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Achieved    int    `json:"achieved"`
	UnlockTime  int64  `json:"unlocktime"`
}

// PlayerAchievements lists a game's achievements with whether the account has them.
func (c *Client) PlayerAchievements(ctx context.Context, appID string) ([]Achievement, error) {
	me, err := c.account()
	if err != nil {
		return nil, err
	}
	q := url.Values{"steamid": {me}, "appid": {appID}, "l": {"english"}}
	status, body, err := c.request(ctx, "/ISteamUserStats/GetPlayerAchievements/v1/", q, true)
	if err != nil {
		return nil, err
	}

	// Steam explains most refusals in the body, with a 400 or 403.
	var res struct {
		PlayerStats struct {
			Success      bool          `json:"success"`
			Error        string        `json:"error"`
			Achievements []Achievement `json:"achievements"`
		} `json:"playerstats"`
	}
	if status == 200 || status == 400 || status == 403 {
		if json.Unmarshal(body, &res) == nil && !res.PlayerStats.Success && res.PlayerStats.Error != "" {
			msg := strings.ToLower(res.PlayerStats.Error)
			switch {
			case strings.Contains(msg, "no stats"), strings.Contains(msg, "not have stats"):
				return nil, ErrNoAchievements
			case strings.Contains(msg, "not public"), strings.Contains(msg, "private"):
				return nil, ErrPrivate
			}
			return nil, errors.New("Steam said: " + res.PlayerStats.Error)
		}
	}
	if err := statusError(status); err != nil {
		return nil, err
	}
	if err := decode(body, &res); err != nil {
		return nil, err
	}
	if len(res.PlayerStats.Achievements) == 0 {
		return nil, ErrNoAchievements
	}
	return res.PlayerStats.Achievements, nil
}

// SchemaAchievement is what the game's developer says about an achievement.
type SchemaAchievement struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Description string `json:"description"`
	Hidden      int    `json:"hidden"`
	Icon        string `json:"icon"`
	IconGray    string `json:"icongray"`
}

// Schema lists a game's achievements with their icons. It is empty for a game without any.
func (c *Client) Schema(ctx context.Context, appID string) ([]SchemaAchievement, error) {
	var res struct {
		Game struct {
			AvailableGameStats struct {
				Achievements []SchemaAchievement `json:"achievements"`
			} `json:"availableGameStats"`
		} `json:"game"`
	}
	if err := c.get(ctx, "/ISteamUserStats/GetSchemaForGame/v2/", url.Values{"appid": {appID}, "l": {"english"}}, &res); err != nil {
		return nil, err
	}
	return res.Game.AvailableGameStats.Achievements, nil
}

// percent reads a number that Steam sometimes writes as text.
type percent float64

func (p *percent) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return err
	}
	*p = percent(f)
	return nil
}

// GlobalPercentages is how many players have each achievement, by API name. It
// needs no key, and it is a nicety: callers can do without it.
func (c *Client) GlobalPercentages(ctx context.Context, appID string) (map[string]float64, error) {
	status, body, err := c.request(ctx, "/ISteamUserStats/GetGlobalAchievementPercentagesForApp/v2/", url.Values{"gameid": {appID}}, false)
	if err != nil {
		return nil, err
	}
	if err := statusError(status); err != nil {
		return nil, err
	}
	var res struct {
		AchievementPercentages struct {
			Achievements []struct {
				Name    string  `json:"name"`
				Percent percent `json:"percent"`
			} `json:"achievements"`
		} `json:"achievementpercentages"`
	}
	if err := decode(body, &res); err != nil {
		return nil, err
	}
	out := make(map[string]float64, len(res.AchievementPercentages.Achievements))
	for _, a := range res.AchievementPercentages.Achievements {
		out[a.Name] = float64(a.Percent)
	}
	return out, nil
}
