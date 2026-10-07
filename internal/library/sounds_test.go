package library

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSounds(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	steam := filepath.Join(home, ".local", "share", "Steam")
	os.MkdirAll(filepath.Join(steam, "steamapps"), 0o755)
	os.MkdirAll(filepath.Join(steam, "steamui", "sounds"), 0o755)
	os.WriteFile(filepath.Join(steam, "steamui", "sounds", "deck_ui_navigation.wav"), []byte("RIFFdata"), 0o644)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("next")) })
	h := Sounds(next)
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		return rec
	}

	if rec := get("/sfx/navigation"); rec.Code != 200 || rec.Body.String() != "RIFFdata" {
		t.Errorf("navigation: %d %q", rec.Code, rec.Body.String())
	}
	if rec := get("/sfx/confirm"); rec.Code != 404 {
		t.Errorf("missing file should be 404, got %d", rec.Code)
	}
	if rec := get("/sfx/../../etc/passwd"); rec.Code != 404 {
		t.Errorf("unknown name must be refused, got %d", rec.Code)
	}
	if rec := get("/other"); rec.Body.String() != "next" {
		t.Errorf("other paths must pass through, got %q", rec.Body.String())
	}
}
