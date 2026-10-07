package epic

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// Tri-state choice for options that can follow the global setting.
const (
	ChoiceDefault = "default"
	ChoiceOn      = "on"
	ChoiceOff     = "off"
)

// GameSettings are the per-game launch options.
type GameSettings struct {
	// ProtonPath overrides the global Proton build. Empty follows the global choice.
	ProtonPath string `json:"protonPath"`
	// LaunchArgs are extra arguments handed to the game.
	LaunchArgs string `json:"launchArgs"`
	// Env is one KEY=VALUE per line.
	Env string `json:"env"`
	// MangoHud and GameMode wrap the game in those tools when installed.
	MangoHud bool `json:"mangoHud"`
	GameMode bool `json:"gameMode"`
	// Offline launches without contacting Epic; it also skips the update check.
	Offline bool `json:"offline"`
	// SkipUpdateCheck lets an outdated game start instead of demanding an update.
	SkipUpdateCheck bool `json:"skipUpdateCheck"`
	// CloudSaves and AutoUpdate are "default", "on" or "off".
	CloudSaves string `json:"cloudSaves"`
	AutoUpdate string `json:"autoUpdate"`
	// SavePath overrides where Shelf thinks the game keeps its saves.
	SavePath string `json:"savePath"`
}

func defaultGameSettings() GameSettings {
	return GameSettings{CloudSaves: ChoiceDefault, AutoUpdate: ChoiceDefault}
}

var envKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// parseEnv reads KEY=VALUE lines, ignoring blanks and #comments.
func parseEnv(text string) ([]string, error) {
	var out []string
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || !envKeyRe.MatchString(key) {
			return nil, fmt.Errorf("environment line %d should look like KEY=value", i+1)
		}
		out = append(out, key+"="+val)
	}
	return out, nil
}

// splitArgs splits a command line into words, honouring quotes and backslashes.
func splitArgs(s string) ([]string, error) {
	var (
		args  []string
		cur   strings.Builder
		quote rune
		has   bool
	)
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case quote == '\'':
			if r == '\'' {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case quote == '"':
			switch {
			case r == '"':
				quote = 0
			case r == '\\' && i+1 < len(runes) && (runes[i+1] == '"' || runes[i+1] == '\\'):
				i++
				cur.WriteRune(runes[i])
			default:
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote, has = r, true
		case r == '\\' && i+1 < len(runes):
			i++
			cur.WriteRune(runes[i])
			has = true
		case r == ' ' || r == '\t' || r == '\n':
			if cur.Len() > 0 || has {
				args = append(args, cur.String())
				cur.Reset()
				has = false
			}
		default:
			cur.WriteRune(r)
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("launch arguments have an unclosed quote")
	}
	if cur.Len() > 0 || has {
		args = append(args, cur.String())
	}
	return args, nil
}

func (g GameSettings) validate() error {
	for _, c := range []string{g.CloudSaves, g.AutoUpdate} {
		if c != ChoiceDefault && c != ChoiceOn && c != ChoiceOff {
			return fmt.Errorf("invalid option %q", c)
		}
	}
	if _, err := parseEnv(g.Env); err != nil {
		return err
	}
	if _, err := splitArgs(g.LaunchArgs); err != nil {
		return err
	}
	if g.SavePath != "" && !filepath.IsAbs(g.SavePath) {
		return fmt.Errorf("save folder must be an absolute path")
	}
	if g.ProtonPath != "" {
		if _, ok := resolveProtonExact(g.ProtonPath); !ok {
			return fmt.Errorf("that Proton build was not found")
		}
	}
	return nil
}

// choose resolves a tri-state against the global default.
func choose(choice string, global bool) bool {
	switch choice {
	case ChoiceOn:
		return true
	case ChoiceOff:
		return false
	}
	return global
}

type gameSettingsStore struct {
	mu   sync.Mutex
	path string
	all  map[string]GameSettings
}

func newGameSettingsStore() *gameSettingsStore {
	s := &gameSettingsStore{all: map[string]GameSettings{}}
	if dir := configDir(); dir != "" {
		s.path = filepath.Join(dir, "epic-games.json")
	}
	if data, err := os.ReadFile(s.path); err == nil {
		_ = json.Unmarshal(data, &s.all)
	}
	return s
}

func (s *gameSettingsStore) get(appName string) GameSettings {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.all[appName]
	if !ok {
		return defaultGameSettings()
	}
	if g.CloudSaves == "" {
		g.CloudSaves = ChoiceDefault
	}
	if g.AutoUpdate == "" {
		g.AutoUpdate = ChoiceDefault
	}
	return g
}

func (s *gameSettingsStore) set(appName string, g GameSettings) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.all[appName] = g
	return writeJSON(s.path, s.all)
}

// GameSettings returns a game's launch options.
func (m *Manager) GameSettings(appName string) (GameSettings, error) {
	if !appNameRe.MatchString(appName) {
		return GameSettings{}, fmt.Errorf("invalid game id")
	}
	return m.games.get(appName), nil
}

// SetGameSettings validates and stores a game's launch options.
func (m *Manager) SetGameSettings(appName string, g GameSettings) error {
	if !appNameRe.MatchString(appName) {
		return fmt.Errorf("invalid game id")
	}
	g.LaunchArgs = strings.TrimSpace(g.LaunchArgs)
	g.SavePath = strings.TrimSpace(g.SavePath)
	if err := g.validate(); err != nil {
		return err
	}
	return m.games.set(appName, g)
}

// Tools reports which optional launch wrappers are installed.
type Tools struct {
	MangoHud bool `json:"mangoHud"`
	GameMode bool `json:"gameMode"`
}

func (m *Manager) Tools() Tools {
	return Tools{MangoHud: hasBinary("mangohud"), GameMode: hasBinary("gamemoderun")}
}
