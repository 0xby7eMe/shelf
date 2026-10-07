package epic

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"shelf/internal/library"
)

// names and in string values.
const sampleReg = `WINE REGISTRY Version 2

[Software\\Wow6432Node\\Ubisoft\\Launcher] 1791379270
#time=1dd565eb7e4b7e0
"InstallDir"="C:\\Program Files (x86)\\Ubisoft\\Ubisoft Game Launcher\\"
"Version"="13380"

[Software\\Wow6432Node\\Ubisoft\\Launcher\\Installs] 1791379270
#time=1dd565eb7d7fff0

[Software\\Wow6432Node\\Ubisoft\\Launcher\\Installs\\5487] 1791380000
#time=1dd565eb7d7fff1
"InstallDir"="C:/Program Files (x86)/Ubisoft/Ubisoft Game Launcher/games/Riders Republic/"

[Software\\Wow6432Node\\Ubisoft\\Launcher\\Installs\\720] 1791380001
"InstallDir"="Z:\\home\\max\\Games\\Some \"Game\"\\"

[Software\\Wow6432Node\\Ubisoft\\Launcher\\Installs\\999\\Extra] 1791380002
"InstallDir"="C:\\not\\a\\game\\"

[Software\\Classes\\uplay\\Shell\\Open\\Command] 1791379270
@="\"C:\\Program Files (x86)\\Ubisoft\\Ubisoft Game Launcher\\UbisoftConnect.exe\" \"%1\""
`

func TestParseUbisoftInstalls(t *testing.T) {
	got := parseUbisoftInstalls(strings.NewReader(sampleReg))
	want := map[string]string{
		"5487": "C:/Program Files (x86)/Ubisoft/Ubisoft Game Launcher/games/Riders Republic/",
		"720":  `Z:\home\max\Games\Some "Game"\`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
	if len(parseUbisoftInstalls(strings.NewReader(""))) != 0 {
		t.Error("empty registry should give no games")
	}
}

func TestWinToUnix(t *testing.T) {
	pfx := t.TempDir()
	game := filepath.Join(pfx, "drive_c", "Program Files (x86)", "Ubisoft", "Ubisoft Game Launcher", "games", "Riders Republic")
	os.MkdirAll(game, 0o755)
	other := t.TempDir()
	os.MkdirAll(filepath.Join(other, "Games", "Mixed Case"), 0o755)
	os.MkdirAll(filepath.Join(pfx, "dosdevices"), 0o755)
	os.Symlink(other, filepath.Join(pfx, "dosdevices", "d:"))

	cases := map[string]string{
		`C:\Program Files (x86)\Ubisoft\Ubisoft Game Launcher\games\Riders Republic\`: game,
		`c:/program files (x86)/ubisoft/ubisoft game launcher/games/riders republic/`: game, // case differs from disk
		`D:\Games\mixed case`: filepath.Join(other, "Games", "Mixed Case"),
		`Z:\usr\bin`:          "/usr/bin",
		`C:\not\there\yet`:    filepath.Join(pfx, "drive_c", "not", "there", "yet"),
		`E:\unmapped`:         "",
		`relative\path`:       "",
	}
	for in, want := range cases {
		if got := winToUnix(pfx, in); got != want {
			t.Errorf("winToUnix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUbisoftGameFor(t *testing.T) {
	folders := map[string]string{"riders republic": "Riders"}
	cases := []struct{ cmd, want string }{
		{`C:\Program Files (x86)\Ubisoft\Ubisoft Game Launcher\games\Riders Republic\RidersRepublic.exe`, "Riders"},
		{`Z:\home\max\Games\Riders Republic\bin\game.exe -arg`, "Riders"},
		{`C:\Program Files (x86)\Ubisoft\Ubisoft Game Launcher\upc.exe -upc_desktop_mode`, ""},
		{`vim /home/max/Games/Riders Republic/notes.txt`, ""}, // not a Windows program
		{`C:\Games\Riders Republic Fan Site\x.exe`, ""},       // a different folder with the same start
	}
	for _, c := range cases {
		if got := ubisoftGameFor(c.cmd, folders); got != c.want {
			t.Errorf("ubisoftGameFor(%q) = %q, want %q", c.cmd, got, c.want)
		}
	}
}

func TestConnectEnv(t *testing.T) {
	build := ProtonBuild{Name: "GE", Path: "/p"}
	soft := strings.Join(connectEnv(build, "/games/x", true), "\n")
	game := strings.Join(connectEnv(build, "/games/x", false), "\n")
	if !strings.Contains(soft, "PROTON_NO_D3D11=1") || !strings.Contains(soft, "PROTON_USE_XALIA=0") {
		t.Errorf("software rendering env: %s", soft)
	}
	if strings.Contains(game, "PROTON_NO_D3D11") || !strings.Contains(game, "PROTON_USE_XALIA=0") {
		t.Errorf("game env must keep Direct3D 11: %s", game)
	}
	if !strings.Contains(game, "STEAM_COMPAT_INSTALL_PATH=/games/x") {
		t.Errorf("install path missing: %s", game)
	}
}

func TestStopPrefixProcesses(t *testing.T) {
	prefix := filepath.Join(t.TempDir(), "prefix")
	mine := exec.Command("sleep", "30")
	mine.Env = append(os.Environ(), "STEAM_COMPAT_DATA_PATH="+prefix)
	if err := mine.Start(); err != nil {
		t.Fatal(err)
	}
	other := exec.Command("sleep", "30") // somewhere else: must survive
	other.Env = append(os.Environ(), "STEAM_COMPAT_DATA_PATH="+prefix+"-other")
	if err := other.Start(); err != nil {
		t.Fatal(err)
	}
	defer other.Process.Kill()
	reaped := make(chan struct{})
	go func() { mine.Wait(); close(reaped) }()

	if n := stopPrefixProcesses(prefix); n != 1 {
		t.Errorf("stopped %d processes, want 1", n)
	}
	select {
	case <-reaped:
	case <-time.After(5 * time.Second):
		t.Error("the process in the prefix is still running")
	}
	if other.ProcessState != nil {
		t.Error("a process in a different prefix was stopped")
	}
	if n := stopPrefixProcesses(prefix); n != 0 {
		t.Errorf("nothing should be left, found %d", n)
	}
}

func TestUbisoftRenderingDefault(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	os.MkdirAll(filepath.Join(cfg, "shelf"), 0o755)
	os.WriteFile(filepath.Join(cfg, "shelf", "epic.json"), []byte(`{"installDir":"/g"}`), 0o644)
	if !newSettingsStore().get().UbisoftSoftwareRendering {
		t.Error("software rendering should be on for settings saved before the option existed")
	}
}

// --- Connect's files ---

func varint(v uint64) []byte {
	var b []byte
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

// record encodes one top-level record the way Connect writes them.
func record(install, launch uint64, tag byte, body string) []byte {
	var msg []byte
	msg = append(msg, 0x08)
	msg = append(msg, varint(install)...)
	msg = append(msg, 0x10)
	msg = append(msg, varint(launch)...)
	msg = append(msg, tag)
	msg = append(msg, varint(uint64(len(body)))...)
	msg = append(msg, body...)
	out := append([]byte{0x0a}, varint(uint64(len(msg)))...)
	return append(out, msg...)
}

func gameYAML(name string, extra string) string {
	return "version: 2.0\nroot:\n  name: " + name + "\n  start_game:\n    online:\n      executables: []\n" + extra
}

func configFile() []byte {
	var f []byte
	f = append(f, record(5487, 5487, 0x1a, gameYAML("Riders Republic", ""))...)
	f = append(f, record(720, 4000, 0x1a, gameYAML(`"Far Cry 3"`, ""))...) // launch differs from install
	f = append(f, record(11, 11, 0x1a, gameYAML("Steam Thing", "  third_party_platform:\n    name: steam\n"))...)
	f = append(f, record(12, 12, 0x1a, gameYAML("Not Mine", ""))...) // in the catalog, not owned
	f = append(f, record(13, 13, 0x1a, gameYAML("NAME", "  installer:\n    game_identifier: Fallback Name\n"))...)
	f = append(f, record(15, 15, 0x1a, "version: 2.0\r\nroot:\r\n  name: l1\r\n  thumb_image: l2\r\n  background_image: bg.jpg\r\n  start_game:\r\n    online: {}\r\n"+
		"  uplay:\r\n    thumb_image: nested.jpg\r\nlocalizations:\r\n  default:\r\n    l1: Localized Game\r\n    l2: poster.png\r\n  de-DE:\r\n    l2: de.png\r\n")...)
	f = append(f, record(14, 14, 0x1a, "version: 2.0\nroot:\n  name: No Start\n")...) // not a game
	return f
}

func ownershipFileData(ids ...uint64) []byte {
	f := make([]byte, ownershipHeader)
	for _, id := range ids {
		f = append(f, record(id, id, 0x22, "ownership")...)
	}
	return f
}

func TestParseUbiConfigurations(t *testing.T) {
	// Junk before and between records must not hide the games.
	data := append([]byte("\x01\x02header\x00"), configFile()...)
	got := parseUbiConfigurations(data)
	var names []string
	for _, g := range got {
		names = append(names, g.Name)
	}
	want := []string{"Riders Republic", "Far Cry 3", "Steam Thing", "Not Mine", "Fallback Name", "Localized Game"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("names %v, want %v", names, want)
	}
	if got[1].InstallID != 720 || got[1].LaunchID != 4000 || got[1].Platform != "" {
		t.Errorf("ids: %+v", got[1])
	}
	art := got[5]
	if art.Cover != ubiAssets+"poster.png" || art.Hero != ubiAssets+"bg.jpg" {
		t.Errorf("art must come from the root fields and the default language: %+v", art)
	}
	if got[0].Cover != "" || got[0].Hero != "" {
		t.Errorf("a game without art has none: %+v", got[0])
	}
	if got[2].Platform != "Steam" {
		t.Errorf("a Steam game must say so: %+v", got[2])
	}
	if len(parseUbiConfigurations(nil)) != 0 || len(parseUbiConfigurations([]byte("garbage"))) != 0 {
		t.Error("no records should give no games")
	}
}

func TestParseUbiOwnership(t *testing.T) {
	got := parseUbiOwnership(ownershipFileData(5487, 4000, 300))
	if !got[5487] || !got[4000] || !got[300] || got[12] || len(got) != 3 {
		t.Errorf("owned: %v", got)
	}
	if len(parseUbiOwnership(make([]byte, 10))) != 0 {
		t.Error("a short file has no records")
	}
}

// --- Connect: setup and playing, against a fake Proton ---

const fakeProton = `#!/bin/sh
echo "proton $* [prefix=$STEAM_COMPAT_DATA_PATH] d3d11=$PROTON_NO_D3D11 xalia=$PROTON_USE_XALIA" >> "$GAMES/proton.log"
case "$2" in
*Installer*)
  d="$STEAM_COMPAT_DATA_PATH/pfx/drive_c/Program Files (x86)/Ubisoft/Ubisoft Game Launcher"
  mkdir -p "$d" && : > "$d/UbisoftConnect.exe"
  echo "installing quietly"
  ;;
esac
`

func ubiTestManager(t *testing.T) *Manager {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "cfg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	return New(library.NewHistory(), nil)
}

func ubisoftEnv(t *testing.T) (*Manager, string) {
	t.Helper()
	m := ubiTestManager(t)
	games := filepath.Join(t.TempDir(), "games")
	os.MkdirAll(games, 0o755)
	t.Setenv("GAMES", games)

	home, _ := os.UserHomeDir()
	steam := filepath.Join(home, ".local", "share", "Steam")
	proton := filepath.Join(steam, "compatibilitytools.d", "GE-Proton9-1")
	os.MkdirAll(filepath.Join(steam, "steamapps"), 0o755)
	os.MkdirAll(proton, 0o755)
	os.WriteFile(filepath.Join(proton, "proton"), []byte(fakeProton), 0o755)
	return m, games
}

func waitSetup(t *testing.T, m *Manager) UbisoftSetupState {
	t.Helper()
	for i := 0; i < 250; i++ {
		if st := m.UbisoftSetupState(); st.State == "done" || st.State == "failed" {
			return st
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("setup didn't finish")
	return UbisoftSetupState{}
}

func TestUbisoftSetup(t *testing.T) {
	m, games := ubisoftEnv(t)

	installer := append([]byte("MZ"), make([]byte, 64)...)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(installer) }))
	defer srv.Close()
	oldURL, oldMin := ubisoftInstallerURL, minInstallerSize
	ubisoftInstallerURL, minInstallerSize = srv.URL+"/UbisoftConnectInstaller.exe", 10
	defer func() { ubisoftInstallerURL, minInstallerSize = oldURL, oldMin }()

	if m.UbisoftStatus().ConnectInstalled {
		t.Fatal("nothing is installed yet")
	}
	if err := m.UbisoftSetup(); err != nil {
		t.Fatal(err)
	}
	if st := waitSetup(t, m); st.State != "done" {
		t.Fatalf("setup: %+v", st)
	}
	if st := m.UbisoftStatus(); !st.ConnectInstalled || st.Proton != "GE-Proton9-1" {
		t.Errorf("status: %+v", st)
	}
	calls, _ := os.ReadFile(filepath.Join(games, "proton.log"))
	if !strings.Contains(string(calls), "run "+installerCache()+" /S") ||
		!strings.Contains(string(calls), "prefix="+ubisoftPrefix()) {
		t.Errorf("installer not run in the shared prefix:\n%s", calls)
	}
}

func TestUbisoftSetupRejectsBadDownloads(t *testing.T) {
	m, _ := ubisoftEnv(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("<html>not found</html>"))
	}))
	defer srv.Close()
	oldURL, oldMin := ubisoftInstallerURL, minInstallerSize
	ubisoftInstallerURL, minInstallerSize = srv.URL, 10
	defer func() { ubisoftInstallerURL, minInstallerSize = oldURL, oldMin }()

	if err := m.UbisoftSetup(); err != nil {
		t.Fatal(err)
	}
	if st := waitSetup(t, m); st.State != "failed" || !strings.Contains(st.Error, "isn't a Windows installer") {
		t.Fatalf("an error page must not be run as an installer: %+v", st)
	}
	if connectInstalled() {
		t.Error("nothing should have been installed")
	}
}

// writeConnect puts Connect in the state it has once an account signed in.
func writeConnect(t *testing.T, owned ...uint64) {
	t.Helper()
	data := filepath.Join(ubisoftPrefix(), "pfx", "drive_c", "users", "steamuser", "AppData", "Local", "Ubisoft Game Launcher")
	os.MkdirAll(filepath.Join(data, "cache", "configuration"), 0o755)
	os.WriteFile(filepath.Join(data, "cache", "configuration", "configurations"), configFile(), 0o644)
	dir := filepath.Join(data, "cache", "ownership")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "acct-1"), ownershipFileData(owned...), 0o644)
}

func TestUbisoftLibrary(t *testing.T) {
	m, _ := ubisoftEnv(t)

	if g, err := m.UbisoftProvider().Scan(); err != nil || len(g) != 0 {
		t.Fatalf("without Connect: %v %v", g, err)
	}
	os.MkdirAll(connectDir(), 0o755)
	os.WriteFile(connectExe(), nil, 0o644)
	if st := m.UbisoftStatus(); !st.ConnectInstalled || st.SignedIn {
		t.Fatalf("before sign-in: %+v", st)
	}
	if err := m.UbisoftSync(); err == nil || !strings.Contains(err.Error(), "sign in") {
		t.Fatalf("sync before sign-in: %v", err)
	}

	writeConnect(t, 5487, 4000, 13)
	if err := m.UbisoftSync(); err != nil {
		t.Fatal(err)
	}
	if st := m.UbisoftStatus(); !st.SignedIn || st.Account != "acct-1" || st.Games != 3 {
		t.Fatalf("after sign-in: %+v", st)
	}
	games, err := m.UbisoftProvider().Scan()
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, g := range games {
		ids = append(ids, g.ID+"="+g.Name)
		if g.Source != library.SourceUbisoft || g.Installed {
			t.Errorf("game: %+v", g)
		}
	}
	want := []string{"ubisoft:uplay-13=Fallback Name", "ubisoft:uplay-720=Far Cry 3", "ubisoft:uplay-5487=Riders Republic"}
	if len(ids) != 3 {
		t.Fatalf("library %v", ids)
	}
	if !reflect.DeepEqual(ids, want) {
		t.Errorf("library %v, want %v", ids, want)
	}

	// A new account signing in replaces the library.
	time.Sleep(10 * time.Millisecond)
	dir := filepath.Join(connectDataDir(), "cache", "ownership")
	os.WriteFile(filepath.Join(dir, "acct-2"), ownershipFileData(5487), 0o644)
	if g, _ := m.UbisoftProvider().Scan(); len(g) != 1 || g[0].Name != "Riders Republic" {
		t.Errorf("second account: %+v", g)
	}
	if _, err := m.UbisoftStoreURL("uplay-5487"); err != nil {
		t.Error(err)
	}
	if _, err := m.UbisoftStoreURL("uplay-720"); err == nil {
		t.Error("a game the account doesn't own has no page")
	}
}

func TestUbisoftLaunchAndInstall(t *testing.T) {
	m, games := ubisoftEnv(t)
	key := "uplay-5487"

	if err := m.UbisoftLaunch(key); err == nil {
		t.Fatal("an unknown game must not launch")
	}
	os.MkdirAll(connectDir(), 0o755)
	os.WriteFile(connectExe(), nil, 0o644)
	writeConnect(t, 5487, 4000)
	if err := m.UbisoftLaunch(key); err == nil || !strings.Contains(err.Error(), "isn't installed yet") {
		t.Fatalf("without the game: %v", err)
	}
	defer stopPrefixProcesses(ubisoftPrefix())

	if err := m.UbisoftInstall(key); err != nil {
		t.Fatal(err)
	}
	waitFor(t, games, "run start uplay://install/5487")

	pfx := filepath.Join(ubisoftPrefix(), "pfx")
	gameDir := filepath.Join(pfx, "drive_c", "Program Files (x86)", "Ubisoft", "Ubisoft Game Launcher", "games", "Riders Republic")
	os.MkdirAll(gameDir, 0o755)
	os.WriteFile(filepath.Join(pfx, "system.reg"), []byte(sampleReg), 0o644)

	g, _ := m.UbisoftProvider().Scan()
	var found bool
	for _, x := range g {
		if x.ExternalID == key {
			found = x.Installed && x.InstallPath == gameDir
		}
	}
	if !found {
		t.Fatalf("scan after install: %+v", g)
	}
	if err := m.UbisoftLaunch(key); err != nil {
		t.Fatal(err)
	}
	// A game must never run with Direct3D 11 switched off.
	calls := waitFor(t, games, "run start uplay://launch/5487/0")
	if !strings.Contains(calls, "prefix="+ubisoftPrefix()) || !strings.Contains(calls, "d3d11= xalia=0") {
		t.Errorf("proton wasn't asked to launch in the Ubisoft prefix:\n%s", calls)
	}
}

func waitFor(t *testing.T, games, want string) string {
	t.Helper()
	var calls string
	for i := 0; i < 150; i++ {
		b, _ := os.ReadFile(filepath.Join(games, "proton.log"))
		calls = string(b)
		if strings.Contains(calls, want) {
			return calls
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("never saw %q in:\n%s", want, calls)
	return ""
}

// Older Connect versions kept their records next to the program.
func TestConnectDataDirLegacy(t *testing.T) {
	ubisoftEnv(t)
	legacy := filepath.Join(connectDir(), "cache", "configuration")
	os.MkdirAll(legacy, 0o755)
	os.WriteFile(filepath.Join(legacy, "configurations"), configFile(), 0o644)
	if got := connectDataDir(); got != connectDir() {
		t.Errorf("legacy data dir %q", got)
	}
	modern := filepath.Join(ubisoftPrefix(), "pfx", "drive_c", "users", "u", "AppData", "Local", "Ubisoft Game Launcher", "cache", "configuration")
	os.MkdirAll(modern, 0o755)
	os.WriteFile(filepath.Join(modern, "configurations"), configFile(), 0o644)
	if got := connectDataDir(); !strings.Contains(got, "AppData") {
		t.Errorf("the AppData records should win: %q", got)
	}
}

// A Steam game is listed, but Connect can't install or start it.
func TestUbisoftSteamGame(t *testing.T) {
	m, _ := ubisoftEnv(t)
	os.MkdirAll(connectDir(), 0o755)
	os.WriteFile(connectExe(), nil, 0o644)
	writeConnect(t, 11, 5487)
	games, _ := m.UbisoftProvider().Scan()
	var steam *library.Game
	for i := range games {
		if games[i].ExternalID == "uplay-11" {
			steam = &games[i]
		}
	}
	if steam == nil || steam.ThirdParty != "Steam" {
		t.Fatalf("steam game: %+v", games)
	}
	if err := m.UbisoftInstall("uplay-11"); err == nil || !strings.Contains(err.Error(), "Steam") {
		t.Errorf("install: %v", err)
	}
	if err := m.UbisoftLaunch("uplay-11"); err == nil || !strings.Contains(err.Error(), "Steam") {
		t.Errorf("launch: %v", err)
	}
}
