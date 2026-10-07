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
	"strings"
	"sync"
	"time"
)

// Job kinds. All of them are one legendary process reporting progress.
const (
	KindInstall = "install"
	KindUpdate  = "update"
	KindRepair  = "repair"
	KindVerify  = "verify"
	KindImport  = "import"
)

// Job states reported to the frontend.
const (
	StateQueued     = "queued"     // waiting for another job to finish
	StateInstalling = "installing" // running, whatever the kind
	StateDone       = "done"
	StateFailed     = "failed"
	StateCancelled  = "cancelled"
)

// Progress is a snapshot of a job, sent as the "epic:install" event.
type Progress struct {
	AppName string  `json:"appName"`
	Kind    string  `json:"kind"`
	State   string  `json:"state"`
	Percent float64 `json:"percent"`
	ETA     string  `json:"eta,omitempty"`
	Speed   string  `json:"speed,omitempty"`
	Error   string  `json:"error,omitempty"`
	// Damaged is set when a verify found corrupt or missing files, so the UI can offer a repair.
	Damaged bool `json:"damaged,omitempty"`
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
	verifyRe   = regexp.MustCompile(`Verification progress:\s*\d+/\d+\s*\(([\d.]+)%\)\s*\[([\d.]+ \S+/s)\]`)
	damagedRe  = regexp.MustCompile(`Verification failed, (\d+) file\(s\) corrupted, (\d+) file\(s\) are missing`)
)

// parseProgress extracts what it can from one line of legendary's output.
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
	if m := verifyRe.FindStringSubmatch(line); m != nil {
		if pct, err := strconv.ParseFloat(m[1], 64); err == nil {
			p.Percent, p.Speed = pct, m[2]
			changed = true
		}
	}
	return changed
}

// scanLines splits on \n and \r, since progress output redraws one line.
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

// InstallStates returns every job currently running, for a freshly loaded UI.
func (m *Manager) InstallStates() []Progress {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Progress, 0, len(m.installs))
	for _, j := range m.installs {
		out = append(out, j.snapshot())
	}
	return out
}

// isRunningGame reports whether the game's process is alive.
func (m *Manager) isRunningGame(appName string) bool {
	for _, n := range m.Running() {
		if n == appName {
			return true
		}
	}
	return false
}

// startJob runs a legendary command in the background with progress events.
// Jobs that change the install data queue behind each other, because legendary
// only lets one process modify it at a time.
func (m *Manager) startJob(kind, appName string, args []string) error {
	if !m.Account().LoggedIn {
		return fmt.Errorf("not logged in to Epic Games")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd, err := m.command(ctx, args...)
	if err != nil {
		cancel()
		return err
	}

	exclusive := kind != KindVerify
	job := &installJob{cancel: cancel, p: Progress{AppName: appName, Kind: kind, State: StateQueued}}
	m.mu.Lock()
	if _, busy := m.installs[appName]; busy {
		m.mu.Unlock()
		cancel()
		return fmt.Errorf("this game is already being worked on")
	}
	m.installs[appName] = job
	m.mu.Unlock()
	m.send("epic:install", job.snapshot())

	go func() {
		if exclusive {
			select {
			case m.jobSlot <- struct{}{}:
				defer func() { <-m.jobSlot }()
			case <-ctx.Done():
				m.finishJob(job, StateCancelled, "", false)
				return
			}
		}

		// legendary logs to stderr but verify writes its progress to stdout;
		// one pipe keeps the lines in order.
		r, w, err := os.Pipe()
		if err != nil {
			m.finishJob(job, StateFailed, err.Error(), false)
			return
		}
		defer r.Close()
		cmd.Stdout, cmd.Stderr = w, w
		if err := cmd.Start(); err != nil {
			w.Close()
			m.finishJob(job, StateFailed, err.Error(), false)
			return
		}
		w.Close() // the child holds its own copy

		job.mu.Lock()
		job.p.State = StateInstalling
		job.mu.Unlock()
		m.send("epic:install", job.snapshot())

		var lines []string
		var lastEmit time.Time
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		sc.Split(scanLines)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" {
				continue
			}
			if len(lines) >= 200 {
				lines = lines[1:]
			}
			lines = append(lines, line)

			job.mu.Lock()
			changed := parseProgress(line, &job.p)
			job.mu.Unlock()
			if changed && time.Since(lastEmit) > 250*time.Millisecond {
				lastEmit = time.Now()
				m.send("epic:install", job.snapshot())
			}
		}
		_, _ = io.Copy(io.Discard, r)

		runErr := cmd.Wait()
		switch {
		case ctx.Err() != nil:
			m.finishJob(job, StateCancelled, "", false)
		case kind == KindVerify:
			state, msg, damaged := verifyVerdict(lines, runErr)
			m.finishJob(job, state, msg, damaged)
		case runErr != nil:
			m.finishJob(job, StateFailed, failureLine(lines), false)
		default:
			m.finishJob(job, StateDone, "", false)
		}
	}()
	return nil
}

// verifyVerdict reads legendary's verdict from its output rather than its
// exit status, which doesn't reflect corrupt files.
func verifyVerdict(lines []string, runErr error) (state, msg string, damaged bool) {
	for _, l := range lines {
		if m := damagedRe.FindStringSubmatch(l); m != nil {
			return StateFailed, fmt.Sprintf("%s corrupted and %s missing files", m[1], m[2]), true
		}
	}
	for _, l := range lines {
		if strings.Contains(l, "Verification finished successfully") {
			return StateDone, "", false
		}
	}
	if runErr != nil {
		return StateFailed, failureLine(lines), false
	}
	return StateFailed, "Verification didn't finish", false
}

// failureLine picks the most telling line of a failed run.
func failureLine(lines []string) string {
	for i := len(lines) - 1; i >= 0; i-- {
		l := lines[i]
		if strings.Contains(l, "ERROR") || strings.Contains(l, "CRITICAL") || strings.Contains(l, "FATAL") {
			return tail(l, 300)
		}
	}
	if len(lines) > 0 {
		return tail(lines[len(lines)-1], 300)
	}
	return "unknown error"
}

func (m *Manager) finishJob(job *installJob, state, msg string, damaged bool) {
	job.cancel()
	job.mu.Lock()
	job.p.State, job.p.Error, job.p.Damaged = state, msg, damaged
	if state == StateDone {
		job.p.Percent = 100
	}
	final := job.p
	job.mu.Unlock()

	m.mu.Lock()
	delete(m.installs, final.AppName)
	m.mu.Unlock()

	m.send("epic:install", final)
	m.send("library:changed", nil)
}

func (m *Manager) checkGame(appName string, needInstalled bool) error {
	if !appNameRe.MatchString(appName) {
		return fmt.Errorf("invalid game id")
	}
	_, installed := m.readInstalled()[appName]
	if needInstalled && !installed {
		return fmt.Errorf("game is not installed")
	}
	if needInstalled && m.isRunningGame(appName) {
		return fmt.Errorf("close the game first")
	}
	return nil
}

// Install downloads a game into the configured install folder.
func (m *Manager) Install(appName string) error {
	if err := m.checkGame(appName, false); err != nil {
		return err
	}
	if _, ok := m.readInstalled()[appName]; ok {
		return fmt.Errorf("already installed")
	}
	base := m.settings.get().InstallDir
	if err := os.MkdirAll(base, 0o755); err != nil {
		return fmt.Errorf("install folder: %w", err)
	}
	// Windows build: Epic has no Linux builds.
	return m.startJob(KindInstall, appName, []string{
		"install", appName, "--base-path", base, "--platform", "Windows", "--skip-sdl", "-y"})
}

// Update brings an installed game up to the latest version.
func (m *Manager) Update(appName string) error {
	if err := m.checkGame(appName, true); err != nil {
		return err
	}
	return m.startJob(KindUpdate, appName, []string{
		"install", appName, "--update-only", "--skip-sdl", "-y"})
}

// Repair re-downloads corrupt or missing files.
func (m *Manager) Repair(appName string) error {
	if err := m.checkGame(appName, true); err != nil {
		return err
	}
	return m.startJob(KindRepair, appName, []string{
		"install", appName, "--repair", "--skip-sdl", "-y"})
}

// Verify checks installed files against Epic's manifest without changing them.
func (m *Manager) Verify(appName string) error {
	if err := m.checkGame(appName, true); err != nil {
		return err
	}
	return m.startJob(KindVerify, appName, []string{"verify", appName})
}

// CancelInstall stops a running job. Legendary resumes downloads on the next try.
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
	if err := m.checkGame(appName, true); err != nil {
		return err
	}
	m.mu.Lock()
	_, busy := m.installs[appName]
	m.mu.Unlock()
	if busy {
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
