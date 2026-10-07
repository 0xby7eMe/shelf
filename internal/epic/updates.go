package epic

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// UpdateInfo describes one game with a newer version available.
type UpdateInfo struct {
	AppName   string `json:"appName"`
	Title     string `json:"title"`
	Installed string `json:"installed"`
	Latest    string `json:"latest"`
}

// latestVersions reads the build versions legendary last fetched from Epic,
// per platform, from assets.json.
func (m *Manager) latestVersions() map[string]map[string]string {
	out := map[string]map[string]string{}
	if m.cfgDir == "" {
		return out
	}
	data, err := os.ReadFile(filepath.Join(m.cfgDir, "assets.json"))
	if err != nil {
		return out
	}
	var raw map[string][]struct {
		AppName      string `json:"app_name"`
		BuildVersion string `json:"build_version"`
	}
	if json.Unmarshal(data, &raw) != nil {
		return out
	}
	for platform, assets := range raw {
		versions := make(map[string]string, len(assets))
		for _, a := range assets {
			versions[a.AppName] = a.BuildVersion
		}
		out[platform] = versions
	}
	return out
}

// pendingUpdates maps app name to the latest version for every installed game
// that is out of date. It works from files on disk, so it is cheap to call.
func (m *Manager) pendingUpdates(installed map[string]installedGame) map[string]string {
	latest := m.latestVersions()
	out := map[string]string{}
	for name, g := range installed {
		platform := g.Platform
		if platform == "" {
			platform = "Windows"
		}
		if v, ok := latest[platform][name]; ok && v != "" && g.Version != "" && v != g.Version {
			out[name] = v
		}
	}
	return out
}

// Updates lists the installed games that have a newer version.
func (m *Manager) Updates() []UpdateInfo {
	installed := m.readInstalled()
	out := []UpdateInfo{}
	for name, latest := range m.pendingUpdates(installed) {
		out = append(out, UpdateInfo{AppName: name, Title: installed[name].Title, Installed: installed[name].Version, Latest: latest})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out
}

// CheckUpdates asks Epic for the latest versions and returns the games that are behind.
func (m *Manager) CheckUpdates() ([]UpdateInfo, error) {
	if !m.Account().LoggedIn {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	if _, err := m.run(ctx, "list-installed", "--check-updates", "--csv"); err != nil {
		return nil, err
	}
	m.send("library:changed", nil)
	return m.Updates(), nil
}

// updateRequired is true when launching would be refused for an outdated game.
func (m *Manager) updateRequired(appName string, gs GameSettings) (UpdateInfo, bool) {
	if gs.SkipUpdateCheck || gs.Offline {
		return UpdateInfo{}, false
	}
	for _, u := range m.Updates() {
		if u.AppName == appName {
			return u, true
		}
	}
	return UpdateInfo{}, false
}
