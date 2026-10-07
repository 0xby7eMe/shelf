package epic

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSplitArgs(t *testing.T) {
	cases := map[string][]string{
		``:                       nil,
		`-windowed -nolog`:       {"-windowed", "-nolog"},
		`-name "John Doe" 'a b'`: {"-name", "John Doe", "a b"},
		`a\ b ""`:                {"a b", ""},
		`  spaced   out  `:       {"spaced", "out"},
		`say "he said \"hi\""`:   {"say", `he said "hi"`},
	}
	for in, want := range cases {
		got, err := splitArgs(in)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("splitArgs(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := splitArgs(`"open`); err == nil {
		t.Error("unclosed quote should fail")
	}
}

func TestParseEnv(t *testing.T) {
	got, err := parseEnv("# comment\n\nDXVK_HUD=fps\n PROTON_LOG = 1 \nEMPTY=\n")
	want := []string{"DXVK_HUD=fps", "PROTON_LOG= 1", "EMPTY="}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, %v", got, err)
	}
	for _, bad := range []string{"NOEQUALS", "1BAD=x", "=x", "A B=c"} {
		if _, err := parseEnv(bad); err == nil {
			t.Errorf("%q should fail", bad)
		}
	}
}

func TestChoose(t *testing.T) {
	if !choose(ChoiceDefault, true) || choose(ChoiceDefault, false) || !choose(ChoiceOn, false) || choose(ChoiceOff, true) {
		t.Error("tri-state resolution is wrong")
	}
}

func TestSetConfigValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.ini")
	os.WriteFile(path, []byte("[Legendary]\ndisable_update_check = false\n"), 0o644)

	set := func(section, key, val string) {
		t.Helper()
		if err := setConfigValue(path, section, key, val); err != nil {
			t.Fatal(err)
		}
	}
	set("App.env", "STEAM_COMPAT_DATA_PATH", "/a")
	set("App.env", "STEAM_COMPAT_DATA_PATH", "/b") // replaced, not duplicated
	set("Other.env", "STEAM_COMPAT_DATA_PATH", "/c")
	set("App.env", "OTHER", "1") // added to an existing section

	data, _ := os.ReadFile(path)
	text := string(data)
	if strings.Count(text, "STEAM_COMPAT_DATA_PATH = /b") != 1 || strings.Contains(text, "/a") {
		t.Errorf("not replaced:\n%s", text)
	}
	for _, want := range []string{"[Legendary]", "disable_update_check = false", "[Other.env]", "STEAM_COMPAT_DATA_PATH = /c", "OTHER = 1"} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q:\n%s", want, text)
		}
	}
	if err := setConfigValue(path, "x", "k", "100%"); err == nil {
		t.Error("percent should be refused")
	}
}

func TestVerify(t *testing.T) {
	var p Progress
	if !parseProgress("Verification progress: 12/3000 (41.5%) [88.2 MiB/s]\t", &p) || p.Percent != 41.5 || p.Speed != "88.2 MiB/s" {
		t.Errorf("verify progress: %+v", p)
	}

	state, msg, damaged := verifyVerdict([]string{"[cli] INFO: Verification finished successfully."}, nil)
	if state != StateDone || msg != "" || damaged {
		t.Errorf("success: %s %q %v", state, msg, damaged)
	}
	state, msg, damaged = verifyVerdict([]string{"[cli] ERROR: Verification failed, 2 file(s) corrupted, 5 file(s) are missing."}, nil)
	if state != StateFailed || !damaged || !strings.Contains(msg, "2 corrupted and 5 missing") {
		t.Errorf("damaged: %s %q %v", state, msg, damaged)
	}
	state, _, damaged = verifyVerdict([]string{"[cli] ERROR: Game \"x\" is not installed"}, errors.New("exit 1"))
	if state != StateFailed || damaged {
		t.Errorf("error: %s %v", state, damaged)
	}
	if state, _, _ = verifyVerdict(nil, nil); state != StateFailed {
		t.Error("silence must not count as success")
	}
}

func TestPendingUpdates(t *testing.T) {
	cfg := t.TempDir()
	m := &Manager{cfgDir: cfg}
	os.WriteFile(filepath.Join(cfg, "assets.json"), []byte(`{"Windows":[
		{"app_name":"Old","build_version":"2.0"},
		{"app_name":"Same","build_version":"1.0"}]}`), 0o644)

	installed := map[string]installedGame{
		"Old":     {Version: "1.0", Platform: "Windows"},
		"Same":    {Version: "1.0", Platform: "Windows"},
		"Unknown": {Version: "1.0", Platform: "Windows"}, // no asset info: not an update
	}
	got := m.pendingUpdates(installed)
	if len(got) != 1 || got["Old"] != "2.0" {
		t.Errorf("got %v", got)
	}
}

func TestMancpnAndEgstoreScan(t *testing.T) {
	if mancpnApp([]byte(`{"FormatVersion":0,"AppName":"Sugar"}`)) != "Sugar" || mancpnApp([]byte("nope")) != "" {
		t.Error("mancpn parsing")
	}

	root := t.TempDir()
	game := filepath.Join(root, "SomeGame")
	os.MkdirAll(filepath.Join(game, ".egstore"), 0o755)
	os.WriteFile(filepath.Join(game, ".egstore", "ABC.mancpn"), []byte(`{"AppName":"Sugar"}`), 0o644)
	os.MkdirAll(filepath.Join(root, "NotEpic"), 0o755)

	got := scanEgstore([]string{root, filepath.Join(root, "missing")})
	if len(got) != 1 || got[0].appName != "Sugar" || got[0].path != game {
		t.Errorf("got %+v", got)
	}
}

func TestSettingsKeepDefaultsForOldFiles(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	os.MkdirAll(filepath.Join(cfg, "shelf"), 0o755)
	// A file written before the update settings existed.
	os.WriteFile(filepath.Join(cfg, "shelf", "epic.json"), []byte(`{"installDir":"/games","protonPath":""}`), 0o644)

	s := newSettingsStore().get()
	if s.InstallDir != "/games" || !s.AutoCheckUpdates || s.AutoUpdate || s.CloudSaves {
		t.Errorf("settings: %+v", s)
	}
}

func TestGameSettingsValidate(t *testing.T) {
	ok := GameSettings{CloudSaves: ChoiceOn, AutoUpdate: ChoiceDefault, Env: "A=1", LaunchArgs: `-x "y z"`}
	if err := ok.validate(); err != nil {
		t.Error(err)
	}
	for name, bad := range map[string]GameSettings{
		"choice": {CloudSaves: "maybe", AutoUpdate: ChoiceDefault},
		"env":    {CloudSaves: ChoiceDefault, AutoUpdate: ChoiceDefault, Env: "oops"},
		"args":   {CloudSaves: ChoiceDefault, AutoUpdate: ChoiceDefault, LaunchArgs: `"x`},
		"path":   {CloudSaves: ChoiceDefault, AutoUpdate: ChoiceDefault, SavePath: "relative"},
		"proton": {CloudSaves: ChoiceDefault, AutoUpdate: ChoiceDefault, ProtonPath: "/nonexistent"},
	} {
		if bad.validate() == nil {
			t.Errorf("%s should be rejected", name)
		}
	}
}

func TestPrefixesAndDelete(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", root)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "cfg"))

	big := filepath.Join(prefixRoot(), "Big", "pfx")
	small := filepath.Join(prefixRoot(), "Small")
	os.MkdirAll(big, 0o755)
	os.MkdirAll(small, 0o755)
	os.WriteFile(filepath.Join(big, "data"), make([]byte, 200_000), 0o644)
	os.WriteFile(filepath.Join(small, "data"), make([]byte, 5_000), 0o644)
	os.Symlink("/", filepath.Join(big, "z:")) // must not be followed
	os.MkdirAll(filepath.Join(prefixRoot(), "not a game"), 0o755)

	m := New(nil, nil)
	got := m.Prefixes()
	if len(got) != 2 || got[0].AppName != "Big" || got[0].Bytes < 200_000 || got[0].Bytes > 400_000 || got[1].AppName != "Small" {
		t.Fatalf("prefixes: %+v", got)
	}
	if got[0].Installed {
		t.Error("nothing is installed")
	}

	if err := m.DeletePrefix("../x"); err == nil {
		t.Error("path traversal accepted")
	}
	if err := m.DeletePrefix("Big"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(prefixRoot(), "Big")); !os.IsNotExist(err) {
		t.Error("prefix still there")
	}
}

func TestPosterSize(t *testing.T) {
	if got := posterSize("https://cdn1.epicgames.com/a.jpg"); got != "https://cdn1.epicgames.com/a.jpg?h=600&resize=1&w=400" {
		t.Error(got)
	}
	if got := posterSize(""); got != "" {
		t.Error("empty must stay empty")
	}
	if got := posterSize("https://x/a.jpg?v=1"); got != "https://x/a.jpg?v=1" {
		t.Error("existing query must be left alone")
	}
}
