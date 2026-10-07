package library

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Steam struct{}

func NewSteam() *Steam { return &Steam{} }

func (s *Steam) Source() Source { return SourceSteam }

func (s *Steam) Scan() ([]Game, error) {
	root, err := findSteamRoot()
	if err != nil {
		return nil, err
	}

	stats := readPlaytime(root)
	seen := map[string]bool{}
	games := []Game{}

	for _, lib := range libraryDirs(root) {
		manifests, _ := filepath.Glob(filepath.Join(lib, "steamapps", "appmanifest_*.acf"))
		for _, m := range manifests {
			g, ok := parseManifest(m, lib)
			if !ok || seen[g.ExternalID] {
				continue
			}
			seen[g.ExternalID] = true

			if st, ok := stats[g.ExternalID]; ok {
				g.PlaytimeMinutes = st.minutes
				if st.lastPlayed > g.LastPlayed {
					g.LastPlayed = st.lastPlayed
				}
			}
			games = append(games, g)
		}
	}
	return games, nil
}

func findSteamRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	candidates := []string{
		filepath.Join(home, ".local/share/Steam"),
		filepath.Join(home, ".steam/steam"),
		filepath.Join(home, ".var/app/com.valvesoftware.Steam/.local/share/Steam"), // Flatpak
	}
	for _, c := range candidates {
		if st, err := os.Stat(filepath.Join(c, "steamapps")); err == nil && st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("steam installation not found")
}

func libraryDirs(root string) []string {
	dirs := []string{root}

	data, err := os.ReadFile(filepath.Join(root, "steamapps", "libraryfolders.vdf"))
	if err == nil {
		if doc, err := parseVDF(data); err == nil {
			for _, entry := range doc.obj("libraryfolders") {
				if e, ok := entry.(vdfNode); ok {
					if p := e.str("path"); p != "" {
						dirs = append(dirs, p)
					}
				}
			}
		}
	}

	seen := map[string]bool{}
	out := []string{}
	for _, d := range dirs {
		if r, err := filepath.EvalSymlinks(d); err == nil {
			d = r
		}
		if !seen[d] {
			seen[d] = true
			out = append(out, d)
		}
	}
	return out
}

var toolPrefixes = []string{
	"Proton ",
	"Steam Linux Runtime",
	"Steamworks Common Redistributables",
}

func isSteamTool(name string) bool {
	for _, p := range toolPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func parseManifest(path, lib string) (Game, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Game{}, false
	}
	doc, err := parseVDF(data)
	if err != nil {
		return Game{}, false
	}
	app := doc.obj("appstate")
	if app == nil {
		return Game{}, false
	}

	appid, name := app.str("appid"), app.str("name")
	if appid == "" || name == "" || isSteamTool(name) {
		return Game{}, false
	}

	flags, _ := strconv.Atoi(app.str("stateflags"))
	last, _ := strconv.ParseInt(app.str("lastplayed"), 10, 64)
	size, _ := strconv.ParseInt(app.str("sizeondisk"), 10, 64)

	return Game{
		ID:          "steam:" + appid,
		Source:      SourceSteam,
		ExternalID:  appid,
		Name:        name,
		Cover:       "/cover/" + appid,
		Installed:   flags&4 != 0,
		LastPlayed:  last,
		SizeBytes:   size,
		InstallPath: filepath.Join(lib, "steamapps", "common", app.str("installdir")),
	}, true
}

type userStat struct {
	minutes    int
	lastPlayed int64
}

func readPlaytime(root string) map[string]userStat {
	out := map[string]userStat{}
	files, _ := filepath.Glob(filepath.Join(root, "userdata", "*", "config", "localconfig.vdf"))

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		doc, err := parseVDF(data)
		if err != nil {
			continue
		}
		apps := doc.path("userlocalconfigstore", "software", "valve", "steam", "apps")
		for appid, v := range apps {
			a, ok := v.(vdfNode)
			if !ok {
				continue
			}
			mins, _ := strconv.Atoi(a.str("playtime"))
			last, _ := strconv.ParseInt(a.str("lastplayed"), 10, 64)

			cur := out[appid]
			if mins > cur.minutes {
				cur.minutes = mins
			}
			if last > cur.lastPlayed {
				cur.lastPlayed = last
			}
			out[appid] = cur
		}
	}
	return out
}

// SteamRoot returns the Steam installation directory, if there is one.
func SteamRoot() (string, error) { return findSteamRoot() }

// SteamLibraryDirs lists every Steam library folder, root included.
func SteamLibraryDirs(root string) []string { return libraryDirs(root) }
