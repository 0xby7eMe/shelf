package library

import (
	"path/filepath"
	"strconv"
)

// Bits of an app manifest's StateFlags that tell a download apart.
const (
	steamUpdateRequired = 2
	steamFullyInstalled = 4
	steamUpdatePaused   = 512
	steamUpdateStarted  = 1024
)

// SteamDownload is a game Steam is installing or updating, read from its app
// manifest. Steam only writes the byte counts now and then, so Percent moves
// in steps, and is -1 while Steam hasn't said how much there is to fetch.
type SteamDownload struct {
	AppID   string  `json:"appId"`
	Update  bool    `json:"update"` // the game is installed and getting an update
	Paused  bool    `json:"paused"`
	Done    int64   `json:"done"`
	Total   int64   `json:"total"`
	Percent float64 `json:"percent"`
}

// SteamDownloads lists the downloads Steam has started, in every library.
func SteamDownloads() []SteamDownload {
	root, err := findSteamRoot()
	if err != nil {
		return []SteamDownload{}
	}
	out := []SteamDownload{}
	seen := map[string]bool{}
	for _, lib := range libraryDirs(root) {
		manifests, _ := filepath.Glob(filepath.Join(lib, "steamapps", "appmanifest_*.acf"))
		for _, m := range manifests {
			d, ok := readSteamDownload(m)
			if ok && !seen[d.AppID] {
				seen[d.AppID] = true
				out = append(out, d)
			}
		}
	}
	return out
}

func readSteamDownload(path string) (SteamDownload, bool) {
	app, ok := readManifest(path)
	if !ok {
		return SteamDownload{}, false
	}
	flags, _ := strconv.Atoi(app.str("stateflags"))
	return steamDownloadOf(app.str("appid"), flags,
		atoi64(app.str("bytesdownloaded")), atoi64(app.str("bytestodownload")))
}

// steamDownloadOf decides from a manifest's fields whether a download is going.
// A game that is installed and merely needs an update counts only once Steam
// has started that update, so pending updates don't look like downloads.
func steamDownloadOf(appID string, flags int, done, total int64) (SteamDownload, bool) {
	installed := flags&steamFullyInstalled != 0
	started := flags&steamUpdateStarted != 0
	if appID == "" || !(started || (!installed && flags&steamUpdateRequired != 0)) {
		return SteamDownload{}, false
	}
	d := SteamDownload{
		AppID:   appID,
		Update:  installed,
		Paused:  flags&steamUpdatePaused != 0,
		Done:    done,
		Total:   total,
		Percent: -1,
	}
	if total > 0 {
		d.Percent = min(100, float64(done)*100/float64(total))
	}
	return d, true
}

func atoi64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}
