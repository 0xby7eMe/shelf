package epic

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPendingGogUpdates(t *testing.T) {
	ubiTestManager(t)
	installed := map[string]gogInstall{
		"gog-1": {Version: "1.0"},
		"gog-2": {Version: "2.0"},
		"gog-3": {Version: ""}, // GOG named no version: nothing to compare
		"gog-4": {Version: "4.0"},
	}
	writeGogLatest(map[string]string{"gog-1": "1.1", "gog-2": "2.0", "gog-3": "3.1"})
	setGogLatest("gog-4", "") // an empty answer keeps what is known
	got := pendingGogUpdates(installed)
	if len(got) != 1 || got["gog-1"] != "1.1" {
		t.Errorf("pending = %v", got)
	}
}

func TestGogUpdate(t *testing.T) {
	linuxOnly(t)
	m, rec, f, games := gogInstallEnv(t)
	var mu sync.Mutex
	var announced []UpdatesEvent
	m.SetEmitter(func(name string, data any) {
		rec.emit(name, data)
		if e, ok := data.(UpdatesEvent); ok && name == "epic:updates" {
			mu.Lock()
			announced = append(announced, e)
			mu.Unlock()
		}
	})

	if err := m.GogInstall("gog-42"); err != nil {
		t.Fatal(err)
	}
	if p := rec.waitFinal(t, KindInstall); p.State != StateDone {
		t.Fatalf("install: %+v", p)
	}
	before := m.gogInstalled()["gog-42"]

	if ups, err := m.GogCheckUpdates(); err != nil || len(ups) != 0 {
		t.Fatalf("up to date right after installing: %+v, %v", ups, err)
	}

	f.mu.Lock()
	f.version = "1.3 (gog-4)"
	f.mu.Unlock()
	ups, err := m.GogCheckUpdates()
	if err != nil || len(ups) != 1 {
		t.Fatalf("updates = %+v, %v", ups, err)
	}
	if u := ups[0]; u.AppName != "gog-42" || u.Title != "Test Game" || u.Installed != "1.2 (gog-3)" || u.Latest != "1.3 (gog-4)" {
		t.Errorf("update = %+v", u)
	}
	lib, _ := m.GogProvider().Scan()
	if !lib[1].UpdateAvailable {
		t.Errorf("the library shows it: %+v", lib[1])
	}

	// The automatic round announces it once, and installs it when asked to.
	s := m.settings.get()
	s.GogAutoCheckUpdates, s.GogAutoUpdate = true, true
	m.settings.set(s)
	os.Remove(filepath.Join(games, "proton.log"))
	m.gogAutoUpdateRound()
	if p := rec.waitFinal(t, KindUpdate); p.State != StateDone {
		t.Fatalf("update: %+v", p)
	}
	mu.Lock()
	if len(announced) != 1 || !announced[0].Auto || announced[0].Titles[0] != "Test Game" {
		t.Errorf("announced = %+v", announced)
	}
	mu.Unlock()

	// The new installer ran over the same folder, and nothing else changed.
	log, _ := os.ReadFile(filepath.Join(games, "proton.log"))
	if !strings.Contains(string(log), "/DIR="+winPath(before.InstallPath)) {
		t.Errorf("proton.log:\n%s", log)
	}
	after := m.gogInstalled()["gog-42"]
	if after.Version != "1.3 (gog-4)" || after.InstallPath != before.InstallPath || after.InstalledAt != before.InstalledAt {
		t.Errorf("after = %+v, before = %+v", after, before)
	}
	if ups := m.GogUpdates(); len(ups) != 0 {
		t.Errorf("still behind: %+v", ups)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(before.InstallPath), ".shelf-downloads", "gog-42")); !os.IsNotExist(err) {
		t.Error("the downloaded installer is cleaned up")
	}

	if err := m.GogUpdate("gog-43"); err == nil {
		t.Error("updating a game that isn't installed")
	}
}

func TestGogGameServices(t *testing.T) {
	m := ubiTestManager(t)
	f := newFakeGog(t)
	signedIn(t, m, time.Now().Unix()+3600)
	if m.GogUserID() != "9" {
		t.Fatalf("user = %q", m.GogUserID())
	}

	var out struct {
		Items []struct {
			Key string `json:"achievement_key"`
		} `json:"items"`
	}
	var asked string
	err := m.GogGameGet(t.Context(), "gog-42", func(client string) string {
		asked = client
		return f.srv.URL + "/clients/" + client + "/users/9/achievements"
	}, &out)
	if err != nil || asked != "game-client" || len(out.Items) != 1 || out.Items[0].Key != "first" {
		t.Fatalf("achievements = %+v, client %q, %v", out, asked, err)
	}
	st, err := os.Stat(gogClientsPath())
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("game clients are kept privately: %v", err)
	}

	// A game without Galaxy says so, and is remembered, here and after a restart.
	noGalaxy := func(err error) bool {
		ng, ok := err.(interface{ NoGalaxy() bool })
		return ok && ng.NoGalaxy()
	}
	addr := func(c string) string { return f.srv.URL + "/clients/" + c }
	if err := m.GogGameGet(t.Context(), "gog-43", addr, &out); !noGalaxy(err) {
		t.Fatalf("err = %v", err)
	}
	f.mu.Lock()
	n := len(f.builds)
	f.mu.Unlock()
	m2 := New(m.hist, nil) // after a restart
	signedIn(t, m2, time.Now().Unix()+3600)
	if err := m2.GogGameGet(t.Context(), "gog-43", addr, &out); !noGalaxy(err) {
		t.Fatalf("err = %v", err)
	}
	f.mu.Lock()
	if len(f.builds) != n {
		t.Errorf("asked GOG again: %v", f.builds)
	}
	f.mu.Unlock()

	// The login only goes to GOG.
	for _, bad := range []string{"https://evil.example/x", "https://chat.gog.com.evil.example/x", "http://chat.gog.com/x", "https://u:p@chat.gog.com/x"} {
		if err := m.GogGet(t.Context(), bad, &out); err == nil || !strings.Contains(err.Error(), "not a GOG address") {
			t.Errorf("%s: %v", bad, err)
		}
		if err := m.GogGameGet(t.Context(), "gog-42", func(string) string { return bad }, &out); err == nil {
			t.Errorf("game: %s accepted", bad)
		}
	}
	for _, good := range []string{"https://chat.gog.com/users/9/friends", "https://presence.gog.com/statuses"} {
		if !gogServiceURL(good) {
			t.Errorf("%s refused", good)
		}
	}

	// Signing out forgets the games' tokens.
	m.GogLogout()
	m.gogMu.Lock()
	left := len(m.gogGameToks)
	m.gogMu.Unlock()
	if left != 0 || m.GogUserID() != "" {
		t.Errorf("tokens left: %d", left)
	}
}
