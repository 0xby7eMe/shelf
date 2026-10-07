package library

import (
	"os"
	"sort"
	"syscall"
)

// Volume is a filesystem that holds game files.
type Volume struct {
	Path  string `json:"path"`
	Total uint64 `json:"total"`
	Free  uint64 `json:"free"`
}

// Volumes reports space for the filesystems behind paths, once per filesystem.
// Paths that don't exist are skipped.
func Volumes(paths []string) []Volume {
	seen := map[uint64]bool{}
	out := []Volume{}
	for _, p := range paths {
		st, err := os.Stat(p)
		if err != nil {
			continue
		}
		sys, ok := st.Sys().(*syscall.Stat_t)
		if !ok || seen[uint64(sys.Dev)] {
			continue
		}
		var fs syscall.Statfs_t
		if syscall.Statfs(p, &fs) != nil {
			continue
		}
		seen[uint64(sys.Dev)] = true
		out = append(out, Volume{
			Path:  p,
			Total: fs.Blocks * uint64(fs.Bsize),
			Free:  fs.Bavail * uint64(fs.Bsize),
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}
