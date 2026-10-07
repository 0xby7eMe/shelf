package epic

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"shelf/internal/library"
)

// GE-Proton is Proton with extra fixes. Ubisoft Connect is known to behave
// better with it than with Valve's builds, so Shelf can fetch the newest
// release itself, the way ProtonUp-Qt does.

// geReleaseURL is a variable so tests can point it elsewhere.
var geReleaseURL = "https://api.github.com/repos/GloriousEggroll/proton-ge-custom/releases/latest"

// ProtonInstallState is the progress of installing GE-Proton.
type ProtonInstallState struct {
	State   string  `json:"state"` // "", "running", "done" or "failed"
	Percent float64 `json:"percent"`
	Message string  `json:"message"`
	Name    string  `json:"name,omitempty"` // the build that was installed
	Error   string  `json:"error,omitempty"`
}

func (m *Manager) ProtonInstallState() ProtonInstallState {
	m.ubiMu.Lock()
	defer m.ubiMu.Unlock()
	return m.protonInstall
}

func (m *Manager) setProtonInstall(s ProtonInstallState) {
	m.ubiMu.Lock()
	m.protonInstall = s
	m.ubiMu.Unlock()
	m.send("proton:install", s)
}

// protonInstallDir is where a new build goes: Steam's folder for custom
// compatibility tools, or Heroic's when Steam isn't installed.
func protonInstallDir() (string, error) {
	if steam, err := library.SteamRoot(); err == nil {
		return filepath.Join(steam, "compatibilitytools.d"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "heroic", "tools", "proton"), nil
}

// InstallProtonGE downloads the newest GE-Proton and unpacks it.
func (m *Manager) InstallProtonGE() error {
	m.ubiMu.Lock()
	if m.protonInstall.State == "running" {
		m.ubiMu.Unlock()
		return fmt.Errorf("GE-Proton is already being installed")
	}
	m.protonInstall = ProtonInstallState{State: "running", Message: "Looking for the newest release"}
	m.ubiMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		report := func(pct float64, msg string) {
			m.setProtonInstall(ProtonInstallState{State: "running", Percent: pct, Message: msg})
		}
		report(0, "Looking for the newest release")
		name, err := m.installProtonGE(ctx, report)
		if err != nil {
			m.logf("proton", "", "GE-Proton install failed: %v", err)
			m.setProtonInstall(ProtonInstallState{State: "failed", Error: err.Error()})
			return
		}
		m.logf("proton", "", "installed %s", name)
		m.setProtonInstall(ProtonInstallState{State: "done", Percent: 100, Name: name, Message: name + " is installed"})
		m.send("library:changed", nil)
	}()
	return nil
}

// geArch is the name GE-Proton gives this CPU in its downloads.
func geArch() (string, bool) {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64", true
	case "arm64":
		return "aarch64", true
	}
	return "", false
}

type geRelease struct {
	Tag    string `json:"tag_name"`
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func httpGet(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Shelf")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("%s answered %s", url, resp.Status)
	}
	return resp, nil
}

func (m *Manager) installProtonGE(ctx context.Context, report func(float64, string)) (string, error) {
	resp, err := httpGet(ctx, geReleaseURL)
	if err != nil {
		return "", fmt.Errorf("couldn't look up GE-Proton: %w", err)
	}
	var rel geRelease
	err = json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&rel)
	resp.Body.Close()
	if err != nil {
		return "", fmt.Errorf("unexpected answer from GitHub: %w", err)
	}
	// The tag becomes a folder name, so it has to be a plain one.
	if !strings.HasPrefix(rel.Tag, "GE-Proton") || strings.ContainsAny(rel.Tag, `/\`) || strings.Contains(rel.Tag, "..") {
		return "", fmt.Errorf("unexpected release %q", rel.Tag)
	}
	arch, ok := geArch()
	if !ok {
		return "", fmt.Errorf("GE-Proton isn't built for this CPU (%s)", runtime.GOARCH)
	}
	// Newer releases name their files per architecture; older ones don't.
	var tarURL, sumURL, base string
	for _, candidate := range []string{rel.Tag + "-" + arch, rel.Tag} {
		tarURL, sumURL = "", ""
		for _, a := range rel.Assets {
			switch a.Name {
			case candidate + ".tar.gz":
				tarURL = a.URL
			case candidate + ".sha512sum":
				sumURL = a.URL
			}
		}
		if tarURL != "" && sumURL != "" {
			base = candidate
			break
		}
	}
	if base == "" {
		return "", fmt.Errorf("%s has no download with a checksum for %s", rel.Tag, arch)
	}

	root, err := protonInstallDir()
	if err != nil {
		return "", err
	}
	final := filepath.Join(root, rel.Tag)
	if isProtonDir(rel.Tag, final) {
		return rel.Tag, nil
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}

	// Checksum first: it's small, and a release without one isn't installed.
	resp, err = httpGet(ctx, sumURL)
	if err != nil {
		return "", fmt.Errorf("couldn't get the checksum: %w", err)
	}
	sumData, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	resp.Body.Close()
	want, _, _ := strings.Cut(strings.TrimSpace(string(sumData)), " ")
	if len(want) != sha512.Size*2 {
		return "", fmt.Errorf("the checksum file is malformed")
	}

	report(0, "Downloading "+rel.Tag)
	archive := filepath.Join(root, "."+rel.Tag+".tar.gz.part")
	defer os.Remove(archive)
	if err := downloadVerified(ctx, tarURL, archive, sha512.New(), want, func(f float64) { report(f*80, "Downloading "+rel.Tag) }); err != nil {
		return "", err
	}

	report(82, "Unpacking")
	tmp := filepath.Join(root, "."+rel.Tag+".part")
	os.RemoveAll(tmp)
	if err := extractTarGz(archive, tmp); err != nil {
		os.RemoveAll(tmp)
		return "", fmt.Errorf("couldn't unpack it: %w", err)
	}
	// The archive's folder carries the architecture; the installed one doesn't,
	// which is the name Steam and Shelf show.
	inner := ""
	for _, name := range []string{base, rel.Tag} {
		if dir := filepath.Join(tmp, name); isProtonDir(rel.Tag, dir) {
			inner = dir
			break
		}
	}
	if inner == "" {
		os.RemoveAll(tmp)
		return "", fmt.Errorf("the download doesn't contain a Proton build")
	}
	if err := os.Rename(inner, final); err != nil {
		os.RemoveAll(tmp)
		return "", err
	}
	os.RemoveAll(tmp)
	return rel.Tag, nil
}

// downloadVerified saves url to dest and checks its digest, computed with h,
// against the hex string wantSum.
func downloadVerified(ctx context.Context, url, dest string, h hash.Hash, wantSum string, progress func(float64)) error {
	resp, err := httpGet(ctx, url)
	if err != nil {
		return fmt.Errorf("couldn't download it: %w", err)
	}
	defer resp.Body.Close()
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()

	var done int64
	buf := make([]byte, 256*1024)
	last := time.Now()
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				return err
			}
			h.Write(buf[:n])
			done += int64(n)
			if resp.ContentLength > 0 && time.Since(last) > 250*time.Millisecond {
				last = time.Now()
				progress(float64(done) / float64(resp.ContentLength))
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return fmt.Errorf("the download was interrupted: %w", rerr)
		}
	}
	if hex.EncodeToString(h.Sum(nil)) != strings.ToLower(wantSum) {
		return fmt.Errorf("the download is corrupt: its checksum doesn't match")
	}
	progress(1)
	return nil
}

// extractTarGz unpacks a .tar.gz into dest, refusing anything that would
// land outside it.
func extractTarGz(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	return extractTar(gz, dest)
}

// extractTar unpacks a tar stream into dest, refusing anything that would
// land outside it.
func extractTar(r io.Reader, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	absDest, err := filepath.Abs(dest)
	if err != nil {
		return err
	}

	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(absDest, filepath.Clean("/"+hdr.Name))
		if target != absDest && !strings.HasPrefix(target, absDest+string(filepath.Separator)) {
			return fmt.Errorf("unsafe path %q in the archive", hdr.Name)
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)&0o777|0o200)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
		case tar.TypeSymlink:
			// A link may point anywhere inside the build, never outside it.
			resolved := hdr.Linkname
			if !filepath.IsAbs(resolved) {
				resolved = filepath.Join(filepath.Dir(target), resolved)
			}
			if resolved != absDest && !strings.HasPrefix(resolved, absDest+string(filepath.Separator)) {
				return fmt.Errorf("unsafe link %q in the archive", hdr.Name)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(hdr.Linkname, target); err != nil {
				return err
			}
		}
	}
}
