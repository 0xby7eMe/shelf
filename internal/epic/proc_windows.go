package epic

import (
	"io/fs"
	"os/exec"
	"strconv"
	"syscall"

	"golang.org/x/sys/windows"
)

// ownGroup starts cmd in a process group of its own, without a console window.
func ownGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW,
	}
}

// hideConsole keeps a console program from flashing a window.
func hideConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
}

// killGroup ends cmd and everything it started.
func killGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		killTree(cmd.Process.Pid)
	}
}

// killTree ends a process and the ones it started. Windows has no process
// groups to signal, so taskkill walks the tree.
func killTree(pid int) {
	c := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
	hideConsole(c)
	_ = c.Run()
}

// terminate asks a process to close; kill makes it.
func terminate(pid int) {
	c := exec.Command("taskkill", "/PID", strconv.Itoa(pid))
	hideConsole(c)
	_ = c.Run()
}

func kill(pid int) {
	if h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, uint32(pid)); err == nil {
		_ = windows.TerminateProcess(h, 1)
		windows.CloseHandle(h)
	}
}

// allocated is the space a file takes; Windows reports its size.
func allocated(info fs.FileInfo) int64 { return info.Size() }
