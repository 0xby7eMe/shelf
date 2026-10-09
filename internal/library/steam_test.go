package library

import (
	"context"
	"errors"
	"testing"

	"shelf/internal/steamapi"
)

func TestParseLastSteamUser(t *testing.T) {
	marked := `"users" { "111" { "Timestamp" "900" } "222" { "MostRecent" "1" "Timestamp" "100" } }`
	if id, err := parseLastSteamUser([]byte(marked)); err != nil || id != "222" {
		t.Errorf("marked: %q, %v", id, err)
	}
	byTime := `"users" { "111" { "Timestamp" "900" } "222" { "Timestamp" "100" } }`
	if id, err := parseLastSteamUser([]byte(byTime)); err != nil || id != "111" {
		t.Errorf("by time: %q, %v", id, err)
	}
	if _, err := parseLastSteamUser([]byte(`"users" { }`)); err == nil {
		t.Error("no accounts: want an error")
	}
}

type fakeOwned struct {
	games      []steamapi.OwnedGame
	err        error
	configured bool
}

func (f fakeOwned) OwnedGames(context.Context) ([]steamapi.OwnedGame, error) { return f.games, f.err }
func (f fakeOwned) Configured() bool                                         { return f.configured }

func TestAddOwnedMergesTheWebAPILibrary(t *testing.T) {
	installed := []Game{{ID: "steam:620", ExternalID: "620", Name: "Portal 2", Installed: true, PlaytimeMinutes: 10, LastPlayed: 500}}
	api := fakeOwned{configured: true, games: []steamapi.OwnedGame{
		{AppID: 620, Name: "Portal 2", PlaytimeMinutes: 90, LastPlayed: 100},
		{AppID: 10, Name: "Counter-Strike", PlaytimeMinutes: 30, LastPlayed: 7},
		{AppID: 228980, Name: "Steamworks Common Redistributables"},
		{AppID: 5},
	}}

	got := (&Steam{owned: api}).addOwned(append([]Game(nil), installed...))
	if len(got) != 2 {
		t.Fatalf("games = %+v", got)
	}
	if p := got[0]; p.PlaytimeMinutes != 90 || p.LastPlayed != 500 || !p.Installed {
		t.Errorf("installed game not merged: %+v", p)
	}
	cs := got[1]
	if cs.ID != "steam:10" || cs.Installed || cs.Cover != "/cover/10" || cs.PlaytimeMinutes != 30 || cs.Source != SourceSteam {
		t.Errorf("owned game = %+v", cs)
	}
}

func TestAddOwnedLeavesGamesAloneWithoutAKeyOrOnError(t *testing.T) {
	base := []Game{{ID: "steam:1", ExternalID: "1"}}
	for name, s := range map[string]*Steam{
		"no api":     {},
		"no key":     {owned: fakeOwned{games: []steamapi.OwnedGame{{AppID: 2, Name: "x"}}}},
		"api failed": {owned: fakeOwned{configured: true, err: errors.New("down")}},
	} {
		if got := s.addOwned(append([]Game(nil), base...)); len(got) != 1 {
			t.Errorf("%s: %+v", name, got)
		}
	}
}

func TestSteamDownloadOf(t *testing.T) {
	cases := []struct {
		name         string
		flags        int
		done, total  int64
		want, update bool
		paused       bool
		percent      float64
	}{
		{name: "installed", flags: 4},
		{name: "update pending, not started", flags: 6},
		{name: "first install", flags: 1026, done: 250, total: 1000, want: true, percent: 25},
		{name: "queued install, size unknown", flags: 2, want: true, percent: -1},
		{name: "update running", flags: 1030, done: 1, total: 4, want: true, update: true, percent: 25},
		{name: "update paused", flags: 1542, done: 1, total: 2, want: true, update: true, paused: true, percent: 50},
	}
	for _, c := range cases {
		d, ok := steamDownloadOf("620", c.flags, c.done, c.total)
		if ok != c.want {
			t.Errorf("%s: downloading = %v, want %v", c.name, ok, c.want)
			continue
		}
		if ok && (d.Update != c.update || d.Paused != c.paused || d.Percent != c.percent || d.AppID != "620") {
			t.Errorf("%s: got %+v", c.name, d)
		}
	}
	if _, ok := steamDownloadOf("", 1026, 0, 0); ok {
		t.Error("no app id: want no download")
	}
}
