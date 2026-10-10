package epic

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"shelf/internal/library"
)

// gogInstall is a game Shelf installed from GOG, as kept in gog-installed.json.
type gogInstall struct {
	Title       string `json:"title"`
	InstallPath string `json:"installPath"`
	Version     string `json:"version"`
	InstallSize int64  `json:"installSize"`
	InstalledAt int64  `json:"installedAt"`
}

var gogInstalledMu sync.Mutex

func gogInstalledPath() string {
	if dir := configDir(); dir != "" {
		return filepath.Join(dir, "gog-installed.json")
	}
	return ""
}

// gogInstalled lists the GOG games Shelf installed, by key, leaving out any
// whose folder has since disappeared.
func (m *Manager) gogInstalled() map[string]gogInstall {
	gogInstalledMu.Lock()
	defer gogInstalledMu.Unlock()
	out := map[string]gogInstall{}
	data, err := os.ReadFile(gogInstalledPath())
	if err != nil {
		return out
	}
	var raw map[string]gogInstall
	if json.Unmarshal(data, &raw) != nil {
		return out
	}
	for key, g := range raw {
		if !gogKeyRe.MatchString(key) {
			continue
		}
		if st, err := os.Stat(g.InstallPath); err != nil || !st.IsDir() {
			continue
		}
		out[key] = g
	}
	return out
}

// setGogInstalled records a game as installed, or forgets it when g is nil.
func setGogInstalled(key string, g *gogInstall) error {
	gogInstalledMu.Lock()
	defer gogInstalledMu.Unlock()
	all := map[string]gogInstall{}
	if data, err := os.ReadFile(gogInstalledPath()); err == nil {
		_ = json.Unmarshal(data, &all)
	}
	if g == nil {
		delete(all, key)
	} else {
		all[key] = *g
	}
	return writeJSON(gogInstalledPath(), all)
}

// gogProductDetails is the part of GOG's product API that says what to download.
type gogProductDetails struct {
	Title     string `json:"title"`
	Downloads struct {
		Installers []gogInstaller `json:"installers"`
	} `json:"downloads"`
}

type gogInstaller struct {
	ID        string `json:"id"`
	OS        string `json:"os"`
	Language  string `json:"language"`
	Version   string `json:"version"`
	TotalSize int64  `json:"total_size"`
	Files     []struct {
		ID       string `json:"id"`
		Size     int64  `json:"size"`
		Downlink string `json:"downlink"`
	} `json:"files"`
}

// pickGogInstaller chooses the Windows installer, in English when there is one.
func pickGogInstaller(list []gogInstaller) (gogInstaller, bool) {
	var first *gogInstaller
	for i, in := range list {
		if in.OS != "windows" || len(in.Files) == 0 {
			continue
		}
		if in.Language == "en" {
			return in, true
		}
		if first == nil {
			first = &list[i]
		}
	}
	if first != nil {
		return *first, true
	}
	return gogInstaller{}, false
}

// gogFolderRe keeps folder names to what every file system and Wine accept.
var gogFolderRe = regexp.MustCompile(`[^A-Za-z0-9 ._()+&'-]+`)

// gogFolderName is the install folder for a game: its title, made safe.
func gogFolderName(title, key string) string {
	name := strings.Join(strings.Fields(gogFolderRe.ReplaceAllString(title, " ")), " ")
	name = strings.Trim(name, " .")
	if name == "" {
		return key
	}
	return name
}

// gogSetupArgs are the Inno Setup switches that install a game without a
// window into dir, logging to log. Both are paths as the installer's Windows
// sees them.
func gogSetupArgs(dir, log string) []string {
	return []string{
		"/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART", "/SP-", "/NOICONS",
		"/DIR=" + dir,
		"/LOG=" + log,
	}
}

// winPath is how Wine sees a Linux path: through drive Z:.
func winPath(p string) string { return `Z:` + strings.ReplaceAll(p, "/", `\`) }

// gogInfoFile is the goggame-<id>.info every GOG install has, naming what to run.
func gogInfoFile(dir, key string) string {
	return filepath.Join(dir, "goggame-"+strings.TrimPrefix(key, "gog-")+".info")
}

// GogInstall downloads a game's Windows installer and runs it, through Proton on Linux.
// It waits its turn in the downloads queue like any other install.
func (m *Manager) GogInstall(key string) error {
	if macOS {
		return errNeedsProton("Installing GOG games from their Windows installers")
	}
	if !gogKeyRe.MatchString(key) {
		return fmt.Errorf("invalid game id")
	}
	if !m.GogAccount().LoggedIn {
		return fmt.Errorf("not signed in to GOG")
	}
	if _, ok := m.gogInstalled()[key]; ok {
		return fmt.Errorf("already installed")
	}
	game, ok := m.gogProductByKey(key)
	if !ok {
		return fmt.Errorf("this game isn't in your GOG library")
	}
	if _, ok := resolveProton(m.settings.get().ProtonPath); useProton && !ok {
		return fmt.Errorf("no Proton installation found. Install Proton through Steam or ProtonUp-Qt")
	}
	base := m.settings.get().InstallDir
	if err := os.MkdirAll(base, 0o755); err != nil {
		return fmt.Errorf("install folder: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	job := &installJob{cancel: cancel, p: Progress{AppName: key, Kind: KindInstall, State: StateQueued}}
	job.run = func() {
		job.mu.Lock()
		job.p.State = StateInstalling
		job.mu.Unlock()
		m.send("epic:install", job.snapshot())

		err := m.runGogInstall(ctx, job, game, base)
		switch {
		case ctx.Err() != nil:
			m.finishJob(job, StateCancelled, "", false)
		case err != nil:
			m.finishJob(job, StateFailed, err.Error(), false)
		default:
			m.finishJob(job, StateDone, "", false)
		}
	}
	return m.enqueue(job, true)
}

// runGogInstall does the work of an install: pick a folder, then download and
// run the installer into it.
func (m *Manager) runGogInstall(ctx context.Context, job *installJob, game gogProduct, base string) error {
	key := game.key()
	dir := filepath.Join(base, gogFolderName(game.Title, key))
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		if _, err := os.Stat(gogInfoFile(dir, key)); err != nil {
			// Something else lives there already; don't install over it.
			dir = filepath.Join(base, gogFolderName(game.Title, key)+" ("+key+")")
		}
	}
	return m.runGogInstaller(ctx, job, KindInstall, game, base, dir, time.Now().Unix())
}

// runGogInstaller downloads the current installer, checks it, runs it into
// dir and records the game. An update is the same thing over the old folder:
// GOG's installers replace what an older version left there.
func (m *Manager) runGogInstaller(ctx context.Context, job *installJob, kind string, game gogProduct, base, dir string, installedAt int64) error {
	key := game.key()
	inst, err := m.gogCurrentInstaller(ctx, game.ID)
	if err != nil {
		return err
	}
	m.logf(kind, key, "installer %s (%s, version %s, %d files)", inst.ID, inst.Language, inst.Version, len(inst.Files))

	// The download sits beside the games, so it is on the same disk.
	dlDir := filepath.Join(base, ".shelf-downloads", key)
	if err := os.MkdirAll(dlDir, 0o755); err != nil {
		return err
	}
	var total int64
	for _, f := range inst.Files {
		total += f.Size
	}
	prog := &gogProgress{m: m, job: job, total: total, start: time.Now()}

	var setup string
	for _, f := range inst.Files {
		name, err := m.gogDownload(ctx, f.Downlink, dlDir, f.Size, prog)
		if err != nil {
			return err
		}
		if setup == "" && strings.HasSuffix(strings.ToLower(name), ".exe") {
			setup = filepath.Join(dlDir, name)
		}
	}
	if setup == "" {
		return fmt.Errorf("the download has no installer program in it")
	}

	job.mu.Lock()
	job.p.Percent, job.p.ETA, job.p.Indeterminate, job.p.Speed = 100, "", true, "Running the installer"
	job.mu.Unlock()
	m.send("epic:install", job.snapshot())
	if err := m.runGogSetup(ctx, kind, key, setup, dir); err != nil {
		return err
	}
	if _, err := os.Stat(gogInfoFile(dir, key)); err != nil {
		return fmt.Errorf("the installer finished without installing the game. See the log for details")
	}

	size := diskUsage(ctx, dir)
	if err := setGogInstalled(key, &gogInstall{
		Title:       game.Title,
		InstallPath: dir,
		Version:     inst.Version,
		InstallSize: size,
		InstalledAt: installedAt,
	}); err != nil {
		return err
	}
	setGogLatest(key, inst.Version)
	// The installer files aren't needed any more.
	if err := os.RemoveAll(dlDir); err != nil {
		m.logf(kind, key, "couldn't remove the downloaded installer: %v", err)
	}
	m.logf(kind, key, "installed version %s to %s", inst.Version, dir)
	return nil
}

// gogCurrentInstaller asks GOG's product API for the installer Shelf uses.
func (m *Manager) gogCurrentInstaller(ctx context.Context, id int64) (gogInstaller, error) {
	var details gogProductDetails
	if err := m.gogGet(ctx, fmt.Sprintf("%s/products/%d?expand=downloads", gogAPIBase, id), false, &details); err != nil {
		return gogInstaller{}, err
	}
	inst, ok := pickGogInstaller(details.Downloads.Installers)
	if !ok {
		return gogInstaller{}, fmt.Errorf("GOG has no Windows installer for this game")
	}
	return inst, nil
}

// gogProgress turns bytes downloaded into the queue's progress, speed and ETA.
type gogProgress struct {
	m     *Manager
	job   *installJob
	total int64
	done  int64
	start time.Time
	last  time.Time

	// for the speed shown: bytes at the last sample
	sampleAt    time.Time
	sampleBytes int64
	speed       float64
}

func (p *gogProgress) add(n int64) {
	p.done += n
	now := time.Now()
	if now.Sub(p.last) < 250*time.Millisecond && p.done < p.total {
		return
	}
	p.last = now
	if p.sampleAt.IsZero() {
		p.sampleAt, p.sampleBytes = now, p.done
	} else if dt := now.Sub(p.sampleAt).Seconds(); dt >= 1 {
		inst := float64(p.done-p.sampleBytes) / dt
		if p.speed == 0 {
			p.speed = inst
		} else {
			p.speed = 0.7*p.speed + 0.3*inst
		}
		p.sampleAt, p.sampleBytes = now, p.done
	}

	p.job.mu.Lock()
	if p.total > 0 {
		p.job.p.Percent = float64(p.done) * 100 / float64(p.total)
	}
	if p.speed > 0 {
		p.job.p.Speed = formatRate(p.speed)
		p.job.p.ETA = formatETA(time.Duration(float64(p.total-p.done) / p.speed * float64(time.Second)))
	}
	p.job.mu.Unlock()
	p.m.send("epic:install", p.job.snapshot())
}

func formatRate(bytesPerSec float64) string {
	mib := bytesPerSec / (1 << 20)
	if mib >= 1 {
		return fmt.Sprintf("%.1f MiB/s", mib)
	}
	return fmt.Sprintf("%.0f KiB/s", bytesPerSec/(1<<10))
}

// formatETA writes a duration the way legendary does, so the queue looks the same: 01:02:03.
func formatETA(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	s := int64(d.Seconds())
	return fmt.Sprintf("%02d:%02d:%02d", s/3600, s/60%60, s%60)
}

// gogDownlink is where GOG's API sends a request for one installer file.
type gogDownlink struct {
	Downlink string `json:"downlink"`
	Checksum string `json:"checksum"`
}

// gogFileNameRe is what an installer file may be called on disk.
var gogFileNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._()+,-]*$`)

// gogDownloadClient has no overall time limit: a file can take hours. Stalls
// are caught by the context and the transport's own timeouts.
var gogDownloadClient = &http.Client{Transport: &http.Transport{
	Proxy:                 http.ProxyFromEnvironment,
	ResponseHeaderTimeout: 60 * time.Second,
	IdleConnTimeout:       90 * time.Second,
}}

// gogDownload fetches one installer file into dir, carrying on from where an
// earlier attempt stopped, and checks it against GOG's checksum. It returns
// the file's name, which the installer relies on to find its other parts.
func (m *Manager) gogDownload(ctx context.Context, downlink, dir string, size int64, prog *gogProgress) (string, error) {
	if !strings.HasPrefix(downlink, gogAPIBase+"/") {
		return "", fmt.Errorf("unexpected download address from GOG")
	}
	var link gogDownlink
	if err := m.gogGet(ctx, downlink, true, &link); err != nil {
		return "", err
	}
	u, err := url.Parse(link.Downlink)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return "", fmt.Errorf("unexpected download address from GOG")
	}
	name, _ := url.PathUnescape(path.Base(u.Path))
	if !gogFileNameRe.MatchString(name) {
		return "", fmt.Errorf("unexpected file name in the download: %q", name)
	}
	dest := filepath.Join(dir, name)

	want := m.gogChecksum(ctx, link.Checksum)
	have := int64(0)
	if st, err := os.Stat(dest); err == nil {
		have = st.Size()
	}
	if size > 0 && have > size {
		os.Remove(dest)
		have = 0
	}
	prog.add(have)

	if size == 0 || have < size {
		if err := m.gogFetch(ctx, link.Downlink, dest, have, prog); err != nil {
			return "", err
		}
	}

	if want != "" {
		got, err := fileMD5(dest)
		if err != nil {
			return "", err
		}
		if !strings.EqualFold(got, want) {
			os.Remove(dest)
			return "", fmt.Errorf("%s was damaged on the way. Install again to download it afresh", name)
		}
	}
	return name, nil
}

// gogFetch downloads src into dest, asking only for the bytes after offset.
func (m *Manager) gogFetch(ctx context.Context, src, dest string, offset int64, prog *gogProgress) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return err
	}
	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	resp, err := gogDownloadClient.Do(req)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	flags := os.O_CREATE | os.O_WRONLY
	switch resp.StatusCode {
	case http.StatusPartialContent:
		flags |= os.O_APPEND
	case http.StatusOK:
		// The server sent everything; start over.
		flags |= os.O_TRUNC
		prog.add(-offset)
	default:
		return fmt.Errorf("download: GOG's server answered %s", resp.Status)
	}
	f, err := os.OpenFile(dest, flags, 0o644)
	if err != nil {
		return err
	}
	buf := make([]byte, 256<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				return werr
			}
			prog.add(int64(n))
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("download: %w", rerr)
		}
	}
	return f.Close()
}

// gogChecksum reads the MD5 GOG publishes for a file. An empty answer means
// there is nothing to check against, which GOG does for some older files.
func (m *Manager) gogChecksum(ctx context.Context, src string) string {
	if src == "" {
		return ""
	}
	resp, err := m.gogRequest(ctx, src, false)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var doc struct {
		MD5 string `xml:"md5,attr"`
	}
	if xml.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&doc) != nil {
		return ""
	}
	if len(doc.MD5) != 32 {
		return ""
	}
	return doc.MD5
}

func fileMD5(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// runGogSetup runs a GOG installer without any windows, into dir, inside the
// game's own Proton prefix. GOG's installers are Inno Setup programs.
func (m *Manager) runGogSetup(ctx context.Context, kind, key, setup, dir string) error {
	if onWindows {
		log := filepath.Join(filepath.Dir(setup), "setup.log")
		return m.runInstaller(ctx, kind, key, setup, gogSetupArgs(dir, log), filepath.Dir(setup))
	}
	build, ok := resolveProton(m.settings.get().ProtonPath)
	if !ok {
		return fmt.Errorf("no Proton installation found")
	}
	prefix := prefixDir(key)
	if err := os.MkdirAll(prefix, 0o755); err != nil {
		return err
	}
	log := winPath(filepath.Join(filepath.Dir(setup), "setup.log"))
	args := append([]string{"run", setup}, gogSetupArgs(winPath(dir), log)...)
	cmd := exec.Command(filepath.Join(build.Path, "proton"), args...)
	cmd.Env = append(library.ChildEnv(), protonEnv(build, key, dir)...)
	cmd.Env = append(cmd.Env, "PROTON_USE_XALIA=0")
	ownGroup(cmd)
	m.logf(kind, key, "proton: %s, prefix: %s", build.Name, prefix)

	// Cancelling stops the installer and everything it started in the prefix.
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			stopPrefixProcesses(prefix)
			killGroup(cmd)
		case <-done:
		}
	}()

	if err := m.runLogged(cmd, kind, key); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("the installer failed: %v. See the log for details", err)
	}
	// Some installers leave helpers running a moment; they would count as playing.
	stopPrefixProcesses(prefix)
	return nil
}

// GogUninstall removes a game's files. Its Proton prefix, with any saves kept
// there, stays until it is deleted under Storage.
func (m *Manager) GogUninstall(key string) error {
	if !gogKeyRe.MatchString(key) {
		return fmt.Errorf("invalid game id")
	}
	g, ok := m.gogInstalled()[key]
	if !ok {
		return fmt.Errorf("game is not installed")
	}
	if m.isRunningGame(key) {
		return fmt.Errorf("close the game first")
	}
	m.mu.Lock()
	_, busy := m.installs[key]
	m.mu.Unlock()
	if busy {
		return fmt.Errorf("game is busy")
	}

	// Only ever delete a folder that is plainly this game's.
	dir := filepath.Clean(g.InstallPath)
	home, _ := os.UserHomeDir()
	volumeRoot := filepath.VolumeName(dir) + string(filepath.Separator)
	if !filepath.IsAbs(dir) || dir == "/" || dir == volumeRoot || dir == home || dir == filepath.Clean(m.settings.get().InstallDir) {
		return fmt.Errorf("refusing to delete %s", dir)
	}
	if _, err := os.Stat(gogInfoFile(dir, key)); err != nil {
		return fmt.Errorf("%s doesn't look like this game's folder, so Shelf left it alone", dir)
	}
	if onWindows {
		// The installer registered the game with Windows; its uninstaller
		// takes that back. What it leaves behind goes with the folder.
		if unins := filepath.Join(dir, "unins000.exe"); fileExists(unins) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			err := m.runInstaller(ctx, "uninstall", key, unins, []string{"/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART"}, dir)
			cancel()
			if err != nil {
				m.logf("uninstall", key, "the game's uninstaller failed: %v", err)
			}
		}
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := setGogInstalled(key, nil); err != nil {
		return err
	}
	m.logf("uninstall", key, "removed %s", dir)
	m.send("library:changed", nil)
	return nil
}
