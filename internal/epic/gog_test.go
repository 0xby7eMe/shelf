package epic

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"shelf/internal/library"
)

func TestExtractGogCode(t *testing.T) {
	code := "Abc123-_xyzXYZ0987654321"
	for _, in := range []string{
		code,
		" " + code + "\n",
		`"` + code + `"`,
		"https://embed.gog.com/on_login_success?origin=client&code=" + code,
	} {
		got, err := ExtractGogCode(in)
		if err != nil || got != code {
			t.Errorf("ExtractGogCode(%q) = %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "short", "https://embed.gog.com/on_login_success?origin=client", "has spaces in it 1234567890"} {
		if _, err := ExtractGogCode(in); err == nil {
			t.Errorf("ExtractGogCode(%q) should fail", in)
		}
	}
}

func TestPickGogInstaller(t *testing.T) {
	file := []struct {
		ID       string `json:"id"`
		Size     int64  `json:"size"`
		Downlink string `json:"downlink"`
	}{{ID: "f"}}
	list := []gogInstaller{
		{ID: "mac", OS: "mac", Language: "en", Files: file},
		{ID: "win-de", OS: "windows", Language: "de", Files: file},
		{ID: "win-en", OS: "windows", Language: "en", Files: file},
	}
	if in, ok := pickGogInstaller(list); !ok || in.ID != "win-en" {
		t.Errorf("picked %+v", in)
	}
	if in, ok := pickGogInstaller(list[:2]); !ok || in.ID != "win-de" {
		t.Errorf("without English, any Windows installer: %+v", in)
	}
	if _, ok := pickGogInstaller(list[:1]); ok {
		t.Error("a Mac installer is no use")
	}
}

func TestGogFolderAndPaths(t *testing.T) {
	if got := gogFolderName("The Witcher: Enhanced Edition", "gog-1"); got != "The Witcher Enhanced Edition" {
		t.Errorf("folder = %q", got)
	}
	if got := gogFolderName("../..", "gog-1"); got != "gog-1" {
		t.Errorf("folder = %q", got)
	}
	if got := winPath("/home/a b/Games"); got != `Z:\home\a b\Games` {
		t.Errorf("winPath = %q", got)
	}
}

func TestGogTarget(t *testing.T) {
	dir := t.TempDir()
	write := func(info string) {
		os.WriteFile(gogInfoFile(dir, "gog-7"), []byte(info), 0o644)
	}

	write(`{"playTasks":[
		{"category":"tool","type":"FileTask","path":"tools\\cfg.exe"},
		{"type":"URLTask","link":"https://example.com","isPrimary":true},
		{"category":"game","type":"FileTask","path":"bin\\game.exe","arguments":"-a \"b c\"","workingDir":"bin"}]}`)
	got, err := gogTarget(dir, "gog-7")
	if err != nil {
		t.Fatal(err)
	}
	if got.Exe != filepath.Join(dir, "bin", "game.exe") || got.Dir != filepath.Join(dir, "bin") {
		t.Errorf("target = %+v", got)
	}
	if strings.Join(got.Args, "|") != "-a|b c" {
		t.Errorf("args = %q", got.Args)
	}

	write(`{"playTasks":[{"category":"game","type":"FileTask","path":"other.exe"},{"isPrimary":true,"type":"FileTask","path":"main.exe"}]}`)
	if got, _ := gogTarget(dir, "gog-7"); got.Exe != filepath.Join(dir, "main.exe") || got.Dir != dir {
		t.Errorf("the primary task wins: %+v", got)
	}

	for _, bad := range []string{`..\\..\\evil.exe`, `C:\\Windows\\evil.exe`, `/bin/sh`} {
		write(fmt.Sprintf(`{"playTasks":[{"isPrimary":true,"type":"FileTask","path":%q}]}`, strings.ReplaceAll(bad, `\\`, `\`)))
		if _, err := gogTarget(dir, "gog-7"); err == nil {
			t.Errorf("path %s should be refused", bad)
		}
	}
}

// --- a fake GOG, for the login, the library and installing ---

type fakeGog struct {
	srv *httptest.Server

	mu      sync.Mutex
	grants  []string // grant_type of each token request
	ranges  []string // Range headers on downloads
	badCode bool
	setup   []byte
	bin     []byte
	md5     map[string]string // file name -> checksum served
}

func newFakeGog(t *testing.T) *fakeGog {
	t.Helper()
	f := &fakeGog{
		setup: bytes.Repeat([]byte("MZsetup"), 4000),
		bin:   bytes.Repeat([]byte("data"), 9000),
		md5:   map[string]string{},
	}
	for name, data := range map[string][]byte{"setup_test game.exe": f.setup, "setup_test game-1.bin": f.bin} {
		sum := md5.Sum(data)
		f.md5[name] = hex.EncodeToString(sum[:])
	}

	mux := http.NewServeMux()
	auth := func(w http.ResponseWriter, r *http.Request) bool {
		if r.Header.Get("Authorization") != "Bearer access-1" && r.Header.Get("Authorization") != "Bearer access-2" {
			http.Error(w, "no", http.StatusUnauthorized)
			return false
		}
		return true
	}
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		f.mu.Lock()
		f.grants = append(f.grants, q.Get("grant_type"))
		bad := f.badCode
		f.mu.Unlock()
		if q.Get("client_id") != gogClientID || q.Get("client_secret") != gogClientSecret {
			http.Error(w, `{"error":"invalid_client"}`, http.StatusUnauthorized)
			return
		}
		switch {
		case bad:
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":"invalid_grant","error_description":"Code doesn't exist"}`)
		case q.Get("grant_type") == "authorization_code" && q.Get("redirect_uri") == gogRedirect:
			fmt.Fprint(w, `{"access_token":"access-1","refresh_token":"refresh-1","expires_in":3600,"user_id":"9"}`)
		case q.Get("grant_type") == "refresh_token" && q.Get("refresh_token") == "refresh-1":
			fmt.Fprint(w, `{"access_token":"access-2","refresh_token":"refresh-2","expires_in":3600,"user_id":"9"}`)
		default:
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":"invalid_grant"}`)
		}
	})
	mux.HandleFunc("/userData.json", func(w http.ResponseWriter, r *http.Request) {
		if auth(w, r) {
			fmt.Fprint(w, `{"username":"geralt","isLoggedIn":true}`)
		}
	})
	mux.HandleFunc("/account/getFilteredProducts", func(w http.ResponseWriter, r *http.Request) {
		if !auth(w, r) {
			return
		}
		if r.URL.Query().Get("page") == "1" {
			fmt.Fprint(w, `{"totalPages":2,"products":[{"id":42,"title":"Test Game","image":"//img.example/abc","url":"/en/game/test_game","isGame":true,"worksOn":{"Windows":true}}]}`)
			return
		}
		fmt.Fprint(w, `{"totalPages":2,"products":[{"id":43,"title":"Artless","image":"//img.example/def","slug":"artless","isGame":true,"worksOn":{"Linux":true}}]}`)
	})
	mux.HandleFunc("/platforms/gog/external_releases/42", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"game":{"vertical_cover":{"url_format":"https://images.gog.com/v{formatter}.{ext}?namespace=gamesdb"},"background":{"url_format":"https://images.gog.com/b{formatter}.{ext}?namespace=gamesdb"}}}`)
	})
	mux.HandleFunc("/products/42", func(w http.ResponseWriter, r *http.Request) {
		base := f.srv.URL + "/products/42/downlink/installer/"
		fmt.Fprintf(w, `{"title":"Test Game","downloads":{"installers":[
			{"id":"installer_mac_en","os":"mac","language":"en","version":"1","files":[{"id":"en2installer0","size":5,"downlink":%q}]},
			{"id":"installer_windows_en","os":"windows","language":"en","version":"1.2 (gog-3)","total_size":%d,"files":[
				{"id":"en1installer0","size":%d,"downlink":%q},
				{"id":"en1installer1","size":%d,"downlink":%q}]}]}}`,
			base+"en2installer0", len(f.setup)+len(f.bin), len(f.setup), base+"en1installer0", len(f.bin), base+"en1installer1")
	})
	mux.HandleFunc("/products/42/downlink/installer/", func(w http.ResponseWriter, r *http.Request) {
		if !auth(w, r) {
			return
		}
		name := "setup_test%20game.exe"
		if strings.HasSuffix(r.URL.Path, "en1installer1") {
			name = "setup_test%20game-1.bin"
		}
		fmt.Fprintf(w, `{"downlink":%q,"checksum":%q}`, f.srv.URL+"/cdn/"+name+"?token=x", f.srv.URL+"/sum/"+name)
	})
	mux.HandleFunc("/cdn/", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.ranges = append(f.ranges, r.Header.Get("Range"))
		f.mu.Unlock()
		data := f.setup
		if strings.HasSuffix(r.URL.Path, ".bin") {
			data = f.bin
		}
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(data))
	})
	mux.HandleFunc("/sum/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/sum/")
		f.mu.Lock()
		sum := f.md5[name]
		f.mu.Unlock()
		fmt.Fprintf(w, `<file name=%q available="1" md5=%q chunks="1" total_size="1"></file>`, name, sum)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)

	oldAuth, oldEmbed, oldAPI, oldDB := gogAuthBase, gogEmbedBase, gogAPIBase, gogGamesDBBase
	gogAuthBase, gogEmbedBase, gogAPIBase, gogGamesDBBase = f.srv.URL, f.srv.URL, f.srv.URL, f.srv.URL
	t.Cleanup(func() { gogAuthBase, gogEmbedBase, gogAPIBase, gogGamesDBBase = oldAuth, oldEmbed, oldAPI, oldDB })
	return f
}

// signedIn saves a login as GogLogin would, without the background library sync.
func signedIn(t *testing.T, m *Manager, expires int64) {
	t.Helper()
	if err := m.setGogLogin(&gogToken{AccessToken: "access-1", RefreshToken: "refresh-1", Expires: expires, Username: "geralt"}); err != nil {
		t.Fatal(err)
	}
}

func TestGogLoginAndLibrary(t *testing.T) {
	m := ubiTestManager(t)
	f := newFakeGog(t)

	if m.GogAccount().LoggedIn {
		t.Fatal("not signed in yet")
	}
	if games, _ := m.GogProvider().Scan(); games != nil {
		t.Fatal("no games without a login")
	}

	f.badCode = true
	if err := m.GogLogin("Abc123-_xyzXYZ0987654321"); err == nil || !strings.Contains(err.Error(), "didn't accept") {
		t.Fatalf("a refused code: %v", err)
	}
	f.mu.Lock()
	f.badCode = false
	f.mu.Unlock()

	if err := m.GogLogin("https://embed.gog.com/on_login_success?origin=client&code=Abc123-_xyzXYZ0987654321"); err != nil {
		t.Fatal(err)
	}
	if acc := m.GogAccount(); !acc.LoggedIn || acc.Name != "geralt" {
		t.Fatalf("account = %+v", acc)
	}
	st, err := os.Stat(gogTokenPath())
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("the login must be private: %v %v", st.Mode(), err)
	}

	if err := m.GogSync(); err != nil {
		t.Fatal(err)
	}
	games, err := m.GogProvider().Scan()
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 2 {
		t.Fatalf("both pages: %+v", games)
	}
	art, plain := games[1], games[0] // sorted by name: Artless, Test Game
	if art.ID != "gog:gog-42" || art.Source != library.SourceGog || art.ExternalID != "gog-42" || art.Installed {
		t.Errorf("game = %+v", art)
	}
	if art.Cover != "https://images.gog.com/v.jpg?namespace=gamesdb" || art.Hero != "https://images.gog.com/b.jpg?namespace=gamesdb" {
		t.Errorf("art = %q, %q", art.Cover, art.Hero)
	}
	if plain.Name != "Artless" || plain.Cover != "https://img.example/def.jpg" || plain.Hero != "" {
		t.Errorf("without GamesDB art, the store image: %+v", plain)
	}

	if u, _ := m.GogStoreURL("gog-42"); u != "https://www.gog.com/en/game/test_game" {
		t.Errorf("store = %q", u)
	}
	if u, _ := m.GogStoreURL("gog-43"); u != "https://www.gog.com/game/artless" {
		t.Errorf("store = %q", u)
	}

	// A fresh start reads the library from disk.
	m2 := New(library.NewHistory(), nil)
	if games, _ := m2.GogProvider().Scan(); len(games) != 2 {
		t.Errorf("cached library: %d games", len(games))
	}

	if err := m.GogLogout(); err != nil {
		t.Fatal(err)
	}
	if m.GogAccount().LoggedIn {
		t.Error("signed out")
	}
	if _, err := os.Stat(gogTokenPath()); !os.IsNotExist(err) {
		t.Error("the login file is gone")
	}
}

func TestGogTokenRefresh(t *testing.T) {
	m := ubiTestManager(t)
	f := newFakeGog(t)
	signedIn(t, m, time.Now().Unix()-10)

	tok, err := m.gogAccessToken(t.Context())
	if err != nil || tok != "access-2" {
		t.Fatalf("token = %q, %v", tok, err)
	}
	if saved := m.gogLogin(); saved.RefreshToken != "refresh-2" || saved.Username != "geralt" {
		t.Errorf("saved = %+v", saved)
	}
	if tok, _ := m.gogAccessToken(t.Context()); tok != "access-2" {
		t.Error("a fresh token is reused")
	}
	f.mu.Lock()
	n := len(f.grants)
	f.mu.Unlock()
	if n != 1 {
		t.Errorf("token requests = %d", n)
	}

	// GOG forgetting the login asks the user to sign in again.
	m.setGogLogin(&gogToken{AccessToken: "x", RefreshToken: "stale", Expires: 1})
	if _, err := m.gogAccessToken(t.Context()); err != errGogSignedOut {
		t.Errorf("err = %v", err)
	}
}

// fakeGogProton plays GOG's installer, writing the files an install has,
// and the game itself, which only logs how it was started.
const fakeGogProton = `#!/bin/sh
echo "proton $* [prefix=$STEAM_COMPAT_DATA_PATH] [pwd=$PWD]" >> "$GAMES/proton.log"
for a in "$@"; do
  case "$a" in
  /DIR=*)
    d=$(printf '%s' "${a#/DIR=Z:}" | tr '\\' '/')
    [ -n "$FAIL_SETUP" ] && exit 0
    mkdir -p "$d/bin"
    : > "$d/bin/game.exe"
    printf '{"playTasks":[{"isPrimary":true,"type":"FileTask","path":"bin\\\\game.exe","arguments":"-windowed"}]}' > "$d/goggame-42.info"
    ;;
  esac
done
`

func gogInstallEnv(t *testing.T) (*Manager, *recorder, *fakeGog, string) {
	t.Helper()
	m, games := ubisoftEnv(t)
	home, _ := os.UserHomeDir()
	proton := filepath.Join(home, ".local", "share", "Steam", "compatibilitytools.d", "GE-Proton9-1", "proton")
	os.WriteFile(proton, []byte(fakeGogProton), 0o755)
	rec := &recorder{}
	m.SetEmitter(rec.emit)
	m.settings.set(Settings{InstallDir: filepath.Join(home, "Games", "Shelf")})
	f := newFakeGog(t)
	signedIn(t, m, time.Now().Unix()+3600)
	if err := m.GogSync(); err != nil {
		t.Fatal(err)
	}
	return m, rec, f, games
}

func TestGogInstallPlayUninstall(t *testing.T) {
	linuxOnly(t)
	m, rec, f, games := gogInstallEnv(t)
	base := m.settings.get().InstallDir

	// Half the installer is already there from an earlier, cancelled try.
	dl := filepath.Join(base, ".shelf-downloads", "gog-42")
	os.MkdirAll(dl, 0o755)
	os.WriteFile(filepath.Join(dl, "setup_test game.exe"), f.setup[:len(f.setup)/2], 0o644)

	if err := m.GogInstall("gog-42"); err != nil {
		t.Fatal(err)
	}
	if p := rec.waitFinal(t, KindInstall); p.State != StateDone {
		t.Fatalf("install: %+v", p)
	}
	f.mu.Lock()
	ranges := append([]string{}, f.ranges...)
	f.mu.Unlock()
	if len(ranges) != 2 || ranges[0] != fmt.Sprintf("bytes=%d-", len(f.setup)/2) || ranges[1] != "" {
		t.Errorf("the installer resumes, the rest downloads whole: %q", ranges)
	}

	dir := filepath.Join(base, "Test Game")
	in, ok := m.gogInstalled()["gog-42"]
	if !ok || in.InstallPath != dir || in.Version != "1.2 (gog-3)" {
		t.Fatalf("installed = %+v", in)
	}
	if _, err := os.Stat(dl); !os.IsNotExist(err) {
		t.Error("the downloaded installer is cleaned up")
	}
	log, _ := os.ReadFile(filepath.Join(games, "proton.log"))
	for _, want := range []string{
		"run " + filepath.Join(dl, "setup_test game.exe") + " /VERYSILENT",
		"/DIR=" + winPath(dir),
		"[prefix=" + prefixDir("gog-42") + "]",
	} {
		if !strings.Contains(string(log), want) {
			t.Errorf("proton.log lacks %q:\n%s", want, log)
		}
	}

	g, _ := m.GogProvider().Scan()
	if !g[1].Installed || g[1].InstallPath != dir {
		t.Errorf("library = %+v", g[1])
	}
	if err := m.GogInstall("gog-42"); err == nil {
		t.Error("installing twice")
	}

	// Playing runs the primary task through Proton, in the game's folder.
	os.Remove(filepath.Join(games, "proton.log"))
	if err := m.GogLaunch("gog-42"); err != nil {
		t.Fatal(err)
	}
	var played string
	for i := 0; i < 200 && !strings.Contains(played, "game.exe"); i++ {
		time.Sleep(10 * time.Millisecond)
		data, _ := os.ReadFile(filepath.Join(games, "proton.log"))
		played = string(data)
	}
	want := "proton run " + filepath.Join(dir, "bin", "game.exe") + " -windowed [prefix=" + prefixDir("gog-42") + "] [pwd=" + filepath.Join(dir, "bin") + "]"
	if !strings.Contains(played, want) {
		t.Errorf("launch:\n%s\nwant %s", played, want)
	}
	for i := 0; i < 200 && m.isRunningGame("gog-42"); i++ {
		time.Sleep(10 * time.Millisecond)
	}

	if err := m.GogUninstall("gog-42"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("the game's folder is removed")
	}
	if _, ok := m.gogInstalled()["gog-42"]; ok {
		t.Error("no longer installed")
	}
}

func TestGogInstallFailures(t *testing.T) {
	linuxOnly(t)
	m, rec, f, _ := gogInstallEnv(t)

	if err := m.GogInstall("gog-999"); err == nil {
		t.Error("a game outside the library")
	}
	if err := m.GogInstall("../etc"); err == nil {
		t.Error("a bad id")
	}

	// A file that doesn't match GOG's checksum is thrown away.
	f.mu.Lock()
	f.md5["setup_test game-1.bin"] = strings.Repeat("0", 32)
	f.mu.Unlock()
	if err := m.GogInstall("gog-42"); err != nil {
		t.Fatal(err)
	}
	p := rec.waitFinal(t, KindInstall)
	if p.State != StateFailed || !strings.Contains(p.Error, "damaged") {
		t.Fatalf("damaged download: %+v", p)
	}
	if _, err := os.Stat(filepath.Join(m.settings.get().InstallDir, ".shelf-downloads", "gog-42", "setup_test game-1.bin")); !os.IsNotExist(err) {
		t.Error("the damaged file is removed")
	}
}

func TestGogSetupThatInstallsNothing(t *testing.T) {
	linuxOnly(t)
	m, rec, _, _ := gogInstallEnv(t)
	t.Setenv("FAIL_SETUP", "1")
	if err := m.GogInstall("gog-42"); err != nil {
		t.Fatal(err)
	}
	if p := rec.waitFinal(t, KindInstall); p.State != StateFailed || !strings.Contains(p.Error, "without installing") {
		t.Fatalf("install: %+v", p)
	}
	if _, ok := m.gogInstalled()["gog-42"]; ok {
		t.Error("not recorded as installed")
	}
}

func TestGogUninstallLeavesForeignFolders(t *testing.T) {
	m := ubiTestManager(t)
	dir := filepath.Join(t.TempDir(), "Not A Game")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("mine"), 0o644)
	setGogInstalled("gog-42", &gogInstall{Title: "Test Game", InstallPath: dir})

	if err := m.GogUninstall("gog-42"); err == nil {
		t.Fatal("a folder without the game's info file is left alone")
	}
	if _, err := os.Stat(filepath.Join(dir, "keep.txt")); err != nil {
		t.Error("nothing was deleted")
	}

	data, _ := os.ReadFile(gogInstalledPath())
	var all map[string]gogInstall
	json.Unmarshal(data, &all)
	if _, ok := all["gog-42"]; !ok {
		t.Error("still recorded")
	}
}
