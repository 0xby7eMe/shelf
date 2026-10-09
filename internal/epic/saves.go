package epic

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SaveEvent is sent as "epic:saves" when a cloud save sync changed something or failed.
type SaveEvent struct {
	AppName string `json:"appName"`
	Phase   string `json:"phase"` // before | after | manual
	State   string `json:"state"` // downloaded | uploaded | failed | unchanged
	Error   string `json:"error,omitempty"`
}

// thirdPartyStoreOf names the launcher an owned game must be installed through, or "".
func (m *Manager) thirdPartyStoreOf(appName string) string {
	m.mu.Lock()
	owned := m.owned
	m.mu.Unlock()
	for _, o := range owned {
		if o.AppName == appName {
			return o.thirdPartyStore()
		}
	}
	return ""
}

func (m *Manager) supportsCloudSaves(appName string) bool {
	m.mu.Lock()
	owned := m.owned
	m.mu.Unlock()
	for _, o := range owned {
		if o.AppName == appName {
			return o.supportsCloudSaves()
		}
	}
	return false
}

func (m *Manager) cloudSavesEnabled(appName string) bool {
	// Saves are looked for inside the game's Proton prefix, which Mac games don't have.
	return !macOS && choose(m.games.get(appName).CloudSaves, m.settings.get().CloudSaves) && m.supportsCloudSaves(appName)
}

// setConfigValue sets key in a section of legendary's config.ini, creating
// either when missing. Values legendary's parser would treat specially are refused.
func setConfigValue(path, section, key, value string) error {
	if strings.ContainsAny(value, "%\n\r") {
		return fmt.Errorf("unsupported character in config value")
	}
	data, _ := os.ReadFile(path)
	lines := strings.Split(string(data), "\n")
	header := "[" + section + "]"

	start := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == header {
			start = i
			break
		}
	}
	entry := key + " = " + value
	switch {
	case start < 0:
		if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, header, entry, "")
	default:
		end := len(lines)
		for i := start + 1; i < len(lines); i++ {
			if strings.HasPrefix(strings.TrimSpace(lines[i]), "[") {
				end = i
				break
			}
		}
		replaced := false
		for i := start + 1; i < end; i++ {
			k, _, ok := strings.Cut(lines[i], "=")
			if ok && strings.EqualFold(strings.TrimSpace(k), key) {
				lines[i] = entry
				replaced = true
				break
			}
		}
		if !replaced {
			lines = append(lines[:start+1], append([]string{entry}, lines[start+1:]...)...)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

// pointLegendaryAtPrefix tells legendary where the game's Proton prefix is,
// which is how it works out where Windows games keep their saves.
func (m *Manager) pointLegendaryAtPrefix(appName string) error {
	return setConfigValue(filepath.Join(m.cfgDir, "config.ini"), appName+".env", "STEAM_COMPAT_DATA_PATH", prefixDir(appName))
}

// prefixReady reports whether Proton has created the prefix yet. Syncing
// before that would make legendary create folders inside a half-built prefix.
func prefixReady(appName string) bool {
	st, err := os.Stat(filepath.Join(prefixDir(appName), "pfx", "drive_c"))
	return err == nil && st.IsDir()
}

const (
	dirDown = "down"
	dirUp   = "up"
	dirBoth = "both"
)

// syncSaves runs legendary's save sync in one direction (or both) and reports what happened.
func (m *Manager) syncSaves(appName, direction string) (state string, err error) {
	if !m.Account().LoggedIn {
		return "", fmt.Errorf("not logged in to Epic Games")
	}
	if !prefixReady(appName) {
		return "unchanged", nil
	}
	if err := m.pointLegendaryAtPrefix(appName); err != nil {
		return "", err
	}

	args := []string{"-y", "sync-saves", appName, "--accept-path"}
	switch direction {
	case dirDown:
		args = append(args, "--skip-upload")
	case dirUp:
		args = append(args, "--skip-download")
	}
	if p := m.games.get(appName).SavePath; p != "" {
		args = append(args, "--save-path", p)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	m.logf("saves", appName, "$ %s", commandLine(args))
	out, err := m.runAll(ctx, args...)
	m.log.AddLines("saves", appName, out)
	if err != nil {
		m.logf("saves", appName, "ERROR: %v", err)
	}
	switch {
	case err != nil:
		return "", err
	case strings.Contains(out, "Downloading remote savegame"):
		return "downloaded", nil
	case strings.Contains(out, "Uploading local savegame"):
		return "uploaded", nil
	}
	return "unchanged", nil
}

func (m *Manager) reportSaves(appName, phase, state string, err error, always bool) {
	ev := SaveEvent{AppName: appName, Phase: phase, State: state}
	if err != nil {
		ev.State, ev.Error = "failed", err.Error()
	}
	if always || ev.State != "unchanged" {
		m.send("epic:saves", ev)
	}
}

// SyncSaves syncs a game's saves both ways on demand.
func (m *Manager) SyncSaves(appName string) error {
	if err := m.checkGame(appName, true); err != nil {
		return err
	}
	if !m.supportsCloudSaves(appName) {
		return fmt.Errorf("this game doesn't support cloud saves")
	}
	if !prefixReady(appName) {
		return fmt.Errorf("start the game once first so its saves exist")
	}
	// The outcome, failures included, reaches the UI as an event.
	state, err := m.syncSaves(appName, dirBoth)
	m.reportSaves(appName, "manual", state, err, true)
	return nil
}

// GameStopped is called when a game's process ends; it uploads newer saves.
func (m *Manager) GameStopped(appName string) {
	if _, ok := m.readInstalled()[appName]; !ok || !m.cloudSavesEnabled(appName) {
		return
	}
	go func() {
		state, err := m.syncSaves(appName, dirUp)
		m.reportSaves(appName, "after", state, err, false)
	}()
}
