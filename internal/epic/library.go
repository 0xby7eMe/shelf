package epic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"shelf/internal/library"
)

type keyImage struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// ownedGame is the part of `legendary list --json` Shelf cares about.
type ownedGame struct {
	AppName  string `json:"app_name"`
	AppTitle string `json:"app_title"`
	Metadata struct {
		KeyImages        []keyImage `json:"keyImages"`
		CustomAttributes map[string]struct {
			Value string `json:"value"`
		} `json:"customAttributes"`
	} `json:"metadata"`
}

// installedGame mirrors an entry of legendary's installed.json.
type installedGame struct {
	AppName     string `json:"app_name"`
	Title       string `json:"title"`
	InstallPath string `json:"install_path"`
	Executable  string `json:"executable"`
	LaunchParam string `json:"launch_parameters"`
	IsDLC       bool   `json:"is_dlc"`
	Version     string `json:"version"`
	InstallSize int64  `json:"install_size"`
	Platform    string `json:"platform"`
}

// thirdPartyStore is the launcher a game has to be installed through, if any.
// Epic only hands out an activation for those; legendary can't download them.
func (o ownedGame) thirdPartyStore() string {
	attrs := o.Metadata.CustomAttributes
	if v := attrs["ThirdPartyManagedApp"].Value; v != "" {
		return storeName(v)
	}
	return storeName(attrs["ThirdPartyManagedProvider"].Value)
}

func storeName(raw string) string {
	switch strings.ToLower(raw) {
	case "":
		return ""
	case "ubisoftconnect":
		return "Ubisoft Connect"
	case "origin", "the ea app":
		return "the EA app"
	}
	return raw
}

// supportsCloudSaves reports whether Epic gave the game a cloud save folder.
func (o ownedGame) supportsCloudSaves() bool {
	return o.Metadata.CustomAttributes["CloudSaveFolder"].Value != ""
}

func (m *Manager) ownedCachePath() string {
	if m.cfgDir == "" {
		return ""
	}
	return filepath.Join(configDir(), "epic-library.json")
}

func (m *Manager) invalidateOwned() {
	m.mu.Lock()
	m.owned = nil
	m.mu.Unlock()
	if p := m.ownedCachePath(); p != "" {
		os.Remove(p)
	}
}

// fetchOwned asks legendary for the library. With refresh it hits Epic's API;
// without, legendary answers from its own metadata cache when it can.
func (m *Manager) fetchOwned(ctx context.Context, refresh bool) ([]ownedGame, error) {
	args := []string{"list", "--json"}
	if refresh {
		args = append(args, "--force-refresh")
	}
	out, err := m.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	var games []ownedGame
	if err := json.Unmarshal(out, &games); err != nil {
		return nil, fmt.Errorf("unexpected legendary output: %w", err)
	}

	m.mu.Lock()
	m.owned = games
	m.mu.Unlock()
	if p := m.ownedCachePath(); p != "" {
		if data, err := json.Marshal(games); err == nil {
			_ = os.WriteFile(p, data, 0o644)
		}
	}
	return games, nil
}

func (m *Manager) ownedGames(ctx context.Context) ([]ownedGame, error) {
	m.mu.Lock()
	cached := m.owned
	m.mu.Unlock()
	if cached != nil {
		return cached, nil
	}

	if p := m.ownedCachePath(); p != "" {
		if data, err := os.ReadFile(p); err == nil {
			var games []ownedGame
			if json.Unmarshal(data, &games) == nil {
				m.mu.Lock()
				m.owned = games
				m.mu.Unlock()
				return games, nil
			}
		}
	}
	return m.fetchOwned(ctx, false)
}

// SyncLibrary re-downloads the owned games list from Epic.
func (m *Manager) SyncLibrary() error {
	if !m.Account().LoggedIn {
		return fmt.Errorf("not logged in to Epic Games")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err := m.fetchOwned(ctx, true); err != nil {
		return err
	}
	m.send("library:changed", nil)
	return nil
}

func (m *Manager) readInstalled() map[string]installedGame {
	out := map[string]installedGame{}
	if m.cfgDir == "" {
		return out
	}
	data, err := os.ReadFile(filepath.Join(m.cfgDir, "installed.json"))
	if err != nil {
		return out
	}
	var raw map[string]installedGame
	if json.Unmarshal(data, &raw) != nil {
		return out
	}
	for name, g := range raw {
		if g.IsDLC {
			continue
		}
		g.AppName = name
		out[name] = g
	}
	return out
}

func pickImage(imgs []keyImage, kinds ...string) string {
	for _, k := range kinds {
		for _, img := range imgs {
			if img.Type == k && img.URL != "" {
				return img.URL
			}
		}
	}
	return ""
}

// posterSize asks Epic's image CDN for a poster-sized copy. The originals are
// around 900 KB each, which adds up to far too much decoding for a big library.
func posterSize(u string) string {
	if u == "" || strings.Contains(u, "?") {
		return u
	}
	return u + "?h=600&resize=1&w=400"
}

// Provider returns the library.Provider for Epic games.
func (m *Manager) Provider() library.Provider { return provider{m} }

type provider struct{ m *Manager }

func (provider) Source() library.Source { return library.SourceEpic }

func (p provider) Scan() ([]library.Game, error) {
	m := p.m
	if !m.Account().LoggedIn {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	owned, err := m.ownedGames(ctx)
	if err != nil {
		return nil, err
	}
	installed := m.readInstalled()
	updates := m.pendingUpdates(installed)

	games := make([]library.Game, 0, len(owned))
	seen := map[string]bool{}
	for _, o := range owned {
		if !appNameRe.MatchString(o.AppName) || seen[o.AppName] {
			continue
		}
		seen[o.AppName] = true

		g := library.Game{
			ID:         "epic:" + o.AppName,
			Source:     library.SourceEpic,
			ExternalID: o.AppName,
			Name:       o.AppTitle,
			Cover:      posterSize(pickImage(o.Metadata.KeyImages, "DieselGameBoxTall", "OfferImageTall", "Thumbnail")),
			Hero:       pickImage(o.Metadata.KeyImages, "DieselGameBox", "OfferImageWide", "DieselStoreFrontWide"),
		}
		g.CloudSaves = o.supportsCloudSaves()
		g.ThirdParty = o.thirdPartyStore()
		if in, ok := installed[o.AppName]; ok {
			g.Installed = true
			g.InstallPath = in.InstallPath
			g.Version = in.Version
			g.SizeBytes = in.InstallSize
			g.AntiCheat = antiCheatOf(in.InstallPath)
			_, g.UpdateAvailable = updates[o.AppName]
		}
		g.PlaytimeMinutes, g.LastPlayed = m.hist.Totals(o.AppName)
		games = append(games, g)
	}
	return games, nil
}

// StoreURL returns a store page for a game. Epic's slugs aren't in the library
// metadata, so this searches the store by title.
func (m *Manager) StoreURL(appName string) (string, error) {
	if !appNameRe.MatchString(appName) {
		return "", fmt.Errorf("invalid game id")
	}
	m.mu.Lock()
	owned := m.owned
	m.mu.Unlock()
	for _, o := range owned {
		if o.AppName == appName {
			return "https://store.epicgames.com/browse?q=" + url.QueryEscape(o.AppTitle), nil
		}
	}
	return "", fmt.Errorf("unknown game")
}
