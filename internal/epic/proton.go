package epic

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"shelf/internal/library"
)

// ProtonBuild is a Proton installation that can run Windows games.
type ProtonBuild struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// FindProton looks for Proton in the places Steam, ProtonUp-Qt, distro packages
// and Heroic put it, best candidate first.
func FindProton() []ProtonBuild {
	var roots []string // directories whose children are Proton builds

	home, _ := os.UserHomeDir()
	if steam, err := library.SteamRoot(); err == nil {
		roots = append(roots, filepath.Join(steam, "compatibilitytools.d"))
		for _, lib := range library.SteamLibraryDirs(steam) {
			roots = append(roots, filepath.Join(lib, "steamapps", "common"))
		}
	}
	if home != "" {
		roots = append(roots,
			filepath.Join(home, ".steam", "root", "compatibilitytools.d"),
			filepath.Join(home, ".config", "heroic", "tools", "proton"),
			filepath.Join(home, ".var", "app", "com.heroicgameslauncher.hgl", "config", "heroic", "tools", "proton"),
		)
	}
	roots = append(roots, "/usr/share/steam/compatibilitytools.d")

	seen := map[string]bool{}
	var out []ProtonBuild
	for _, root := range roots {
		entries, _ := os.ReadDir(root)
		for _, e := range entries {
			dir := filepath.Join(root, e.Name())
			if !isProtonDir(e.Name(), dir) {
				continue
			}
			real, err := filepath.EvalSymlinks(dir)
			if err != nil || seen[real] {
				continue
			}
			seen[real] = true
			out = append(out, ProtonBuild{Name: e.Name(), Path: real})
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		ri, rj := protonRank(out[i].Name), protonRank(out[j].Name)
		if ri != rj {
			return ri < rj
		}
		return naturalLess(out[j].Name, out[i].Name) // newest first
	})
	return out
}

func isProtonDir(name, dir string) bool {
	// steamapps/common also holds games and the Steam Linux Runtime.
	lower := strings.ToLower(name)
	if !strings.Contains(lower, "proton") {
		return false
	}
	st, err := os.Stat(filepath.Join(dir, "proton"))
	return err == nil && !st.IsDir() && st.Mode()&0o111 != 0
}

// protonRank orders families: GE-Proton first, then Experimental, then the numbered releases.
func protonRank(name string) int {
	switch {
	case strings.HasPrefix(name, "GE-Proton"):
		return 0
	case strings.Contains(name, "Experimental"):
		return 1
	default:
		return 2
	}
}

// naturalLess compares strings so that "GE-Proton9-10" sorts after "GE-Proton9-9".
func naturalLess(a, b string) bool {
	ra, rb := []rune(a), []rune(b)
	i, j := 0, 0
	for i < len(ra) && j < len(rb) {
		if unicode.IsDigit(ra[i]) && unicode.IsDigit(rb[j]) {
			si := i
			for i < len(ra) && unicode.IsDigit(ra[i]) {
				i++
			}
			sj := j
			for j < len(rb) && unicode.IsDigit(rb[j]) {
				j++
			}
			na := strings.TrimLeft(string(ra[si:i]), "0")
			nb := strings.TrimLeft(string(rb[sj:j]), "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			continue
		}
		if ra[i] != rb[j] {
			return ra[i] < rb[j]
		}
		i++
		j++
	}
	return len(ra)-i < len(rb)-j
}

// resolveProton returns the build to use: the chosen path if it still exists,
// otherwise the best one found.
func resolveProton(chosen string) (ProtonBuild, bool) {
	builds := FindProton()
	for _, b := range builds {
		if chosen != "" && b.Path == chosen {
			return b, true
		}
	}
	if len(builds) == 0 {
		return ProtonBuild{}, false
	}
	return builds[0], true
}

func resolveProtonExact(path string) (ProtonBuild, bool) {
	for _, b := range FindProton() {
		if b.Path == path {
			return b, true
		}
	}
	return ProtonBuild{}, false
}
