package epic

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"shelf/internal/library"
)

// Ubisoft Connect is Ubisoft's launcher and the only thing that can download
// and start its games. Shelf runs it the way Heroic runs launchers: installed
// once, silently, into a Proton prefix of its own that all Ubisoft games share.
// The account Shelf signs in to (see ubisoft_api.go) provides the library;
// Connect itself is signed in once by the user, in its own window, and from
// then on installs and starts games when Shelf hands it a uplay:// address.

// ubisoftStore is the launcher a Ubisoft game is installed through.
const ubisoftStore = "Ubisoft Connect"

// ubisoftSource tags log lines and the setup events.
const ubisoftSource = "ubisoft"

// ubisoftPrefixName is the prefix folder all Ubisoft games share.
const ubisoftPrefixName = "ubisoft-connect"

// ubisoftInstallerURL is Ubisoft's own download for the Connect installer.
// It is a variable so tests can point it elsewhere.
var ubisoftInstallerURL = "https://static3.cdn.ubi.com/orbit/launcher_installer/UbisoftConnectInstaller.exe"

// UbisoftStatus is what the UI needs to guide the user through the setup.
type UbisoftStatus struct {
	ConnectInstalled bool   `json:"connectInstalled"`
	Proton           string `json:"proton"` // build that will run it, empty if none found
	ProtonIsGE       bool   `json:"protonIsGE"`
	Running          bool   `json:"running"` // Ubisoft Connect or one of its games is open
	// SignedIn is true once an account has signed in to Connect, which then records what it owns.
	SignedIn bool   `json:"signedIn"`
	Account  string `json:"account"` // Ubisoft account id, from Connect's files
	Games    int    `json:"games"`   // owned games Connect lists
}

func (m *Manager) UbisoftStatus() UbisoftStatus {
	st := UbisoftStatus{ConnectInstalled: connectInstalled()}
	st.Running = st.ConnectInstalled && len(prefixProcesses(ubisoftPrefix())) > 0
	if st.ConnectInstalled {
		local := m.readConnect()
		st.SignedIn, st.Account, st.Games = local.account != "", local.account, len(local.games)
	}
	if b, ok := resolveProton(m.settings.get().ProtonPath); ok {
		st.Proton, st.ProtonIsGE = b.Name, strings.HasPrefix(b.Name, "GE-Proton")
	}
	return st
}

// UbisoftSetupState is the progress of installing Ubisoft Connect.
type UbisoftSetupState struct {
	State   string  `json:"state"` // "", "running", "done" or "failed"
	Percent float64 `json:"percent"`
	Message string  `json:"message"`
	Error   string  `json:"error,omitempty"`
}

func (m *Manager) UbisoftSetupState() UbisoftSetupState {
	m.ubiMu.Lock()
	defer m.ubiMu.Unlock()
	return m.ubiSetup
}

func (m *Manager) setSetup(s UbisoftSetupState) {
	m.ubiMu.Lock()
	m.ubiSetup = s
	m.ubiMu.Unlock()
	m.send("ubisoft:setup", s)
}

// UbisoftSetup downloads Ubisoft's installer and runs it silently in the
// shared prefix. It can be run again to repair a broken install.
func (m *Manager) UbisoftSetup() error {
	if macOS {
		return errNeedsProton("Ubisoft Connect")
	}
	m.ubiMu.Lock()
	if m.ubiSetup.State == "running" {
		m.ubiMu.Unlock()
		return fmt.Errorf("setup is already running")
	}
	m.ubiSetup = UbisoftSetupState{State: "running", Message: "Starting"}
	m.ubiMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		report := func(pct float64, msg string) {
			m.setSetup(UbisoftSetupState{State: "running", Percent: pct, Message: msg})
		}
		report(0, "Starting")
		if err := m.setupConnect(ctx, report); err != nil {
			m.logf(ubisoftSource, "", "setup failed: %v", err)
			m.setSetup(UbisoftSetupState{State: "failed", Error: err.Error()})
			return
		}
		m.setSetup(UbisoftSetupState{State: "done", Percent: 100, Message: "Ubisoft Connect is installed"})
		m.send("library:changed", nil)
	}()
	return nil
}

func (m *Manager) setupConnect(ctx context.Context, report func(float64, string)) error {
	build, ok := resolveProton(m.settings.get().ProtonPath)
	if !ok {
		return fmt.Errorf("no Proton found. Install GE-Proton through ProtonUp-Qt, or Proton through Steam")
	}

	installer := installerCache()
	report(0, "Downloading")
	if err := m.downloadInstaller(ctx, installer, func(frac float64) { report(frac*60, "Downloading") }); err != nil {
		return err
	}

	if err := os.MkdirAll(ubisoftPrefix(), 0o755); err != nil {
		return err
	}
	report(62, "Installing")
	m.logf(ubisoftSource, "", "proton: %s, prefix: %s", build.Name, ubisoftPrefix())

	cmd := exec.CommandContext(ctx, filepath.Join(build.Path, "proton"), "run", installer, "/S")
	cmd.Env = append(library.ChildEnv(), connectEnv(build, filepath.Dir(installer), false)...)
	if err := m.runLogged(cmd, ubisoftSource, ""); err != nil {
		return fmt.Errorf("the installer failed: %w", err)
	}
	if !connectInstalled() {
		return fmt.Errorf("the installer finished but Ubisoft Connect isn't in the prefix. See the log for details")
	}
	m.logf(ubisoftSource, "", "Ubisoft Connect is installed")
	return nil
}

func ubisoftPrefix() string { return filepath.Join(prefixRoot(), ubisoftPrefixName) }

func connectExe() string {
	return filepath.Join(ubisoftPrefix(), "pfx", "drive_c", "Program Files (x86)",
		"Ubisoft", "Ubisoft Game Launcher", "UbisoftConnect.exe")
}

func connectInstalled() bool {
	st, err := os.Stat(connectExe())
	return err == nil && !st.IsDir()
}

func installerCache() string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "shelf", "UbisoftConnectInstaller.exe")
}

func (m *Manager) downloadInstaller(ctx context.Context, dest string, progress func(float64)) error {
	if looksLikeInstaller(dest) {
		m.logf(ubisoftSource, "", "using the installer downloaded earlier")
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ubisoftInstallerURL, nil)
	if err != nil {
		return err
	}
	m.logf(ubisoftSource, "", "downloading %s", ubisoftInstallerURL)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("couldn't download the installer: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("couldn't download the installer: %s", resp.Status)
	}

	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	var done int64
	total := resp.ContentLength
	buf := make([]byte, 256*1024)
	last := time.Now()
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				os.Remove(tmp)
				return werr
			}
			done += int64(n)
			if total > 0 && time.Since(last) > 250*time.Millisecond {
				last = time.Now()
				progress(float64(done) / float64(total))
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			os.Remove(tmp)
			return fmt.Errorf("the download was interrupted: %w", rerr)
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	if total > 0 && done != total {
		os.Remove(tmp)
		return fmt.Errorf("the download was cut short")
	}
	if err := os.Rename(tmp, dest); err != nil {
		return err
	}
	if !looksLikeInstaller(dest) {
		os.Remove(dest)
		return fmt.Errorf("what was downloaded isn't a Windows installer")
	}
	progress(1)
	return nil
}

// looksLikeInstaller checks for a Windows executable of a plausible size.
func looksLikeInstaller(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.Size() < minInstallerSize {
		return false
	}
	magic := make([]byte, 2)
	_, err = io.ReadFull(f, magic)
	return err == nil && string(magic) == "MZ"
}

// minInstallerSize guards against error pages saved as the installer. It is a
// variable so tests don't need a 200 MB file.
var minInstallerSize int64 = 10 << 20

// runLogged runs a command, copying its output into the log, and waits for it.
func (m *Manager) runLogged(cmd *exec.Cmd, source, app string) error {
	r, w, err := os.Pipe()
	if err != nil {
		return err
	}
	defer r.Close()
	cmd.Stdout, cmd.Stderr = w, w
	m.logf(source, app, "$ %s", strings.Join(cmd.Args, " "))
	if err := cmd.Start(); err != nil {
		w.Close()
		return err
	}
	w.Close()

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		m.log.Add(source, app, sc.Text())
	}
	return cmd.Wait()
}

// UbisoftReset deletes the Ubisoft Connect prefix: the login, and every game
// installed inside it. Epic and cloud data are untouched.
func (m *Manager) UbisoftReset() error {
	if len(m.runningUbisoft()) > 0 {
		return fmt.Errorf("a Ubisoft game is running; close it first")
	}
	// Connect itself may be open, and would write into a prefix being deleted.
	stopPrefixProcesses(ubisoftPrefix())
	m.mu.Lock()
	m.connect, m.reg = nil, nil
	m.mu.Unlock()
	m.ubiMu.Lock()
	m.ubiCache, m.ubiSizes, m.ubiPending = nil, nil, nil
	m.ubiMu.Unlock()

	if err := m.DeletePrefix(ubisoftPrefixName); err != nil {
		return err
	}
	m.send("library:changed", nil)
	return nil
}

// --- running Ubisoft Connect and its games ---

func (m *Manager) ubisoftBuild() (ProtonBuild, error) {
	if !connectInstalled() {
		return ProtonBuild{}, fmt.Errorf("set up Ubisoft Connect first (Settings, Integrations, Ubisoft)")
	}
	build, ok := resolveProton(m.settings.get().ProtonPath)
	if !ok {
		return ProtonBuild{}, fmt.Errorf("no Proton installation found")
	}
	return build, nil
}

// UbisoftOpenConnect starts Ubisoft Connect so the user can log in or install
// games. While it runs, Shelf notices games being installed.
func (m *Manager) UbisoftOpenConnect() error {
	if macOS {
		return errNeedsProton("Ubisoft Connect")
	}
	build, err := m.ubisoftBuild()
	if err != nil {
		return err
	}
	m.mu.Lock()
	if m.connect != nil {
		m.mu.Unlock()
		return nil // already open
	}
	m.mu.Unlock()
	if len(m.runningUbisoft()) > 0 {
		return fmt.Errorf("a Ubisoft game is running; close it first")
	}
	// Anything left over from an earlier session would keep its old environment.
	stopPrefixProcesses(ubisoftPrefix())

	cmd := exec.Command(filepath.Join(build.Path, "proton"), "run", connectExe())
	cmd.Env = append(library.ChildEnv(), connectEnv(build, "", m.settings.get().UbisoftSoftwareRendering)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := m.startPumped(cmd, "ubisoft", ""); err != nil {
		return err
	}

	m.mu.Lock()
	m.connect = cmd
	m.mu.Unlock()
	go m.watchConnect(cmd)
	return nil
}

// watchConnect notices when Connect closes.
func (m *Manager) watchConnect(cmd *exec.Cmd) {
	m.startUbiWatch()
	_ = cmd.Wait()
	m.mu.Lock()
	if m.connect == cmd {
		m.connect = nil
	}
	m.mu.Unlock()
	m.send("library:changed", nil)
}

// connectFingerprint changes when a game is installed or removed, and when
// Connect records a new account or library.
func (m *Manager) connectFingerprint() string {
	own, _ := ownershipFile()
	return installsFingerprint(m.ubisoftInstalls()) + fileStamp(connectConfigPath()) + fileStamp(own)
}

func installsFingerprint(installs map[string]string) string {
	var b strings.Builder
	for id, dir := range installs {
		b.WriteString(id + "=" + dir + ";")
	}
	return b.String()
}

// startPumped starts a command whose output goes to the log window and, for games, a log file.
func (m *Manager) startPumped(cmd *exec.Cmd, source, app string) error {
	r, w, err := os.Pipe()
	if err != nil {
		return err
	}
	cmd.Stdout, cmd.Stderr = w, w
	m.logf(source, app, "$ %s", strings.Join(cmd.Args, " "))
	if err := cmd.Start(); err != nil {
		r.Close()
		w.Close()
		m.logf(source, app, "ERROR: %v", err)
		return err
	}
	w.Close()

	var file *os.File
	if app != "" {
		file, _ = openLog(app)
	}
	go func() {
		defer r.Close()
		if file != nil {
			defer file.Close()
		}
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			line := sc.Text()
			if file != nil {
				fmt.Fprintln(file, line)
			}
			m.log.Add(source, app, line)
		}
	}()
	return nil
}

// containsEnv reports whether a NUL-separated environment holds the given entry.
func containsEnv(environ, entry []byte) bool {
	for _, kv := range strings.Split(string(environ), "\x00") {
		if kv == string(entry) {
			return true
		}
	}
	return false
}

// --- environments and stopping ---

// connectEnv is the environment for anything that runs in the Ubisoft prefix.
// Xalia, Proton's gamepad helper, crashes on Connect's window and is of no use
// there. With softwareRendering, Direct3D 11 is switched off so that Connect's
// Chromium falls back to its bundled software renderer, which draws where the
// GPU path leaves the window black. Never use that for a game.
func connectEnv(build ProtonBuild, installPath string, softwareRendering bool) []string {
	env := append(protonEnvAt(build, ubisoftPrefix(), installPath), "PROTON_USE_XALIA=0")
	if softwareRendering {
		env = append(env, "PROTON_NO_D3D11=1")
	}
	return env
}

// prefixProcesses lists the processes that were started inside a prefix.
func prefixProcesses(prefix string) []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	want := "STEAM_COMPAT_DATA_PATH=" + prefix
	var pids []int
	for _, e := range entries {
		n := e.Name()
		if n == "" || n[0] < '0' || n[0] > '9' {
			continue
		}
		env, err := os.ReadFile("/proc/" + n + "/environ")
		if err != nil || !containsEnv(env, []byte(want)) {
			continue
		}
		pid := 0
		fmt.Sscanf(n, "%d", &pid)
		if pid > 0 && pid != os.Getpid() {
			pids = append(pids, pid)
		}
	}
	return pids
}

// stopPrefixProcesses closes everything running in a prefix: politely first,
// then by force. It returns how many processes it found.
func stopPrefixProcesses(prefix string) int {
	pids := prefixProcesses(prefix)
	if len(pids) == 0 {
		return 0
	}
	for _, p := range pids {
		_ = syscall.Kill(p, syscall.SIGTERM)
	}
	for i := 0; i < 20 && len(prefixProcesses(prefix)) > 0; i++ {
		time.Sleep(200 * time.Millisecond)
	}
	for _, p := range prefixProcesses(prefix) {
		_ = syscall.Kill(p, syscall.SIGKILL)
	}
	for i := 0; i < 10 && len(prefixProcesses(prefix)) > 0; i++ {
		time.Sleep(100 * time.Millisecond)
	}
	return len(pids)
}

// UbisoftCloseConnect closes Ubisoft Connect and anything running inside it.
func (m *Manager) UbisoftCloseConnect() error {
	if len(m.runningUbisoft()) > 0 {
		return fmt.Errorf("a Ubisoft game is running; close the game first")
	}
	stopPrefixProcesses(ubisoftPrefix())
	m.mu.Lock()
	m.connect = nil
	m.mu.Unlock()
	return nil
}

// --- playing and installing ---

// hand gives a uplay:// address to Connect through `proton run start`, which
// passes it to Windows and so to the launcher, starting it if need be.
func (m *Manager) hand(build ProtonBuild, uri, installDir string, extra []string, gs GameSettings, app string) (*exec.Cmd, error) {
	wrapper, err := wrapperParts(gs, build)
	if err != nil {
		return nil, err
	}
	env, err := parseEnv(gs.Env)
	if err != nil {
		return nil, err
	}

	// Connect may be open from logging in, running with software rendering.
	// A game inherits its environment, so it has to start afresh. Another
	// Ubisoft game that is already running must not be disturbed.
	if len(m.runningUbisoft()) > 0 {
		m.logf("launch", app, "another Ubisoft game is running; leaving Ubisoft Connect as it is")
	} else if stopped := stopPrefixProcesses(ubisoftPrefix()); stopped > 0 {
		m.logf("launch", app, "restarted Ubisoft Connect in game mode (%d processes closed)", stopped)
		m.mu.Lock()
		m.connect = nil
		m.mu.Unlock()
	}

	args := append(wrapper[1:], "start", uri)
	cmd := exec.Command(wrapper[0], args...)
	cmd.Env = append(library.ChildEnv(), connectEnv(build, installDir, false)...)
	cmd.Env = append(cmd.Env, extra...)
	cmd.Env = append(cmd.Env, env...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	m.logf("launch", app, "proton: %s, prefix: %s", build.Name, ubisoftPrefix())
	if err := m.startPumped(cmd, "launch", app); err != nil {
		return nil, err
	}
	return cmd, nil
}

// launchWait is how long a game gets to appear after Connect was asked to
// start it. Connect may update itself first, so it is generous.
const launchWait = 2 * time.Minute

// watchLaunch reports a launch that came to nothing. Connect starts the game
// itself, so a launch worked if the game's process shows up; Shelf's own
// command exits right away either way.
func (m *Manager) watchLaunch(key, name string, exited <-chan error, started time.Time) {
	fail := func(msg string) {
		m.logf("launch", key, "ERROR: %s", msg)
		m.send("epic:launch-error", map[string]string{
			"appName": key,
			"message": fmt.Sprintf("%s (log: %s)", msg, logPath(key)),
		})
	}

	var cmdErr error
	cmdDone := false

	deadline := time.After(launchWait)
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case err := <-exited:
			cmdDone, cmdErr = true, err
			if err != nil {
				msg := "Proton couldn't hand the game to Ubisoft Connect"
				if last := lastLogLine(key); last != "" {
					msg = tail(last, 200)
				}
				fail(fmt.Sprintf("%s didn't start: %s", name, msg))
				return
			}
		case <-tick.C:
			for _, id := range m.runningUbisoft() {
				if id == key {
					return // it is running
				}
			}
		case <-deadline:
			if !cmdDone || cmdErr == nil {
				fail(fmt.Sprintf("%s didn't start after %d seconds. Ubisoft Connect may be asking for something: open it and look", name, int(time.Since(started).Seconds())))
			}
			return
		}
	}
}

// lastLogLine is the last non-empty line of a game's log file.
func lastLogLine(key string) string {
	data, err := os.ReadFile(logPath(key))
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			return l
		}
	}
	return ""
}

// UbisoftLaunch starts an installed Ubisoft game through Connect.
func (m *Manager) UbisoftLaunch(key string) error {
	if macOS {
		return errNeedsProton("Playing Ubisoft games")
	}
	g, ok := m.ubiLookup(key)
	if !ok {
		return fmt.Errorf("unknown game")
	}
	if g.Platform != "" {
		return fmt.Errorf("%s belongs to %s. Start it there", g.Name, g.Platform)
	}
	build, err := m.ubisoftBuild()
	if err != nil {
		return err
	}
	dir, installed := m.ubisoftInstalls()[strconv.Itoa(g.InstallID)]
	if !installed {
		return fmt.Errorf("%s isn't installed yet. Install it from its page first", g.Name)
	}
	if m.isRunningGame(key) {
		return fmt.Errorf("already running")
	}

	uri := "uplay://launch/" + strconv.Itoa(g.LaunchID) + "/0"
	// BattlEye games need the runtime in the environment of the game, which
	// is whatever Connect was started with.
	beEnv, err := battleyeEnvFor(dir, g.Name)
	if err != nil {
		return err
	}

	// Connect may be downloading, and it was started with software rendering,
	// which a game would inherit. Restarting it would end the download.
	if len(m.ubisoftInstalling()) > 0 && (m.settings.get().UbisoftSoftwareRendering || len(beEnv) > 0) {
		return fmt.Errorf("Ubisoft Connect is still downloading a game. Wait for it to finish before playing")
	}

	cmd, err := m.hand(build, uri, dir, beEnv, m.games.get(key), key)
	if err != nil {
		return err
	}
	started := time.Now()
	m.mu.Lock()
	m.running[key] = cmd
	m.mu.Unlock()
	exited := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		m.mu.Lock()
		delete(m.running, key)
		m.mu.Unlock()
		exited <- err
	}()
	go m.watchLaunch(key, g.Name, exited, started)
	return nil
}

// sendLink hands a uplay:// address to Connect, starting it if it isn't open.
// Connect's window is what shows downloads and asks questions, so it runs in
// the mode that draws it properly. A restart would end a download in progress,
// so Connect is left alone while one is running.
func (m *Manager) sendLink(uri string) error {
	build, err := m.ubisoftBuild()
	if err != nil {
		return err
	}
	if len(m.runningUbisoft()) == 0 && len(m.ubisoftInstalling()) == 0 {
		stopPrefixProcesses(ubisoftPrefix())
	}
	wrapper, err := wrapperParts(GameSettings{}, build)
	if err != nil {
		return err
	}
	cmd := exec.Command(wrapper[0], append(wrapper[1:], "start", uri)...)
	cmd.Env = append(library.ChildEnv(), connectEnv(build, "", m.settings.get().UbisoftSoftwareRendering)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := m.startPumped(cmd, ubisoftSource, ""); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	m.startUbiWatch()
	return nil
}

// UbisoftInstall asks Connect to install a game. Connect does the download;
// Shelf shows it as installing until Connect marks the game complete.
func (m *Manager) UbisoftInstall(key string) error {
	if macOS {
		return errNeedsProton("Installing Ubisoft games")
	}
	g, ok := m.ubiLookup(key)
	if !ok {
		return fmt.Errorf("unknown game")
	}
	if g.Platform != "" {
		return fmt.Errorf("%s belongs to %s. Install it there", g.Name, g.Platform)
	}
	id := strconv.Itoa(g.InstallID)
	if _, done := m.ubisoftInstalls()[id]; done {
		return fmt.Errorf("%s is already installed", g.Name)
	}
	if err := m.sendLink("uplay://install/" + id); err != nil {
		return err
	}
	m.ubiMu.Lock()
	if m.ubiPending == nil {
		m.ubiPending = map[string]time.Time{}
	}
	m.ubiPending[key] = time.Now()
	m.ubiMu.Unlock()
	m.broadcastInstalling()
	return nil
}

// UbisoftUninstall asks Connect to remove a game. Connect shows its own
// confirmation, and removing the files is its job.
func (m *Manager) UbisoftUninstall(key string) error {
	g, ok := m.ubiLookup(key)
	if !ok {
		return fmt.Errorf("unknown game")
	}
	if g.Platform != "" {
		return fmt.Errorf("%s belongs to %s. Remove it there", g.Name, g.Platform)
	}
	if m.isRunningGame(key) {
		return fmt.Errorf("close the game first")
	}
	id := strconv.Itoa(g.InstallID)
	if _, done := m.ubisoftInstalls()[id]; !done {
		return fmt.Errorf("%s isn't installed", g.Name)
	}
	return m.sendLink("uplay://uninstall/" + id)
}

// --- installs in progress ---

// UbisoftInstalling is a game Connect is downloading.
type UbisoftInstalling struct {
	Key   string `json:"key"`
	Bytes int64  `json:"bytes"` // written to disk so far
}

// pendingGrace is how long an Install request counts before Connect has
// recorded the download, and how long it holds if Connect never does.
const pendingGrace = 3 * time.Minute

// ubisoftInstalling lists the games being downloaded right now: those Connect
// has started and not finished while it is open, plus the ones just requested.
func (m *Manager) ubisoftInstalling() []UbisoftInstalling {
	complete, partial := m.ubisoftRegistry()
	running := len(prefixProcesses(ubisoftPrefix())) > 0

	var out []UbisoftInstalling
	for _, g := range m.readConnect().games {
		if g.Platform != "" {
			continue
		}
		id := strconv.Itoa(g.InstallID)
		key := g.key()
		if _, done := complete[id]; done {
			m.ubiMu.Lock()
			delete(m.ubiPending, key)
			m.ubiMu.Unlock()
			continue
		}

		m.ubiMu.Lock()
		since, pending := m.ubiPending[key]
		if pending && time.Since(since) > pendingGrace {
			delete(m.ubiPending, key)
			pending = false
		}
		m.ubiMu.Unlock()

		dir, started := partial[id]
		switch {
		case started && running:
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			out = append(out, UbisoftInstalling{Key: key, Bytes: diskUsage(ctx, dir)})
			cancel()
		case pending && running:
			out = append(out, UbisoftInstalling{Key: key})
		}
	}
	return out
}

// UbisoftInstallStates is what the UI shows for downloads, on demand.
func (m *Manager) UbisoftInstallStates() []UbisoftInstalling {
	out := m.ubisoftInstalling()
	if out == nil {
		out = []UbisoftInstalling{}
	}
	return out
}

func (m *Manager) broadcastInstalling() { m.send("ubisoft:installing", m.UbisoftInstallStates()) }

// startUbiWatch keeps the library and the downloads current while Connect is
// open or a game is installing: installing a game changes the prefix's
// registry and Connect's records without any event of its own.
func (m *Manager) startUbiWatch() {
	m.ubiMu.Lock()
	if m.watching {
		m.ubiMu.Unlock()
		return
	}
	m.watching = true
	m.ubiMu.Unlock()

	go func() {
		seen := m.connectFingerprint()
		had := false
		idle := 0
		tick := time.NewTicker(3 * time.Second)
		defer tick.Stop()
		for range tick.C {
			if now := m.connectFingerprint(); now != seen {
				seen = now
				m.send("library:changed", nil)
			}
			list := m.ubisoftInstalling()
			if len(list) > 0 || had {
				m.send("ubisoft:installing", append([]UbisoftInstalling{}, list...))
				had = len(list) > 0
			}
			// Connect opens a moment after it is asked to; give it time.
			if len(list) == 0 && len(prefixProcesses(ubisoftPrefix())) == 0 {
				if idle++; idle >= 3 {
					break
				}
			} else {
				idle = 0
			}
		}
		m.ubiMu.Lock()
		m.watching = false
		m.ubiMu.Unlock()
		m.send("library:changed", nil)
	}()
}

// --- noticing a running game ---

// ubisoftGameFor returns the id whose install folder appears in a process
// command line. Wine shows Windows paths there, so backslashes are treated as slashes.
func ubisoftGameFor(cmdline string, folders map[string]string) string {
	norm := strings.ToLower(strings.ReplaceAll(cmdline, `\`, "/"))
	if !strings.Contains(norm, ".exe") {
		return ""
	}
	for folder, id := range folders {
		if strings.Contains(norm, "/"+folder+"/") {
			return id
		}
	}
	return ""
}

// runningUbisoft finds Ubisoft games running in the shared prefix, by game key.
// Connect itself stays open after a game closes, so it is the game's own
// process, found by its install folder, that counts.
func (m *Manager) runningUbisoft() []string {
	if !connectInstalled() {
		return nil
	}
	installs := m.ubisoftInstalls()
	if len(installs) == 0 {
		return nil
	}
	folders := map[string]string{} // lower-case folder name -> game key
	for _, o := range m.readConnect().games {
		if dir, ok := installs[strconv.Itoa(o.InstallID)]; ok {
			folders[strings.ToLower(filepath.Base(dir))] = o.key()
		}
	}
	if len(folders) == 0 {
		return nil
	}

	want := []byte("STEAM_COMPAT_DATA_PATH=" + ubisoftPrefix())
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	found := map[string]bool{}
	for _, e := range entries {
		if n := e.Name(); n == "" || n[0] < '0' || n[0] > '9' {
			continue
		}
		cmdline, err := os.ReadFile("/proc/" + e.Name() + "/cmdline")
		if err != nil {
			continue
		}
		id := ubisoftGameFor(strings.ReplaceAll(string(cmdline), "\x00", " "), folders)
		if id == "" || found[id] {
			continue
		}
		// Another program may merely mention the path; the prefix settles it.
		if env, err := os.ReadFile("/proc/" + e.Name() + "/environ"); err == nil && containsEnv(env, want) {
			found[id] = true
		}
	}
	out := make([]string, 0, len(found))
	for id := range found {
		out = append(out, id)
	}
	return out
}
