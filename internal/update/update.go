// Package update tells Shelf when a new release is out and, where it can,
// installs it. Only the release files of Shelf's own repository are ever
// downloaded, and only after their SHA-256 matches the one published beside them.
package update

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	appImageAsset = "shelf-linux-x86_64.AppImage"
	binaryAsset   = "shelf-arch-x86_64.tar.gz"
	maxDownload   = 400 << 20
)

var versionRe = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)

func parseVersion(v string) ([3]int, bool) {
	m := versionRe.FindStringSubmatch(v)
	if m == nil {
		return [3]int{}, false
	}
	var out [3]int
	for i := range out {
		out[i], _ = strconv.Atoi(m[i+1])
	}
	return out, true
}

// Newer reports whether latest is a later release than current. Anything that
// isn't a plain vX.Y.Z version is never newer.
func Newer(latest, current string) bool {
	l, ok1 := parseVersion(latest)
	c, ok2 := parseVersion(current)
	if !ok1 || !ok2 {
		return false
	}
	for i := range l {
		if l[i] != c[i] {
			return l[i] > c[i]
		}
	}
	return false
}

// Settings are what is saved between runs.
type Settings struct {
	AutoCheck bool   `json:"autoCheck"`
	Skipped   string `json:"skipped,omitempty"` // a release the user doesn't want to hear about
	Latest    string `json:"latest,omitempty"`  // the newest release seen
	URL       string `json:"url,omitempty"`
	Notes     string `json:"notes,omitempty"`
	CheckedAt int64  `json:"checkedAt,omitempty"`
}

// Store keeps Settings in a file.
type Store struct {
	mu   sync.Mutex
	path string
	s    Settings
}

func NewStore() *Store {
	st := &Store{s: Settings{AutoCheck: true}}
	base, err := os.UserConfigDir()
	if err != nil {
		return st
	}
	dir := filepath.Join(base, "shelf")
	if os.MkdirAll(dir, 0o755) != nil {
		return st
	}
	st.path = filepath.Join(dir, "updates.json")
	if raw, err := os.ReadFile(st.path); err == nil {
		saved := Settings{AutoCheck: true}
		if json.Unmarshal(raw, &saved) == nil {
			st.s = saved
		}
	}
	return st
}

func (st *Store) get() Settings {
	st.mu.Lock()
	defer st.mu.Unlock()
	return st.s
}

func (st *Store) update(f func(*Settings)) error {
	st.mu.Lock()
	defer st.mu.Unlock()
	next := st.s
	f(&next)
	if st.path != "" {
		raw, err := json.MarshalIndent(next, "", "  ")
		if err != nil {
			return err
		}
		tmp := st.path + ".tmp"
		if err := os.WriteFile(tmp, raw, 0o644); err != nil {
			return err
		}
		if err := os.Rename(tmp, st.path); err != nil {
			return err
		}
	}
	st.s = next
	return nil
}

// Status is what the interface shows.
type Status struct {
	Current     string `json:"current"`
	Supported   bool   `json:"supported"` // a release build, which has a version to compare
	AutoCheck   bool   `json:"autoCheck"`
	Latest      string `json:"latest,omitempty"`
	Available   bool   `json:"available"` // a newer release exists
	Skipped     bool   `json:"skipped"`   // ...but the user said not to mention it
	URL         string `json:"url,omitempty"`
	Notes       string `json:"notes,omitempty"`
	CheckedAt   int64  `json:"checkedAt,omitempty"`
	CanInstall  bool   `json:"canInstall"`
	InstallNote string `json:"installNote,omitempty"` // why it can't install itself
}

// Updater checks for releases and installs them.
type Updater struct {
	current string
	repo    string
	api     string // GitHub's API address; tests point it elsewhere
	allow   string // downloads must start with this
	client  *http.Client
	store   *Store

	getenv func(string) string
	exe    func() (string, error)
}

// New makes an updater for the running version, such as "v0.4.1". A version
// that isn't a release ("dev", a git describe) never checks.
func New(current, repo string, store *Store) *Updater {
	return &Updater{
		current: current,
		repo:    repo,
		api:     "https://api.github.com",
		allow:   "https://github.com/" + repo + "/releases/download/",
		client:  &http.Client{Timeout: 30 * time.Second},
		store:   store,
		getenv:  os.Getenv,
		exe:     os.Executable,
	}
}

func (u *Updater) supported() bool {
	_, ok := parseVersion(u.current)
	return ok && (runtime.GOOS == "linux" || runtime.GOOS == "darwin")
}

// Status describes the current state without asking the network.
func (u *Updater) Status() Status {
	s := u.store.get()
	st := Status{
		Current:   u.current,
		Supported: u.supported(),
		AutoCheck: s.AutoCheck,
		Latest:    s.Latest,
		URL:       s.URL,
		Notes:     s.Notes,
		CheckedAt: s.CheckedAt,
	}
	st.Available = st.Supported && Newer(s.Latest, u.current)
	st.Skipped = st.Available && s.Skipped == s.Latest
	if st.Available {
		if _, why := u.target(); why != "" {
			st.InstallNote = why
		} else {
			st.CanInstall = true
		}
	}
	return st
}

type release struct {
	Tag    string `json:"tag_name"`
	URL    string `json:"html_url"`
	Body   string `json:"body"`
	Assets []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func (u *Updater) latest(ctx context.Context) (release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.api+"/repos/"+u.repo+"/releases/latest", nil)
	if err != nil {
		return release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "shelf/"+u.current)
	resp, err := u.client.Do(req)
	if err != nil {
		return release{}, fmt.Errorf("couldn't reach GitHub: %w", err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return release{}, errors.New("there is no release yet")
	case http.StatusForbidden, http.StatusTooManyRequests:
		return release{}, errors.New("GitHub asked us to slow down. Try again later.")
	default:
		return release{}, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	var r release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&r); err != nil {
		return release{}, fmt.Errorf("GitHub sent something unexpected: %w", err)
	}
	if _, ok := parseVersion(r.Tag); !ok {
		return release{}, fmt.Errorf("unexpected release name %q", r.Tag)
	}
	return r, nil
}

// Check asks GitHub for the newest release and remembers the answer.
func (u *Updater) Check(ctx context.Context) (Status, error) {
	if !u.supported() {
		return u.Status(), nil
	}
	r, err := u.latest(ctx)
	if err != nil {
		return u.Status(), err
	}
	notes := r.Body
	if len(notes) > 4000 {
		notes = notes[:4000] + "…"
	}
	err = u.store.update(func(s *Settings) {
		s.Latest, s.URL, s.Notes, s.CheckedAt = r.Tag, r.URL, notes, time.Now().Unix()
	})
	return u.Status(), err
}

// DueForCheck reports whether an automatic check should run now.
func (u *Updater) DueForCheck(every time.Duration) bool {
	s := u.store.get()
	return u.supported() && s.AutoCheck && time.Since(time.Unix(s.CheckedAt, 0)) >= every
}

func (u *Updater) Skip(tag string) error {
	return u.store.update(func(s *Settings) { s.Skipped = tag })
}

func (u *Updater) SetAutoCheck(on bool) error {
	return u.store.update(func(s *Settings) { s.AutoCheck = on })
}

type installTarget struct {
	path  string // the file to replace
	asset string // the release file to download
}

// target finds what to replace. If this copy can't update itself, the second
// result says why, in words for the user.
func (u *Updater) target() (installTarget, string) {
	if runtime.GOOS == "darwin" {
		// An app bundle is swapped by hand: macOS checks it when it is first opened.
		return installTarget{}, "Download the new Shelf.app from the release page and replace the old one."
	}
	if runtime.GOARCH != "amd64" {
		return installTarget{}, "Releases are only built for x86_64."
	}
	t := installTarget{asset: binaryAsset}
	if img := u.getenv("APPIMAGE"); img != "" {
		t = installTarget{path: img, asset: appImageAsset}
	} else {
		exe, err := u.exe()
		if err != nil {
			return installTarget{}, "Shelf can't tell where it is installed."
		}
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		t.path = exe
		if strings.HasPrefix(exe, "/usr/") || strings.HasPrefix(exe, "/opt/") {
			return installTarget{}, "Shelf was installed by your package manager, so update it there."
		}
	}
	f, err := os.CreateTemp(filepath.Dir(t.path), ".shelf-write-test-*")
	if err != nil {
		return installTarget{}, "Shelf can't write to " + filepath.Dir(t.path) + ", so update it the way you installed it."
	}
	f.Close()
	os.Remove(f.Name())
	return t, ""
}

// Install downloads the newest release and replaces the running copy. The new
// version runs after the next start. It returns the tag it installed.
func (u *Updater) Install(ctx context.Context, progress func(done, total int64)) (string, error) {
	st := u.Status()
	if !st.Available {
		return "", errors.New("there is no newer release")
	}
	t, why := u.target()
	if why != "" {
		return "", errors.New(why)
	}
	r, err := u.latest(ctx)
	if err != nil {
		return "", err
	}
	if !Newer(r.Tag, u.current) {
		return "", errors.New("there is no newer release")
	}

	var assetURL, sumURL string
	for _, a := range r.Assets {
		switch a.Name {
		case t.asset:
			assetURL = a.URL
		case t.asset + ".sha256":
			sumURL = a.URL
		}
	}
	if assetURL == "" || sumURL == "" {
		return "", fmt.Errorf("release %s has no %s with a checksum", r.Tag, t.asset)
	}
	for _, link := range []string{assetURL, sumURL} {
		if !strings.HasPrefix(link, u.allow) {
			return "", fmt.Errorf("refusing to download from %s", link)
		}
	}

	want, err := u.fetchChecksum(ctx, sumURL)
	if err != nil {
		return "", err
	}

	dl, err := os.CreateTemp(filepath.Dir(t.path), ".shelf-update-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(dl.Name())
	got, err := u.download(ctx, assetURL, dl, progress)
	dl.Close()
	if err != nil {
		return "", err
	}
	if got != want {
		return "", errors.New("the download doesn't match its checksum, so it was thrown away")
	}

	newFile := dl.Name()
	if t.asset == binaryAsset {
		if newFile, err = extractBinary(dl.Name(), filepath.Dir(t.path)); err != nil {
			return "", err
		}
		defer os.Remove(newFile)
	}
	if err := checkELF(newFile); err != nil {
		return "", err
	}
	if err := os.Chmod(newFile, 0o755); err != nil {
		return "", err
	}
	// A rename replaces the file in one step, even while it runs.
	if err := os.Rename(newFile, t.path); err != nil {
		return "", err
	}
	return r.Tag, nil
}

func (u *Updater) fetchChecksum(ctx context.Context, link string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return "", err
	}
	resp, err := u.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("couldn't fetch the checksum: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("couldn't fetch the checksum: %s", resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", err
	}
	sum := strings.ToLower(strings.Fields(string(raw) + " ")[0])
	if len(sum) != 64 {
		return "", errors.New("the published checksum is malformed")
	}
	return sum, nil
}

func (u *Updater) download(ctx context.Context, link string, dst io.Writer, progress func(done, total int64)) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, link, nil)
	if err != nil {
		return "", err
	}
	// A download is slow; the client's overall timeout would cut it short.
	client := *u.client
	client.Timeout = 0
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("couldn't download the update: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("couldn't download the update: %s", resp.Status)
	}
	if resp.ContentLength > maxDownload {
		return "", errors.New("the update is bigger than expected")
	}

	h := sha256.New()
	var done int64
	buf := make([]byte, 64<<10)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			done += int64(n)
			if done > maxDownload {
				return "", errors.New("the update is bigger than expected")
			}
			h.Write(buf[:n])
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return "", werr
			}
			if progress != nil {
				progress(done, resp.ContentLength)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("the download broke off: %w", err)
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// extractBinary pulls the shelf program out of the release tarball, whose
// entries may or may not start with "./".
func extractBinary(tarball, dir string) (string, error) {
	f, err := os.Open(tarball)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("the update isn't a valid archive: %w", err)
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return "", errors.New("the archive has no shelf program in it")
		}
		if err != nil {
			return "", fmt.Errorf("the update isn't a valid archive: %w", err)
		}
		if h.Typeflag != tar.TypeReg || path.Base(path.Clean(h.Name)) != "shelf" {
			continue
		}
		out, err := os.CreateTemp(dir, ".shelf-binary-*")
		if err != nil {
			return "", err
		}
		_, err = io.Copy(out, io.LimitReader(tr, maxDownload))
		out.Close()
		if err != nil {
			os.Remove(out.Name())
			return "", err
		}
		return out.Name(), nil
	}
}

// checkELF makes sure what is about to replace Shelf is a Linux program.
func checkELF(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	magic := make([]byte, 4)
	if _, err := io.ReadFull(f, magic); err != nil || string(magic) != "\x7fELF" {
		return errors.New("the update isn't a program, so it was not installed")
	}
	return nil
}

// RelaunchPath is what to start again after an update.
func (u *Updater) RelaunchPath() (string, error) {
	if img := u.getenv("APPIMAGE"); img != "" {
		return img, nil
	}
	exe, err := u.exe()
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(exe)
}
