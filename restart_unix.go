//go:build !windows

package main

import (
	"os/exec"
	"syscall"

	"shelf/internal/library"
)

// relaunch starts path once this copy has quit. The new copy has to wait for
// this one to let go of the single-instance lock.
func relaunch(path string) error {
	cmd := exec.Command("sh", "-c", `sleep 2; exec "$0"`, path)
	cmd.Env = library.ChildEnv()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

// waitForPrevious does nothing: on Linux and macOS the shell waits instead.
func waitForPrevious([]string) {}

// trayIcon is the tray's picture: the app icon as it is.
func trayIcon() []byte { return icon }
