package library

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// UsualPath leaves PATH alone: Windows starts programs with the user's own.
func UsualPath() {}

// steamRootCandidates are where Steam lives on Windows: the folder Steam
// records in the registry, then its default folder.
func steamRootCandidates(string) []string {
	var out []string
	if k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam`, registry.QUERY_VALUE); err == nil {
		if p, _, err := k.GetStringValue("SteamPath"); err == nil && p != "" {
			out = append(out, filepath.Clean(p))
		}
		k.Close()
	}
	for _, view := range []uint32{registry.WOW64_32KEY, registry.WOW64_64KEY} {
		if k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Valve\Steam`, registry.QUERY_VALUE|view); err == nil {
			if p, _, err := k.GetStringValue("InstallPath"); err == nil && p != "" {
				out = append(out, filepath.Clean(p))
			}
			k.Close()
		}
	}
	for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles"} {
		if dir := os.Getenv(env); dir != "" {
			out = append(out, filepath.Join(dir, "Steam"))
		}
	}
	return out
}

// openExternal hands a link or folder to the program Windows opens it with.
func openExternal(target string) error {
	verb, _ := windows.UTF16PtrFromString("open")
	file, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, file, nil, nil, windows.SW_SHOWNORMAL)
}

// OpenURI opens a link of any scheme, such as uplay://, with the program
// registered for it. Callers check what they open.
func OpenURI(uri string) error { return openExternal(uri) }

// pathKey is how two spellings of one folder are told apart: Windows paths
// ignore case, and Steam writes some with forward slashes.
func pathKey(p string) string { return strings.ToLower(filepath.Clean(p)) }

// Process is a running program: its id, the file it runs and its command line.
type Process struct {
	PID  uint32
	Exe  string   // full path, empty if Windows won't say
	Args []string // empty if Windows won't say
}

// Processes lists the running programs. Programs of other users, and some
// that run elevated, can't be looked into; they come without a path.
func Processes(withArgs bool) []Process {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snap)

	var out []Process
	var e windows.ProcessEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	for err = windows.Process32First(snap, &e); err == nil; err = windows.Process32Next(snap, &e) {
		if e.ProcessID == 0 || e.ProcessID == 4 {
			continue // the idle process and the kernel
		}
		p := Process{PID: e.ProcessID}
		h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, e.ProcessID)
		if err == nil {
			p.Exe = imagePath(h)
			if withArgs {
				p.Args = commandLine(h)
			}
			windows.CloseHandle(h)
		}
		if p.Exe == "" {
			p.Exe = windows.UTF16ToString(e.ExeFile[:])
		}
		out = append(out, p)
	}
	return out
}

func imagePath(h windows.Handle) string {
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n := uint32(len(buf))
	if windows.QueryFullProcessImageName(h, 0, &buf[0], &n) != nil {
		return ""
	}
	return windows.UTF16ToString(buf[:n])
}

// processCommandLineInformation is the ProcessCommandLineInformation class of
// NtQueryInformationProcess, which answers with a UNICODE_STRING.
const processCommandLineInformation = 60

func commandLine(h windows.Handle) []string {
	buf := make([]byte, 4096)
	for range 3 {
		var need uint32
		err := windows.NtQueryInformationProcess(h, processCommandLineInformation, unsafe.Pointer(&buf[0]), uint32(len(buf)), &need)
		if err == nil {
			us := (*windows.NTUnicodeString)(unsafe.Pointer(&buf[0]))
			if us.Buffer == nil || us.Length == 0 {
				return nil
			}
			line := windows.UTF16ToString(unsafe.Slice(us.Buffer, us.Length/2))
			args, err := windows.DecomposeCommandLine(line)
			if err != nil {
				return strings.Fields(line)
			}
			return args
		}
		if need <= uint32(len(buf)) {
			return nil
		}
		buf = make([]byte, need)
	}
	return nil
}

// ProcessArgs lists the command line of every process, one slice per process.
func ProcessArgs() [][]string {
	var out [][]string
	for _, p := range Processes(true) {
		if len(p.Args) > 0 {
			out = append(out, p.Args)
		}
	}
	return out
}

// runningAppIDs asks Steam which game it is running: on Windows it keeps the
// id in the registry while a game runs.
func runningAppIDs() []string {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam`, registry.QUERY_VALUE)
	if err != nil {
		return nil
	}
	defer k.Close()
	id, _, err := k.GetIntegerValue("RunningAppID")
	if err != nil || id == 0 {
		return nil
	}
	return []string{strconv.FormatUint(id, 10)}
}
