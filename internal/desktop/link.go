package desktop

import (
	"net/url"
	"regexp"
	"strings"
)

// A game id is a store and the store's own id, e.g. "steam:620".
var gameIDRe = regexp.MustCompile(`^(steam|epic|ubisoft):[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// LaunchURL is the link that starts a game: shelf://launch/steam:620.
func LaunchURL(gameID string) string { return "shelf://launch/" + gameID }

// ParseLaunchURL finds the game a shelf://launch/<id> link names.
func ParseLaunchURL(arg string) (string, bool) {
	u, err := url.Parse(arg)
	if err != nil || u.Scheme != "shelf" || u.Host != "launch" {
		return "", false
	}
	id := strings.TrimPrefix(u.Path, "/")
	if !gameIDRe.MatchString(id) {
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
