//go:build !windows

package library

import (
	"os"
	"strconv"
	"syscall"
)

// volumeOf names the filesystem behind p, by device, and its space.
func volumeOf(p string) (key string, total, free uint64, ok bool) {
	st, err := os.Stat(p)
	if err != nil {
		return "", 0, 0, false
	}
	sys, isUnix := st.Sys().(*syscall.Stat_t)
	if !isUnix {
		return "", 0, 0, false
	}
	var fs syscall.Statfs_t
	if syscall.Statfs(p, &fs) != nil {
		return "", 0, 0, false
	}
	return strconv.FormatUint(uint64(sys.Dev), 10), fs.Blocks * uint64(fs.Bsize), fs.Bavail * uint64(fs.Bsize), true
}
