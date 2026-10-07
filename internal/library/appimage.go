package library

import (
	"os"
	"path/filepath"
	"strings"
)

// Inside an AppImage the launcher points WebKit and GTK at the libraries that
// travel with the app. Those settings must not leak into the programs Shelf
// starts (legendary, Proton, xdg-open): they come from the system and would
// load the wrong libraries.

// EnterAppImage changes into the AppImage's usr/ folder when running from one.
// WebKit's helper processes are found through a path relative to it; see
// scripts/build-appimage.sh. It does nothing outside an AppImage.
func EnterAppImage() {
	if dir := os.Getenv("APPDIR"); dir != "" {
		_ = os.Chdir(filepath.Join(dir, "usr"))
	}
}

// appImageVars are set only to point at the bundled copies of things.
var appImageVars = map[string]bool{
	"APPDIR": true, "APPIMAGE": true, "ARGV0": true, "OWD": true,
	"GDK_PIXBUF_MODULE_FILE": true, "GDK_PIXBUF_MODULEDIR": true,
	"GTK_PATH": true, "GTK_EXE_PREFIX": true, "GTK_DATA_PREFIX": true, "GTK_IM_MODULE_FILE": true,
	"GIO_MODULE_DIR": true, "GIO_EXTRA_MODULES": true, "GSETTINGS_SCHEMA_DIR": true,
	"GI_TYPELIB_PATH": true, "GST_PLUGIN_SYSTEM_PATH": true, "GST_PLUGIN_SYSTEM_PATH_1_0": true,
	"GST_PLUGIN_SCANNER": true, "GST_PLUGIN_SCANNER_1_0": true, "GST_REGISTRY": true, "GST_REGISTRY_1_0": true,
}

// searchPathVars list directories, with the AppImage's own prepended.
var searchPathVars = map[string]bool{
	"PATH": true, "LD_LIBRARY_PATH": true, "XDG_DATA_DIRS": true, "XDG_CONFIG_DIRS": true,
	"PYTHONPATH": true, "PERL5LIB": true, "QT_PLUGIN_PATH": true,
}

// ChildEnv returns the environment for programs Shelf starts. Outside an
// AppImage that is just the current environment.
func ChildEnv() []string { return cleanEnv(os.Environ(), os.Getenv("APPDIR")) }

func cleanEnv(env []string, appDir string) []string {
	if appDir == "" {
		return env
	}
	prefix := filepath.Clean(appDir)
	out := make([]string, 0, len(env))
	for _, kv := range env {
		name, value, _ := strings.Cut(kv, "=")
		switch {
		case appImageVars[name]:
			continue
		case searchPathVars[name]:
			var keep []string
			for _, p := range strings.Split(value, ":") {
				if p != "" && p != prefix && !strings.HasPrefix(filepath.Clean(p), prefix+"/") {
					keep = append(keep, p)
				}
			}
			if len(keep) == 0 {
				continue
			}
			out = append(out, name+"="+strings.Join(keep, ":"))
		default:
			out = append(out, kv)
		}
	}
	return out
}
