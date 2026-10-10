// Package epic connects Shelf to the Epic Games Store by driving legendary,
// the same open-source CLI Heroic uses. Games are Windows builds, so they run
// through Proton.
package epic

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"shelf/internal/applog"
	"shelf/internal/library"
)

// appNameRe matches Epic app names such as "Fortnite" or a 32-char hex id.
// Anchoring on an alphanumeric first char keeps them from being read as flags.
var appNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)

// ErrNoLegendary means the legendary binary is not installed.
var ErrNoLegendary = fmt.Errorf("legendary is not installed. Install it from Settings, Integrations, Epic Games")

type Manager struct {
	settings *settingsStore
	games    *gameSettingsStore
	hist     *library.History
	emit     func(event string, data any)
	log      *applog.Log

	// legendary keeps its own config, kept apart from a user's existing legendary/Heroic login.
	cfgDir string

	mu       sync.Mutex
	installs map[string]*installJob
	running  map[string]*exec.Cmd
	owned    []ownedGame   // nil until loaded
	queue    []*installJob // waiting, in order
	active   *installJob   // the running queued job, if any
	paused   bool

	// Ubisoft Connect: the launcher in its prefix, and what it has recorded.
	ubiMu            sync.Mutex
	ubiCache         *ubiLocalCache
	ubiSetup         UbisoftSetupState
	protonInstall    ProtonInstallState
	battleyeInstall  ProtonInstallState
	legendaryInstall ProtonInstallState
	connect          *exec.Cmd
	watching         bool
	ubiSizes         map[string]ubiSizeEntry // install folder -> last measured size
	ubiSizing        map[string]bool         // folders being measured now
	ubiPending       map[string]time.Time    // game key -> when Install was pressed
	reg              *regCache

	announcedUpdates map[string]string // app -> version already shown to the user
	announcedImport  bool

	// GOG: the login, and the library as GOG last listed it.
	gogMu    sync.Mutex
	gogTok   *gogToken
	gogOwned []gogProduct // nil until loaded
	gogHTTP  *http.Client
	// gogRefreshMu lets one request at a time renew the login, so workers
	// running at once don't each spend the refresh token.
	gogRefreshMu sync.Mutex

	// GOG Galaxy services of single games (achievements): each game's own
	// client, and a token for it.
	gogClients  map[string]gogGameClient // by key; loaded from disk on first use
	gogGameToks map[string]gogGameToken  // by key, in memory only
}

// New creates a Manager. emit publishes frontend events and may be nil.
func New(hist *library.History, emit func(string, any)) *Manager {
	if emit == nil {
		emit = func(string, any) {}
	}
	m := &Manager{
		settings: newSettingsStore(),
		games:    newGameSettingsStore(),
		hist:     hist,
		emit:     emit,
		log:      applog.New(3000),
		installs: map[string]*installJob{},
		running:  map[string]*exec.Cmd{},

		announcedUpdates: map[string]string{},
		gogHTTP:          &http.Client{Timeout: 60 * time.Second},
		gogGameToks:      map[string]gogGameToken{},
	}
	if dir := configDir(); dir != "" {
		m.cfgDir = filepath.Join(dir, "legendary")
	}
	return m
}

// Log returns the log shown in the log window.
func (m *Manager) Log() *applog.Log { return m.log }

// logf records a line for the log window.
func (m *Manager) logf(source, app, format string, args ...any) {
	m.log.Add(source, app, fmt.Sprintf(format, args...))
}

// commandLine renders legendary arguments the way a person would type them.
func commandLine(args []string) string {
	parts := make([]string, len(args))
	for i, a := range args {
		if strings.ContainsAny(a, " \t'\"") {
			a = shellQuote(a)
		}
		parts[i] = a
	}
	return "legendary " + strings.Join(parts, " ")
}

// SetEmitter replaces the event publisher, for use once the app context exists.
func (m *Manager) SetEmitter(emit func(string, any)) {
	m.mu.Lock()
	m.emit = emit
	m.mu.Unlock()
}

func (m *Manager) send(event string, data any) {
	m.mu.Lock()
	emit := m.emit
	m.mu.Unlock()
	emit(event, data)
}

func hasBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func findLegendary() (string, error) {
	if p, err := exec.LookPath("legendary"); err == nil {
		return p, nil
	}
	if home, err := os.UserHomeDir(); err == nil {
		for _, p := range []string{
			filepath.Join(home, ".local", "bin", "legendary"),
			filepath.Join(configDir(), "bin", legendaryBinName(runtime.GOOS)),
		} {
			if st, err := os.Stat(p); err == nil && !st.IsDir() {
				return p, nil
			}
		}
	}
	return "", ErrNoLegendary
}

// command builds a legendary invocation pinned to Shelf's own config dir.
func (m *Manager) command(ctx context.Context, args ...string) (*exec.Cmd, error) {
	bin, err := findLegendary()
	if err != nil {
		return nil, err
	}
	if m.cfgDir == "" {
		return nil, fmt.Errorf("no config directory available")
	}
	if err := os.MkdirAll(m.cfgDir, 0o755); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = append(library.ChildEnv(), "LEGENDARY_CONFIG_PATH="+m.cfgDir)
	hideConsole(cmd)
	return cmd, nil
}

// run executes legendary and returns stdout. Failures carry the tail of stderr.
func (m *Manager) run(ctx context.Context, args ...string) ([]byte, error) {
	cmd, err := m.command(ctx, args...)
	if err != nil {
		return nil, err
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		m.logf("legendary", "", "$ %s", commandLine(args))
		m.log.AddLines("legendary", "", stderr.String())
		return nil, fmt.Errorf("legendary %s: %w: %s", args[0], err, tail(stderr.String(), 400))
	}
	return stdout.Bytes(), nil
}

// runAll is run for commands whose useful output goes to stderr too.
func (m *Manager) runAll(ctx context.Context, args ...string) (string, error) {
	cmd, err := m.command(ctx, args...)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err = cmd.Run()
	if err != nil {
		m.logf("legendary", "", "$ %s", commandLine(args))
	}
	if err != nil {
		m.log.AddLines("legendary", "", out.String())
		return "", fmt.Errorf("legendary %s: %w: %s", args[0], err, tail(out.String(), 400))
	}
	return out.String(), nil
}

func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		s = "…" + s[len(s)-n:]
	}
	return s
}
