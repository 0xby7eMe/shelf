package desktop

import (
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseLaunchURL(t *testing.T) {
	good := map[string]string{
		"shelf://launch/steam:620":          "steam:620",
		"shelf://launch/epic:Fortnite":      "epic:Fortnite",
		"shelf://launch/ubisoft:5059":       "ubisoft:5059",
		"shelf://launch/epic:Some.Game_1-x": "epic:Some.Game_1-x",
	}
	for in, want := range good {
		if got, ok := ParseLaunchURL(in); !ok || got != want {
			t.Errorf("ParseLaunchURL(%q) = %q, %v", in, got, ok)
		}
	}
	for _, in := range []string{
		"", "shelf://launch/", "shelf://launch/steam", "shelf://launch/gog:1",
		"shelf://open/steam:620", "http://launch/steam:620", "shelf://launch/steam:../x",
		"shelf://launch/steam:1/extra", "--flag", "shelf://launch/steam:%20", "shelf://launch/steam:1;rm",
	} {
		if id, ok := ParseLaunchURL(in); ok {
			t.Errorf("ParseLaunchURL(%q) accepted as %q", in, id)
		}
	}
	if id, ok := LaunchFromArgs([]string{"-x", "shelf://nope", "shelf://launch/steam:7"}); !ok || id != "steam:7" {
		t.Errorf("LaunchFromArgs = %q, %v", id, ok)
	}
	if id, _ := ParseLaunchURL(LaunchURL("epic:Abc")); id != "epic:Abc" {
		t.Errorf("round trip failed: %q", id)
	}
}

func TestSettingsValidate(t *testing.T) {
	cases := []struct {
		s  Settings
		ok bool
	}{
		{Settings{}, true},
		{Settings{DiscordClientID: "123456789012345678"}, true},
		{Settings{DiscordEnabled: true, DiscordClientID: "123456789012345678"}, true},
		{Settings{DiscordEnabled: true}, false},
		{Settings{DiscordClientID: "abc"}, false},
		{Settings{DiscordClientID: "12345"}, false},
	}
	for _, c := range cases {
		if err := c.s.Validate(); (err == nil) != c.ok {
			t.Errorf("Validate(%+v) = %v", c.s, err)
		}
	}
}

func TestStorePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop.json")
	a := &Store{path: path}
	want := Settings{MenuEntries: true, Tray: true}
	if err := a.Set(want); err != nil {
		t.Fatal(err)
	}
	if err := a.Set(Settings{DiscordEnabled: true}); err == nil {
		t.Error("invalid settings saved")
	}
	if a.Get() != want {
		t.Errorf("invalid settings changed state: %+v", a.Get())
	}
	raw, _ := os.ReadFile(path)
	var got Settings
	if json.Unmarshal(raw, &got) != nil || got != want {
		t.Errorf("file holds %s", raw)
	}
}

func TestSyncEntries(t *testing.T) {
	dir := t.TempDir()
	other := filepath.Join(dir, "shelf.desktop") // the package's own entry, must survive
	os.WriteFile(other, []byte("[Desktop Entry]\nName=Shelf\n"), 0o644)
	foreign := filepath.Join(dir, "shelf-notes.desktop") // looks like ours but isn't marked
	os.WriteFile(foreign, []byte("[Desktop Entry]\nName=Notes\n"), 0o644)

	exe := `/opt/My "Apps"/shelf`
	games := []Entry{
		{ID: "steam:620", Name: "Portal 2"},
		{ID: "epic:Fortnite", Name: "Line\nbreak $HOME 100%"},
		{ID: "bad id", Name: "Ignored"},
	}
	w, r, err := SyncEntries(dir, exe, games)
	if err != nil || w != 2 || r != 0 {
		t.Fatalf("first sync: %d written, %d removed, %v", w, r, err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "shelf-steam-620.desktop"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{
		"Name=Portal 2\n",
		`Exec="/opt/My \"Apps\"/shelf" "shelf://launch/steam:620"` + "\n",
		"Categories=Game;\n",
		entryMarker,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("entry lacks %q:\n%s", want, s)
		}
	}
	epicRaw, _ := os.ReadFile(filepath.Join(dir, "shelf-epic-Fortnite.desktop"))
	if strings.Count(string(epicRaw), "\n") != strings.Count(s, "\n") {
		t.Errorf("a newline in the game name leaked into the entry:\n%s", epicRaw)
	}

	// Nothing changed: nothing written.
	if w, r, _ := SyncEntries(dir, exe, games); w != 0 || r != 0 {
		t.Errorf("second sync: %d written, %d removed", w, r)
	}

	// A game goes away.
	w, r, _ = SyncEntries(dir, exe, games[:1])
	if w != 0 || r != 1 {
		t.Errorf("after uninstall: %d written, %d removed", w, r)
	}
	if _, err := os.Stat(filepath.Join(dir, "shelf-epic-Fortnite.desktop")); !os.IsNotExist(err) {
		t.Error("stale entry kept")
	}
	for _, p := range []string{other, foreign} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s was touched", filepath.Base(p))
		}
	}

	if n, err := RemoveEntries(dir); err != nil || n != 1 {
		t.Errorf("RemoveEntries = %d, %v", n, err)
	}
}

func TestURLHandlerFile(t *testing.T) {
	dir := t.TempDir()
	if URLHandlerRegistered(dir) {
		t.Fatal("registered before anything was written")
	}
	// A PATH with no helper programs makes the helpers no-ops.
	t.Setenv("PATH", t.TempDir())
	if err := RegisterURLHandler(dir, "/usr/bin/shelf", nil); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, handlerFile))
	for _, want := range []string{`Exec="/usr/bin/shelf" %u`, "MimeType=x-scheme-handler/shelf;", "NoDisplay=true"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("handler lacks %q:\n%s", want, raw)
		}
	}
	if !URLHandlerRegistered(dir) {
		t.Error("not registered after writing")
	}
	// RemoveEntries must not delete the handler.
	RemoveEntries(dir)
	if !URLHandlerRegistered(dir) {
		t.Error("RemoveEntries deleted the handler")
	}
	if err := UnregisterURLHandler(dir, nil); err != nil || URLHandlerRegistered(dir) {
		t.Errorf("unregister: %v", err)
	}
	if err := UnregisterURLHandler(dir, nil); err != nil {
		t.Errorf("unregistering twice: %v", err)
	}
}

// fakeDiscord speaks enough of Discord's IPC for Presence: it answers the
// handshake with READY, then replies to every command and reports it.
func fakeDiscord(t *testing.T, path string, cmds chan<- map[string]any) {
	t.Helper()
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				for {
					var head [8]byte
					if _, err := io.ReadFull(c, head[:]); err != nil {
						return
					}
					op := binary.LittleEndian.Uint32(head[:4])
					body := make([]byte, binary.LittleEndian.Uint32(head[4:]))
					if _, err := io.ReadFull(c, body); err != nil {
						return
					}
					var msg map[string]any
					json.Unmarshal(body, &msg)
					switch op {
					case opHandshake:
						writeFrame(c, opFrame, map[string]any{"cmd": "DISPATCH", "evt": "READY"})
					case opFrame:
						cmds <- msg
						writeFrame(c, opFrame, map[string]any{"cmd": "SET_ACTIVITY", "evt": nil})
					case opClose:
						return
					}
				}
			}()
		}
	}()
}

func TestPresence(t *testing.T) {
	dir, err := os.MkdirTemp("", "ipc") // short path: unix sockets have a length limit
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	path := filepath.Join(dir, "discord-ipc-0")

	p := NewPresence()
	p.sockets = func() []string { return []string{filepath.Join(dir, "discord-ipc-9"), path} }
	t.Cleanup(p.Disable)

	start := time.Now().Unix()
	p.Set(&Activity{Details: "Portal 2", State: "on Steam", Start: start})
	p.Enable("123456789012345678") // Discord isn't up yet: nothing happens, nothing breaks
	if p.Connected() {
		t.Fatal("connected to nothing")
	}

	cmds := make(chan map[string]any, 8)
	fakeDiscord(t, path, cmds)
	p.Enable("123456789012345678") // reconnect now that it is up

	if !p.Connected() {
		t.Fatal("not connected")
	}
	msg := <-cmds
	args := msg["args"].(map[string]any)
	act := args["activity"].(map[string]any)
	if msg["cmd"] != "SET_ACTIVITY" || act["details"] != "Portal 2" || act["state"] != "on Steam" {
		t.Errorf("sent %+v", msg)
	}
	if ts := act["timestamps"].(map[string]any); int64(ts["start"].(float64)) != start {
		t.Errorf("timestamps = %+v", ts)
	}

	p.Set(nil)
	msg = <-cmds
	if _, has := msg["args"].(map[string]any)["activity"]; has {
		t.Errorf("clearing still sent an activity: %+v", msg)
	}
}

func TestSocketCandidates(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	got := socketCandidates()
	if got[0] != "/run/user/1000/discord-ipc-0" {
		t.Errorf("first candidate = %q", got[0])
	}
	found := false
	for _, c := range got {
		if c == "/run/user/1000/app/com.discordapp.Discord/discord-ipc-3" {
			found = true
		}
	}
	if !found {
		t.Error("the Flatpak socket is not tried")
	}
}
