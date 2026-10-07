package epic

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// LoginURL is legendary's redirect to Epic's login. After signing in, the page
// shows a JSON blob containing an authorizationCode.
const LoginURL = "https://legendary.gl/epiclogin"

var codeRe = regexp.MustCompile(`^[A-Za-z0-9]{32}$`)

// Account describes the connection state shown in the UI.
type Account struct {
	LegendaryFound bool   `json:"legendaryFound"`
	LoggedIn       bool   `json:"loggedIn"`
	Name           string `json:"name"`
}

func (m *Manager) Account() Account {
	_, err := findLegendary()
	acc := Account{LegendaryFound: err == nil}

	data, err := os.ReadFile(filepath.Join(m.cfgDir, "user.json"))
	if err != nil {
		return acc
	}
	var u struct {
		DisplayName string `json:"displayName"`
		AccountID   string `json:"account_id"`
	}
	if json.Unmarshal(data, &u) != nil || (u.DisplayName == "" && u.AccountID == "") {
		return acc
	}
	acc.LoggedIn = true
	acc.Name = u.DisplayName
	return acc
}

// ExtractCode accepts what users actually paste: the bare code, a quoted code,
// or the whole JSON document from the login page.
func ExtractCode(input string) (string, error) {
	in := strings.TrimSpace(input)
	if strings.HasPrefix(in, "{") {
		var doc struct {
			Code string `json:"authorizationCode"`
		}
		if err := json.Unmarshal([]byte(in), &doc); err != nil {
			return "", fmt.Errorf("couldn't read the pasted login response")
		}
		in = doc.Code
	}
	in = strings.Trim(in, " \t\r\n\"'")
	if !codeRe.MatchString(in) {
		return "", fmt.Errorf("that doesn't look like an authorization code")
	}
	return in, nil
}

func (m *Manager) Login(input string) error {
	code, err := ExtractCode(input)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if _, err := m.run(ctx, "auth", "--code", code); err != nil {
		return err
	}
	m.invalidateOwned()
	m.send("library:changed", nil)
	return nil
}

func (m *Manager) Logout() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := m.run(ctx, "auth", "--delete"); err != nil {
		return err
	}
	m.invalidateOwned()
	m.send("library:changed", nil)
	return nil
}
