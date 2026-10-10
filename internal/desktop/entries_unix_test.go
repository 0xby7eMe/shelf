//go:build !windows

package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncEntries(t *testing.T) {
	dir := t.TempDir()
	other := filepath.Join(dir, "shelf.desktop") // the package's own entry, must survive
	os.WriteFile(other, []byte("[Desktop Entry]\nName=Shelf\n"), 0o644)
	foreign := filepath.Join(dir, "shelf-notes.desktop") // looks like ours but isn't marked
	os.WriteFile(foreign, []byte("[Desktop Entry]\nName=Notes\n"), 0o644)

	exe := `/opt/My "Apps"/shelf`
	games := []Entry{
		{ID: "steam:620", Name: "Portal 2"},
		{ID: "epic:Fortnite", Name: "Line\nbreak $HOME 100%"},
		{ID: "bad id", Name: "Ignored"},
	}
	w, r, err := SyncEntries(dir, exe, games)
	if err != nil || w != 2 || r != 0 {
		t.Fatalf("first sync: %d written, %d removed, %v", w, r, err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "shelf-steam-620.desktop"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, want := range []string{
		"Name=Portal 2\n",
		`Exec="/opt/My \"Apps\"/shelf" "shelf://launch/steam:620"` + "\n",
		"Categories=Game;\n",
		entryMarker,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("entry lacks %q:\n%s", want, s)
		}
	}
	epicRaw, _ := os.ReadFile(filepath.Join(dir, "shelf-epic-Fortnite.desktop"))
	if strings.Count(string(epicRaw), "\n") != strings.Count(s, "\n") {
		t.Errorf("a newline in the game name leaked into the entry:\n%s", epicRaw)
	}

	// Nothing changed: nothing written.
	if w, r, _ := SyncEntries(dir, exe, games); w != 0 || r != 0 {
		t.Errorf("second sync: %d written, %d removed", w, r)
	}

	// A game goes away.
	w, r, _ = SyncEntries(dir, exe, games[:1])
	if w != 0 || r != 1 {
		t.Errorf("after uninstall: %d written, %d removed", w, r)
	}
	if _, err := os.Stat(filepath.Join(dir, "shelf-epic-Fortnite.desktop")); !os.IsNotExist(err) {
		t.Error("stale entry kept")
	}
	for _, p := range []string{other, foreign} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s was touched", filepath.Base(p))
		}
	}

	if n, err := RemoveEntries(dir); err != nil || n != 1 {
		t.Errorf("RemoveEntries = %d, %v", n, err)
	}
}

func TestURLHandlerFile(t *testing.T) {
	dir := t.TempDir()
	if URLHandlerRegistered(dir) {
		t.Fatal("registered before anything was written")
	}
	// A PATH with no helper programs makes the helpers no-ops.
	t.Setenv("PATH", t.TempDir())
	if err := RegisterURLHandler(dir, "/usr/bin/shelf", nil); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, handlerFile))
	for _, want := range []string{`Exec="/usr/bin/shelf" %u`, "MimeType=x-scheme-handler/shelf;", "NoDisplay=true"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("handler lacks %q:\n%s", want, raw)
		}
	}
	if !URLHandlerRegistered(dir) {
		t.Error("not registered after writing")
	}
	// RemoveEntries must not delete the handler.
	RemoveEntries(dir)
	if !URLHandlerRegistered(dir) {
		t.Error("RemoveEntries deleted the handler")
	}
	if err := UnregisterURLHandler(dir, nil); err != nil || URLHandlerRegistered(dir) {
		t.Errorf("unregister: %v", err)
	}
	if err := UnregisterURLHandler(dir, nil); err != nil {
		t.Errorf("unregistering twice: %v", err)
	}
}
