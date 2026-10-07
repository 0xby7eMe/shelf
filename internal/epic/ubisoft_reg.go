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

var (
	installsKeyRe = regexp.MustCompile(`(?i)^software\\\\wow6432node\\\\ubisoft\\\\launcher\\\\installs\\\\(\d+)$`)
	installDirRe  = regexp.MustCompile(`^"InstallDir"="(.*)"\s*$`)
	regUnescape   = strings.NewReplacer(`\\`, `\`, `\"`, `"`)
)

// parseUbisoftInstalls returns game id -> Windows install path from a Wine registry file.
func parseUbisoftInstalls(r io.Reader) map[string]string {
	out := map[string]string{}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)

	current := ""
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "["):
			current = ""
			if end := strings.LastIndex(line, "]"); end > 0 {
				if m := installsKeyRe.FindStringSubmatch(line[1:end]); m != nil {
					current = m[1]
				}
			}
		case current != "":
			if m := installDirRe.FindStringSubmatch(line); m != nil {
				if dir := strings.TrimSpace(regUnescape.Replace(m[1])); dir != "" {
					out[current] = dir
				}
			}
		}
	}
	return out
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
	installs map[string]string
}

// ubisoftInstalls returns game id -> install folder for everything Ubisoft
// Connect has installed in its prefix. The registry file is only read again
// when it has changed.
func (m *Manager) ubisoftInstalls() map[string]string {
	pfx := filepath.Join(ubisoftPrefix(), "pfx")
	path := filepath.Join(pfx, "system.reg")
	st, err := os.Stat(path)
	if err != nil {
		return map[string]string{}
	}

	m.mu.Lock()
	c := m.reg
	m.mu.Unlock()
	if c != nil && c.size == st.Size() && c.modified == st.ModTime().UnixNano() {
		return c.installs
	}

	f, err := os.Open(path)
	if err != nil {
		return map[string]string{}
	}
	defer f.Close()

	installs := map[string]string{}
	for id, winPath := range parseUbisoftInstalls(f) {
		if unix := winToUnix(pfx, winPath); unix != "" {
			installs[id] = unix
		}
	}

	m.mu.Lock()
	m.reg = &regCache{size: st.Size(), modified: st.ModTime().UnixNano(), installs: installs}
	m.mu.Unlock()
	return installs
}
