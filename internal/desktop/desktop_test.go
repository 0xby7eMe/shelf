package desktop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseLaunchURL(t *testing.T) {
	good := map[string]string{
		"shelf://launch/steam:620":          "steam:620",
		"shelf://launch/epic:Fortnite":      "epic:Fortnite",
		"shelf://launch/gog:gog-1207658924": "gog:gog-1207658924",
		"shelf://launch/ubisoft:5059":       "ubisoft:5059",
		"shelf://launch/epic:Some.Game_1-x": "epic:Some.Game_1-x",
	}
	for in, want := range good {
		if got, ok := ParseLaunchURL(in); !ok || got != want {
			t.Errorf("ParseLaunchURL(%q) = %q, %v", in, got, ok)
		}
	}
	for _, in := range []string{
		"", "shelf://launch/", "shelf://launch/steam", "shelf://launch/itch:1",
		"shelf://open/steam:620", "http://launch/steam:620", "shelf://launch/steam:../x",
		"shelf://launch/steam:1/extra", "--flag", "shelf://launch/steam:%20", "shelf://launch/steam:1;rm",
	} {
		if id, ok := ParseLaunchURL(in); ok {
			t.Errorf("ParseLaunchURL(%q) accepted as %q", in, id)
		}
	}
	if id, ok := LaunchFromArgs([]string{"-x", "shelf://nope", "shelf://launch/steam:7"}); !ok || id != "steam:7" {
		t.Errorf("LaunchFromArgs = %q, %v", id, ok)
	}
	if id, _ := ParseLaunchURL(LaunchURL("epic:Abc")); id != "epic:Abc" {
		t.Errorf("round trip failed: %q", id)
	}
}

func TestStorePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "desktop.json")
	a := &Store{path: path}
	want := Settings{MenuEntries: true, Tray: true}
	if err := a.Set(want); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	var got Settings
	if json.Unmarshal(raw, &got) != nil || got != want {
		t.Errorf("file holds %s", raw)
	}
}
