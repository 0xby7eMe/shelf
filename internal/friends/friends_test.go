package friends

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"shelf/internal/steamapi"
)

type fake struct {
	source  string
	ready   bool
	friends []Friend
	err     error
	calls   atomic.Int32
	cfg     map[string]string
}

func (f *fake) Source() string { return f.source }
func (f *fake) Name() string   { return strings.ToUpper(f.source) }
func (f *fake) Fields() []Field {
	return []Field{{Key: "token", Label: "Token", Secret: true}, {Key: "user", Label: "User"}}
}
func (f *fake) Configure(v map[string]string) {
	f.cfg = v
	f.ready = v["token"] != ""
}
func (f *fake) Ready() (bool, string) {
	if !f.ready {
		return false, "needs a token"
	}
	return true, ""
}
func (f *fake) Friends(context.Context) ([]Friend, error) {
	f.calls.Add(1)
	return append([]Friend(nil), f.friends...), f.err
}

func memStore() *Store { return &Store{data: map[string]map[string]string{}} }

func TestSnapshotMergesSortsAndTagsSources(t *testing.T) {
	a := &fake{source: "a", ready: true, friends: []Friend{
		{ID: "1", Name: "zed", Status: StatusOffline},
		{ID: "2", Name: "Amy", Status: StatusPlaying, Playing: &Playing{Source: "a", ID: "9", Name: "G"}},
	}}
	b := &fake{source: "b", ready: true, friends: []Friend{{ID: "3", Name: "bob", Status: StatusOnline}}}
	off := &fake{source: "c"} // not ready: never asked
	h := NewHub(memStore(), a, b, off)
	// NewHub configures providers from the (empty) store, which unreadies the fakes.
	a.ready, b.ready = true, true

	snap := h.Snapshot(context.Background(), false)
	var order []string
	for _, f := range snap.Friends {
		order = append(order, f.Source+":"+f.Name)
	}
	if got := strings.Join(order, " "); got != "a:Amy b:bob a:zed" {
		t.Errorf("order = %s", got)
	}
	if off.calls.Load() != 0 {
		t.Error("a provider that isn't ready was asked for friends")
	}
	if len(snap.Providers) != 3 || snap.Providers[0].Count != 2 || snap.Providers[2].Ready {
		t.Errorf("providers = %+v", snap.Providers)
	}
	if snap.Providers[2].Message != "needs a token" {
		t.Errorf("message = %q", snap.Providers[2].Message)
	}
}

func TestSnapshotCachesUntilForced(t *testing.T) {
	a := &fake{source: "a"}
	h := NewHub(memStore(), a)
	a.ready = true
	now := time.Unix(1000, 0)
	h.now = func() time.Time { return now }

	h.Snapshot(context.Background(), false)
	h.Snapshot(context.Background(), false)
	if a.calls.Load() != 1 {
		t.Errorf("calls = %d, want 1 (cached)", a.calls.Load())
	}
	h.Snapshot(context.Background(), true)
	if a.calls.Load() != 2 {
		t.Errorf("calls = %d, want 2 after force", a.calls.Load())
	}
	now = now.Add(cacheFor + time.Second)
	h.Snapshot(context.Background(), false)
	if a.calls.Load() != 3 {
		t.Errorf("calls = %d, want 3 after expiry", a.calls.Load())
	}
}

func TestAProviderErrorStaysWithThatProvider(t *testing.T) {
	bad := &fake{source: "a", err: errors.New("down")}
	good := &fake{source: "b", friends: []Friend{{ID: "1", Name: "x", Status: StatusOnline}}}
	h := NewHub(memStore(), bad, good)
	bad.ready, good.ready = true, true
	snap := h.Snapshot(context.Background(), false)
	if snap.Providers[0].Message != "down" || len(snap.Friends) != 1 {
		t.Errorf("snap = %+v", snap)
	}
}

func TestConfigureKeepsBlankSecretsAndNeverReturnsThem(t *testing.T) {
	a := &fake{source: "a"}
	store := memStore()
	h := NewHub(store, a)

	if err := h.Configure("a", map[string]string{"token": " secret ", "user": "me"}); err != nil {
		t.Fatal(err)
	}
	if a.cfg["token"] != "secret" || !a.ready {
		t.Fatalf("cfg = %v", a.cfg)
	}
	// Resubmitting with the secret blank keeps it; a plain field is cleared.
	if err := h.Configure("a", map[string]string{"token": "", "user": ""}); err != nil {
		t.Fatal(err)
	}
	if a.cfg["token"] != "secret" || a.cfg["user"] != "" {
		t.Errorf("cfg = %v", a.cfg)
	}
	snap := h.Snapshot(context.Background(), false)
	raw := fmt.Sprintf("%+v", snap)
	if strings.Contains(raw, "secret") || !snap.Providers[0].Fields[0].Set {
		t.Errorf("secret leaked or not marked set: %s", raw)
	}

	if err := h.Disconnect("a"); err != nil {
		t.Fatal(err)
	}
	if a.ready || len(store.Get("a")) != 0 {
		t.Error("disconnect left something behind")
	}
	if err := h.Configure("nope", nil); err == nil {
		t.Error("unknown launcher accepted")
	}
}

func TestUnsupportedSaysWhy(t *testing.T) {
	u := NewUnsupported("epic", "Epic Games", "no way in")
	snap := NewHub(memStore(), u).Snapshot(context.Background(), false)
	p := snap.Providers[0]
	if p.Ready || p.Message != "no way in" || len(p.Fields) != 0 {
		t.Errorf("provider = %+v", p)
	}
}

func TestSteamFriendMapsPresence(t *testing.T) {
	cases := []struct {
		p    steamapi.Player
		want Status
	}{
		{steamapi.Player{PersonaState: 1}, StatusOnline},
		{steamapi.Player{PersonaState: 6}, StatusOnline},
		{steamapi.Player{PersonaState: 3}, StatusAway},
		{steamapi.Player{PersonaState: 4}, StatusAway},
		{steamapi.Player{PersonaState: 0}, StatusOffline},
		{steamapi.Player{PersonaState: 1, GameID: "620", GameName: "Portal 2"}, StatusPlaying},
	}
	for _, c := range cases {
		if got := steamFriend(c.p).Status; got != c.want {
			t.Errorf("%+v: %s, want %s", c.p, got, c.want)
		}
	}
	f := steamFriend(steamapi.Player{PersonaState: 1, GameID: "620", GameName: "Portal 2"})
	if p := f.Playing; p == nil || p.Source != "steam" || p.ID != "620" || p.Name != "Portal 2" {
		t.Errorf("playing = %+v", f.Playing)
	}
	if f := steamFriend(steamapi.Player{GameID: "9"}); f.Playing == nil || f.Playing.Name != "a game" {
		t.Errorf("unnamed game = %+v", f.Playing)
	}
}
