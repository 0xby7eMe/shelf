package epic

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type tarEntry struct {
	name, body, link string
	typ              byte
}

func makeTarGz(t *testing.T, entries []tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		typ := e.typ
		if typ == 0 {
			typ = tar.TypeReg
		}
		hdr := &tar.Header{Name: e.name, Typeflag: typ, Mode: 0o755, Size: int64(len(e.body)), Linkname: e.link}
		if typ != tar.TypeReg {
			hdr.Size = 0
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if typ == tar.TypeReg {
			tw.Write([]byte(e.body))
		}
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

// fakeGE serves a release, its archive and its checksum.
func fakeGE(t *testing.T, tag, base string, archive []byte, sum string) {
	t.Helper()
	if sum == "" {
		h := sha512.Sum512(archive)
		sum = hex.EncodeToString(h[:])
	}
	mux := http.NewServeMux()
	var srv *httptest.Server
	mux.HandleFunc("/release", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"tag_name":%q,"assets":[{"name":%q,"browser_download_url":%q},{"name":%q,"browser_download_url":%q}]}`,
			tag, base+".tar.gz", srv.URL+"/tar", base+".sha512sum", srv.URL+"/sum")
	})
	mux.HandleFunc("/tar", func(w http.ResponseWriter, r *http.Request) { w.Write(archive) })
	mux.HandleFunc("/sum", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, "%s  %s.tar.gz\n", sum, base) })
	srv = httptest.NewServer(mux)
	old := geReleaseURL
	geReleaseURL = srv.URL + "/release"
	t.Cleanup(func() { geReleaseURL = old; srv.Close() })
}

func waitProton(t *testing.T, m *Manager) ProtonInstallState {
	t.Helper()
	for i := 0; i < 250; i++ {
		if st := m.ProtonInstallState(); st.State == "done" || st.State == "failed" {
			return st
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("install didn't finish")
	return ProtonInstallState{}
}

func geEnv(t *testing.T) *Manager {
	m := ubiTestManager(t)
	home, _ := os.UserHomeDir()
	os.MkdirAll(filepath.Join(home, ".local", "share", "Steam", "steamapps"), 0o755)
	return m
}

func goodBuild(tag string) []tarEntry {
	return []tarEntry{
		{name: tag + "/", typ: tar.TypeDir},
		{name: tag + "/proton", body: "#!/bin/sh\n"},
		{name: tag + "/files/bin/wine", body: "x"},
		{name: tag + "/files/lib", link: "bin", typ: tar.TypeSymlink},
	}
}

func TestInstallProtonGE(t *testing.T) {
	m := geEnv(t)
	tag := "GE-Proton10-99"
	fakeGE(t, tag, tag, makeTarGz(t, goodBuild(tag)), "")

	if err := m.InstallProtonGE(); err != nil {
		t.Fatal(err)
	}
	if st := waitProton(t, m); st.State != "done" || st.Name != tag {
		t.Fatalf("install: %+v", st)
	}
	var found bool
	for _, b := range FindProton() {
		found = found || b.Name == tag
	}
	if !found {
		t.Fatalf("Shelf doesn't see the new build: %+v", FindProton())
	}
	if b, ok := resolveProton(""); !ok || b.Name != tag {
		t.Errorf("GE-Proton should be the default: %+v", b)
	}
	home, _ := os.UserHomeDir()
	root := filepath.Join(home, ".local", "share", "Steam", "compatibilitytools.d")
	entries, _ := os.ReadDir(root)
	if len(entries) != 1 {
		t.Errorf("only the build should be left, found %v", entries)
	}

	// Installing again is a no-op that still reports success.
	if err := m.InstallProtonGE(); err != nil {
		t.Fatal(err)
	}
	if st := waitProton(t, m); st.State != "done" {
		t.Fatalf("second install: %+v", st)
	}
}

func TestInstallProtonGERejectsBadDownloads(t *testing.T) {
	cases := map[string]struct {
		entries []tarEntry
		sum     string
		want    string
	}{
		"checksum":   {goodBuild("GE-Proton10-99"), strings.Repeat("0", 128), "checksum doesn't match"},
		"traversal":  {[]tarEntry{{name: "../../evil", body: "x"}}, "", "doesn't contain a Proton build"},
		"link":       {[]tarEntry{{name: "GE-Proton10-99/l", link: "/etc/passwd", typ: tar.TypeSymlink}}, "", "unsafe link"},
		"not proton": {[]tarEntry{{name: "GE-Proton10-99/readme", body: "x"}}, "", "doesn't contain a Proton build"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			m := geEnv(t)
			fakeGE(t, "GE-Proton10-99", "GE-Proton10-99", makeTarGz(t, c.entries), c.sum)
			if err := m.InstallProtonGE(); err != nil {
				t.Fatal(err)
			}
			st := waitProton(t, m)
			if st.State != "failed" || !strings.Contains(st.Error, c.want) {
				t.Fatalf("want failure containing %q, got %+v", c.want, st)
			}
			home, _ := os.UserHomeDir()
			root := filepath.Join(home, ".local", "share", "Steam", "compatibilitytools.d")
			if entries, _ := os.ReadDir(root); len(entries) != 0 {
				t.Errorf("nothing may be left behind: %v", entries)
			}
			if _, err := os.Stat(filepath.Join(filepath.Dir(root), "evil")); err == nil {
				t.Error("a file escaped the install folder")
			}
		})
	}
}

func TestInstallProtonGERejectsOddTags(t *testing.T) {
	m := geEnv(t)
	fakeGE(t, "../../etc", "../../etc", makeTarGz(t, nil), "")
	m.InstallProtonGE()
	if st := waitProton(t, m); st.State != "failed" || !strings.Contains(st.Error, "unexpected release") {
		t.Fatalf("%+v", st)
	}
}

// Current releases name everything per architecture, including the folder
// inside the archive. It is installed under the plain name.
func TestInstallProtonGEPerArchitecture(t *testing.T) {
	arch, ok := geArch()
	if !ok {
		t.Skip("no GE-Proton build for this CPU")
	}
	m := geEnv(t)
	tag := "GE-Proton11-7"
	fakeGE(t, tag, tag+"-"+arch, makeTarGz(t, goodBuild(tag+"-"+arch)), "")

	if err := m.InstallProtonGE(); err != nil {
		t.Fatal(err)
	}
	if st := waitProton(t, m); st.State != "done" || st.Name != tag {
		t.Fatalf("install: %+v", st)
	}
	home, _ := os.UserHomeDir()
	root := filepath.Join(home, ".local", "share", "Steam", "compatibilitytools.d")
	if entries, _ := os.ReadDir(root); len(entries) != 1 || entries[0].Name() != tag {
		t.Errorf("want just %s, found %v", tag, entries)
	}
	if b, ok := resolveProton(""); !ok || b.Name != tag {
		t.Errorf("Shelf should use it: %+v", b)
	}
}

// A release with no download for this CPU says so instead of failing oddly.
func TestInstallProtonGENoMatchingDownload(t *testing.T) {
	m := geEnv(t)
	fakeGE(t, "GE-Proton11-7", "GE-Proton11-7-riscv64", makeTarGz(t, nil), "")
	m.InstallProtonGE()
	if st := waitProton(t, m); st.State != "failed" || !strings.Contains(st.Error, "no download with a checksum") {
		t.Fatalf("%+v", st)
	}
}
