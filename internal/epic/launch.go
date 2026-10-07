package epic

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	prefix := prefixDir(appName)
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

func (m *Manager) LaunchInfo(appName string) (LaunchInfo, error) {
	if !appNameRe.MatchString(appName) {
		return LaunchInfo{}, fmt.Errorf("invalid game id")
	}
	build, ok := resolveProton(m.settings.get().ProtonPath)
	if !ok {
		return LaunchInfo{}, fmt.Errorf("no Proton found")
	}
	return LaunchInfo{Proton: build.Name, Prefix: prefixDir(appName)}, nil
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
	build, ok := resolveProton(m.settings.get().ProtonPath)
	if !ok {
		return fmt.Errorf("no Proton installation found. Install Proton through Steam or ProtonUp-Qt")
	}

	for _, name := range m.Running() {
		if name == appName {
			return fmt.Errorf("already running")
		}
	}

	prefix := prefixDir(appName)
	if err := os.MkdirAll(prefix, 0o755); err != nil {
		return err
	}

	// --no-wine stops legendary from using Wine itself; the wrapper becomes
	// the command prefix, giving `proton run <game.exe> …`.
	wrapper := shellQuote(filepath.Join(build.Path, "proton")) + " run"
	cmd, err := m.command(context.Background(), "launch", appName, "--no-wine", "--wrapper", wrapper)
	if err != nil {
		return err
	}
	cmd.Env = append(cmd.Env, protonEnv(build, appName, game.InstallPath)...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	logf, err := openLog(appName)
	if err == nil {
		cmd.Stdout, cmd.Stderr = logf, logf
	}
	if err := cmd.Start(); err != nil {
		if logf != nil {
			logf.Close()
		}
		return err
	}

	started := time.Now()
	m.mu.Lock()
	m.running[appName] = cmd
	m.mu.Unlock()

	go func() {
		waitErr := cmd.Wait()
		if logf != nil {
			logf.Close()
		}
		m.mu.Lock()
		delete(m.running, appName)
		m.mu.Unlock()

		if waitErr != nil && time.Since(started) < quickExit {
			m.send("epic:launch-error", map[string]string{
				"appName": appName,
				"message": launchFailure(appName, waitErr),
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

	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func logPath(appName string) string {
	return filepath.Join(dataDir(), "logs", "epic-"+appName+".log")
}

func openLog(appName string) (*os.File, error) {
	p := logPath(appName)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return nil, err
	}
	return os.Create(p)
}

func launchFailure(appName string, err error) string {
	msg := err.Error()
	if data, rerr := os.ReadFile(logPath(appName)); rerr == nil {
		lines := strings.Split(strings.TrimSpace(string(data)), "\n")
		if n := len(lines); n > 0 && lines[n-1] != "" {
			msg = lines[n-1]
		}
	}
	return fmt.Sprintf("Launch failed: %s (log: %s)", tail(msg, 200), logPath(appName))
}
