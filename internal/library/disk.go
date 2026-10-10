package library

import "sort"

// Volume is a filesystem that holds game files.
type Volume struct {
	Path  string `json:"path"`
	Total uint64 `json:"total"`
	Free  uint64 `json:"free"`
}

// Volumes reports space for the filesystems behind paths, once per filesystem.
// Paths that don't exist are skipped.
func Volumes(paths []string) []Volume {
	seen := map[string]bool{}
	out := []Volume{}
	for _, p := range paths {
		key, total, free, ok := volumeOf(p)
		if !ok || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, Volume{Path: p, Total: total, Free: free})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
