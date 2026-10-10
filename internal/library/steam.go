package library

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"shelf/internal/steamapi"
)

// OwnedLister knows every game an account owns, installed or not. It is what
// the Steam Web API provides; without it only installed games are listed,
// because Steam keeps no names for the rest on disk.
type OwnedLister interface {
	OwnedGames(ctx context.Context) ([]steamapi.OwnedGame, error)
	Configured() bool
}

type Steam struct{ owned OwnedLister }

// NewSteam reads installed games from Steam's files and, when owned is given
// and has a key, adds the games that aren't installed.
func NewSteam(owned OwnedLister) *Steam { return &Steam{owned: owned} }

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
	return s.addOwned(games), nil
}

// addOwned merges the Web API's library into the installed games: what is
// missing is added as not installed, and play time and last played take the
// larger of what Steam's files and the API say.
func (s *Steam) addOwned(games []Game) []Game {
	if s.owned == nil || !s.owned.Configured() {
		return games
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	owned, err := s.owned.OwnedGames(ctx)
	if err != nil {
		log.Printf("library: steam web api: %v", err)
		return games
	}

	byID := make(map[string]int, len(games))
	for i, g := range games {
		byID[g.ExternalID] = i
	}
	for _, o := range owned {
		id := o.ID()
		if i, ok := byID[id]; ok {
			g := &games[i]
			g.PlaytimeMinutes = max(g.PlaytimeMinutes, o.PlaytimeMinutes)
			g.LastPlayed = max(g.LastPlayed, o.LastPlayed)
			continue
		}
		if o.Name == "" || isSteamTool(o.Name) {
			continue
		}
		games = append(games, Game{
			ID:              "steam:" + id,
			Source:          SourceSteam,
			ExternalID:      id,
			Name:            o.Name,
			Cover:           "/cover/" + id,
			PlaytimeMinutes: o.PlaytimeMinutes,
			LastPlayed:      o.LastPlayed,
		})
	}
	return games
}

func findSteamRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	for _, c := range steamRootCandidates(home) {
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
		if k := pathKey(d); !seen[k] {
			seen[k] = true
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

// SteamUserID is the 64-bit id of the account Steam last signed in with.
func SteamUserID() (string, error) {
	root, err := findSteamRoot()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(root, "config", "loginusers.vdf"))
	if err != nil {
		return "", err
	}
	return parseLastSteamUser(data)
}

// parseLastSteamUser picks the account marked as the most recent one, or else
// the one that signed in last.
func parseLastSteamUser(data []byte) (string, error) {
	doc, err := parseVDF(data)
	if err != nil {
		return "", err
	}
	best, bestTime := "", int64(-1)
	for id, v := range doc.obj("users") {
		u, ok := v.(vdfNode)
		if !ok {
			continue
		}
		if u.str("MostRecent") == "1" {
			return id, nil
		}
		if t, _ := strconv.ParseInt(u.str("Timestamp"), 10, 64); t > bestTime {
			best, bestTime = id, t
		}
	}
	if best == "" {
		return "", fmt.Errorf("no Steam account is signed in")
	}
	return best, nil
}
