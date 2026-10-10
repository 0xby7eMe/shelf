package main

import (
	_ "embed"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	"shelf/internal/library"
)

// afterFlag tells a new copy which copy it replaces, so it waits for that one
// to quit and let go of the single-instance lock.
const afterFlag = "--after-pid="

// relaunch starts path, which waits for this copy to quit before it opens.
func relaunch(path string) error {
	cmd := exec.Command(path, afterFlag+strconv.Itoa(os.Getpid()))
	cmd.Env = library.ChildEnv()
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

// waitForPrevious waits, for a while, for the copy named by --after-pid to quit.
func waitForPrevious(args []string) {
	for _, a := range args {
		v, ok := strings.CutPrefix(a, afterFlag)
		if !ok {
			continue
		}
		pid, err := strconv.ParseUint(v, 10, 32)
		if err != nil {
			return
		}
		h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
		if err != nil {
			return // already gone
		}
		windows.WaitForSingleObject(h, uint32((20 * time.Second).Milliseconds()))
		windows.CloseHandle(h)
		return
	}
}

//go:embed build/windows/icon.ico
var windowsIcon []byte

// trayIcon is the tray's picture: Windows only takes an .ico there.
func trayIcon() []byte { return windowsIcon }
