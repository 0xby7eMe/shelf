package epic

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"shelf/internal/library"
)

// asMac runs the rest of a test as if on macOS: Epic's Mac builds, no Proton.
func asMac(t *testing.T) {
	t.Helper()
	was, proton := macOS, useProton
	macOS, useProton = true, false
	t.Cleanup(func() { macOS, useProton = was, proton })
}

const fakeMacLegendary = `#!/bin/sh
case "$1" in
list)
  echo '[{"app_name":"Sugar","app_title":"Sugar Game","metadata":{"keyImages":[]}}]'
  ;;
install)
  echo "ARGS: $*" > "$GAMES/install.out"
  printf '{"Sugar":{"app_name":"Sugar","title":"Sugar Game","install_path":"%s/Sugar","executable":"Sugar.app","platform":"Mac"}}' "$GAMES" > "$LEGENDARY_CONFIG_PATH/installed.json"
  ;;
launch)
  { echo "ARGS: $*"; env | grep -E '^STEAM_COMPAT' ; } > "$GAMES/launch.out"
  ;;
esac
`

func TestMacEpicInstallsAndRunsMacBuilds(t *testing.T) {
	unixOnly(t)
	asMac(t)
	root := t.TempDir()
	games := filepath.Join(root, "games")
	bin := filepath.Join(root, "bin")
	os.MkdirAll(bin, 0o755)
	os.MkdirAll(filepath.Join(games, "Sugar"), 0o755)
	os.WriteFile(filepath.Join(bin, "legendary"), []byte(fakeMacLegendary), 0o755)
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("USERPROFILE", filepath.Join(root, "home")) // where Windows looks
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "cfg"))
	t.Setenv("APPDATA", filepath.Join(root, "cfg")) // where Windows looks
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("LOCALAPPDATA", filepath.Join(root, "data")) // where Windows looks
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	t.Setenv("GAMES", games)

	rec := &recorder{}
	m := New(library.NewHistory(), rec.emit)
	m.settings.set(Settings{InstallDir: games})
	os.MkdirAll(m.cfgDir, 0o755)
	os.WriteFile(filepath.Join(m.cfgDir, "user.json"), []byte(`{"displayName":"tester","account_id":"abc"}`), 0o600)

	if err := m.Install("Sugar"); err != nil {
		t.Fatal(err)
	}
	if p := rec.waitFinal(t, KindInstall); p.State != StateDone {
		t.Fatalf("install: %+v", p)
	}
	out, _ := os.ReadFile(filepath.Join(games, "install.out"))
	if !strings.Contains(string(out), "--platform Mac") {
		t.Errorf("installs the Mac build: %s", out)
	}

	// No Proton anywhere, and none needed.
	if info, err := m.LaunchInfo("Sugar"); err != nil || info.Proton != "macOS" {
		t.Errorf("launch info = %+v, %v", info, err)
	}
	if err := m.Launch("Sugar"); err != nil {
		t.Fatal(err)
	}
	var launched string
	for i := 0; i < 200 && launched == ""; i++ {
		time.Sleep(10 * time.Millisecond)
		data, _ := os.ReadFile(filepath.Join(games, "launch.out"))
		launched = string(data)
	}
	if !strings.HasPrefix(launched, "ARGS: launch Sugar") || strings.Contains(launched, "--wrapper") || strings.Contains(launched, "STEAM_COMPAT") {
		t.Errorf("a Mac build starts as it is:\n%s", launched)
	}
	if _, err := os.Stat(prefixDir("Sugar")); !os.IsNotExist(err) {
		t.Error("no Proton prefix on macOS")
	}
	if m.cloudSavesEnabled("Sugar") {
		t.Error("cloud saves are off on macOS")
	}
}

func TestMacRefusesWhatNeedsProton(t *testing.T) {
	asMac(t)
	m := ubiTestManager(t)
	var pe *protonError
	for name, err := range map[string]error{
		"gog install":     m.GogInstall("gog-42"),
		"gog launch":      m.GogLaunch("gog-42"),
		"ubisoft setup":   m.UbisoftSetup(),
		"ubisoft install": m.UbisoftInstall("uplay-1"),
		"ge-proton":       m.InstallProtonGE(),
	} {
		if !errors.As(err, &pe) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if games, err := m.UbisoftProvider().Scan(); games != nil || err != nil {
		t.Errorf("no Ubisoft library on macOS: %v %v", games, err)
	}
	if epicPlatform() != "Mac" {
		t.Error("Epic platform")
	}
}
