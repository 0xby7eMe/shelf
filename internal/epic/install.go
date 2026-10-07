package epic

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"sync"
	"time"
)

// Install states reported to the frontend.
const (
	StateInstalling = "installing"
	StateDone       = "done"
	StateFailed     = "failed"
	StateCancelled  = "cancelled"
)

// Progress is a snapshot of an install, sent as the "epic:install" event.
type Progress struct {
	AppName string  `json:"appName"`
	State   string  `json:"state"`
	Percent float64 `json:"percent"`
	ETA     string  `json:"eta,omitempty"`
	Speed   string  `json:"speed,omitempty"`
	Error   string  `json:"error,omitempty"`
}

type installJob struct {
	cancel context.CancelFunc
	mu     sync.Mutex
	p      Progress
}

func (j *installJob) snapshot() Progress {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.p
}

var (
	progressRe = regexp.MustCompile(`Progress:\s*([\d.]+)%.*?ETA:\s*([\d:]+)`)
	speedRe    = regexp.MustCompile(`Download\s+-\s+([\d.]+ \S+/s)`)
)

// parseProgress extracts what it can from one line of legendary's log output.
func parseProgress(line string, p *Progress) bool {
	changed := false
	if m := progressRe.FindStringSubmatch(line); m != nil {
		if pct, err := strconv.ParseFloat(m[1], 64); err == nil {
			p.Percent, p.ETA = pct, m[2]
			changed = true
		}
	}
	if m := speedRe.FindStringSubmatch(line); m != nil {
		p.Speed = m[1]
		changed = true
	}
	return changed
}

// scanLines splits on \n and \r, since progress output may redraw one line.
func scanLines(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

// InstallStates returns every install currently running, for a freshly loaded UI.
func (m *Manager) InstallStates() []Progress {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Progress, 0, len(m.installs))
	for _, j := range m.installs {
		out = append(out, j.snapshot())
	}
	return out
}

// Install downloads a game into the configured install folder in the background.
func (m *Manager) Install(appName string) error {
	if !appNameRe.MatchString(appName) {
		return fmt.Errorf("invalid game id")
	}
	if !m.Account().LoggedIn {
		return fmt.Errorf("not logged in to Epic Games")
	}
	if _, ok := m.readInstalled()[appName]; ok {
		return fmt.Errorf("already installed")
	}
	base := m.settings.get().InstallDir
	if err := os.MkdirAll(base, 0o755); err != nil {
		return fmt.Errorf("install folder: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Windows build: Epic has no Linux builds, and without this legendary
	// would pick Mac or refuse on some titles.
	cmd, err := m.command(ctx, "install", appName,
		"--base-path", base, "--platform", "Windows", "--skip-sdl", "-y")
	if err != nil {
		cancel()
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return err
	}

	job := &installJob{cancel: cancel, p: Progress{AppName: appName, State: StateInstalling}}
	m.mu.Lock()
	if _, busy := m.installs[appName]; busy {
		m.mu.Unlock()
		cancel()
		return fmt.Errorf("already installing")
	}
	m.installs[appName] = job
	m.mu.Unlock()

	if err := cmd.Start(); err != nil {
		m.finishInstall(appName, job, StateFailed, err.Error())
		return err
	}
	m.send("epic:install", job.snapshot())

	go func() {
		var lastLine string
		var lastEmit time.Time
		sc := bufio.NewScanner(stderr)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		sc.Split(scanLines)
		for sc.Scan() {
			line := sc.Text()
			if line != "" {
				lastLine = line
			}
			job.mu.Lock()
			changed := parseProgress(line, &job.p)
			job.mu.Unlock()
			if changed && time.Since(lastEmit) > 250*time.Millisecond {
				lastEmit = time.Now()
				m.send("epic:install", job.snapshot())
			}
		}
		_, _ = io.Copy(io.Discard, stderr)

		switch err := cmd.Wait(); {
		case ctx.Err() != nil:
			m.finishInstall(appName, job, StateCancelled, "")
		case err != nil:
			m.finishInstall(appName, job, StateFailed, tail(lastLine, 300))
		default:
			m.finishInstall(appName, job, StateDone, "")
		}
	}()
	return nil
}

func (m *Manager) finishInstall(appName string, job *installJob, state, msg string) {
	job.cancel()
	job.mu.Lock()
	job.p.State, job.p.Error = state, msg
	if state == StateDone {
		job.p.Percent = 100
	}
	final := job.p
	job.mu.Unlock()

	m.mu.Lock()
	delete(m.installs, appName)
	m.mu.Unlock()

	m.send("epic:install", final)
	m.send("library:changed", nil)
}

// CancelInstall stops a running install. Legendary resumes it on the next try.
func (m *Manager) CancelInstall(appName string) {
	m.mu.Lock()
	job := m.installs[appName]
	m.mu.Unlock()
	if job != nil {
		job.cancel()
	}
}

// Uninstall removes a game's files. Its Wine prefix and saves are left alone.
func (m *Manager) Uninstall(appName string) error {
	if !appNameRe.MatchString(appName) {
		return fmt.Errorf("invalid game id")
	}
	m.mu.Lock()
	_, installing := m.installs[appName]
	_, running := m.running[appName]
	m.mu.Unlock()
	if installing || running {
		return fmt.Errorf("game is busy")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if _, err := m.run(ctx, "uninstall", appName, "-y"); err != nil {
		return err
	}
	m.send("library:changed", nil)
	return nil
}
