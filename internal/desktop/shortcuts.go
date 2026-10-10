package desktop

import (
	"sort"
	"strings"

	"shelf/internal/library"
)

// On Windows each game gets a shortcut in a Start menu folder of Shelf's own.
// Shortcuts are named after the game, so the names have to be ones Windows
// accepts, and two games of the same name need telling apart.

// reservedNames are file names Windows keeps for devices.
var reservedNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true, "com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true, "lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// shortcutName makes a game's name into a file name Windows accepts.
func shortcutName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r < ' ' || r == 0x7f:
			b.WriteRune(' ')
		case strings.ContainsRune(`<>:"/\|?*`, r):
			b.WriteRune(' ')
		default:
			b.WriteRune(r)
		}
	}
	s := strings.Join(strings.Fields(b.String()), " ")
	if r := []rune(s); len(r) > maxEntryName {
		s = string(r[:maxEntryName])
	}
	s = strings.TrimRight(s, ". ")
	if s == "" || reservedNames[strings.ToLower(s)] {
		s = "Game " + s
	}
	return s
}

var storeNames = map[library.Source]string{
	library.SourceSteam: "Steam", library.SourceEpic: "Epic", library.SourceGog: "GOG", library.SourceUbisoft: "Ubisoft",
}

// shortcutFiles names a .lnk file for every game with a valid id. A name two
// games share gets the store added, and the id if that isn't enough.
func shortcutFiles(games []Entry) map[string]string {
	valid := make([]Entry, 0, len(games))
	count := map[string]int{}
	for _, g := range games {
		if !library.ValidGameID(g.ID) {
			continue
		}
		valid = append(valid, g)
		count[strings.ToLower(shortcutName(g.Name))]++
	}
	sort.Slice(valid, func(i, j int) bool { return valid[i].ID < valid[j].ID })

	out := map[string]string{}
	taken := map[string]bool{}
	for _, g := range valid {
		base := shortcutName(g.Name)
		if count[strings.ToLower(base)] > 1 {
			store, _, _ := strings.Cut(g.ID, ":")
			if s := storeNames[library.Source(store)]; s != "" {
				base += " (" + s + ")"
			}
		}
		if taken[strings.ToLower(base)] {
			base += " (" + shortcutName(g.ID) + ")"
		}
		taken[strings.ToLower(base)] = true
		out[g.ID] = base + ".lnk"
	}
	return out
}
