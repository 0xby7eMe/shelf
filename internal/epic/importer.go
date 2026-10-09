package epic

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Importable is an Epic game found on disk that Shelf doesn't manage yet.
type Importable struct {
	AppName string `json:"appName"`
	Title   string `json:"title"`
	Path    string `json:"path"`
	// Source says where it was found, e.g. "Heroic" or "Epic Games Launcher".
	Source string `json:"source"`
}

type foundGame struct {
	appName, path, source string
}

// otherLaunchers lists installed.json files of legendary setups Shelf doesn't own.
func otherLaunchers() []foundGame {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	sources := []struct{ name, path string }{
		{"legendary", filepath.Join(home, ".config", "legendary", "installed.json")},
		{"Heroic", filepath.Join(home, ".config", "heroic", "legendaryConfig", "legendary", "installed.json")},
		{"Heroic", filepath.Join(home, ".var", "app", "com.heroicgameslauncher.hgl", "config", "heroic", "legendaryConfig", "legendary", "installed.json")},
		{"Heroic", filepath.Join(home, "Library", "Application Support", "heroic", "legendaryConfig", "legendary", "installed.json")}, // macOS
	}

	var out []foundGame
	for _, src := range sources {
		data, err := os.ReadFile(src.path)
		if err != nil {
			continue
		}
		var games map[string]installedGame
		if json.Unmarshal(data, &games) != nil {
			continue
		}
		for name, g := range games {
			if !g.IsDLC && g.InstallPath != "" {
				out = append(out, foundGame{appName: name, path: g.InstallPath, source: src.name})
			}
		}
	}
	return out
}

// mancpnApp reads the app name from an Epic Games Launcher .mancpn file.
func mancpnApp(data []byte) string {
	var doc struct {
		AppName string `json:"AppName"`
	}
	if json.Unmarshal(data, &doc) != nil {
		return ""
	}
	return doc.AppName
}

// scanRoots lists folders whose subfolders may hold Epic installs.
func scanRoots(installDir string) []string {
	home, _ := os.UserHomeDir()
	roots := []string{installDir}
	if home != "" {
		games := filepath.Join(home, "Games")
		roots = append(roots, games, filepath.Join(games, "Heroic"), filepath.Join(games, "Epic"))
		// Epic Games Launcher run through Wine keeps games in its prefix.
		prefixes, _ := filepath.Glob(filepath.Join(games, "*", "Prefixes", "*", "drive_c", "Program Files", "Epic Games"))
		roots = append(roots, prefixes...)
		lutris, _ := filepath.Glob(filepath.Join(games, "*", "drive_c", "Program Files", "Epic Games"))
		roots = append(roots, lutris...)
	}
	return roots
}

// scanEgstore finds games that carry Epic's own .egstore metadata.
func scanEgstore(roots []string) []foundGame {
	var out []foundGame
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(root, e.Name())
			files, _ := filepath.Glob(filepath.Join(dir, ".egstore", "*.mancpn"))
			for _, f := range files {
				data, err := os.ReadFile(f)
				if err != nil {
					continue
				}
				if name := mancpnApp(data); name != "" {
					out = append(out, foundGame{appName: name, path: dir, source: "Epic Games Launcher"})
				}
			}
		}
	}
	return out
}

// FindImportable looks for installed Epic games Shelf could adopt: ones from
// other launchers' legendary setups and ones with Epic Games Launcher metadata.
// Only games on the signed-in account are offered.
func (m *Manager) FindImportable() ([]Importable, error) {
	if !m.Account().LoggedIn {
		return nil, fmt.Errorf("not logged in to Epic Games")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	owned, err := m.ownedGames(ctx)
	if err != nil {
		return nil, err
	}
	titles := make(map[string]string, len(owned))
	for _, o := range owned {
		titles[o.AppName] = o.AppTitle
	}

	have := m.readInstalled()
	found := append(otherLaunchers(), scanEgstore(scanRoots(m.settings.get().InstallDir))...)

	seen := map[string]bool{}
	out := []Importable{}
	for _, f := range found {
		title, mine := titles[f.appName]
		if !mine || !appNameRe.MatchString(f.appName) || seen[f.appName] {
			continue
		}
		if _, installed := have[f.appName]; installed {
			continue
		}
		if st, err := os.Stat(f.path); err != nil || !st.IsDir() {
			continue
		}
		seen[f.appName] = true
		out = append(out, Importable{AppName: f.appName, Title: title, Path: f.path, Source: f.source})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Title) < strings.ToLower(out[j].Title) })
	return out, nil
}

// Import adopts an existing install without copying or moving any files.
func (m *Manager) Import(appName, path string) error {
	if err := m.checkGame(appName, false); err != nil {
		return err
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("install path must be absolute")
	}
	if st, err := os.Stat(path); err != nil || !st.IsDir() {
		return fmt.Errorf("install folder not found")
	}
	return m.startJob(KindImport, appName, []string{"-y", "import", appName, path, "--skip-dlcs"})
}

// ImportAll imports everything FindImportable lists, one after another.
func (m *Manager) ImportAll() (int, error) {
	list, err := m.FindImportable()
	if err != nil {
		return 0, err
	}
	n := 0
	var firstErr error
	for _, g := range list {
		if err := m.Import(g.AppName, g.Path); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("%s: %w", g.Title, err)
			}
			continue
		}
		n++
	}
	return n, firstErr
}
