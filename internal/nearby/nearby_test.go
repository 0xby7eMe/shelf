package nearby

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestNameKey(t *testing.T) {
	for in, want := range map[string]string{
		"DOOM Eternal™":           "doometernal",
		"Doom Eternal":            "doometernal",
		"Baldur's Gate 3":         "baldursgate3",
		"  Hades II  ":            "hadesii",
		"Ōkami HD":                "ōkamihd",
		"™®":                      "",
		"S.T.A.L.K.E.R. 2: Heart": "stalker2heart",
	} {
		if got := nameKey(in); got != want {
			t.Errorf("nameKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCommon(t *testing.T) {
	mine := []Game{
		{Source: "steam", ID: "620", Name: "Portal 2", Installed: true},
		{Source: "epic", ID: "Fortnite", Name: "Fortnite"},
		{Source: "steam", ID: "1091500", Name: "Cyberpunk 2077"},
		{Source: "gog", ID: "1423049311", Name: "Cyberpunk 2077", Installed: true},
		{Source: "steam", ID: "570", Name: "Dota 2"},
		{Source: "steam", ID: "999", Name: "Renamed Locally"},
	}
	theirs := []Game{
		{Source: "steam", ID: "620", Name: "Portal 2", Installed: true},
		{Source: "steam", ID: "1091500", Name: "Cyberpunk 2077™"},
		{Source: "epic", ID: "Fortnite", Name: "Fortnite", Installed: true},
		{Source: "steam", ID: "999", Name: "Original Name"},
		{Source: "steam", ID: "440", Name: "Team Fortress 2"},
	}
	got := Common(mine, theirs)
	want := []Shared{
		{Key: "portal2", Name: "Portal 2", GameID: "steam:620", Installed: true, Sources: []string{"steam"}, TheirSources: []string{"steam"}, TheirInstalled: true},
		{Key: "cyberpunk2077", Name: "Cyberpunk 2077", GameID: "gog:1423049311", Installed: true, Sources: []string{"steam", "gog"}, TheirSources: []string{"steam"}},
		{Key: "fortnite", Name: "Fortnite", GameID: "epic:Fortnite", Sources: []string{"epic"}, TheirSources: []string{"epic"}, TheirInstalled: true},
		{Key: "renamedlocally", Name: "Renamed Locally", GameID: "steam:999", Sources: []string{"steam"}, TheirSources: []string{"steam"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Common:\n got %+v\nwant %+v", got, want)
	}

	if got := Common(mine, nil); len(got) != 0 || got == nil {
		t.Fatalf("nothing in common should be an empty list, got %#v", got)
	}
}

func TestEveryone(t *testing.T) {
	a := []Shared{
		{Key: "portal2", Name: "Portal 2", TheirSources: []string{"steam"}, TheirInstalled: true},
		{Key: "fortnite", Name: "Fortnite", TheirSources: []string{"epic"}, TheirInstalled: true},
	}
	b := []Shared{
		{Key: "portal2", Name: "Portal 2", TheirSources: []string{"epic"}},
		{Key: "dota2", Name: "Dota 2"},
	}
	got := Everyone([][]Shared{a, b})
	if len(got) != 1 || got[0].Key != "portal2" {
		t.Fatalf("Everyone = %+v, want only Portal 2", got)
	}
	if got[0].TheirInstalled {
		t.Error("installed for everyone although one of them hasn't installed it")
	}
	if !reflect.DeepEqual(got[0].TheirSources, []string{"epic", "steam"}) {
		t.Errorf("sources = %v", got[0].TheirSources)
	}
	if got := Everyone([][]Shared{a}); len(got) != 0 {
		t.Errorf("one peer is not everyone: %+v", got)
	}
}

func TestStore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "shelf", "nearby.json")
	st := NewStore(path)
	if !st.Get().Enabled {
		t.Error("on by default")
	}
	id := st.ID()
	if len(id) != 16 {
		t.Fatalf("id %q", id)
	}
	if err := st.Set(Settings{Enabled: false, Name: "  Max   the\tgreat  "}); err != nil {
		t.Fatal(err)
	}
	again := NewStore(path)
	if again.ID() != id {
		t.Error("id changed after a restart")
	}
	if s := again.Get(); s.Enabled || s.Name != "Max the great" {
		t.Errorf("settings after a restart: %+v", s)
	}
	if again.DisplayName() != "Max the great" {
		t.Errorf("display name %q", again.DisplayName())
	}
}

// node is a Service with the discovery replaced: tests hand entries to found.
type node struct {
	*Service
	events chan string
	addr   *net.TCPAddr
}

func newNode(t *testing.T, ctx context.Context, name string, games []Game) *node {
	t.Helper()
	st := NewStore("")
	if err := st.Set(Settings{Enabled: true, Name: name}); err != nil {
		t.Fatal(err)
	}
	n := &node{events: make(chan string, 64)}
	n.Service = New(st, func(event string, _ any) {
		select {
		case n.events <- event:
		default:
		}
	})
	n.listen = func() (net.Listener, error) {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err == nil {
			n.addr = ln.Addr().(*net.TCPAddr)
		}
		return ln, err
	}
	n.announce = func(int, []string, string) (func(), error) { return func() {}, nil }
	n.browse = func(ctx context.Context, _ func(Entry)) error { <-ctx.Done(); return nil }
	n.netState = func() string { return "" }
	n.allow = func(ip net.IP) bool { return ip.IsLoopback() }
	n.SetLibrary(games)
	n.Start(ctx)
	t.Cleanup(n.Stop)
	return n
}

func (n *node) entry(session string) Entry {
	return Entry{ID: n.store.ID(), Session: session, Host: "box", Port: n.addr.Port, Addrs: []net.IP{n.addr.IP}}
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestPeersCompareLibraries(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	alice := newNode(t, ctx, "Alice", []Game{
		{Source: "steam", ID: "620", Name: "Portal 2", Installed: true},
		{Source: "steam", ID: "440", Name: "Team Fortress 2"},
	})
	bob := newNode(t, ctx, "Bob", []Game{
		{Source: "epic", ID: "Portal2", Name: "Portal 2", Installed: true},
		{Source: "steam", ID: "570", Name: "Dota 2"},
	})

	// Seeing yourself changes nothing.
	alice.found(alice.entry("self"))
	alice.mu.Lock()
	seen := len(alice.peers)
	alice.mu.Unlock()
	if seen != 0 {
		t.Fatal("found itself")
	}

	alice.found(bob.entry("b1"))
	bob.found(alice.entry("a1"))
	waitFor(t, "both to see each other", func() bool {
		return len(alice.Snapshot().Peers) == 1 && len(bob.Snapshot().Peers) == 1
	})

	snap := alice.Snapshot()
	p := snap.Peers[0]
	if p.Name != "Bob" || p.Games != 2 || p.Host != "box" {
		t.Fatalf("peer %+v", p)
	}
	if len(p.Common) != 1 || p.Common[0].Name != "Portal 2" || p.Common[0].GameID != "steam:620" ||
		!p.Common[0].TheirInstalled || p.Common[0].TheirSources[0] != "epic" {
		t.Fatalf("common %+v", p.Common)
	}

	// Bob buys Team Fortress 2: Alice sees it on the next check.
	bob.SetLibrary([]Game{
		{Source: "epic", ID: "Portal2", Name: "Portal 2", Installed: true},
		{Source: "steam", ID: "570", Name: "Dota 2"},
		{Source: "steam", ID: "440", Name: "Team Fortress 2"},
	})
	alice.mu.Lock()
	alice.peers[bob.store.ID()].poke()
	alice.mu.Unlock()
	waitFor(t, "the new game", func() bool {
		s := alice.Snapshot()
		return len(s.Peers) == 1 && len(s.Peers[0].Common) == 2
	})

	// Bob switches it off: Alice lets him go after the failed checks.
	if _, err := bob.SetSettings(Settings{Enabled: false, Name: "Bob"}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxFailures; i++ {
		alice.mu.Lock()
		if p := alice.peers[bob.store.ID()]; p != nil {
			p.poke()
		}
		alice.mu.Unlock()
		time.Sleep(50 * time.Millisecond)
	}
	waitFor(t, "Bob to be gone", func() bool { return len(alice.Snapshot().Peers) == 0 })
}

func TestEveryoneAcrossPeers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	portal := Game{Source: "steam", ID: "620", Name: "Portal 2", Installed: true}
	me := newNode(t, ctx, "Me", []Game{portal, {Source: "steam", ID: "440", Name: "Team Fortress 2"}})
	a := newNode(t, ctx, "A", []Game{portal, {Source: "steam", ID: "440", Name: "Team Fortress 2"}})
	b := newNode(t, ctx, "B", []Game{portal})

	me.found(a.entry("a"))
	me.found(b.entry("b"))
	waitFor(t, "both peers", func() bool { return len(me.Snapshot().Peers) == 2 })
	every := me.Snapshot().Everyone
	if len(every) != 1 || every[0].Name != "Portal 2" || !every[0].TheirInstalled {
		t.Fatalf("everyone %+v", every)
	}
}

func TestRestartedPeerIsReplaced(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	me := newNode(t, ctx, "Me", nil)
	other := newNode(t, ctx, "Other", []Game{{Source: "steam", ID: "1", Name: "One"}})

	me.found(other.entry("first"))
	waitFor(t, "the peer", func() bool { return len(me.Snapshot().Peers) == 1 })
	me.mu.Lock()
	first := me.peers[other.store.ID()]
	me.mu.Unlock()

	me.found(other.entry("second"))
	me.mu.Lock()
	second := me.peers[other.store.ID()]
	me.mu.Unlock()
	if first == second || second.Session != "second" {
		t.Fatal("a new session of the same Shelf should replace the old one")
	}
	waitFor(t, "the peer again", func() bool { return len(me.Snapshot().Peers) == 1 })
}

func TestServeOnlyTheList(t *testing.T) {
	s := New(NewStore(""), nil)
	s.SetLibrary([]Game{{Source: "steam", ID: "620", Name: "Portal 2"}})
	srv := httptest.NewServer(http.HandlerFunc(s.serve))
	defer srv.Close()

	resp, err := http.Get(srv.URL + libraryPath)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	etag := resp.Header.Get("ETag")
	if resp.StatusCode != 200 || etag == "" {
		t.Fatalf("status %d etag %q", resp.StatusCode, etag)
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+libraryPath, nil)
	req.Header.Set("If-None-Match", etag)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotModified {
		t.Errorf("unchanged list: status %d", resp.StatusCode)
	}

	for _, path := range []string{"/", "/shelf/nearby/v1/other"} {
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("%s: status %d", path, resp.StatusCode)
		}
	}
}

func TestLanAddr(t *testing.T) {
	for addr, want := range map[string]bool{
		"192.168.1.20": true,
		"10.0.0.5":     true,
		"172.16.3.4":   true,
		"169.254.1.1":  true,
		"127.0.0.1":    false,
		"8.8.8.8":      false,
		"fd00::1":      true,
		"fe80::1":      false, // needs a zone
		"2001:db8::1":  false,
		"224.0.0.251":  false,
	} {
		if got := lanAddr(net.ParseIP(addr)); got != want {
			t.Errorf("lanAddr(%s) = %v, want %v", addr, got, want)
		}
	}
}

// The real mDNS round trip, when the machine allows multicast.
func TestMDNSRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("uses the network")
	}
	stop, err := mdnsAnnounce(Port, []string{"id=roundtrip", "v=1"}, "test-"+newSession())
	if err != nil {
		t.Skipf("no multicast here: %v", err)
	}
	defer stop()

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	var once sync.Once
	got := make(chan Entry, 1)
	go mdnsBrowse(ctx, func(e Entry) {
		if e.ID == "roundtrip" {
			once.Do(func() { got <- e })
		}
	})
	select {
	case e := <-got:
		if e.Port != Port || len(e.Addrs) == 0 {
			t.Fatalf("entry %+v", e)
		}
	case <-ctx.Done():
		t.Skip("no answer over multicast; the network may block it")
	}
}
