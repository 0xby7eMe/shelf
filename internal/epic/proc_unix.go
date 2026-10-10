//go:build !windows

package epic

import (
	"io/fs"
	"os/exec"
	"syscall"
)

// ownGroup starts cmd in a process group of its own, so that it and whatever
// it starts can be stopped together, and Shelf closing doesn't take it along.
func ownGroup(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true} }

// killGroup ends cmd and everything it started.
func killGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}

// terminate asks a process to close; kill makes it.
func terminate(pid int) { _ = syscall.Kill(pid, syscall.SIGTERM) }
func kill(pid int)      { _ = syscall.Kill(pid, syscall.SIGKILL) }

// allocated is the space a file takes on disk, which can be less than its
// size (sparse files) or more (the last block).
func allocated(info fs.FileInfo) int64 {
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return st.Blocks * 512
	}
	return info.Size()
}

// hideConsole keeps a console program from opening a window, on Windows.
func hideConsole(*exec.Cmd) {}
