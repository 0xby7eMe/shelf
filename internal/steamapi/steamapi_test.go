package steamapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func testClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := New(func() (string, error) { return "me", nil })
	c.BaseURL = srv.URL
	c.path = filepath.Join(t.TempDir(), "steam.json")
	c.key = "KEY"
	return c
}

func TestReadyExplainsWhatIsMissing(t *testing.T) {
	c := New(func() (string, error) { return "", errors.New("no steam") })
	c.path, c.key = "", ""
	if ok, why := c.Ready(); ok || !strings.Contains(why, "Web API key") {
		t.Errorf("no key: %v %q", ok, why)
	}
	c.key = "K"
	if ok, why := c.Ready(); ok || !strings.Contains(why, "Sign in to Steam") {
		t.Errorf("no account: %v %q", ok, why)
	}
}

func TestKeyIsSavedPrivately(t *testing.T) {
	c := testClient(t, nil)
	if err := c.SetKey("  SECRET  "); err != nil {
		t.Fatal(err)
	}
	if c.Key() != "SECRET" || loadKey(c.path) != "SECRET" {
		t.Errorf("key = %q, saved = %q", c.Key(), loadKey(c.path))
	}
	if st, _ := os.Stat(c.path); st.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v", st.Mode().Perm())
	}
	if err := c.SetKey(""); err != nil {
		t.Fatal(err)
	}
	if c.Configured() || loadKey(c.path) != "" {
		t.Error("key not forgotten")
	}
}

func TestOwnedGamesAreCachedAndSurviveOutages(t *testing.T) {
	var calls atomic.Int32
	fail := atomic.Bool{}
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if fail.Load() {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if r.URL.Query().Get("steamid") != "me" || r.URL.Query().Get("include_appinfo") != "1" {
			t.Errorf("query = %v", r.URL.Query())
		}
		fmt.Fprint(w, `{"response":{"game_count":2,"games":[{"appid":620,"name":"Portal 2","playtime_forever":90,"rtime_last_played":5},{"appid":10,"name":"Counter-Strike"}]}}`)
	})
	ctx := context.Background()

	games, err := c.OwnedGames(ctx)
	if err != nil || len(games) != 2 || games[0].ID() != "620" || games[0].PlaytimeMinutes != 90 {
		t.Fatalf("games = %+v, err = %v", games, err)
	}
	c.OwnedGames(ctx)
	if calls.Load() != 1 {
		t.Errorf("calls = %d, want 1 (cached)", calls.Load())
	}

	// A stale answer beats none when Steam is down.
	c.mu.Lock()
	c.owned.at = c.owned.at.Add(-2 * ownedFor)
	c.mu.Unlock()
	fail.Store(true)
	if games, err := c.OwnedGames(ctx); err != nil || len(games) != 2 {
		t.Errorf("during outage: %+v, %v", games, err)
	}

	// Changing the key throws the old library away.
	c.SetKey("OTHER")
	if _, err := c.OwnedGames(ctx); err == nil {
		t.Error("cache survived a key change")
	}
}

func TestPrivateGameDetailsAreReported(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"response":{}}`) })
	if _, err := c.OwnedGames(context.Background()); !errors.Is(err, ErrPrivate) {
		t.Errorf("err = %v", err)
	}
}

func TestErrorsAreFriendlyAndKeepTheKeyOut(t *testing.T) {
	for status, want := range map[int]error{http.StatusForbidden: ErrBadKey, http.StatusUnauthorized: ErrPrivate} {
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(status) })
		if _, err := c.Friends(context.Background()); !errors.Is(err, want) {
			t.Errorf("status %d: %v", status, err)
		}
	}
	c := testClient(t, nil)
	c.BaseURL = "http://127.0.0.1:1"
	c.key = "SECRETKEY"
	if _, err := c.Friends(context.Background()); err == nil || strings.Contains(err.Error(), "SECRETKEY") {
		t.Errorf("err = %v", err)
	}
}

func TestPlayersAreAskedForHundredAtATime(t *testing.T) {
	var batches []int
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		batches = append(batches, len(strings.Split(r.URL.Query().Get("steamids"), ",")))
		fmt.Fprint(w, `{"response":{"players":[]}}`)
	})
	ids := make([]string, 230)
	for i := range ids {
		ids[i] = fmt.Sprint(i)
	}
	if _, err := c.Players(context.Background(), ids); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(batches) != "[100 100 30]" {
		t.Errorf("batches = %v", batches)
	}
}

func TestFriendsAndOverview(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ISteamUser/GetFriendList/v1/":
			fmt.Fprint(w, `{"friendslist":{"friends":[{"steamid":"1"}]}}`)
		case "/ISteamUser/GetPlayerSummaries/v2/":
			fmt.Fprint(w, `{"response":{"players":[{"steamid":"1","personaname":"Amy","personastate":1,"gameid":"620","gameextrainfo":"Portal 2"}]}}`)
		case "/IPlayerService/GetSteamLevel/v1/":
			fmt.Fprint(w, `{"response":{"player_level":42}}`)
		case "/IPlayerService/GetRecentlyPlayedGames/v1/":
			fmt.Fprint(w, `{"response":{"games":[{"playtime_2weeks":30},{"playtime_2weeks":15}]}}`)
		case "/IPlayerService/GetOwnedGames/v1/":
			fmt.Fprint(w, `{"response":{"game_count":3,"games":[{"appid":1,"name":"a","playtime_forever":60},{"appid":2,"name":"b"},{"appid":3,"name":"c","playtime_forever":120}]}}`)
		}
	})
	ctx := context.Background()

	fs, err := c.Friends(ctx)
	if err != nil || len(fs) != 1 || fs[0].GameName != "Portal 2" {
		t.Fatalf("friends = %+v, %v", fs, err)
	}

	o, err := c.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// The same fake profile answers for "me", which is enough to check the sums.
	if o.Name != "Amy" || o.Level != 42 || o.Owned != 3 || o.Played != 2 || o.TotalMinutes != 180 || o.TwoWeekMinutes != 45 || o.GamesPrivate {
		t.Errorf("overview = %+v", o)
	}
}

func TestOverviewWithPrivateGameDetailsStillWorks(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ISteamUser/GetPlayerSummaries/v2/":
			fmt.Fprint(w, `{"response":{"players":[{"steamid":"me","personaname":"Me"}]}}`)
		case "/IPlayerService/GetOwnedGames/v1/":
			fmt.Fprint(w, `{"response":{}}`)
		default:
			w.WriteHeader(http.StatusUnauthorized)
		}
	})
	o, err := c.Overview(context.Background())
	if err != nil || !o.GamesPrivate || o.Name != "Me" || o.Level != 0 {
		t.Errorf("overview = %+v, %v", o, err)
	}
}
