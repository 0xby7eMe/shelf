package desktop

import (
	"net/url"
	"strings"

	"shelf/internal/library"
)

// maxEntryName is the longest game name a menu entry or shortcut carries.
const maxEntryName = 120

// Entry is a game that gets an application menu entry.
type Entry struct {
	ID   string // "steam:620"
	Name string
}

// LaunchURL is the link that starts a game: shelf://launch/steam:620.
func LaunchURL(gameID string) string { return "shelf://launch/" + gameID }

// ParseLaunchURL finds the game a shelf://launch/<id> link names.
func ParseLaunchURL(arg string) (string, bool) {
	u, err := url.Parse(arg)
	if err != nil || u.Scheme != "shelf" || u.Host != "launch" {
		return "", false
	}
	id := strings.TrimPrefix(u.Path, "/")
	if !library.ValidGameID(id) {
		return "", false
	}
	return id, true
}

// LaunchFromArgs returns the game named by the first launch link in args.
func LaunchFromArgs(args []string) (string, bool) {
	for _, a := range args {
		if id, ok := ParseLaunchURL(a); ok {
			return id, true
		}
	}
	return "", false
}
