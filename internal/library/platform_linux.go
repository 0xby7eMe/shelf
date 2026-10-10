//go:build !darwin && !windows

package library

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// openCommand opens links and folders with the desktop's default program.
const openCommand = "xdg-open"

// steamRootCandidates are the places Steam lives on Linux.
func steamRootCandidates(home string) []string {
	return []string{
		filepath.Join(home, ".local/share/Steam"),
		filepath.Join(home, ".steam/steam"),
		filepath.Join(home, ".var/app/com.valvesoftware.Steam/.local/share/Steam"), // Flatpak
	}
}

// ProcessArgs lists the command line of every process, one slice per process.
func ProcessArgs() [][]string {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var out [][]string
	for _, e := range entries {
		name := e.Name()
		if name == "" || name[0] < '0' || name[0] > '9' {
			continue
		}
		data, err := os.ReadFile("/proc/" + name + "/cmdline")
		if err != nil || len(data) == 0 {
			continue
		}
		out = append(out, strings.Split(strings.TrimRight(string(data), "\x00"), "\x00"))
	}
	return out
}

// runningAppIDs finds Steam games by the SteamLaunch wrapper Steam starts them with.
func runningAppIDs() []string {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if name == "" || name[0] < '0' || name[0] > '9' {
			continue
		}
		data, err := os.ReadFile("/proc/" + name + "/cmdline")
		if err != nil || !bytes.Contains(data, []byte("SteamLaunch")) {
			continue
		}
		args := strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
		if id, ok := appIDFromArgs(args); ok {
			seen[id] = true
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	return ids
}

// UsualPath leaves PATH alone: a Linux desktop starts apps with the user's own.
func UsualPath() {}
