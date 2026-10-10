package library

import (
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

// volumeOf names the volume behind p, by its mount point (such as C:\), and its space.
func volumeOf(p string) (key string, total, free uint64, ok bool) {
	if _, err := os.Stat(p); err != nil {
		return "", 0, 0, false
	}
	path, err := windows.UTF16PtrFromString(p)
	if err != nil {
		return "", 0, 0, false
	}
	buf := make([]uint16, windows.MAX_PATH+1)
	if windows.GetVolumePathName(path, &buf[0], uint32(len(buf))) != nil {
		return "", 0, 0, false
	}
	var avail, all, allFree uint64
	if windows.GetDiskFreeSpaceEx(path, &avail, &all, &allFree) != nil {
		return "", 0, 0, false
	}
	return strings.ToLower(windows.UTF16ToString(buf)), all, avail, true
}
