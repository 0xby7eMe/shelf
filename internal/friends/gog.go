package friends

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// GogSession is what the GOG provider needs from the GOG login, which the
// Epic manager keeps (Settings, Integrations, GOG).
type GogSession interface {
	// GogUserID is the account's user id, or "" when signed out.
	GogUserID() string
	// GogGet fetches a JSON document from a GOG service, signed in.
	GogGet(ctx context.Context, rawURL string, out any) error
	// GogTitle names a game of the library by its key ("gog-1207658924").
	GogTitle(key string) string
}

// Where GOG Galaxy keeps friends and who is online. Variables so tests can use a local server.
var (
	gogChatBase     = "https://chat.gog.com"
	gogPresenceBase = "https://presence.gog.com"
)

// GOG reads the friends list GOG Galaxy shows, with the account Shelf signed in to GOG with.
type GOG struct {
	s GogSession
}

func NewGOG(s GogSession) *GOG { return &GOG{s: s} }

func (g *GOG) Source() string              { return "gog" }
func (g *GOG) Name() string                { return "GOG" }
func (g *GOG) Fields() []Field             { return nil }
func (g *GOG) Configure(map[string]string) {}

func (g *GOG) Ready() (bool, string) {
	if g.s.GogUserID() == "" {
		return false, "Sign in to GOG under Settings, Integrations, GOG."
	}
	return true, ""
}

type gogFriends struct {
	Items []struct {
		UserID   string `json:"user_id"`
		Username string `json:"username"`
		Images   struct {
			Medium   string `json:"medium"`
			Medium2x string `json:"medium_2x"`
		} `json:"images"`
	} `json:"items"`
}

// gogStatuses lists the users of a request who are online. data is what their
// client last reported, such as the game they are in.
type gogStatuses struct {
	Items []struct {
		UserID string `json:"user_id"`
		Data   struct {
			Presence string `json:"presence"`
			GameID   any    `json:"game_id"` // a string or a number, depending on the client
		} `json:"data"`
	} `json:"items"`
}

// gogStatusBatch is how many users presence.gog.com takes in one request.
const gogStatusBatch = 100

func (g *GOG) Friends(ctx context.Context) ([]Friend, error) {
	me := g.s.GogUserID()
	if me == "" {
		return nil, fmt.Errorf("not signed in to GOG")
	}
	var list gogFriends
	if err := g.s.GogGet(ctx, gogChatBase+"/users/"+url.PathEscape(me)+"/friends", &list); err != nil {
		return nil, err
	}

	friends := make([]Friend, 0, len(list.Items))
	index := map[string]int{}
	for _, it := range list.Items {
		if it.UserID == "" || it.Username == "" {
			continue
		}
		index[it.UserID] = len(friends)
		friends = append(friends, Friend{
			ID:         it.UserID,
			Name:       it.Username,
			Avatar:     firstNonEmpty(it.Images.Medium2x, it.Images.Medium),
			ProfileURL: "https://www.gog.com/u/" + url.PathEscape(it.Username),
			Status:     StatusOffline,
		})
	}

	// Presence only lists who is online; everyone else is offline.
	for start := 0; start < len(friends); start += gogStatusBatch {
		end := min(start+gogStatusBatch, len(friends))
		ids := make([]string, 0, end-start)
		for _, f := range friends[start:end] {
			if gogGameID(f.ID) == f.ID { // user ids are numbers too
				ids = append(ids, f.ID)
			}
		}
		var st gogStatuses
		u := gogPresenceBase + "/statuses?user_id=" + strings.Join(ids, ",")
		if err := g.s.GogGet(ctx, u, &st); err != nil {
			return nil, fmt.Errorf("couldn't read who is online on GOG: %w", err)
		}
		for _, s := range st.Items {
			i, ok := index[s.UserID]
			if !ok {
				continue
			}
			g.applyStatus(&friends[i], s.Data.Presence, s.Data.GameID)
		}
	}
	return friends, nil
}

func (g *GOG) applyStatus(f *Friend, presence string, gameID any) {
	switch strings.ToLower(presence) {
	case "offline":
		return
	case "away", "idle":
		f.Status = StatusAway
	default:
		f.Status = StatusOnline
	}
	id := gogGameID(gameID)
	if id == "" {
		return
	}
	key := "gog-" + id
	name := g.s.GogTitle(key)
	if name == "" {
		name = "a game"
	}
	f.Status = StatusPlaying
	f.Playing = &Playing{Source: "gog", ID: key, Name: name}
}

// gogGameID reads a GOG id, which clients send as text or as a number.
func gogGameID(v any) string {
	var s string
	switch x := v.(type) {
	case string:
		s = strings.TrimSpace(x)
	case float64:
		if x > 0 && x == float64(int64(x)) {
			s = fmt.Sprintf("%d", int64(x))
		}
	}
	if s == "" || s == "0" || len(s) > 20 {
		return ""
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return s
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
