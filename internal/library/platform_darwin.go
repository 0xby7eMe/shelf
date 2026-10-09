//go:build darwin

package library

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// UsualPath adds the folders Homebrew and pipx install to onto PATH. An app
// opened from the Finder gets only the system folders, so legendary, and the
// Python it runs on, wouldn't be found.
func UsualPath() {
	home, _ := os.UserHomeDir()
	have := map[string]bool{}
	parts := filepath.SplitList(os.Getenv("PATH"))
	for _, p := range parts {
		have[p] = true
	}
	for _, p := range []string{"/opt/homebrew/bin", "/usr/local/bin", filepath.Join(home, ".local", "bin")} {
		if !have[p] {
			parts = append(parts, p)
		}
	}
	os.Setenv("PATH", strings.Join(parts, string(os.PathListSeparator)))
}

// openCommand opens links and folders with the default program.
const openCommand = "open"

// steamRootCandidates is where Steam keeps its files on macOS.
func steamRootCandidates(home string) []string {
	return []string{filepath.Join(home, "Library", "Application Support", "Steam")}
}

// ProcessArgs lists the command line of every process, one slice per process.
// macOS has no /proc, and ps joins the arguments with spaces, so arguments
// that contain spaces come back split.
func ProcessArgs() [][]string {
	out, err := exec.Command("ps", "-axww", "-o", "args=").Output()
	if err != nil {
		return nil
	}
	var procs [][]string
	for _, line := range strings.Split(string(out), "\n") {
		if f := strings.Fields(line); len(f) > 0 {
			procs = append(procs, f)
		}
	}
	return procs
}

// runningAppIDs asks Steam which game it is running. On macOS Steam starts
// games without a wrapper process, but records the game in registry.vdf.
func runningAppIDs() []string {
	root, err := findSteamRoot()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(root, "registry.vdf"))
	if err != nil {
		return nil
	}
	if id := runningAppIDFromRegistry(data); id != "" {
		return []string{id}
	}
	return nil
}
