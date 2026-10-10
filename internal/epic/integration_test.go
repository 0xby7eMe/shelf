package epic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"shelf/internal/library"
)

// fakeLegendary stands in for the real CLI: it answers `list`, "installs" a
// game by writing installed.json, and records how `launch` was invoked.
const fakeLegendary = `#!/bin/sh
case "$1" in
list)
  echo '[{"app_name":"Sugar","app_title":"Sugar Game","metadata":{"keyImages":[{"type":"DieselGameBoxTall","url":"https://x/tall.jpg"},{"type":"DieselGameBox","url":"https://x/wide.jpg"}]}}]'
  ;;
install)
  echo "[DLManager] INFO: = Progress: 50.00% (1/2), Running for 00:00:01, ETA: 00:00:01" >&2
  printf '{"Sugar":{"app_name":"Sugar","title":"Sugar Game","install_path":"%s/Sugar","executable":"g.exe","is_dlc":false}}' "$GAMES" > "$LEGENDARY_CONFIG_PATH/installed.json"
  ;;
launch)
  { echo "ARGS: $*"; env | grep -E '^(STEAM_COMPAT|LEGENDARY)'; } > "$GAMES/launch.out"
  echo "game says hi"
  ;;
esac
`

func TestFakeLegendaryFlow(t *testing.T) {
	linuxOnly(t)
	root := t.TempDir()
	games := filepath.Join(root, "games")
	bin := filepath.Join(root, "bin")
	os.MkdirAll(bin, 0o755)
	os.MkdirAll(games, 0o755)
	os.WriteFile(filepath.Join(bin, "legendary"), []byte(fakeLegendary), 0o755)

	// A fake Proton so launch has something to resolve.
	steam := filepath.Join(root, "home", ".local", "share", "Steam")
	proton := filepath.Join(steam, "compatibilitytools.d", "GE-Proton9-1")
	os.MkdirAll(filepath.Join(steam, "steamapps"), 0o755)
	os.MkdirAll(proton, 0o755)
	os.WriteFile(filepath.Join(proton, "proton"), []byte("#!/bin/sh\n"), 0o755)

	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("USERPROFILE", filepath.Join(root, "home")) // where Windows looks
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "cfg"))
	t.Setenv("APPDATA", filepath.Join(root, "cfg")) // where Windows looks
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "data")) // where Windows looks
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	t.Setenv("GAMES", games)

	m := New(library.NewHistory(), nil)
	if err := m.settings.set(Settings{InstallDir: games}); err != nil {
		t.Fatal(err)
	}

	if m.Account().LoggedIn {
		t.Fatal("should start logged out")
	}
	if g, err := m.Provider().Scan(); err != nil || len(g) != 0 {
		t.Fatalf("logged-out scan: %v %v", g, err)
	}

	os.MkdirAll(m.cfgDir, 0o755)
	os.WriteFile(filepath.Join(m.cfgDir, "user.json"), []byte(`{"displayName":"tester","account_id":"abc"}`), 0o600)
	if acc := m.Account(); !acc.LoggedIn || acc.Name != "tester" || !acc.LegendaryFound {
		t.Fatalf("account: %+v", acc)
	}

	list, err := m.Provider().Scan()
	if err != nil || len(list) != 1 {
		t.Fatalf("scan: %v %v", list, err)
	}
	g := list[0]
	if g.ID != "epic:Sugar" || g.Installed || g.Cover != "https://x/tall.jpg?h=600&resize=1&w=400" || g.Hero != "https://x/wide.jpg" {
		t.Fatalf("game: %+v", g)
	}

	if err := m.Install("Sugar"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for len(m.InstallStates()) > 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	list, _ = m.Provider().Scan()
	if !list[0].Installed || !strings.HasSuffix(list[0].InstallPath, "Sugar") {
		t.Fatalf("after install: %+v", list[0])
	}

	if err := m.Launch("Sugar"); err != nil {
		t.Fatal(err)
	}
	for len(m.Running()) > 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	out, err := os.ReadFile(filepath.Join(games, "launch.out"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"launch Sugar --no-wine --wrapper '" + proton + "/proton' run",
		"STEAM_COMPAT_DATA_PATH=" + filepath.Join(root, "data", "shelf", "prefixes", "Sugar"),
		"STEAM_COMPAT_CLIENT_INSTALL_PATH=" + steam,
		"STEAM_COMPAT_INSTALL_PATH=" + filepath.Join(games, "Sugar"),
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("launch output missing %q:\n%s", want, out)
		}
	}

	// What the game prints reaches the log window.
	logged := false
	for end := time.Now().Add(3 * time.Second); time.Now().Before(end) && !logged; time.Sleep(20 * time.Millisecond) {
		for _, l := range m.Log().Snapshot() {
			if l.Source == "launch" && l.App == "Sugar" && l.Text == "game says hi" {
				logged = true
			}
		}
	}
	if !logged {
		t.Error("game output missing from the log")
	}

	// Per-game options end up on the command line and in the environment.
	gs := defaultGameSettings()
	gs.LaunchArgs = `-windowed -name "A B"`
	gs.Env = "DXVK_HUD=fps\nFOO=bar"
	gs.Offline = true
	if err := m.SetGameSettings("Sugar", gs); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(games, "launch.out"))
	if err := m.Launch("Sugar"); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(5 * time.Second)
	var out2 []byte
	for time.Now().Before(deadline) {
		if out2, _ = os.ReadFile(filepath.Join(games, "launch.out")); len(out2) > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	for _, want := range []string{"--offline -windowed -name A B"} {
		if !strings.Contains(string(out2), want) {
			t.Errorf("launch output missing %q:\n%s", want, out2)
		}
	}
	// A bad setting is refused before it can be saved.
	gs.Env = "broken"
	if m.SetGameSettings("Sugar", gs) == nil {
		t.Error("invalid environment accepted")
	}
}
