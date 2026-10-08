package friends

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
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

func steamServer(t *testing.T, status int) (*httptest.Server, *string) {
	t.Helper()
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.URL.Query().Get("key")
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		switch r.URL.Path {
		case "/ISteamUser/GetFriendList/v1/":
			if r.URL.Query().Get("steamid") != "me" {
				t.Errorf("steamid = %q", r.URL.Query().Get("steamid"))
			}
			fmt.Fprint(w, `{"friendslist":{"friends":[{"steamid":"1"},{"steamid":"2"},{"steamid":"3"},{"steamid":"4"}]}}`)
		case "/ISteamUser/GetPlayerSummaries/v2/":
			fmt.Fprint(w, `{"response":{"players":[
				{"steamid":"1","personaname":"Amy","personastate":1,"gameid":"620","gameextrainfo":"Portal 2","avatarmedium":"https://a/1.jpg","profileurl":"https://p/1"},
				{"steamid":"2","personaname":"Bob","personastate":3},
				{"steamid":"3","personaname":"Cy","personastate":0},
				{"steamid":"4","personaname":"Di","personastate":6}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &gotKey
}

func newTestSteam(base string) *Steam {
	s := NewSteam()
	s.apiBase = base
	s.userID = func() (string, error) { return "me", nil }
	return s
}

func TestSteamFriends(t *testing.T) {
	srv, gotKey := steamServer(t, http.StatusOK)
	s := newTestSteam(srv.URL)
	if ok, _ := s.Ready(); ok {
		t.Fatal("ready without a key")
	}
	s.Configure(map[string]string{"apiKey": "K"})
	if ok, why := s.Ready(); !ok {
		t.Fatal(why)
	}

	fs, err := s.Friends(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if *gotKey != "K" || len(fs) != 4 {
		t.Fatalf("key=%q friends=%d", *gotKey, len(fs))
	}
	want := []Status{StatusPlaying, StatusAway, StatusOffline, StatusOnline}
	for i, f := range fs {
		if f.Status != want[i] {
			t.Errorf("%s: status %s, want %s", f.Name, f.Status, want[i])
		}
	}
	if p := fs[0].Playing; p == nil || p.Source != "steam" || p.ID != "620" || p.Name != "Portal 2" {
		t.Errorf("playing = %+v", fs[0].Playing)
	}
}

func TestSteamErrorsAreFriendlyAndKeepTheKeyOut(t *testing.T) {
	for status, want := range map[int]string{
		http.StatusForbidden:    "rejected the Web API key",
		http.StatusUnauthorized: "friends list is private",
	} {
		srv, _ := steamServer(t, status)
		s := newTestSteam(srv.URL)
		s.Configure(map[string]string{"apiKey": "SECRETKEY"})
		_, err := s.Friends(context.Background())
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("status %d: err = %v", status, err)
		}
	}

	// An unreachable server must not put the request URL, and so the key, in the error.
	s := newTestSteam("http://127.0.0.1:1")
	s.Configure(map[string]string{"apiKey": "SECRETKEY"})
	_, err := s.Friends(context.Background())
	if err == nil || strings.Contains(err.Error(), "SECRETKEY") {
		t.Errorf("err = %v", err)
	}
}

func TestSteamAsksForHundredAtATime(t *testing.T) {
	var batches []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "GetFriendList") {
			var b strings.Builder
			b.WriteString(`{"friendslist":{"friends":[`)
			for i := 0; i < 230; i++ {
				if i > 0 {
					b.WriteString(",")
				}
				fmt.Fprintf(&b, `{"steamid":"%d"}`, i)
			}
			b.WriteString("]}}")
			fmt.Fprint(w, b.String())
			return
		}
		batches = append(batches, len(strings.Split(r.URL.Query().Get("steamids"), ",")))
		fmt.Fprint(w, `{"response":{"players":[]}}`)
	}))
	defer srv.Close()
	s := newTestSteam(srv.URL)
	s.Configure(map[string]string{"apiKey": "K"})
	if _, err := s.Friends(context.Background()); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(batches) != "[100 100 30]" {
		t.Errorf("batches = %v", batches)
	}
}
