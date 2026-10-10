//go:build !windows

package epic

import (
	"context"
	"errors"
)

// The Windows-only helpers in host_windows.go, as they look elsewhere: there
// is nothing to find, and nothing calls them.

func runningUnder(map[string]string) map[string]bool { return map[string]bool{} }
func processRunning(...string) bool                  { return false }
func closePrograms(...string) int                    { return 0 }
func winConnectDir() string                          { return "" }
func winConnectDataDirs() []string                   { return nil }
func winUbisoftRegistry() (complete, partial map[string]string) {
	return map[string]string{}, map[string]string{}
}
func egsManifests() []foundGame { return nil }
func openURI(string) error      { return errors.New("only on Windows") }
func (m *Manager) runInstaller(context.Context, string, string, string, []string, string) error {
	return errors.New("only on Windows")
}

var connectPrograms []string
