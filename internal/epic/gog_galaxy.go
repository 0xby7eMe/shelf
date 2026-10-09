package epic

// What the rest of Shelf needs from the GOG login: friends and achievements
// live on GOG Galaxy's services. Friends answer to the account's own token;
// achievements belong to a game's own Galaxy client, which needs a token of
// its own, made from the account's refresh token.

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Where GOG keeps a game's builds. A variable so tests can use a local server.
var gogContentBase = "https://content-system.gog.com"

// gogGameClient is a game's own GOG Galaxy client, from its build manifest.
type gogGameClient struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

// gogClientsMu keeps writes of gog-clients.json from crossing.
var gogClientsMu sync.Mutex

type gogGameToken struct {
	token   string
	expires time.Time
}

// gogNoGalaxy means a game doesn't use GOG Galaxy's services, so it has no
// achievements. Callers tell it apart with its NoGalaxy method.
type gogNoGalaxy struct{}

func (gogNoGalaxy) Error() string {
	return "this game doesn't use GOG Galaxy, so it has no achievements"
}
func (gogNoGalaxy) NoGalaxy() bool { return true }

var errGogNoGalaxy error = gogNoGalaxy{}

func gogClientsPath() string {
	if dir := configDir(); dir != "" {
		return filepath.Join(dir, "gog-clients.json")
	}
	return ""
}

// GogUserID is the GOG account's user id, or "" when signed out.
func (m *Manager) GogUserID() string {
	if t := m.gogLogin(); t != nil {
		return t.UserID
	}
	return ""
}

// GogTitle is the name of a game in the GOG library, or "" when it isn't in it.
func (m *Manager) GogTitle(key string) string {
	m.gogMu.Lock()
	defer m.gogMu.Unlock()
	for _, g := range m.gogOwned {
		if g.key() == key {
			return g.Title
		}
	}
	return ""
}

// gogServiceURL reports whether rawURL is one of GOG's services, which are
// the only places the login may go.
func gogServiceURL(rawURL string) bool {
	for _, base := range []string{gogAuthBase, gogEmbedBase, gogAPIBase, gogGamesDBBase, gogContentBase} {
		if strings.HasPrefix(rawURL, base+"/") {
			return true
		}
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme != "https" || u.User != nil {
		return false
	}
	host := u.Hostname()
	return host == "gog.com" || strings.HasSuffix(host, ".gog.com")
}

// GogGet fetches a JSON document from one of GOG's services, signed in as the account.
func (m *Manager) GogGet(ctx context.Context, rawURL string, out any) error {
	if !gogServiceURL(rawURL) {
		return fmt.Errorf("not a GOG address: %s", rawURL)
	}
	return m.gogGet(ctx, rawURL, true, out)
}

// GogGameGet fetches a JSON document from a game's own Galaxy services, signed
// with a token for that game. address gets the game's Galaxy client id.
func (m *Manager) GogGameGet(ctx context.Context, key string, address func(clientID string) string, out any) error {
	if !gogKeyRe.MatchString(key) {
		return fmt.Errorf("invalid game id")
	}
	client, err := m.gogGameClient(ctx, key)
	if err != nil {
		return err
	}
	rawURL := address(client.ClientID)
	if !gogServiceURL(rawURL) {
		return fmt.Errorf("not a GOG address: %s", rawURL)
	}
	tok, err := m.gogGameAccessToken(ctx, key, client)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := m.gogHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("couldn't reach GOG: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		// The token may have been revoked early; the next try makes a new one.
		m.gogMu.Lock()
		delete(m.gogGameToks, key)
		m.gogMu.Unlock()
		return fmt.Errorf("GOG refused the game's login. Try again")
	case resp.StatusCode == http.StatusNotFound:
		return errGogNoGalaxy
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("GOG answered %s", resp.Status)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(out); err != nil {
		return fmt.Errorf("unexpected answer from GOG: %w", err)
	}
	return nil
}

// gogGameClient finds a game's Galaxy client: from memory, from disk, or from
// the manifest of its newest build.
func (m *Manager) gogGameClient(ctx context.Context, key string) (gogGameClient, error) {
	m.gogMu.Lock()
	if m.gogClients == nil {
		m.gogClients = map[string]gogGameClient{}
		if data, err := os.ReadFile(gogClientsPath()); err == nil {
			_ = json.Unmarshal(data, &m.gogClients)
		}
	}
	c, ok := m.gogClients[key]
	m.gogMu.Unlock()
	if ok {
		if c.ClientID == "" {
			return c, errGogNoGalaxy
		}
		return c, nil
	}

	c, err := m.fetchGogGameClient(ctx, key)
	if err != nil && !errors.Is(err, errGogNoGalaxy) {
		return c, err
	}
	// Remember games without Galaxy too, so they aren't looked up every scan.
	m.gogMu.Lock()
	m.gogClients[key] = c
	all := make(map[string]gogGameClient, len(m.gogClients))
	for k, v := range m.gogClients {
		all[k] = v
	}
	m.gogMu.Unlock()
	gogClientsMu.Lock()
	_ = writePrivateJSON(gogClientsPath(), all)
	gogClientsMu.Unlock()
	return c, err
}

// fetchGogGameClient reads the client id and secret from the game's newest
// build. Every game that uses Galaxy has them; the Windows build is the one
// almost every game has, so it is tried first.
func (m *Manager) fetchGogGameClient(ctx context.Context, key string) (gogGameClient, error) {
	id := strings.TrimPrefix(key, "gog-")
	for _, platform := range []string{"windows", "osx"} {
		var builds struct {
			Items []struct {
				Link string `json:"link"`
			} `json:"items"`
		}
		u := fmt.Sprintf("%s/products/%s/os/%s/builds?generation=2", gogContentBase, id, platform)
		if err := m.gogGet(ctx, u, true, &builds); err != nil {
			if errors.Is(err, errGogNotFound) {
				continue
			}
			return gogGameClient{}, err
		}
		if len(builds.Items) == 0 || builds.Items[0].Link == "" {
			continue
		}
		var meta gogGameClient
		if err := m.gogManifest(ctx, builds.Items[0].Link, &meta); err != nil {
			return gogGameClient{}, err
		}
		if meta.ClientID == "" || meta.ClientSecret == "" {
			return gogGameClient{}, errGogNoGalaxy
		}
		return meta, nil
	}
	return gogGameClient{}, errGogNoGalaxy
}

// gogManifest reads a build manifest. GOG compresses newer ones with zlib.
func (m *Manager) gogManifest(ctx context.Context, link string, out any) error {
	u, err := url.Parse(link)
	if err != nil || (u.Scheme != "https" && !strings.HasPrefix(link, gogContentBase+"/")) {
		return fmt.Errorf("unexpected build address from GOG")
	}
	resp, err := m.gogRequest(ctx, link, false)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GOG answered %s for the game's build", resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return err
	}
	if zr, err := zlib.NewReader(bytes.NewReader(raw)); err == nil {
		if plain, err := io.ReadAll(io.LimitReader(zr, 64<<20)); err == nil {
			raw = plain
		}
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("couldn't read the game's build: %w", err)
	}
	return nil
}

// gogGameAccessToken gets a token for a game's client from the account's
// refresh token, without starting a new login session, as Galaxy does when a
// game starts.
func (m *Manager) gogGameAccessToken(ctx context.Context, key string, c gogGameClient) (string, error) {
	m.gogMu.Lock()
	t, ok := m.gogGameToks[key]
	m.gogMu.Unlock()
	if ok && time.Now().Before(t.expires.Add(-time.Minute)) {
		return t.token, nil
	}
	login := m.gogLogin()
	if login == nil {
		return "", fmt.Errorf("not signed in to GOG")
	}
	// Refreshes the account's own token if need be, which also catches a revoked login.
	if _, err := m.gogAccessToken(ctx); err != nil {
		return "", err
	}
	login = m.gogLogin()
	fresh, err := m.gogTokenRequest(ctx, url.Values{
		"client_id":           {c.ClientID},
		"client_secret":       {c.ClientSecret},
		"grant_type":          {"refresh_token"},
		"refresh_token":       {login.RefreshToken},
		"without_new_session": {"1"},
	})
	if err != nil {
		return "", err
	}
	m.gogMu.Lock()
	m.gogGameToks[key] = gogGameToken{token: fresh.AccessToken, expires: time.Unix(fresh.Expires, 0)}
	m.gogMu.Unlock()
	return fresh.AccessToken, nil
}

// forgetGogGameTokens drops the games' tokens, which belong to the account.
func (m *Manager) forgetGogGameTokens() {
	m.gogMu.Lock()
	m.gogGameToks = map[string]gogGameToken{}
	m.gogMu.Unlock()
}
