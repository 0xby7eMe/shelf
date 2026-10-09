package epic

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"shelf/internal/library"
)

// quickExit is how soon a launch must fail to be reported as an error
// rather than as a normal short play session.
const quickExit = 20 * time.Second

// shellQuote quotes s for legendary, which splits --wrapper like a shell would.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

// prefixDir is the Proton prefix (compatdata) of a game.
func prefixDir(appName string) string {
	return filepath.Join(dataDir(), "prefixes", appName)
}

// protonEnv builds the environment Proton expects when started outside Steam.
func protonEnv(build ProtonBuild, appName, installPath string) []string {
	return protonEnvAt(build, prefixDir(appName), installPath)
}

// protonEnvAt is protonEnv for a prefix that isn't one game's own.
func protonEnvAt(build ProtonBuild, prefix, installPath string) []string {
	steam, err := library.SteamRoot()
	if err != nil {
		// Proton only needs the variable to point somewhere real.
		steam = prefix
	}
	return []string{
		"STEAM_COMPAT_DATA_PATH=" + prefix,
		"STEAM_COMPAT_CLIENT_INSTALL_PATH=" + steam,
		"STEAM_COMPAT_INSTALL_PATH=" + installPath,
	}
}

// LaunchInfo is what Shelf will use to start a game, shown in the UI.
type LaunchInfo struct {
	Proton string `json:"proton"`
	Prefix string `json:"prefix"`
}

// protonFor picks the Proton build for a game: its own choice, else the global one.
func (m *Manager) protonFor(gs GameSettings) (ProtonBuild, bool) {
	if gs.ProtonPath != "" {
		if b, ok := resolveProtonExact(gs.ProtonPath); ok {
			return b, true
		}
	}
	return resolveProton(m.settings.get().ProtonPath)
}

func (m *Manager) LaunchInfo(appName string) (LaunchInfo, error) {
	if !appNameRe.MatchString(appName) {
		return LaunchInfo{}, fmt.Errorf("invalid game id")
	}
	build, ok := m.protonFor(m.games.get(appName))
	if !ok {
		return LaunchInfo{}, fmt.Errorf("no Proton found")
	}
	return LaunchInfo{Proton: build.Name, Prefix: prefixDir(appName)}, nil
}

// wrapperFor builds legendary's --wrapper value: optional tools, then Proton.
func wrapperFor(gs GameSettings, build ProtonBuild) (string, error) {
	parts, err := wrapperParts(gs, build)
	if err != nil {
		return "", err
	}
	// Only the Proton path can contain spaces; legendary splits the rest like a shell.
	parts[len(parts)-2] = shellQuote(parts[len(parts)-2])
	return strings.Join(parts, " "), nil
}

// wrapperParts lists the command that runs a Windows program for a game:
// optional tools first, then `<proton> run`.
func wrapperParts(gs GameSettings, build ProtonBuild) ([]string, error) {
	var parts []string
	if gs.MangoHud {
		if !hasBinary("mangohud") {
			return nil, fmt.Errorf("MangoHud is turned on for this game but isn't installed")
		}
		parts = append(parts, "mangohud")
	}
	if gs.GameMode {
		if !hasBinary("gamemoderun") {
			return nil, fmt.Errorf("GameMode is turned on for this game but isn't installed")
		}
		parts = append(parts, "gamemoderun")
	}
	parts = append(parts, filepath.Join(build.Path, "proton"), "run")
	return parts, nil
}

// Launch starts an installed game through Proton and returns once it is running.
func (m *Manager) Launch(appName string) error {
	if !appNameRe.MatchString(appName) {
		return fmt.Errorf("invalid game id")
	}
	game, ok := m.readInstalled()[appName]
	if !ok {
		return fmt.Errorf("game is not installed")
	}
	gs := m.games.get(appName)

	build, ok := m.protonFor(gs)
	if !ok {
		return fmt.Errorf("no Proton installation found. Install Proton through Steam or ProtonUp-Qt")
	}
	if m.isRunningGame(appName) {
		return fmt.Errorf("already running")
	}
	m.mu.Lock()
	_, working := m.installs[appName]
	m.mu.Unlock()
	if working {
		return fmt.Errorf("wait for the current download to finish")
	}
	if u, outdated := m.updateRequired(appName, gs); outdated {
		return fmt.Errorf("%s needs an update (%s to %s) before it can start. Update it, or allow outdated launches in its settings", u.Title, u.Installed, u.Latest)
	}

	wrapper, err := wrapperFor(gs, build)
	if err != nil {
		return err
	}
	env, err := parseEnv(gs.Env)
	if err != nil {
		return err
	}
	extra, err := splitArgs(gs.LaunchArgs)
	if err != nil {
		return err
	}
	beEnv, err := battleyeEnvFor(game.InstallPath, game.Title)
	if err != nil {
		return err
	}

	prefix := prefixDir(appName)
	if err := os.MkdirAll(prefix, 0o755); err != nil {
		return err
	}

	// Cloud saves first, but never block the game on them.
	if m.cloudSavesEnabled(appName) {
		state, serr := m.syncSaves(appName, dirDown)
		m.reportSaves(appName, "before", state, serr, false)
	}

	// --no-wine stops legendary from using Wine itself; the wrapper becomes
	// the command prefix, giving `proton run <game.exe> …`.
	args := []string{"launch", appName, "--no-wine", "--wrapper", wrapper}
	if gs.Offline {
		args = append(args, "--offline")
	} else if gs.SkipUpdateCheck {
		args = append(args, "--skip-version-check")
	}
	args = append(args, extra...)

	cmd, err := m.command(context.Background(), args...)
	if err != nil {
		return err
	}
	cmd.Env = append(cmd.Env, protonEnv(build, appName, game.InstallPath)...)
	cmd.Env = append(cmd.Env, beEnv...)
	cmd.Env = append(cmd.Env, env...) // the user's variables win
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// The game keeps writing to this pipe long after legendary itself has
	// exited, until the game closes. It ends up in the log file and the log window.
	logf, _ := openLog(appName)
	out := &gameOutput{done: make(chan struct{})}
	pr, pw, perr := os.Pipe()
	if perr == nil {
		cmd.Stdout, cmd.Stderr = pw, pw
	} else if logf != nil {
		cmd.Stdout, cmd.Stderr = logf, logf
	}

	m.logf("launch", appName, "$ %s", commandLine(args))
	m.logf("launch", appName, "proton: %s, prefix: %s", build.Name, prefix)
	if err := cmd.Start(); err != nil {
		if perr == nil {
			pr.Close()
			pw.Close()
		}
		if logf != nil {
			logf.Close()
		}
		m.logf("launch", appName, "ERROR: %v", err)
		return err
	}
	if perr == nil {
		pw.Close() // the child holds its own copy
		go m.pumpGameOutput(appName, pr, logf, out)
	} else {
		close(out.done)
	}

	started := time.Now()
	m.mu.Lock()
	m.running[appName] = cmd
	m.mu.Unlock()

	go func() {
		waitErr := cmd.Wait()
		m.mu.Lock()
		delete(m.running, appName)
		m.mu.Unlock()

		if waitErr != nil && time.Since(started) < quickExit {
			// If the launcher failed, nothing holds the pipe open, so this ends quickly.
			select {
			case <-out.done:
			case <-time.After(2 * time.Second):
			}
			m.send("epic:launch-error", map[string]string{
				"appName": appName,
				"message": launchFailure(appName, waitErr, out.last()),
			})
		}
	}()
	return nil
}

// epicAppArg returns the app name from the -epicapp=<name> argument that
// legendary passes to every game it launches. Legendary itself exits right
// after starting the game, so this marker on the game's own process (the Proton
// launcher, wine's steam.exe and the game exe all carry it) is what shows the
// game is still running.
func epicAppArg(args []string) (string, bool) {
	for _, a := range args {
		if name, ok := strings.CutPrefix(a, "-epicapp="); ok && appNameRe.MatchString(name) {
			return name, true
		}
	}
	return "", false
}

// scanRunning finds running Epic games by process, so it also catches games
// that Shelf did not start or that outlived a previous Shelf session.
func scanRunning() []string {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if n := e.Name(); n == "" || n[0] < '0' || n[0] > '9' {
			continue
		}
		data, err := os.ReadFile("/proc/" + e.Name() + "/cmdline")
		if err != nil || !bytes.Contains(data, []byte("-epicapp=")) {
			continue
		}
		args := strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
		if name, ok := epicAppArg(args); ok {
			out = append(out, name)
		}
	}
	return out
}

// Running lists the app names that are currently running, whether Shelf
// started them or not.
func (m *Manager) Running() []string {
	seen := map[string]bool{}
	m.mu.Lock()
	for name := range m.running {
		seen[name] = true
	}
	m.mu.Unlock()

	installed := m.readInstalled()
	for _, name := range scanRunning() {
		if _, ok := installed[name]; ok {
			seen[name] = true
		}
	}
	for _, id := range m.runningUbisoft() {
		seen[id] = true
	}
	for _, id := range m.runningGog() {
		seen[id] = true
	}

	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func logPath(appName string) string {
	name := "epic-" + appName
	if gogKeyRe.MatchString(appName) {
		name = appName // already says which store
	}
	return filepath.Join(dataDir(), "logs", name+".log")
}

func openLog(appName string) (*os.File, error) {
	p := logPath(appName)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, err
	}
	return os.Create(p)
}

// gameOutput remembers the end of a game's output for error messages.
type gameOutput struct {
	mu   sync.Mutex
	line string
	done chan struct{}
}

func (g *gameOutput) last() string {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.line
}

// pumpGameOutput copies everything the game prints into the log file and the log window.
func (m *Manager) pumpGameOutput(appName string, r *os.File, file *os.File, out *gameOutput) {
	defer close(out.done)
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
		if strings.TrimSpace(line) == "" {
			continue
		}
		out.mu.Lock()
		out.line = line
		out.mu.Unlock()
		m.log.Add("launch", appName, line)
	}
	m.logf("launch", appName, "process output closed")
}

// LogsDir is where per-game launch logs are written.
func LogsDir() string { return filepath.Join(dataDir(), "logs") }

func launchFailure(appName string, err error, last string) string {
	msg := err.Error()
	if last != "" {
		msg = last
	}
	return fmt.Sprintf("Launch failed: %s (log: %s)", tail(msg, 200), logPath(appName))
}
