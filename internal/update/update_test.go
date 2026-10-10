package update

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNewer(t *testing.T) {
	for _, c := range []struct {
		latest, current string
		want            bool
	}{
		{"v0.4.2", "v0.4.1", true},
		{"v0.5.0", "v0.4.9", true},
		{"v1.0.0", "v0.9.9", true},
		{"v0.10.0", "v0.9.0", true}, // not a string comparison
		{"v0.4.1", "v0.4.1", false},
		{"v0.4.0", "v0.4.1", false},
		{"v0.4.2", "dev", false},
		{"v0.4.2", "v0.4.1-3-gabc", false},
		{"", "v0.4.1", false},
	} {
		if got := Newer(c.latest, c.current); got != c.want {
			t.Errorf("Newer(%q, %q) = %v", c.latest, c.current, got)
		}
	}
}

func memStore() *Store { return &Store{s: Settings{AutoCheck: true}} }

// fakeELF is the smallest thing checkELF accepts.
var fakeELF = append([]byte("\x7fELF"), []byte("new shelf program")...)

func tarball(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, f := range []struct {
		name string
		data []byte
	}{{"./appicon.png", []byte("png")}, {name, content}} {
		tw.WriteHeader(&tar.Header{Name: f.name, Mode: 0o644, Size: int64(len(f.data)), Typeflag: tar.TypeReg})
		tw.Write(f.data)
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func sum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

type server struct {
	*httptest.Server
	assets map[string][]byte
	tag    string
}

func newServer(t *testing.T, tag string, assets map[string][]byte) *server {
	s := &server{assets: assets, tag: tag}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/o/r/releases/latest" {
			var as []string
			for name := range s.assets {
				as = append(as, fmt.Sprintf(`{"name":%q,"browser_download_url":%q}`, name, s.URL+"/o/r/releases/download/"+s.tag+"/"+name))
			}
			fmt.Fprintf(w, `{"tag_name":%q,"html_url":"https://example/release","body":"notes","assets":[%s]}`, s.tag, strings.Join(as, ","))
			return
		}
		if name, ok := strings.CutPrefix(r.URL.Path, "/o/r/releases/download/"+s.tag+"/"); ok {
			if data, ok := s.assets[name]; ok {
				w.Write(data)
				return
			}
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(s.Close)
	return s
}

func testUpdater(t *testing.T, srv *server, current, exe string) *Updater {
	u := New(current, "o/r", memStore())
	u.api = srv.URL
	u.allow = srv.URL + "/o/r/releases/download/"
	u.getenv = func(string) string { return "" }
	u.exe = func() (string, error) { return exe, nil }
	return u
}

func TestCheckRemembersTheLatestRelease(t *testing.T) {
	srv := newServer(t, "v0.5.0", nil)
	u := testUpdater(t, srv, "v0.4.1", filepath.Join(t.TempDir(), "shelf"))

	if st := u.Status(); st.Available || st.Latest != "" {
		t.Fatalf("before a check: %+v", st)
	}
	st, err := u.Check(context.Background())
	if err != nil || !st.Available || st.Latest != "v0.5.0" || st.Notes != "notes" || st.URL == "" || st.Skipped {
		t.Fatalf("after a check: %+v, %v", st, err)
	}

	u.Skip("v0.5.0")
	if st := u.Status(); !st.Available || !st.Skipped {
		t.Errorf("skipped: %+v", st)
	}
	srv.tag = "v0.6.0"
	if st, _ := u.Check(context.Background()); st.Skipped {
		t.Error("a skipped version hides the next one too")
	}

	if u.DueForCheck(time.Hour) {
		t.Error("due again right after a check")
	}
	u.SetAutoCheck(false)
	if u.DueForCheck(0) {
		t.Error("checks run although they are switched off")
	}
}

func TestDevBuildsNeverCheck(t *testing.T) {
	srv := newServer(t, "v9.9.9", nil)
	u := testUpdater(t, srv, "dev", "/x")
	st, err := u.Check(context.Background())
	if err != nil || st.Supported || st.Available || st.Latest != "" {
		t.Errorf("status = %+v, %v", st, err)
	}
	if u.DueForCheck(0) {
		t.Error("a dev build is due for a check")
	}
}

func TestInstallReplacesTheBinaryFromTheTarball(t *testing.T) {
	linuxOnly(t)
	archive := tarball(t, "./shelf", fakeELF)
	srv := newServer(t, "v0.5.0", map[string][]byte{
		binaryAsset:             archive,
		binaryAsset + ".sha256": []byte(sum(archive) + "  " + binaryAsset + "\n"),
	})
	dir := t.TempDir()
	exe := filepath.Join(dir, "shelf")
	os.WriteFile(exe, []byte("old"), 0o755)
	u := testUpdater(t, srv, "v0.4.1", exe)
	u.Check(context.Background())

	var last int64
	tag, err := u.Install(context.Background(), func(done, total int64) { last = done })
	if err != nil || tag != "v0.5.0" {
		t.Fatalf("install: %q, %v", tag, err)
	}
	got, _ := os.ReadFile(exe)
	if !bytes.Equal(got, fakeELF) {
		t.Errorf("binary = %q", got)
	}
	if st, _ := os.Stat(exe); st.Mode().Perm() != 0o755 {
		t.Errorf("mode = %v", st.Mode().Perm())
	}
	if last == 0 {
		t.Error("no progress reported")
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".shelf-*")); len(left) != 0 {
		t.Errorf("temporary files left behind: %v", left)
	}
}

func TestInstallReplacesAnAppImage(t *testing.T) {
	linuxOnly(t)
	srv := newServer(t, "v0.5.0", map[string][]byte{
		appImageAsset:             fakeELF,
		appImageAsset + ".sha256": []byte(sum(fakeELF)),
	})
	dir := t.TempDir()
	img := filepath.Join(dir, "Shelf.AppImage")
	os.WriteFile(img, []byte("old"), 0o755)
	u := testUpdater(t, srv, "v0.4.1", "/somewhere/else")
	u.getenv = func(k string) string {
		if k == "APPIMAGE" {
			return img
		}
		return ""
	}
	u.Check(context.Background())
	if _, err := u.Install(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(img); !bytes.Equal(got, fakeELF) {
		t.Errorf("appimage = %q", got)
	}
}

func TestInstallRefusesAnythingFishy(t *testing.T) {
	linuxOnly(t)
	archive := tarball(t, "./shelf", fakeELF)
	dir := t.TempDir()
	exe := filepath.Join(dir, "shelf")

	cases := map[string]map[string][]byte{
		"wrong checksum": {binaryAsset: archive, binaryAsset + ".sha256": []byte(strings.Repeat("0", 64))},
		"no checksum":    {binaryAsset: archive},
		"not a program":  {binaryAsset: tarball(t, "./shelf", []byte("#!/bin/sh\nrm -rf ~")), binaryAsset + ".sha256": nil},
		"no shelf in it": {binaryAsset: tarball(t, "./other", fakeELF), binaryAsset + ".sha256": nil},
	}
	// The last two need a matching checksum to get as far as their own check.
	cases["not a program"][binaryAsset+".sha256"] = []byte(sum(cases["not a program"][binaryAsset]))
	cases["no shelf in it"][binaryAsset+".sha256"] = []byte(sum(cases["no shelf in it"][binaryAsset]))

	for name, assets := range cases {
		os.WriteFile(exe, []byte("old"), 0o755)
		srv := newServer(t, "v0.5.0", assets)
		u := testUpdater(t, srv, "v0.4.1", exe)
		u.Check(context.Background())
		if _, err := u.Install(context.Background(), nil); err == nil {
			t.Errorf("%s: installed anyway", name)
		}
		if got, _ := os.ReadFile(exe); string(got) != "old" {
			t.Errorf("%s: the program was replaced", name)
		}
	}

	// Links that don't point at this repository's releases are not followed.
	srv := newServer(t, "v0.5.0", map[string][]byte{binaryAsset: archive, binaryAsset + ".sha256": []byte(sum(archive))})
	u := testUpdater(t, srv, "v0.4.1", exe)
	u.allow = "https://github.com/someone-else/"
	u.Check(context.Background())
	if _, err := u.Install(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "refusing") {
		t.Errorf("foreign download link: %v", err)
	}
}

func TestPackageManagerAndReadOnlyInstallsAreExplained(t *testing.T) {
	linuxOnly(t)
	srv := newServer(t, "v0.5.0", nil)
	u := testUpdater(t, srv, "v0.4.1", "/usr/bin/shelf")
	u.Check(context.Background())
	st := u.Status()
	if !st.Available || st.CanInstall || !strings.Contains(st.InstallNote, "package manager") {
		t.Errorf("status = %+v", st)
	}
	if _, err := u.Install(context.Background(), nil); err == nil {
		t.Error("installed over a package manager's file")
	}

	u = testUpdater(t, srv, "v0.4.1", filepath.Join(t.TempDir(), "gone", "shelf"))
	u.Check(context.Background())
	if st := u.Status(); st.CanInstall || st.InstallNote == "" {
		t.Errorf("unwritable: %+v", st)
	}
}

// linuxOnly skips tests of replacing the running copy, which only Linux does.
func linuxOnly(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("Linux builds only replace themselves on Linux")
	}
}

func TestWindowsInstallStepsAsideForTheNewProgram(t *testing.T) {
	newExe := append([]byte("MZ"), []byte("new shelf program")...)
	srv := newServer(t, "v0.5.0", map[string][]byte{
		windowsAsset:             newExe,
		windowsAsset + ".sha256": []byte(sum(newExe) + " *" + windowsAsset),
		binaryAsset:              tarball(t, "./shelf", fakeELF),
	})
	dir := t.TempDir()
	exe := filepath.Join(dir, "shelf.exe")
	os.WriteFile(exe, []byte("old"), 0o755)
	u := testUpdater(t, srv, "v0.4.1", exe)
	u.goos = "windows"
	u.Check(context.Background())
	if _, err := u.Install(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(exe); !bytes.Equal(got, newExe) {
		t.Errorf("program = %q", got)
	}
	if got, _ := os.ReadFile(exe + ".old"); string(got) != "old" {
		t.Errorf("the old program wasn't kept aside: %q", got)
	}
	u.RemoveReplaced()
	if _, err := os.Stat(exe + ".old"); !os.IsNotExist(err) {
		t.Error("the old program is still there after the next start")
	}

	// Something that isn't a Windows program is refused.
	os.WriteFile(exe, []byte("old"), 0o755)
	srv.assets[windowsAsset] = fakeELF
	srv.assets[windowsAsset+".sha256"] = []byte(sum(fakeELF))
	srv.tag = "v0.6.0"
	u.Check(context.Background())
	if _, err := u.Install(context.Background(), nil); err == nil {
		t.Error("installed something that isn't a Windows program")
	}
	if got, _ := os.ReadFile(exe); string(got) != "old" {
		t.Errorf("the program was replaced: %q", got)
	}

	// An install for all users is left to whatever installed it.
	u = testUpdater(t, srv, "v0.4.1", `C:\Program Files\Shelf\shelf.exe`)
	u.goos = "windows"
	u.getenv = func(k string) string {
		if k == "ProgramFiles" {
			return `C:\Program Files`
		}
		return ""
	}
	if !underProgramFiles(`C:\Program Files\Shelf\shelf.exe`, u.getenv) && runtime.GOOS == "windows" {
		t.Error("Program Files not recognised")
	}
}

func TestMacUpdatesPointAtTheReleasePage(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS only")
	}
	u := New("v0.4.1", "o/r", memStore())
	if _, why := u.target(); !strings.Contains(why, "release page") {
		t.Errorf("note = %q", why)
	}
}
