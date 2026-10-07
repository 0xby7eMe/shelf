package library

import (
	"net/http"
	"path/filepath"
	"strings"
)

// uiSounds maps the names the front end asks for to Steam's own Deck UI sounds.
// They are played from the user's Steam installation; Shelf doesn't ship them.
var uiSounds = map[string]string{
	"navigation": "deck_ui_navigation.wav",
	"confirm":    "deck_ui_default_activation.wav",
	"back":       "deck_ui_out_of_game_detail.wav",
	"tab":        "deck_ui_tab_transition_01.wav",
	"blocked":    "deck_ui_bumper_end_02.wav",
}

// Sounds serves /sfx/<name> from Steam's steamui/sounds folder. It answers 404
// when Steam isn't installed or the file is missing, which tells the front end
// to use its built-in sounds.
func Sounds(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, ok := strings.CutPrefix(r.URL.Path, "/sfx/")
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		file, known := uiSounds[name]
		root, err := findSteamRoot()
		if !known || err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=3600")
		http.ServeFile(w, r, filepath.Join(root, "steamui", "sounds", file))
	})
}
