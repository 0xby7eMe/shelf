package epic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"shelf/internal/library"
)

// gogProduct is one game in the account, as GOG lists it, plus the art GamesDB has for it.
type gogProduct struct {
	ID      int64  `json:"id"`
	Title   string `json:"title"`
	Image   string `json:"image"` // protocol-relative and without extension, e.g. //images-1.gog-statics.com/<hash>
	URL     string `json:"url"`   // store page path, e.g. /en/game/the_witcher
	Slug    string `json:"slug"`
	IsGame  bool   `json:"isGame"`
	WorksOn struct {
		Windows bool `json:"Windows"`
		Mac     bool `json:"Mac"`
		Linux   bool `json:"Linux"`
	} `json:"worksOn"`

	Cover   string `json:"cover,omitempty"`
	Hero    string `json:"hero,omitempty"`
	ArtDone bool   `json:"artDone,omitempty"` // GamesDB was asked, whether or not it had art
}

func (p gogProduct) key() string { return gogKey(p.ID) }

// gogProductsPage is one page of GOG's account product list.
type gogProductsPage struct {
	TotalPages int          `json:"totalPages"`
	Products   []gogProduct `json:"products"`
}

func gogLibraryPath() string {
	if dir := configDir(); dir != "" {
		return filepath.Join(dir, "gog-library.json")
	}
	return ""
}

func (m *Manager) invalidateGogOwned() {
	m.gogMu.Lock()
	m.gogOwned = nil
	m.gogMu.Unlock()
	if p := gogLibraryPath(); p != "" {
		os.Remove(p)
	}
}

// fetchGogOwned lists every game in the account, page by page, and looks up
// art for the ones that are new since last time.
func (m *Manager) fetchGogOwned(ctx context.Context) ([]gogProduct, error) {
	var all []gogProduct
	for page := 1; ; page++ {
		var p gogProductsPage
		u := fmt.Sprintf("%s/account/getFilteredProducts?mediaType=1&sortBy=title&page=%d", gogEmbedBase, page)
		if err := m.gogGet(ctx, u, true, &p); err != nil {
			return nil, err
		}
		for _, g := range p.Products {
			if g.ID > 0 && g.Title != "" {
				all = append(all, g)
			}
		}
		if page >= p.TotalPages || page >= 200 {
			break
		}
	}

	// Art doesn't change; keep what is known.
	m.gogMu.Lock()
	known := map[int64]gogProduct{}
	for _, g := range m.gogOwned {
		known[g.ID] = g
	}
	m.gogMu.Unlock()
	if len(known) == 0 {
		for _, g := range readGogLibrary() {
			known[g.ID] = g
		}
	}
	for i, g := range all {
		if k, ok := known[g.ID]; ok && k.ArtDone {
			all[i].Cover, all[i].Hero, all[i].ArtDone = k.Cover, k.Hero, true
		}
	}
	m.fillGogArt(ctx, all)

	m.gogMu.Lock()
	m.gogOwned = all
	m.gogMu.Unlock()
	if p := gogLibraryPath(); p != "" {
		_ = writeJSON(p, all)
	}
	return all, nil
}

func readGogLibrary() []gogProduct {
	data, err := os.ReadFile(gogLibraryPath())
	if err != nil {
		return nil
	}
	var games []gogProduct
	if json.Unmarshal(data, &games) != nil {
		return nil
	}
	return games
}

// gamesDBRelease is the part of a GamesDB entry that holds art.
type gamesDBRelease struct {
	Game struct {
		VerticalCover struct {
			URLFormat string `json:"url_format"`
		} `json:"vertical_cover"`
		Background struct {
			URLFormat string `json:"url_format"`
		} `json:"background"`
	} `json:"game"`
}

// gamesDBImage turns a GamesDB url_format into a plain JPEG address.
func gamesDBImage(format string) string {
	if format == "" {
		return ""
	}
	return strings.NewReplacer("{formatter}", "", "{ext}", "jpg").Replace(format)
}

// fillGogArt asks GamesDB for posters and banners, a few games at a time.
// The account list only has a wide thumbnail, which makes a poor poster.
func (m *Manager) fillGogArt(ctx context.Context, games []gogProduct) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6)
	for i := range games {
		if games[i].ArtDone {
			continue
		}
		wg.Add(1)
		go func(g *gogProduct) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			var rel gamesDBRelease
			u := fmt.Sprintf("%s/platforms/gog/external_releases/%d", gogGamesDBBase, g.ID)
			if err := m.gogGet(ctx, u, false, &rel); err != nil {
				return // try again next sync
			}
			g.Cover = gamesDBImage(rel.Game.VerticalCover.URLFormat)
			g.Hero = gamesDBImage(rel.Game.Background.URLFormat)
			g.ArtDone = true
		}(&games[i])
	}
	wg.Wait()
}

// gogOwnedGames is the library as last fetched, from memory or disk, or GOG if neither has it.
func (m *Manager) gogOwnedGames(ctx context.Context) ([]gogProduct, error) {
	m.gogMu.Lock()
	cached := m.gogOwned
	m.gogMu.Unlock()
	if cached != nil {
		return cached, nil
	}
	if games := readGogLibrary(); games != nil {
		m.gogMu.Lock()
		m.gogOwned = games
		m.gogMu.Unlock()
		return games, nil
	}
	return m.fetchGogOwned(ctx)
}

// gogProductByKey finds a game of the library by its external id.
func (m *Manager) gogProductByKey(key string) (gogProduct, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	games, _ := m.gogOwnedGames(ctx)
	for _, g := range games {
		if g.key() == key {
			return g, true
		}
	}
	return gogProduct{}, false
}

// GogSync lists the account's games again.
func (m *Manager) GogSync() error {
	if !m.GogAccount().LoggedIn {
		return fmt.Errorf("not signed in to GOG")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	games, err := m.fetchGogOwned(ctx)
	if err != nil {
		return err
	}
	m.logf("gog", "", "library: %d games", len(games))
	m.send("library:changed", nil)
	return nil
}

// gogPoster picks the art for a game's poster: GamesDB's cover, or the store thumbnail.
func gogPoster(g gogProduct) string {
	if g.Cover != "" {
		return g.Cover
	}
	if g.Image != "" {
		return "https:" + g.Image + ".jpg"
	}
	return ""
}

// GogProvider returns the library.Provider for GOG games.
func (m *Manager) GogProvider() library.Provider { return gogProvider{m} }

type gogProvider struct{ m *Manager }

func (gogProvider) Source() library.Source { return library.SourceGog }

func (p gogProvider) Scan() ([]library.Game, error) {
	m := p.m
	if !m.GogAccount().LoggedIn {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	owned, err := m.gogOwnedGames(ctx)
	if err != nil {
		return nil, err
	}
	installed := m.gogInstalled()

	games := make([]library.Game, 0, len(owned))
	seen := map[string]bool{}
	for _, o := range owned {
		key := o.key()
		if seen[key] {
			continue
		}
		seen[key] = true
		g := library.Game{
			ID:         "gog:" + key,
			Source:     library.SourceGog,
			ExternalID: key,
			Name:       o.Title,
			Cover:      gogPoster(o),
			Hero:       o.Hero,
		}
		if in, ok := installed[key]; ok {
			g.Installed = true
			g.InstallPath = in.InstallPath
			g.Version = in.Version
			g.SizeBytes = in.InstallSize
			g.AntiCheat = antiCheatOf(in.InstallPath)
		}
		g.PlaytimeMinutes, g.LastPlayed = m.hist.Totals(key)
		games = append(games, g)
	}
	sort.SliceStable(games, func(i, j int) bool { return games[i].Name < games[j].Name })
	return games, nil
}

// GogStoreURL returns a game's page on gog.com.
func (m *Manager) GogStoreURL(key string) (string, error) {
	if !gogKeyRe.MatchString(key) {
		return "", fmt.Errorf("invalid game id")
	}
	g, ok := m.gogProductByKey(key)
	if !ok {
		return "", fmt.Errorf("unknown game")
	}
	switch {
	case strings.HasPrefix(g.URL, "/"):
		return "https://www.gog.com" + g.URL, nil
	case g.Slug != "":
		return "https://www.gog.com/game/" + g.Slug, nil
	}
	return "https://www.gog.com/games?query=" + url.QueryEscape(g.Title), nil
}
