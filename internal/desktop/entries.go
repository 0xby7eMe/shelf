package desktop

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"shelf/internal/library"
)

const (
	entryPrefix = "shelf-"
	// Marks the entries Shelf wrote, so it never touches anyone else's.
	entryMarker  = "X-Shelf-Game=true"
	handlerFile  = "shelf-url-handler.desktop"
	handlerMime  = "x-scheme-handler/shelf"
	maxEntryName = 120
)

// Entry is a game that gets an application menu entry.
type Entry struct {
	ID   string // "steam:620"
	Name string
}

// ApplicationsDir is where the desktop looks for the current user's menu entries.
func ApplicationsDir() string {
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "applications")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".local", "share", "applications")
}

// Executable is the program a menu entry should start: the AppImage when
// running from one, since the unpacked binary only lives until Shelf exits.
func Executable() (string, error) {
	if p := os.Getenv("APPIMAGE"); p != "" {
		return p, nil
	}
	return os.Executable()
}

func entryFileName(id string) string {
	return entryPrefix + strings.NewReplacer(":", "-").Replace(id) + ".desktop"
}

// execQuote quotes one argument of an Exec line as the desktop entry spec asks.
func execQuote(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", `$`, `\$`, `%`, `%%`)
	return `"` + r.Replace(s) + `"`
}

// textValue makes a string safe for a single-line desktop entry value.
func textValue(s string) string {
	s = strings.Join(strings.FieldsFunc(s, func(r rune) bool { return r < ' ' || r == 0x7f }), " ")
	if len([]rune(s)) > maxEntryName {
		s = string([]rune(s)[:maxEntryName])
	}
	return strings.ReplaceAll(s, `\`, `\\`)
}

func renderEntry(exe string, e Entry) string {
	var b strings.Builder
	b.WriteString("[Desktop Entry]\n")
	b.WriteString("Type=Application\n")
	b.WriteString("Name=" + textValue(e.Name) + "\n")
	b.WriteString("Comment=Play with Shelf\n")
	b.WriteString("Exec=" + execQuote(exe) + " " + execQuote(LaunchURL(e.ID)) + "\n")
	b.WriteString("Icon=io.github.0xby7eme.shelf\n")
	b.WriteString("Categories=Game;\n")
	b.WriteString("Terminal=false\n")
	b.WriteString("StartupNotify=false\n")
	b.WriteString(entryMarker + "\n")
	return b.String()
}

// SyncEntries makes the menu entries in dir match games: it writes the missing
// or changed ones and removes those of games that are gone. It returns how many
// files it wrote and removed.
func SyncEntries(dir, exe string, games []Entry) (written, removed int, err error) {
	if dir == "" {
		return 0, 0, os.ErrNotExist
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, 0, err
	}

	wanted := map[string]bool{}
	for _, g := range games {
		if !library.ValidGameID(g.ID) {
			continue
		}
		name := entryFileName(g.ID)
		wanted[name] = true
		content := renderEntry(exe, g)
		path := filepath.Join(dir, name)
		if old, err := os.ReadFile(path); err == nil && string(old) == content {
			continue
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return written, removed, err
		}
		written++
	}

	removed, err = removeEntries(dir, wanted)
	return written, removed, err
}

// RemoveEntries deletes every menu entry Shelf wrote.
func RemoveEntries(dir string) (int, error) {
	if dir == "" {
		return 0, nil
	}
	return removeEntries(dir, nil)
}

func removeEntries(dir string, keep map[string]bool) (int, error) {
	files, err := filepath.Glob(filepath.Join(dir, entryPrefix+"*.desktop"))
	if err != nil {
		return 0, err
	}
	sort.Strings(files)
	removed := 0
	for _, f := range files {
		name := filepath.Base(f)
		if keep[name] || name == handlerFile {
			continue
		}
		raw, err := os.ReadFile(f)
		if err != nil || !strings.Contains(string(raw), entryMarker) {
			continue
		}
		if err := os.Remove(f); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

// RegisterURLHandler makes Shelf the program that opens shelf:// links.
// env is the environment for the helper programs it runs.
func RegisterURLHandler(dir, exe string, env []string) error {
	if dir == "" {
		return os.ErrNotExist
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	content := "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=Shelf\n" +
		"Comment=Opens shelf:// links\n" +
		"Exec=" + execQuote(exe) + " %u\n" +
		"Icon=io.github.0xby7eme.shelf\n" +
		"Terminal=false\n" +
		"NoDisplay=true\n" +
		"MimeType=" + handlerMime + ";\n"
	if err := os.WriteFile(filepath.Join(dir, handlerFile), []byte(content), 0o644); err != nil {
		return err
	}
	runHelper(env, "update-desktop-database", dir)
	return runHelper(env, "xdg-mime", "default", handlerFile, handlerMime)
}

// UnregisterURLHandler removes the handler Shelf registered.
func UnregisterURLHandler(dir string, env []string) error {
	if dir == "" {
		return nil
	}
	err := os.Remove(filepath.Join(dir, handlerFile))
	if os.IsNotExist(err) {
		err = nil
	}
	runHelper(env, "update-desktop-database", dir)
	return err
}

// URLHandlerRegistered reports whether Shelf's handler file is in place.
func URLHandlerRegistered(dir string) bool {
	if dir == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, handlerFile))
	return err == nil
}

// runHelper runs a desktop utility. A missing one is not an error worth
// failing for: the file is in place and most desktops pick it up anyway.
func runHelper(env []string, name string, args ...string) error {
	path, err := exec.LookPath(name)
	if err != nil {
		return nil
	}
	cmd := exec.Command(path, args...)
	cmd.Env = env
	return cmd.Run()
}
