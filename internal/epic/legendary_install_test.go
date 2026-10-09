package epic

import (
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
)

func TestLegendaryAsset(t *testing.T) {
	for _, c := range []struct{ goos, goarch, want string }{
		{"linux", "amd64", "legendary_linux_x64"},
		{"linux", "arm64", "legendary_linux_arm64"},
		{"darwin", "arm64", "legendary_macOS_arm64"},
		{"darwin", "amd64", "legendary_macOS_x64"},
	} {
		if got, ok := legendaryAsset(c.goos, c.goarch); !ok || got != c.want {
			t.Errorf("%s/%s = %q", c.goos, c.goarch, got)
		}
	}
	if _, ok := legendaryAsset("linux", "386"); ok {
		t.Error("no 32-bit build")
	}
}

// fakeLegendaryRelease serves a release whose build for this system is a
// script that answers --version, with the digest given.
func fakeLegendaryRelease(t *testing.T, digest func(sum string) string) {
	t.Helper()
	bin := []byte("#!/bin/sh\necho 'legendary version \"0.21.1\", codename \"Test\"'\n")
	sum := sha256.Sum256(bin)
	name, _ := legendaryAsset(runtime.GOOS, runtime.GOARCH)

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/latest" {
			fmt.Fprintf(w, `{"tag_name":"0.21.1","assets":[{"name":"legendary_windows_x64.exe","browser_download_url":"%s/dl/x.exe"},{"name":%q,"size":%d,"digest":%q,"browser_download_url":"%s/dl/%s"}]}`,
				srv.URL, name, len(bin), digest(hex.EncodeToString(sum[:])), srv.URL, name)
			return
		}
		w.Write(bin)
	}))
	t.Cleanup(srv.Close)
	oldURL, oldPrefix := legendaryReleaseURL, legendaryDownloadPrefix
	legendaryReleaseURL, legendaryDownloadPrefix = srv.URL+"/latest", srv.URL+"/dl/"
	t.Cleanup(func() { legendaryReleaseURL, legendaryDownloadPrefix = oldURL, oldPrefix })
}

func installLegendaryNow(t *testing.T, m *Manager) (string, error) {
	t.Helper()
	return m.installLegendary(t.Context(), runtime.GOOS, runtime.GOARCH, func(float64, string) {})
}

func TestInstallLegendary(t *testing.T) {
	m := ubiTestManager(t)
	t.Setenv("PATH", t.TempDir()) // no legendary anywhere else
	fakeLegendaryRelease(t, func(sum string) string { return "sha256:" + sum })

	if m.Account().LegendaryFound {
		t.Fatal("nothing installed yet")
	}
	tag, err := installLegendaryNow(t, m)
	if err != nil || tag != "0.21.1" {
		t.Fatalf("install: %q, %v", tag, err)
	}
	path := filepath.Join(legendaryBinDir(), "legendary")
	st, err := os.Stat(path)
	if err != nil || st.Mode().Perm()&0o111 == 0 {
		t.Fatalf("legendary should be executable: %v %v", st, err)
	}
	if found, err := findLegendary(); err != nil || found != path {
		t.Errorf("found %q, %v", found, err)
	}
	if !m.Account().LegendaryFound {
		t.Error("Shelf finds the legendary it installed")
	}
	entries, _ := os.ReadDir(legendaryBinDir())
	if len(entries) != 1 {
		t.Errorf("no temporary files are left: %v", entries)
	}
}

func TestInstallLegendaryChecksTheDownload(t *testing.T) {
	for name, digest := range map[string]func(string) string{
		"wrong checksum": func(string) string { return "sha256:" + strings.Repeat("0", 64) },
		"no checksum":    func(string) string { return "" },
	} {
		t.Run(name, func(t *testing.T) {
			m := ubiTestManager(t)
			fakeLegendaryRelease(t, digest)
			if _, err := installLegendaryNow(t, m); err == nil {
				t.Fatal("must refuse")
			}
			if _, err := os.Stat(filepath.Join(legendaryBinDir(), "legendary")); !os.IsNotExist(err) {
				t.Error("nothing is installed")
			}
		})
	}
}

func TestInstallLegendaryOnlyFromItsReleases(t *testing.T) {
	m := ubiTestManager(t)
	fakeLegendaryRelease(t, func(sum string) string { return "sha256:" + sum })
	legendaryDownloadPrefix = "https://github.com/legendary-gl/legendary/releases/download/"
	if _, err := installLegendaryNow(t, m); err == nil || !strings.Contains(err.Error(), "unexpected download address") {
		t.Fatalf("err = %v", err)
	}
}
