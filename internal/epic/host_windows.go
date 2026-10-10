package epic

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"shelf/internal/library"
)

// What Windows itself offers in place of a Proton prefix: the real registry,
// the real process list and the shell.

// runningUnder reports, for each folder, whether a program inside it is
// running. Folders are matched by key: folder -> key.
func runningUnder(folders map[string]string) map[string]bool {
	out := map[string]bool{}
	if len(folders) == 0 {
		return out
	}
	prefixes := make(map[string]string, len(folders))
	for dir, key := range folders {
		prefixes[strings.ToLower(filepath.Clean(dir))+`\`] = key
	}
	for _, p := range library.Processes(false) {
		exe := strings.ToLower(filepath.Clean(p.Exe))
		for prefix, key := range prefixes {
			if strings.HasPrefix(exe, prefix) {
				out[key] = true
			}
		}
	}
	return out
}

// processRunning reports whether a program with one of these file names runs.
func processRunning(names ...string) bool {
	return len(processIDs(names...)) > 0
}

func processIDs(names ...string) []int {
	var pids []int
	for _, p := range library.Processes(false) {
		base := filepath.Base(p.Exe)
		for _, n := range names {
			if strings.EqualFold(base, n) {
				pids = append(pids, int(p.PID))
			}
		}
	}
	return pids
}

// closePrograms closes every program with one of these file names: politely
// first, then by force.
func closePrograms(names ...string) int {
	pids := processIDs(names...)
	for _, p := range pids {
		terminate(p)
	}
	for _, p := range processIDs(names...) {
		killTree(p)
	}
	return len(pids)
}

// errorElevationRequired is what CreateProcess answers for a program whose
// manifest asks for administrator rights.
const errorElevationRequired syscall.Errno = 740

// runInstaller runs an installer and waits for it. Installers that ask for
// administrator rights are started through the shell, which shows Windows'
// own prompt; cancelling ctx ends the installer.
func (m *Manager) runInstaller(ctx context.Context, source, app, exe string, args []string, dir string) error {
	cmd := exec.Command(exe, args...)
	cmd.Dir = dir
	cmd.Env = library.ChildEnv()
	ownGroup(cmd)
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			killGroup(cmd)
		case <-done:
		}
	}()
	err := m.runLogged(cmd, source, app)
	var errno syscall.Errno
	if !errors.As(err, &errno) || errno != errorElevationRequired {
		return err
	}
	m.logf(source, app, "the installer asks for administrator rights; Windows will ask you")
	return shellRunAs(ctx, exe, args, dir)
}

// shellExecuteInfo is SHELLEXECUTEINFOW.
type shellExecuteInfo struct {
	size       uint32
	mask       uint32
	hwnd       windows.Handle
	verb       *uint16
	file       *uint16
	parameters *uint16
	directory  *uint16
	show       int32
	instApp    windows.Handle
	idList     uintptr
	class      *uint16
	keyClass   windows.Handle
	hotKey     uint32
	iconOrMon  windows.Handle
	process    windows.Handle
}

const (
	seeMaskNoCloseProcess = 0x00000040
	seeMaskNoAsync        = 0x00000100
	seeMaskFlagNoUI       = 0x00000400
)

var procShellExecuteEx = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW")

// shellRunAs starts a program with administrator rights, after Windows has
// asked the user, and waits for it to finish.
func shellRunAs(ctx context.Context, exe string, args []string, dir string) error {
	params := make([]string, len(args))
	for i, a := range args {
		params[i] = syscall.EscapeArg(a)
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	param, err := windows.UTF16PtrFromString(strings.Join(params, " "))
	if err != nil {
		return err
	}
	var cwd *uint16
	if dir != "" {
		if cwd, err = windows.UTF16PtrFromString(dir); err != nil {
			return err
		}
	}
	info := shellExecuteInfo{
		mask:       seeMaskNoCloseProcess | seeMaskNoAsync | seeMaskFlagNoUI,
		verb:       verb,
		file:       file,
		parameters: param,
		directory:  cwd,
		show:       windows.SW_SHOWNORMAL,
	}
	info.size = uint32(unsafe.Sizeof(info))
	if r, _, err := procShellExecuteEx.Call(uintptr(unsafe.Pointer(&info))); r == 0 {
		if errors.Is(err, windows.ERROR_CANCELLED) {
			return errors.New("Windows didn't get permission to run the installer")
		}
		return fmt.Errorf("couldn't start the installer: %w", err)
	}
	if info.process == 0 {
		return nil
	}
	defer windows.CloseHandle(info.process)

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = windows.TerminateProcess(info.process, 1)
		case <-done:
		}
	}()
	if _, err := windows.WaitForSingleObject(info.process, windows.INFINITE); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var code uint32
	if err := windows.GetExitCodeProcess(info.process, &code); err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("exit status %d", code)
	}
	return nil
}

// --- Ubisoft Connect, installed in Windows itself ---

const ubisoftLauncherKey = `SOFTWARE\Ubisoft\Launcher`

// winConnectDir is where Ubisoft Connect is installed, as its installer
// recorded, or its usual folder.
func winConnectDir() string {
	for _, view := range []uint32{registry.WOW64_32KEY, registry.WOW64_64KEY} {
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, ubisoftLauncherKey, registry.QUERY_VALUE|view)
		if err != nil {
			continue
		}
		dir, _, err := k.GetStringValue("InstallDir")
		k.Close()
		if err == nil && dir != "" {
			return filepath.Clean(dir)
		}
	}
	base := os.Getenv("ProgramFiles(x86)")
	if base == "" {
		base = `C:\Program Files (x86)`
	}
	return filepath.Join(base, "Ubisoft", "Ubisoft Game Launcher")
}

// winConnectDataDirs are the folders current versions of Connect keep their
// records in, before its own folder.
func winConnectDataDirs() []string {
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		return []string{filepath.Join(local, "Ubisoft Game Launcher")}
	}
	return nil
}

// winUbisoftRegistry reads the games Connect has installed from the registry,
// as Connect writes it on Linux into the prefix: a key under Installs when a
// download starts, and an Uninstall entry once the game is complete.
func winUbisoftRegistry() (complete, partial map[string]string) {
	complete, partial = map[string]string{}, map[string]string{}
	started := map[string]string{}
	for _, view := range []uint32{registry.WOW64_32KEY, registry.WOW64_64KEY} {
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, ubisoftLauncherKey+`\Installs`, registry.ENUMERATE_SUB_KEYS|view)
		if err != nil {
			continue
		}
		ids, _ := k.ReadSubKeyNames(-1)
		k.Close()
		for _, id := range ids {
			g, err := registry.OpenKey(registry.LOCAL_MACHINE, ubisoftLauncherKey+`\Installs\`+id, registry.QUERY_VALUE|view)
			if err != nil {
				continue
			}
			dir, _, err := g.GetStringValue("InstallDir")
			g.Close()
			if err == nil && strings.TrimSpace(dir) != "" {
				started[id] = filepath.Clean(strings.TrimSpace(dir))
			}
		}
	}
	for id, dir := range started {
		if uplayInstallDone(id) {
			complete[id] = dir
		} else {
			partial[id] = dir
		}
	}
	return complete, partial
}

func uplayInstallDone(id string) bool {
	for _, view := range []uint32{registry.WOW64_32KEY, registry.WOW64_64KEY} {
		k, err := registry.OpenKey(registry.LOCAL_MACHINE,
			`SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\Uplay Install `+id, registry.QUERY_VALUE|view)
		if err == nil {
			k.Close()
			return true
		}
	}
	return false
}

// connectPrograms are Ubisoft Connect's own processes.
var connectPrograms = []string{"UbisoftConnect.exe", "upc.exe", "UbisoftGameLauncher.exe", "UplayWebCore.exe"}

// --- Epic Games Launcher's own installs, for importing ---

// egsManifests lists the games the Epic Games Launcher installed, from its
// manifest folder.
func egsManifests() []foundGame {
	data := os.Getenv("ProgramData")
	if data == "" {
		data = `C:\ProgramData`
	}
	files, _ := filepath.Glob(filepath.Join(data, "Epic", "EpicGamesLauncher", "Data", "Manifests", "*.item"))
	var out []foundGame
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if name, dir := parseEGSItem(raw); name != "" && dir != "" {
			out = append(out, foundGame{appName: name, path: filepath.Clean(dir), source: "Epic Games Launcher"})
		}
	}
	return out
}

// openURI hands a link such as uplay://launch/… to the program registered for it.
func openURI(uri string) error { return library.OpenURI(uri) }
