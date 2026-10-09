package epic

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// legendary publishes standalone builds (PyInstaller, no Python needed) for
// each system. Shelf can fetch the right one into its own bin folder, which
// findLegendary already looks in, so nothing has to be installed by hand.

// Where legendary's releases are, and the only place downloads may come from.
// Variables so tests can point them elsewhere.
var (
	legendaryReleaseURL     = "https://api.github.com/repos/legendary-gl/legendary/releases/latest"
	legendaryDownloadPrefix = "https://github.com/legendary-gl/legendary/releases/download/"
)

// legendaryAsset is the name of the build for this system in legendary's releases.
func legendaryAsset(goos, goarch string) (string, bool) {
	arch := map[string]string{"amd64": "x64", "arm64": "arm64"}[goarch]
	if arch == "" {
		return "", false
	}
	switch goos {
	case "linux":
		return "legendary_linux_" + arch, true
	case "darwin":
		return "legendary_macOS_" + arch, true
	}
	return "", false
}

// legendaryBinDir is where Shelf keeps the legendary it downloaded.
func legendaryBinDir() string { return filepath.Join(configDir(), "bin") }

func (m *Manager) LegendaryInstallState() ProtonInstallState {
	m.ubiMu.Lock()
	defer m.ubiMu.Unlock()
	return m.legendaryInstall
}

func (m *Manager) setLegendaryInstall(s ProtonInstallState) {
	m.ubiMu.Lock()
	m.legendaryInstall = s
	m.ubiMu.Unlock()
	m.send("legendary:install", s)
}

// InstallLegendary downloads the newest legendary for this system. Progress
// comes as "legendary:install" events.
func (m *Manager) InstallLegendary() error {
	if _, ok := legendaryAsset(runtime.GOOS, runtime.GOARCH); !ok {
		return fmt.Errorf("legendary has no build for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	m.ubiMu.Lock()
	if m.legendaryInstall.State == "running" {
		m.ubiMu.Unlock()
		return fmt.Errorf("legendary is already being installed")
	}
	m.legendaryInstall = ProtonInstallState{State: "running", Message: "Looking for the newest release"}
	m.ubiMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		report := func(pct float64, msg string) {
			m.setLegendaryInstall(ProtonInstallState{State: "running", Percent: pct, Message: msg})
		}
		report(0, "Looking for the newest release")
		name, err := m.installLegendary(ctx, runtime.GOOS, runtime.GOARCH, report)
		if err != nil {
			m.logf("legendary", "", "legendary install failed: %v", err)
			m.setLegendaryInstall(ProtonInstallState{State: "failed", Error: err.Error()})
			return
		}
		m.logf("legendary", "", "installed legendary %s", name)
		m.setLegendaryInstall(ProtonInstallState{State: "done", Percent: 100, Name: name, Message: "legendary " + name + " is installed"})
		m.send("library:changed", nil)
	}()
	return nil
}

type legendaryRelease struct {
	Tag    string `json:"tag_name"`
	Assets []struct {
		Name   string `json:"name"`
		URL    string `json:"browser_download_url"`
		Size   int64  `json:"size"`
		Digest string `json:"digest"` // "sha256:<hex>", worked out by GitHub
	} `json:"assets"`
}

func (m *Manager) installLegendary(ctx context.Context, goos, goarch string, report func(float64, string)) (string, error) {
	want, _ := legendaryAsset(goos, goarch)
	resp, err := httpGet(ctx, legendaryReleaseURL)
	if err != nil {
		return "", fmt.Errorf("couldn't look up legendary's releases: %w", err)
	}
	var rel legendaryRelease
	err = json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&rel)
	resp.Body.Close()
	if err != nil {
		return "", fmt.Errorf("unexpected answer from GitHub: %w", err)
	}

	var url, sum string
	var size int64
	for _, a := range rel.Assets {
		if a.Name == want {
			url, size = a.URL, a.Size
			sum, _ = strings.CutPrefix(a.Digest, "sha256:")
		}
	}
	if url == "" {
		return "", fmt.Errorf("legendary %s has no %s download", rel.Tag, want)
	}
	if !strings.HasPrefix(url, legendaryDownloadPrefix) {
		return "", fmt.Errorf("unexpected download address %s", url)
	}
	// Without GitHub's checksum there is nothing to check the download against.
	if len(sum) != 64 {
		return "", fmt.Errorf("GitHub gave no checksum for %s, so Shelf won't run it", want)
	}

	dir := legendaryBinDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(dir, ".legendary-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())

	report(0, "Downloading legendary "+rel.Tag)
	resp, err = httpGet(ctx, url)
	if err != nil {
		tmp.Close()
		return "", fmt.Errorf("download: %w", err)
	}
	h := sha256.New()
	var done int64
	buf := make([]byte, 128<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := tmp.Write(buf[:n]); werr != nil {
				resp.Body.Close()
				tmp.Close()
				return "", werr
			}
			h.Write(buf[:n])
			done += int64(n)
			if size > 0 {
				report(float64(done)*95/float64(size), "Downloading legendary "+rel.Tag)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			resp.Body.Close()
			tmp.Close()
			return "", fmt.Errorf("download: %w", rerr)
		}
	}
	resp.Body.Close()
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if got := hex.EncodeToString(h.Sum(nil)); !strings.EqualFold(got, sum) {
		return "", fmt.Errorf("the download doesn't match GitHub's checksum, so it was thrown away")
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return "", err
	}

	report(97, "Checking that it runs")
	if err := runsLegendary(ctx, tmp.Name()); err != nil && goos == "darwin" {
		// An Apple Silicon Mac only runs signed programs; an ad-hoc signature is enough.
		if out, serr := exec.CommandContext(ctx, "codesign", "--force", "--sign", "-", tmp.Name()).CombinedOutput(); serr != nil {
			return "", fmt.Errorf("legendary doesn't start, and signing it failed: %s", strings.TrimSpace(string(out)))
		}
		err = runsLegendary(ctx, tmp.Name())
		if err != nil {
			return "", err
		}
	} else if err != nil {
		return "", err
	}

	if err := os.Rename(tmp.Name(), filepath.Join(dir, "legendary")); err != nil {
		return "", err
	}
	return rel.Tag, nil
}

// runsLegendary checks a downloaded legendary starts and says what it is.
func runsLegendary(ctx context.Context, path string) error {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "--version").CombinedOutput()
	if err != nil || !bytes.Contains(bytes.ToLower(out), []byte("legendary")) {
		return fmt.Errorf("the downloaded legendary doesn't start: %s", tail(strings.TrimSpace(string(out))+" "+fmt.Sprint(err), 200))
	}
	return nil
}
