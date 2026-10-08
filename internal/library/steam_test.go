package library

import "testing"

func TestParseLastSteamUser(t *testing.T) {
	marked := `"users" { "111" { "Timestamp" "900" } "222" { "MostRecent" "1" "Timestamp" "100" } }`
	if id, err := parseLastSteamUser([]byte(marked)); err != nil || id != "222" {
		t.Errorf("marked: %q, %v", id, err)
	}
	byTime := `"users" { "111" { "Timestamp" "900" } "222" { "Timestamp" "100" } }`
	if id, err := parseLastSteamUser([]byte(byTime)); err != nil || id != "111" {
		t.Errorf("by time: %q, %v", id, err)
	}
	if _, err := parseLastSteamUser([]byte(`"users" { }`)); err == nil {
		t.Error("no accounts: want an error")
	}
}
