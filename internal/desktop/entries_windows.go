package desktop

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
	"golang.org/x/sys/windows/registry"
)

// On Windows the "application menu" is a Start menu folder that only Shelf
// writes to, and shelf:// links are a URL protocol registered for the user.

const (
	startMenuFolder = "Shelf Games"
	indexFile       = ".shelf-shortcuts.json"
	protocolKey     = `Software\Classes\shelf`
)

// ApplicationsDir is Shelf's folder in the user's Start menu.
func ApplicationsDir() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return ""
	}
	return filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", startMenuFolder)
}

// Executable is the program a shortcut starts.
func Executable() (string, error) { return os.Executable() }

// shortcutIndex records what each shortcut was made for, so unchanged ones
// aren't written again.
type shortcutIndex struct {
	Exe   string            `json:"exe"`
	Files map[string]string `json:"files"` // file name -> game id
	Names map[string]string `json:"names"` // game id -> game name
}

func readIndex(dir string) shortcutIndex {
	idx := shortcutIndex{Files: map[string]string{}, Names: map[string]string{}}
	if raw, err := os.ReadFile(filepath.Join(dir, indexFile)); err == nil {
		_ = json.Unmarshal(raw, &idx)
	}
	if idx.Files == nil {
		idx.Files = map[string]string{}
	}
	if idx.Names == nil {
		idx.Names = map[string]string{}
	}
	return idx
}

func writeIndex(dir string, idx shortcutIndex) error {
	raw, err := json.Marshal(idx)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, indexFile), raw, 0o644)
}

// SyncEntries makes the shortcuts in dir match games: it writes the missing or
// changed ones and removes those of games that are gone. It returns how many
// it wrote and removed.
func SyncEntries(dir, exe string, games []Entry) (written, removed int, err error) {
	if dir == "" {
		return 0, 0, os.ErrNotExist
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, 0, err
	}
	names := map[string]string{}
	for _, g := range games {
		names[g.ID] = g.Name
	}
	files := shortcutFiles(games)
	old := readIndex(dir)
	next := shortcutIndex{Exe: exe, Files: map[string]string{}, Names: map[string]string{}}

	var todo []shortcut
	for id, file := range files {
		next.Files[file] = id
		next.Names[id] = names[id]
		_, statErr := os.Stat(filepath.Join(dir, file))
		if statErr == nil && old.Exe == exe && old.Files[file] == id && old.Names[id] == names[id] {
			continue
		}
		todo = append(todo, shortcut{
			path:        filepath.Join(dir, file),
			target:      exe,
			args:        `"` + LaunchURL(id) + `"`,
			description: "Play " + names[id] + " with Shelf",
		})
	}
	if len(todo) > 0 {
		if err := writeShortcuts(todo); err != nil {
			return 0, 0, err
		}
		written = len(todo)
	}

	// Only shortcuts Shelf made, as its index says, are ever removed.
	for file := range old.Files {
		if _, keep := next.Files[file]; keep {
			continue
		}
		if err := os.Remove(filepath.Join(dir, file)); err == nil {
			removed++
		} else if !errors.Is(err, os.ErrNotExist) {
			return written, removed, err
		}
	}
	return written, removed, writeIndex(dir, next)
}

// RemoveEntries deletes every shortcut Shelf made, and its folder once empty.
func RemoveEntries(dir string) (int, error) {
	if dir == "" {
		return 0, nil
	}
	removed := 0
	for file := range readIndex(dir).Files {
		if err := os.Remove(filepath.Join(dir, file)); err == nil {
			removed++
		}
	}
	os.Remove(filepath.Join(dir, indexFile))
	os.Remove(dir) // only goes if nothing else is in it
	return removed, nil
}

type shortcut struct {
	path, target, args, description string
}

// writeShortcuts makes .lnk files through the Windows Script Host, on one
// thread set up for COM.
func writeShortcuts(list []shortcut) error {
	errc := make(chan error, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
			var oe *ole.OleError
			// S_FALSE: COM was already set up on this thread, which is fine.
			if !errors.As(err, &oe) || oe.Code() != 1 {
				errc <- err
				return
			}
		}
		defer ole.CoUninitialize()

		unknown, err := oleutil.CreateObject("WScript.Shell")
		if err != nil {
			errc <- err
			return
		}
		defer unknown.Release()
		shell, err := unknown.QueryInterface(ole.IID_IDispatch)
		if err != nil {
			errc <- err
			return
		}
		defer shell.Release()

		for _, s := range list {
			if err := writeShortcut(shell, s); err != nil {
				errc <- err
				return
			}
		}
		errc <- nil
	}()
	return <-errc
}

func writeShortcut(shell *ole.IDispatch, s shortcut) error {
	v, err := oleutil.CallMethod(shell, "CreateShortcut", s.path)
	if err != nil {
		return err
	}
	link := v.ToIDispatch()
	defer link.Release()
	for prop, val := range map[string]string{
		"TargetPath":       s.target,
		"Arguments":        s.args,
		"Description":      s.description,
		"WorkingDirectory": filepath.Dir(s.target),
		"IconLocation":     s.target + ",0",
	} {
		if _, err := oleutil.PutProperty(link, prop, val); err != nil {
			return err
		}
	}
	_, err = oleutil.CallMethod(link, "Save")
	return err
}

// RegisterURLHandler makes Shelf the program that opens shelf:// links, for
// this user.
func RegisterURLHandler(_ string, exe string, _ []string) error {
	set := func(path, name, value string) error {
		k, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE)
		if err != nil {
			return err
		}
		defer k.Close()
		return k.SetStringValue(name, value)
	}
	quoted := `"` + exe + `"`
	for _, v := range []struct{ path, name, value string }{
		{protocolKey, "", "URL:Shelf"},
		{protocolKey, "URL Protocol", ""},
		{protocolKey + `\DefaultIcon`, "", quoted + ",0"},
		{protocolKey + `\shell\open\command`, "", quoted + ` "%1"`},
	} {
		if err := set(v.path, v.name, v.value); err != nil {
			return err
		}
	}
	return nil
}

// UnregisterURLHandler removes the handler Shelf registered.
func UnregisterURLHandler(string, []string) error {
	// Keys go deepest first: Windows only deletes keys without subkeys.
	for _, k := range []string{
		protocolKey + `\shell\open\command`,
		protocolKey + `\shell\open`,
		protocolKey + `\shell`,
		protocolKey + `\DefaultIcon`,
		protocolKey,
	} {
		if err := registry.DeleteKey(registry.CURRENT_USER, k); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return err
		}
	}
	return nil
}

// URLHandlerRegistered reports whether shelf:// links open this Shelf.
func URLHandlerRegistered(string) bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, protocolKey+`\shell\open\command`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	cmd, _, err := k.GetStringValue("")
	if err != nil {
		return false
	}
	exe, err := os.Executable()
	return err == nil && strings.Contains(strings.ToLower(cmd), strings.ToLower(exe))
}
