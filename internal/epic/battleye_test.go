package epic

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ulikunitz/xz"
)

func TestNeedsBattlEye(t *testing.T) {
	mk := func(files ...string) string {
		dir := t.TempDir()
		for _, f := range files {
			p := filepath.Join(dir, f)
			if strings.HasSuffix(f, "/") {
				os.MkdirAll(p, 0o755)
			} else {
				os.WriteFile(p, nil, 0o644)
			}
		}
		return dir
	}
	cases := map[string]struct {
		dir  string
		want bool
	}{
		"folder":        {mk("BattlEye/", "Game.exe"), true},
		"folder case":   {mk("battleye/"), true},
		"service file":  {mk("BEService_x64.exe"), true},
		"client file":   {mk("BEClient_x64.dll"), true},
		"plain game":    {mk("Game.exe", "data/"), false},
		"nested only":   {mk("data/BattlEye/"), false},
		"file named be": {mk("battleye.txt"), false},
		"empty":         {mk(), false},
		"missing dir":   {filepath.Join(t.TempDir(), "nope"), false},
		"no dir":        {"", false},
	}
	for name, c := range cases {
		if got := needsBattlEye(c.dir); got != c.want {
			t.Errorf("%s: needsBattlEye = %v, want %v", name, got, c.want)
		}
	}
	if antiCheatOf(cases["folder"].dir) != "BattlEye" || antiCheatOf(cases["plain game"].dir) != "" {
		t.Error("antiCheatOf should name BattlEye, and nothing otherwise")
	}
}

func fakeRuntimeDir(t *testing.T, dir string) {
	t.Helper()
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "BEClient_x64.so"), []byte("so"), 0o644)
}

func TestFindBattlEyeRuntime(t *testing.T) {
	ubiTestManager(t)
	home, _ := os.UserHomeDir()
	os.MkdirAll(filepath.Join(home, ".local", "share", "Steam", "steamapps"), 0o755)

	if rt := findBattlEyeRuntime(); rt.Installed {
		t.Fatalf("nothing is installed yet: %+v", rt)
	}

	// An incomplete folder doesn't count.
	os.MkdirAll(battleyeShelfDir(), 0o755)
	if findBattlEyeRuntime().Installed {
		t.Error("a folder without the client isn't the runtime")
	}

	fakeRuntimeDir(t, battleyeShelfDir())
	if rt := findBattlEyeRuntime(); !rt.Installed || rt.Source != "shelf" || rt.Path != battleyeShelfDir() {
		t.Errorf("shelf copy: %+v", rt)
	}

	// Steam's own copy wins.
	steamCopy := filepath.Join(home, ".local", "share", "Steam", "steamapps", "common", "Proton BattlEye Runtime")
	fakeRuntimeDir(t, steamCopy)
	if rt := findBattlEyeRuntime(); rt.Source != "steam" || rt.Path != steamCopy {
		t.Errorf("steam copy should win: %+v", rt)
	}
}

func battleyeArchive(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	xw, err := xz.NewWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	tw := tar.NewWriter(xw)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Typeflag: tar.TypeReg, Mode: 0o644, Size: int64(len(body))}); err != nil {
			t.Fatal(err)
		}
		tw.Write([]byte(body))
	}
	tw.Close()
	xw.Close()
	return buf.Bytes()
}

func serveBattlEye(t *testing.T, archive []byte, sum string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(archive) }))
	oldURL, oldSum := battleyeURL, battleyeSHA256
	battleyeURL, battleyeSHA256 = srv.URL+"/battleeye_runtime.tar.xz", sum
	t.Cleanup(func() { battleyeURL, battleyeSHA256 = oldURL, oldSum; srv.Close() })
}

func sumOf(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func waitBattlEye(t *testing.T, m *Manager) ProtonInstallState {
	t.Helper()
	for i := 0; i < 250; i++ {
		if st := m.BattlEyeInstallState(); st.State == "done" || st.State == "failed" {
			return st
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("install didn't finish")
	return ProtonInstallState{}
}

func TestInstallBattlEyeRuntime(t *testing.T) {
	linuxOnly(t)
	m := ubiTestManager(t)
	archive := battleyeArchive(t, map[string]string{
		battleyeArchiveDir + "/BEClient_x64.so":                   "so64",
		battleyeArchiveDir + "/BEClient.so":                       "so32",
		battleyeArchiveDir + "/v1/lib64/wine/beclient_x64.dll.so": "dll",
	})
	serveBattlEye(t, archive, sumOf(archive))

	if err := m.InstallBattlEyeRuntime(); err != nil {
		t.Fatal(err)
	}
	if st := waitBattlEye(t, m); st.State != "done" {
		t.Fatalf("install: %+v", st)
	}
	rt := m.BattlEyeRuntime()
	if !rt.Installed || rt.Source != "shelf" {
		t.Fatalf("runtime: %+v", rt)
	}
	if b, _ := os.ReadFile(filepath.Join(rt.Path, "v1", "lib64", "wine", "beclient_x64.dll.so")); string(b) != "dll" {
		t.Error("the nested files must be unpacked too")
	}
	if entries, _ := os.ReadDir(filepath.Dir(rt.Path)); len(entries) != 1 {
		t.Errorf("only the runtime should be left: %v", entries)
	}

	// Installing again replaces it.
	if err := m.InstallBattlEyeRuntime(); err != nil {
		t.Fatal(err)
	}
	if st := waitBattlEye(t, m); st.State != "done" {
		t.Fatalf("reinstall: %+v", st)
	}
}

func TestInstallBattlEyeRuntimeRejectsBadDownloads(t *testing.T) {
	linuxOnly(t)
	good := battleyeArchive(t, map[string]string{battleyeArchiveDir + "/BEClient_x64.so": "x"})
	cases := map[string]struct {
		archive []byte
		sum     string
		want    string
	}{
		"wrong hash":      {good, strings.Repeat("0", 64), "checksum doesn't match"},
		"not the runtime": {battleyeArchive(t, map[string]string{battleyeArchiveDir + "/readme": "x"}), "", "doesn't contain the BattlEye runtime"},
		"not an archive":  {[]byte("<html>error</html>"), "", "couldn't unpack"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			m := ubiTestManager(t)
			sum := c.sum
			if sum == "" {
				sum = sumOf(c.archive)
			}
			serveBattlEye(t, c.archive, sum)
			if err := m.InstallBattlEyeRuntime(); err != nil {
				t.Fatal(err)
			}
			st := waitBattlEye(t, m)
			if st.State != "failed" || !strings.Contains(st.Error, c.want) {
				t.Fatalf("want failure containing %q, got %+v", c.want, st)
			}
			if m.BattlEyeRuntime().Installed {
				t.Error("a bad download must not leave a runtime behind")
			}
		})
	}
	_ = good
}

func TestBattlEyeEnvFor(t *testing.T) {
	linuxOnly(t)
	ubiTestManager(t)
	home, _ := os.UserHomeDir()
	os.MkdirAll(filepath.Join(home, ".local", "share", "Steam", "steamapps"), 0o755)

	plain := t.TempDir()
	if env, err := battleyeEnvFor(plain, "Plain"); env != nil || err != nil {
		t.Errorf("a game without BattlEye needs nothing: %v %v", env, err)
	}

	be := t.TempDir()
	os.MkdirAll(filepath.Join(be, "BattlEye"), 0o755)
	if _, err := battleyeEnvFor(be, "Riders Republic"); err == nil || !strings.Contains(err.Error(), "Proton BattlEye Runtime") {
		t.Errorf("without the runtime: %v", err)
	}
	fakeRuntimeDir(t, battleyeShelfDir())
	env, err := battleyeEnvFor(be, "Riders Republic")
	if err != nil || len(env) != 1 || env[0] != battleyeEnv+"="+battleyeShelfDir() {
		t.Errorf("with the runtime: %v %v", env, err)
	}
}

// Playing a BattlEye game through Connect needs the runtime, and hands it to Proton.
func TestUbisoftLaunchBattlEye(t *testing.T) {
	linuxOnly(t)
	m, _, games, gameDir := installedEnv(t, true)
	defer stopPrefixProcesses(ubisoftPrefix())
	os.MkdirAll(filepath.Join(gameDir, "BattlEye"), 0o755)

	if g, _ := m.UbisoftProvider().Scan(); true {
		var flagged bool
		for _, x := range g {
			flagged = flagged || (x.ExternalID == "uplay-5487" && x.AntiCheat == "BattlEye")
		}
		if !flagged {
			t.Errorf("the library should flag the game as BattlEye: %+v", g)
		}
	}

	if err := m.UbisoftLaunch("uplay-5487"); err == nil || !strings.Contains(err.Error(), "BattlEye") {
		t.Fatalf("without the runtime: %v", err)
	}

	fakeRuntimeDir(t, battleyeShelfDir())
	if err := m.UbisoftLaunch("uplay-5487"); err != nil {
		t.Fatal(err)
	}
	calls := waitFor(t, games, "run start uplay://launch/5487/0")
	if !strings.Contains(calls, "be="+battleyeShelfDir()) {
		t.Errorf("Proton wasn't given the runtime:\n%s", calls)
	}
}

// A game without BattlEye is left as it was.
func TestUbisoftLaunchWithoutBattlEyeGetsNoRuntime(t *testing.T) {
	linuxOnly(t)
	m, _, games, _ := installedEnv(t, true)
	defer stopPrefixProcesses(ubisoftPrefix())
	fakeRuntimeDir(t, battleyeShelfDir())

	if err := m.UbisoftLaunch("uplay-5487"); err != nil {
		t.Fatal(err)
	}
	calls := waitFor(t, games, "run start uplay://launch/5487/0")
	if !strings.Contains(calls, "be=\n") && !strings.HasSuffix(strings.TrimSpace(calls), "be=") {
		t.Errorf("the runtime must only reach BattlEye games:\n%s", calls)
	}
}
