package epic

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Ubisoft Connect records every game it installs in the prefix's registry, as
// HKLM\Software\Wow6432Node\Ubisoft\Launcher\Installs\<GameID>\InstallDir.
// Wine keeps that registry in a text file, which is all Shelf needs to read to
// know which games are installed and where.
//
// That key appears when a download starts, not when it ends. Connect adds
// HKLM\Software\[Wow6432Node\]Microsoft\Windows\CurrentVersion\Uninstall\Uplay Install <GameID>
// once the game is complete, so that second key is what marks a game as installed.

var (
	installsKeyRe  = regexp.MustCompile(`(?i)^software\\\\wow6432node\\\\ubisoft\\\\launcher\\\\installs\\\\(\d+)$`)
	uninstallKeyRe = regexp.MustCompile(`(?i)^software\\\\(?:wow6432node\\\\)?microsoft\\\\windows\\\\currentversion\\\\uninstall\\\\uplay install (\d+)$`)
	installDirRe   = regexp.MustCompile(`^"InstallDir"="(.*)"\s*$`)
	regUnescape    = strings.NewReplacer(`\\`, `\`, `\"`, `"`)
)

// parseUbisoftInstalls reads a Wine registry file. It returns game id ->
// Windows install path for every game Connect has started installing, and the
// ids whose installation has finished.
func parseUbisoftInstalls(r io.Reader) (installs map[string]string, done map[string]bool) {
	installs, done = map[string]string{}, map[string]bool{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)

	current := ""
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "["):
			current = ""
			if end := strings.LastIndex(line, "]"); end > 0 {
				key := line[1:end]
				if m := installsKeyRe.FindStringSubmatch(key); m != nil {
					current = m[1]
				} else if m := uninstallKeyRe.FindStringSubmatch(key); m != nil {
					done[m[1]] = true
				}
			}
		case current != "":
			if m := installDirRe.FindStringSubmatch(line); m != nil {
				if dir := strings.TrimSpace(regUnescape.Replace(m[1])); dir != "" {
					installs[current] = dir
				}
			}
		}
	}
	return installs, done
}

// winToUnix turns a Windows path inside a Wine prefix into a path on disk.
// pfx is the prefix's pfx folder. It returns "" for paths it can't place.
func winToUnix(pfx, winPath string) string {
	p := strings.ReplaceAll(winPath, `\`, "/")
	if len(p) < 2 || p[1] != ':' {
		return ""
	}
	rest := strings.TrimPrefix(p[2:], "/")

	var root string
	switch letter := strings.ToLower(p[:1]); letter {
	case "c":
		root = filepath.Join(pfx, "drive_c")
	case "z":
		root = "/"
	default:
		target, err := os.Readlink(filepath.Join(pfx, "dosdevices", letter+":"))
		if err != nil {
			return ""
		}
		if !filepath.IsAbs(target) {
			target = filepath.Join(pfx, "dosdevices", target)
		}
		root = target
	}
	return resolveCaseInsensitive(root, rest)
}

// resolveCaseInsensitive joins rest onto root, matching each folder name
// without regard to case, as Windows and Wine do. Names that don't exist are
// used as written.
func resolveCaseInsensitive(root, rest string) string {
	cur := root
	missing := false
	for _, part := range strings.Split(rest, "/") {
		if part == "" || part == "." {
			continue
		}
		next := filepath.Join(cur, part)
		if !missing {
			if _, err := os.Lstat(next); err != nil {
				missing = true
				if entries, rerr := os.ReadDir(cur); rerr == nil {
					for _, e := range entries {
						if strings.EqualFold(e.Name(), part) {
							next, missing = filepath.Join(cur, e.Name()), false
							break
						}
					}
				}
			}
		}
		cur = next
	}
	return cur
}

type regCache struct {
	size     int64
	modified int64
	complete map[string]string // finished installs
	partial  map[string]string // downloads that have started but not finished
}

// ubisoftRegistry returns game id -> install folder, split into finished
// installs and downloads still going. The registry file is only read again
// when it has changed.
func (m *Manager) ubisoftRegistry() (complete, partial map[string]string) {
	pfx := filepath.Join(ubisoftPrefix(), "pfx")
	path := filepath.Join(pfx, "system.reg")
	st, err := os.Stat(path)
	if err != nil {
		return map[string]string{}, map[string]string{}
	}

	m.mu.Lock()
	c := m.reg
	m.mu.Unlock()
	if c != nil && c.size == st.Size() && c.modified == st.ModTime().UnixNano() {
		return c.complete, c.partial
	}

	f, err := os.Open(path)
	if err != nil {
		return map[string]string{}, map[string]string{}
	}
	defer f.Close()

	started, done := parseUbisoftInstalls(f)
	complete, partial = map[string]string{}, map[string]string{}
	for id, winPath := range started {
		unix := winToUnix(pfx, winPath)
		if unix == "" {
			continue
		}
		if done[id] {
			complete[id] = unix
		} else {
			partial[id] = unix
		}
	}

	m.mu.Lock()
	m.reg = &regCache{size: st.Size(), modified: st.ModTime().UnixNano(), complete: complete, partial: partial}
	m.mu.Unlock()
	return complete, partial
}

// ubisoftInstalls returns game id -> install folder for the games Connect has
// finished installing.
func (m *Manager) ubisoftInstalls() map[string]string {
	complete, _ := m.ubisoftRegistry()
	return complete
}
