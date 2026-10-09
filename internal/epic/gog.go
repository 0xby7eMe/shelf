package epic

// GOG has no command line tool that is packaged as widely as legendary, so
// Shelf talks to GOG's own services: the login GOG Galaxy uses, the account's
// product list, GamesDB for art, and the offline installers, which run
// silently through Proton.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// GOG Galaxy's own client. GOG's login only hands codes to registered clients,
// and this is the one every third-party launcher uses.
const (
	gogClientID     = "46899977096215655"
	gogClientSecret = "9d85c43b1482497dbbce61f6e4aa173a433796eeae2ca8c5f6129f2dc4de46d9"
	gogRedirect     = "https://embed.gog.com/on_login_success?origin=client"
)

// Where GOG's services live. Variables so tests can use a local server.
var (
	gogAuthBase    = "https://auth.gog.com"
	gogEmbedBase   = "https://embed.gog.com"
	gogAPIBase     = "https://api.gog.com"
	gogGamesDBBase = "https://gamesdb.gog.com"
)

// GogLoginURL opens GOG's login. Once signed in, the browser lands on a page
// whose address carries the code to paste into Shelf.
var GogLoginURL = gogAuthBase + "/auth?client_id=" + gogClientID +
	"&redirect_uri=" + url.QueryEscape(gogRedirect) + "&response_type=code&layout=client2"

// gogKeyRe matches the external id of a GOG game: "gog-" and the product id.
// The prefix keeps GOG's numbers apart from Steam's in the shared play history.
var gogKeyRe = regexp.MustCompile(`^gog-[0-9]{1,20}$`)

var gogCodeRe = regexp.MustCompile(`^[A-Za-z0-9_-]{16,1000}$`)

// errGogSignedOut means GOG no longer accepts the saved login.
var errGogSignedOut = errors.New("sign in to GOG again (Settings, Integrations, GOG)")

// errGogNotFound is GOG's 404: it has nothing at that address for the game.
var errGogNotFound = errors.New("GOG doesn't know this game")

func gogKey(id int64) string { return fmt.Sprintf("gog-%d", id) }

// GogAccount is the GOG connection shown in the UI.
type GogAccount struct {
	LoggedIn bool   `json:"loggedIn"`
	Name     string `json:"name"`
}

// gogToken is the saved login. GOG's access tokens last an hour; the refresh
// token gets a new one.
type gogToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
	UserID       string `json:"user_id"`
	Expires      int64  `json:"expires"` // unix seconds
	Username     string `json:"username,omitempty"`
}

func gogTokenPath() string {
	if dir := configDir(); dir != "" {
		return filepath.Join(dir, "gog.json")
	}
	return ""
}

// writePrivateJSON is writeJSON for files only the user may read, such as a login.
func writePrivateJSON(path string, v any) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// gogLogin returns the saved login, reading it from disk the first time.
func (m *Manager) gogLogin() *gogToken {
	m.gogMu.Lock()
	defer m.gogMu.Unlock()
	if m.gogTok == nil {
		if data, err := os.ReadFile(gogTokenPath()); err == nil {
			var t gogToken
			if json.Unmarshal(data, &t) == nil && t.RefreshToken != "" {
				m.gogTok = &t
			}
		}
	}
	return m.gogTok
}

func (m *Manager) setGogLogin(t *gogToken) error {
	m.gogMu.Lock()
	m.gogTok = t
	m.gogMu.Unlock()
	if t == nil {
		if err := os.Remove(gogTokenPath()); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	return writePrivateJSON(gogTokenPath(), t)
}

// GogAccount reports whether Shelf is signed in to GOG, and as whom.
func (m *Manager) GogAccount() GogAccount {
	t := m.gogLogin()
	if t == nil {
		return GogAccount{}
	}
	return GogAccount{LoggedIn: true, Name: t.Username}
}

// ExtractGogCode accepts the bare code or the whole address of the page GOG
// shows after signing in, which is what people tend to copy.
func ExtractGogCode(input string) (string, error) {
	in := strings.Trim(strings.TrimSpace(input), `"'`)
	if strings.Contains(in, "code=") {
		u, err := url.Parse(in)
		if err != nil {
			return "", fmt.Errorf("couldn't read the pasted address")
		}
		in = u.Query().Get("code")
	}
	if !gogCodeRe.MatchString(in) {
		return "", fmt.Errorf("that doesn't look like a GOG login code. Paste the whole address of the page GOG showed after signing in")
	}
	return in, nil
}

// gogTokenRequest asks GOG's login service for a token, for GOG Galaxy's
// client unless params name another.
func (m *Manager) gogTokenRequest(ctx context.Context, params url.Values) (*gogToken, error) {
	if params.Get("client_id") == "" {
		params.Set("client_id", gogClientID)
		params.Set("client_secret", gogClientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gogAuthBase+"/token?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := m.gogHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("couldn't reach GOG: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var t gogToken
	var failure struct {
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	_ = json.Unmarshal(body, &failure)
	if resp.StatusCode != http.StatusOK || failure.Error != "" {
		if failure.Error == "invalid_grant" {
			return nil, errGogSignedOut
		}
		msg := failure.Description
		if msg == "" {
			msg = resp.Status
		}
		return nil, fmt.Errorf("GOG refused the login: %s", msg)
	}
	if err := json.Unmarshal(body, &t); err != nil || t.AccessToken == "" || t.RefreshToken == "" {
		return nil, fmt.Errorf("unexpected answer from GOG's login")
	}
	t.Expires = time.Now().Unix() + t.ExpiresIn
	return &t, nil
}

// GogLogin trades the code from GOG's login page for a login and loads the library.
func (m *Manager) GogLogin(input string) error {
	code, err := ExtractGogCode(input)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	t, err := m.gogTokenRequest(ctx, url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"redirect_uri": {gogRedirect},
	})
	if errors.Is(err, errGogSignedOut) {
		return fmt.Errorf("GOG didn't accept that code. Codes work once and expire quickly, so sign in again and paste the new one")
	}
	if err != nil {
		return err
	}
	if err := m.setGogLogin(t); err != nil {
		return err
	}

	var user struct {
		Username string `json:"username"`
	}
	if err := m.gogGet(ctx, gogEmbedBase+"/userData.json", true, &user); err == nil && user.Username != "" {
		t.Username = user.Username
		_ = m.setGogLogin(t)
	}
	m.logf("gog", "", "signed in to GOG as %s", t.Username)

	m.invalidateGogOwned()
	m.forgetGogGameTokens()
	go func() {
		if err := m.GogSync(); err != nil {
			m.logf("gog", "", "ERROR: library: %v", err)
		}
	}()
	return nil
}

// GogLogout forgets the GOG login. Installed games stay on disk.
func (m *Manager) GogLogout() error {
	if err := m.setGogLogin(nil); err != nil {
		return err
	}
	m.invalidateGogOwned()
	m.forgetGogGameTokens()
	m.send("library:changed", nil)
	return nil
}

// gogAccessToken returns a token that is good for a while yet, refreshing it if need be.
func (m *Manager) gogAccessToken(ctx context.Context) (string, error) {
	m.gogRefreshMu.Lock()
	defer m.gogRefreshMu.Unlock()
	t := m.gogLogin()
	if t == nil {
		return "", fmt.Errorf("not signed in to GOG")
	}
	if time.Now().Unix() < t.Expires-60 {
		return t.AccessToken, nil
	}
	fresh, err := m.gogTokenRequest(ctx, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {t.RefreshToken},
	})
	if err != nil {
		if errors.Is(err, errGogSignedOut) {
			m.logf("gog", "", "GOG no longer accepts the saved login")
		}
		return "", err
	}
	fresh.Username = t.Username
	if err := m.setGogLogin(fresh); err != nil {
		return "", err
	}
	return fresh.AccessToken, nil
}

// gogRequest sends a GET, signed in when auth is set.
func (m *Manager) gogRequest(ctx context.Context, rawURL string, auth bool) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if auth {
		tok, err := m.gogAccessToken(ctx)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := m.gogHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("couldn't reach GOG: %w", err)
	}
	return resp, nil
}

// gogGet fetches a JSON document into out.
func (m *Manager) gogGet(ctx context.Context, rawURL string, auth bool, out any) error {
	resp, err := m.gogRequest(ctx, rawURL, auth)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusUnauthorized && auth:
		return errGogSignedOut
	case resp.StatusCode == http.StatusNotFound:
		return errGogNotFound
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("GOG answered %s", resp.Status)
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32<<20)).Decode(out); err != nil {
		return fmt.Errorf("unexpected answer from GOG: %w", err)
	}
	return nil
}
