// Package nearby finds other copies of Shelf on the local network and works out
// which games you have in common with each of them.
//
// Every Shelf announces itself over mDNS (DNS-SD, "_shelf._tcp") and serves a
// short list of its games over HTTP: store, id, name and whether it is
// installed. Nothing else leaves the computer. The other copies fetch that list
// and compare it with their own.
package nearby

import (
	"sort"
	"strings"
	"unicode"
)

// Game is what a peer learns about one of your games.
type Game struct {
	Source    string `json:"source"` // "steam", "epic", ...
	ID        string `json:"id"`     // the store's id, as in library.Game.ExternalID
	Name      string `json:"name"`
	Installed bool   `json:"installed,omitempty"`
}

// Library is the document a Shelf serves to the others.
type Library struct {
	ID    string `json:"id"`   // stays the same across restarts
	Name  string `json:"name"` // how the user wants to be shown
	Games []Game `json:"games"`
}

// Shared is a game both you and a peer have. The same game bought in two
// stores counts once.
type Shared struct {
	Key            string   `json:"key"`    // the same for a game in every peer's list
	Name           string   `json:"name"`   // as your library names it
	GameID         string   `json:"gameId"` // your copy, as "steam:620"; the installed one if any is
	Installed      bool     `json:"installed"`
	Sources        []string `json:"sources"`      // the stores you have it in
	TheirSources   []string `json:"theirSources"` // the stores they have it in
	TheirInstalled bool     `json:"theirInstalled"`
}

// nameKey reduces a title to letters and digits, so "DOOM Eternal™" and
// "Doom Eternal" match across stores.
func nameKey(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func gameRef(g Game) string { return g.Source + ":" + g.ID }

// groupKey is what a game is known by when comparing: its name, or its store
// and id when the name has nothing to go on.
func groupKey(g Game) string {
	if k := nameKey(g.Name); k != "" {
		return k
	}
	return "#" + gameRef(g)
}

// Common lists the games in both libraries, the ones you could play together
// right now (both installed) first, then by name.
func Common(mine, theirs []Game) []Shared {
	byRef := make(map[string]Game, len(theirs))
	byKey := make(map[string][]Game, len(theirs))
	for _, g := range theirs {
		byRef[gameRef(g)] = g
		k := groupKey(g)
		byKey[k] = append(byKey[k], g)
	}

	// Your copies of one game in different stores make one group.
	var order []string
	groups := map[string][]Game{}
	for _, g := range mine {
		k := groupKey(g)
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], g)
	}

	out := []Shared{}
	for _, k := range order {
		group := groups[k]
		matched := map[string]Game{}
		for _, g := range byKey[k] {
			matched[gameRef(g)] = g
		}
		for _, g := range group {
			if t, ok := byRef[gameRef(g)]; ok {
				matched[gameRef(t)] = t
			}
		}
		if len(matched) == 0 {
			continue
		}

		s := Shared{Key: k, Name: group[0].Name, GameID: gameRef(group[0])}
		for _, g := range group {
			s.Sources = appendOnce(s.Sources, g.Source)
			if g.Installed && !s.Installed {
				s.Installed = true
				s.GameID = gameRef(g)
			}
		}
		for _, t := range matched {
			s.TheirSources = appendOnce(s.TheirSources, t.Source)
			s.TheirInstalled = s.TheirInstalled || t.Installed
		}
		sort.Strings(s.TheirSources)
		out = append(out, s)
	}
	sortShared(out)
	return out
}

// Everyone is the games every list has in common with you, for when more than
// one peer is around. A game counts as installed on their side only if it is
// installed for all of them.
func Everyone(lists [][]Shared) []Shared {
	if len(lists) < 2 {
		return []Shared{}
	}
	counts := map[string]int{}
	merged := map[string]Shared{}
	for _, list := range lists {
		for _, s := range list {
			m, ok := merged[s.Key]
			if !ok {
				m = s
				m.TheirSources = nil
				m.TheirInstalled = true
			}
			for _, src := range s.TheirSources {
				m.TheirSources = appendOnce(m.TheirSources, src)
			}
			m.TheirInstalled = m.TheirInstalled && s.TheirInstalled
			merged[s.Key] = m
			counts[s.Key]++
		}
	}
	out := []Shared{}
	for k, s := range merged {
		if counts[k] == len(lists) {
			sort.Strings(s.TheirSources)
			out = append(out, s)
		}
	}
	sortShared(out)
	return out
}

func sortShared(list []Shared) {
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		ra, rb := a.Installed && a.TheirInstalled, b.Installed && b.TheirInstalled
		if ra != rb {
			return ra
		}
		if la, lb := strings.ToLower(a.Name), strings.ToLower(b.Name); la != lb {
			return la < lb
		}
		return a.Key < b.Key
	})
}

func appendOnce(list []string, s string) []string {
	for _, x := range list {
		if x == s {
			return list
		}
	}
	return append(list, s)
}
