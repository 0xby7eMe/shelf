package desktop

import "testing"

func TestShortcutFiles(t *testing.T) {
	got := shortcutFiles([]Entry{
		{ID: "steam:620", Name: "Portal 2"},
		{ID: "steam:1091500", Name: "Cyberpunk 2077"},
		{ID: "gog:gog-1423049311", Name: "Cyberpunk 2077"},
		{ID: "epic:Fortnite", Name: `Fort<nite>: "Battle" Royale?`},
		{ID: "epic:Con", Name: "CON"},
		{ID: "epic:Dots", Name: "Ends with dots..."},
		{ID: "steam:1", Name: "Twin"},
		{ID: "steam:2", Name: "Twin"},
		{ID: "bad id", Name: "Ignored"},
	})
	want := map[string]string{
		"steam:620":          "Portal 2.lnk",
		"steam:1091500":      "Cyberpunk 2077 (Steam).lnk",
		"gog:gog-1423049311": "Cyberpunk 2077 (GOG).lnk",
		"epic:Fortnite":      "Fort nite Battle Royale.lnk",
		"epic:Con":           "Game CON.lnk",
		"epic:Dots":          "Ends with dots.lnk",
		"steam:1":            "Twin (Steam).lnk",
		"steam:2":            "Twin (Steam) (steam 2).lnk",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d files: %v", len(got), got)
	}
	for id, file := range want {
		if got[id] != file {
			t.Errorf("%s: %q, want %q", id, got[id], file)
		}
	}
}
