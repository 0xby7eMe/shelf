package library

import (
	"reflect"
	"runtime"
	"testing"
)

func TestCleanEnv(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("AppImages are Linux only")
	}
	env := []string{
		"HOME=/home/u",
		"APPDIR=/tmp/.mount_x",
		"APPIMAGE=/home/u/Shelf.AppImage",
		"GDK_PIXBUF_MODULE_FILE=/tmp/.mount_x/usr/lib/loaders.cache",
		"PATH=/tmp/.mount_x/usr/bin:/usr/bin:/bin",
		"LD_LIBRARY_PATH=/tmp/.mount_x/usr/lib",
		"XDG_DATA_DIRS=/tmp/.mount_x/usr/share:/usr/local/share:/usr/share",
		"LANG=en_US.UTF-8",
	}
	want := []string{
		"HOME=/home/u",
		"PATH=/usr/bin:/bin",
		"XDG_DATA_DIRS=/usr/local/share:/usr/share",
		"LANG=en_US.UTF-8",
	}
	if got := cleanEnv(env, "/tmp/.mount_x"); !reflect.DeepEqual(got, want) {
		t.Errorf("got  %q\nwant %q", got, want)
	}

	// Outside an AppImage nothing changes, and look-alike paths survive.
	if got := cleanEnv(env, ""); !reflect.DeepEqual(got, env) {
		t.Error("environment must be untouched outside an AppImage")
	}
	keep := []string{"PATH=/tmp/.mount_xyz/bin:/usr/bin"}
	if got := cleanEnv(keep, "/tmp/.mount_x"); !reflect.DeepEqual(got, keep) {
		t.Errorf("sibling directory with the same prefix was removed: %q", got)
	}
}
