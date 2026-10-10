package epic

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"shelf/internal/library"
)

// gogPlayTask is one way to start a game, from its goggame-<id>.info.
type gogPlayTask struct {
	IsPrimary  bool   `json:"isPrimary"`
	Type       string `json:"type"` // "FileTask" runs a program; "URLTask" opens a page
	Category   string `json:"category"`
	Path       string `json:"path"` // relative to the install folder, with backslashes
	Arguments  string `json:"arguments"`
	WorkingDir string `json:"workingDir"`
}

type gogGameInfo struct {
	Name      string        `json:"name"`
	PlayTasks []gogPlayTask `json:"playTasks"`
}

// gogLaunchTarget is what to run for a game, as Linux paths.
type gogLaunchTarget struct {
	Exe  string
	Dir  string
	Args []string
}

// relPath turns a path from a GOG info file into one under dir. A path that
// tries to leave the install folder is refused.
func relPath(dir, p string) (string, error) {
	p = strings.TrimSpace(strings.ReplaceAll(p, `\`, "/"))
	if p == "" {
		return dir, nil
	}
	if filepath.IsAbs(p) || strings.Contains(p, ":") {
		return "", fmt.Errorf("unexpected path %q in the game's info file", p)
	}
	out := filepath.Join(dir, filepath.Clean(p))
	if out != dir && !strings.HasPrefix(out, dir+string(filepath.Separator)) {
		return "", fmt.Errorf("unexpected path %q in the game's info file", p)
	}
	return out, nil
}

// gogTarget reads a game's info file and picks the program that plays it.
func gogTarget(dir, key string) (gogLaunchTarget, error) {
	data, err := os.ReadFile(gogInfoFile(dir, key))
	if err != nil {
		return gogLaunchTarget{}, fmt.Errorf("the game's files are incomplete (no goggame info file). Reinstall it")
	}
	var info gogGameInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return gogLaunchTarget{}, fmt.Errorf("couldn't read the game's info file: %w", err)
	}
	var task *gogPlayTask
	for i, t := range info.PlayTasks {
		if t.Type != "FileTask" || t.Path == "" {
			continue
		}
		if t.IsPrimary {
			task = &info.PlayTasks[i]
			break
		}
		if task == nil && (t.Category == "" || t.Category == "game") {
			task = &info.PlayTasks[i]
		}
	}
	if task == nil {
		return gogLaunchTarget{}, fmt.Errorf("the game's info file doesn't say which program to start")
	}

	exe, err := relPath(dir, task.Path)
	if err != nil {
		return gogLaunchTarget{}, err
	}
	work := filepath.Dir(exe)
	if task.WorkingDir != "" {
		if work, err = relPath(dir, task.WorkingDir); err != nil {
			return gogLaunchTarget{}, err
		}
	}
	args, err := splitArgs(task.Arguments)
	if err != nil {
		return gogLaunchTarget{}, fmt.Errorf("couldn't read the game's arguments: %w", err)
	}
	return gogLaunchTarget{Exe: exe, Dir: work, Args: args}, nil
}

// GogLaunch starts an installed GOG game: on Linux through Proton, in its own
// prefix, and on Windows as it is.
func (m *Manager) GogLaunch(key string) error {
	if macOS {
		return errNeedsProton("Playing GOG games")
	}
	if !gogKeyRe.MatchString(key) {
		return fmt.Errorf("invalid game id")
	}
	g, ok := m.gogInstalled()[key]
	if !ok {
		return fmt.Errorf("game is not installed")
	}
	if m.isRunningGame(key) {
		return fmt.Errorf("already running")
	}
	m.mu.Lock()
	_, working := m.installs[key]
	m.mu.Unlock()
	if working {
		return fmt.Errorf("wait for the current download to finish")
	}

	target, err := gogTarget(g.InstallPath, key)
	if err != nil {
		return err
	}
	gs := m.games.get(key)
	var build ProtonBuild
	var wrapper []string
	if useProton {
		if build, ok = m.protonFor(gs); !ok {
			return fmt.Errorf("no Proton installation found. Install Proton through Steam or ProtonUp-Qt")
		}
		if wrapper, err = wrapperParts(gs, build); err != nil {
			return err
		}
	}
	env, err := parseEnv(gs.Env)
	if err != nil {
		return err
	}
	extra, err := splitArgs(gs.LaunchArgs)
	if err != nil {
		return err
	}
	beEnv, err := battleyeEnvFor(g.InstallPath, g.Title)
	if err != nil {
		return err
	}
	prefix := prefixDir(key)
	var cmd *exec.Cmd
	if useProton {
		if err := os.MkdirAll(prefix, 0o755); err != nil {
			return err
		}
		args := append(append(append(wrapper[1:], target.Exe), target.Args...), extra...)
		cmd = exec.Command(wrapper[0], args...)
		cmd.Env = append(library.ChildEnv(), protonEnv(build, key, g.InstallPath)...)
	} else {
		cmd = exec.Command(target.Exe, append(target.Args, extra...)...)
		cmd.Env = library.ChildEnv()
	}
	cmd.Dir = target.Dir
	cmd.Env = append(cmd.Env, beEnv...)
	cmd.Env = append(cmd.Env, env...) // the user's variables win
	ownGroup(cmd)

	logf, _ := openLog(key)
	out := &gameOutput{done: make(chan struct{})}
	pr, pw, perr := os.Pipe()
	if perr == nil {
		cmd.Stdout, cmd.Stderr = pw, pw
	} else if logf != nil {
		cmd.Stdout, cmd.Stderr = logf, logf
	}

	m.logf("launch", key, "$ %s", strings.Join(cmd.Args, " "))
	if useProton {
		m.logf("launch", key, "proton: %s, prefix: %s", build.Name, prefix)
	}
	if err := cmd.Start(); err != nil {
		if perr == nil {
			pr.Close()
			pw.Close()
		}
		if logf != nil {
			logf.Close()
		}
		m.logf("launch", key, "ERROR: %v", err)
		return err
	}
	if perr == nil {
		pw.Close()
		go m.pumpGameOutput(key, pr, logf, out)
	} else {
		close(out.done)
	}

	started := time.Now()
	m.mu.Lock()
	m.running[key] = cmd
	m.mu.Unlock()

	go func() {
		waitErr := cmd.Wait()
		m.mu.Lock()
		delete(m.running, key)
		m.mu.Unlock()
		if waitErr != nil && time.Since(started) < quickExit {
			select {
			case <-out.done:
			case <-time.After(2 * time.Second):
			}
			m.send("epic:launch-error", map[string]string{
				"appName": key,
				"message": launchFailure(key, waitErr, out.last()),
			})
		}
	}()
	return nil
}

// runningGog finds GOG games that are running, by their prefix: each game has
// its own, so anything running in it is the game, whoever started it.
// Games being installed are left out, since that is their installer.
func (m *Manager) runningGog() []string {
	installed := m.gogInstalled()
	if len(installed) == 0 {
		return nil
	}
	if onWindows {
		return m.runningGogWindows(installed)
	}
	prefixes := compatDataPaths()
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	for key := range installed {
		if _, busy := m.installs[key]; busy {
			continue
		}
		if prefixes[prefixDir(key)] {
			out = append(out, key)
		}
	}
	return out
}

// runningGogWindows finds GOG games by their programs: anything running from
// inside a game's folder is the game.
func (m *Manager) runningGogWindows(installed map[string]gogInstall) []string {
	folders := map[string]string{}
	m.mu.Lock()
	for key, g := range installed {
		if _, busy := m.installs[key]; !busy && g.InstallPath != "" {
			folders[g.InstallPath] = key
		}
	}
	m.mu.Unlock()
	var out []string
	for key := range runningUnder(folders) {
		out = append(out, key)
	}
	return out
}

// compatDataPaths lists the Proton prefixes that have a process running in them.
func compatDataPaths() map[string]bool {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	const key = "STEAM_COMPAT_DATA_PATH="
	out := map[string]bool{}
	for _, e := range entries {
		n := e.Name()
		if n == "" || n[0] < '0' || n[0] > '9' {
			continue
		}
		env, err := os.ReadFile("/proc/" + n + "/environ")
		if err != nil {
			continue
		}
		for _, kv := range strings.Split(string(env), "\x00") {
			if v, ok := strings.CutPrefix(kv, key); ok {
				out[filepath.Clean(v)] = true
				break
			}
		}
	}
	return out
}
