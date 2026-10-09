package epic

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ulikunitz/xz"

	"shelf/internal/library"
)

// Games protected by BattlEye run on Linux only with Valve's BattlEye runtime:
// Proton swaps the game's Windows BEClient for a native one, and it looks for
// that runtime in the folder named by PROTON_BATTLEYE_RUNTIME. Steam installs
// it as the tool "Proton BattlEye Runtime". Other launchers bring their own,
// so Shelf uses Steam's copy when there is one and otherwise downloads the
// same files, as Heroic and Lutris do.

const battleyeEnv = "PROTON_BATTLEYE_RUNTIME"

// battleyeArchiveDir is the folder inside the download. Note the spelling.
const battleyeArchiveDir = "battleeye_runtime"

// The runtime is a fixed release asset on Lutris's build server, pinned by
// hash so what gets unpacked is exactly what was checked. Both are variables
// so tests can point them elsewhere.
var (
	battleyeURL    = "https://github.com/lutris/buildbot/releases/download/2024-05-21/battleeye_runtime.tar.xz"
	battleyeSHA256 = "229eaedf3bc16d5bf548bc2aed94bc48c44e2038337eedacbd17c6c7244cd212"
)

// BattlEyeRuntime says whether the runtime is available and where from.
type BattlEyeRuntime struct {
	Installed bool   `json:"installed"`
	Source    string `json:"source"` // "steam" or "shelf"
	Path      string `json:"path"`
}

func battleyeShelfDir() string { return filepath.Join(dataDir(), "runtimes", "battleye_runtime") }

// validBattlEyeRuntime checks a folder for the file Proton loads.
func validBattlEyeRuntime(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, "BEClient_x64.so"))
	return err == nil && !st.IsDir()
}

// findBattlEyeRuntime looks for Steam's copy first, then Shelf's own.
func findBattlEyeRuntime() BattlEyeRuntime {
	if steam, err := library.SteamRoot(); err == nil {
		for _, lib := range library.SteamLibraryDirs(steam) {
			dir := filepath.Join(lib, "steamapps", "common", "Proton BattlEye Runtime")
			if validBattlEyeRuntime(dir) {
				return BattlEyeRuntime{Installed: true, Source: "steam", Path: dir}
			}
		}
	}
	if dir := battleyeShelfDir(); validBattlEyeRuntime(dir) {
		return BattlEyeRuntime{Installed: true, Source: "shelf", Path: dir}
	}
	return BattlEyeRuntime{}
}

func (m *Manager) BattlEyeRuntime() BattlEyeRuntime { return findBattlEyeRuntime() }

// needsBattlEye reports whether a game's folder shows it uses BattlEye: a
// BattlEye folder, or its service and client files, next to the game.
func needsBattlEye(installDir string) bool {
	if installDir == "" {
		return false
	}
	entries, err := os.ReadDir(installDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if e.IsDir() && name == "battleye" {
			return true
		}
		if !e.IsDir() && (strings.HasPrefix(name, "beservice") || strings.HasPrefix(name, "beclient") || strings.HasPrefix(name, "belauncher")) {
			return true
		}
	}
	return false
}

// antiCheatOf names the anti-cheat a game ships with, for the library.
func antiCheatOf(installDir string) string {
	if needsBattlEye(installDir) {
		return "BattlEye"
	}
	return ""
}

// battleyeEnvFor returns the variable a BattlEye game needs, or an error that
// says what to do when the runtime is missing. Games without BattlEye get nothing.
func battleyeEnvFor(installDir, gameName string) ([]string, error) {
	if macOS || !needsBattlEye(installDir) {
		return nil, nil
	}
	rt := findBattlEyeRuntime()
	if !rt.Installed {
		return nil, fmt.Errorf("%s uses BattlEye, which needs the Proton BattlEye Runtime. Install it from the game's page or Settings first", gameName)
	}
	return []string{battleyeEnv + "=" + rt.Path}, nil
}

func (m *Manager) BattlEyeInstallState() ProtonInstallState {
	m.ubiMu.Lock()
	defer m.ubiMu.Unlock()
	return m.battleyeInstall
}

func (m *Manager) setBattlEyeInstall(s ProtonInstallState) {
	m.ubiMu.Lock()
	m.battleyeInstall = s
	m.ubiMu.Unlock()
	m.send("battleye:install", s)
}

// InstallBattlEyeRuntime downloads the runtime into Shelf's own folder.
func (m *Manager) InstallBattlEyeRuntime() error {
	if macOS {
		return errNeedsProton("the BattlEye runtime")
	}
	m.ubiMu.Lock()
	if m.battleyeInstall.State == "running" {
		m.ubiMu.Unlock()
		return fmt.Errorf("the BattlEye runtime is already being installed")
	}
	m.battleyeInstall = ProtonInstallState{State: "running", Message: "Downloading"}
	m.ubiMu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		report := func(pct float64, msg string) {
			m.setBattlEyeInstall(ProtonInstallState{State: "running", Percent: pct, Message: msg})
		}
		report(0, "Downloading")
		if err := installBattlEyeRuntime(ctx, report); err != nil {
			m.logf("battleye", "", "runtime install failed: %v", err)
			m.setBattlEyeInstall(ProtonInstallState{State: "failed", Error: err.Error()})
			return
		}
		m.logf("battleye", "", "runtime installed in %s", battleyeShelfDir())
		m.setBattlEyeInstall(ProtonInstallState{State: "done", Percent: 100, Message: "Proton BattlEye Runtime is installed"})
		m.send("library:changed", nil)
	}()
	return nil
}

func installBattlEyeRuntime(ctx context.Context, report func(float64, string)) error {
	final := battleyeShelfDir()
	root := filepath.Dir(final)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}

	archive := filepath.Join(root, ".battleye_runtime.tar.xz.part")
	defer os.Remove(archive)
	if err := downloadVerified(ctx, battleyeURL, archive, sha256.New(), battleyeSHA256,
		func(f float64) { report(f*85, "Downloading") }); err != nil {
		return err
	}

	report(88, "Unpacking")
	tmp := filepath.Join(root, ".battleye_runtime.part")
	os.RemoveAll(tmp)
	defer os.RemoveAll(tmp)
	if err := extractTarXz(archive, tmp); err != nil {
		return fmt.Errorf("couldn't unpack it: %w", err)
	}
	// The archive holds one folder, named as Lutris spells it; that folder is the runtime.
	inner := filepath.Join(tmp, battleyeArchiveDir)
	if !validBattlEyeRuntime(inner) {
		return fmt.Errorf("the download doesn't contain the BattlEye runtime")
	}
	if err := os.RemoveAll(final); err != nil {
		return err
	}
	return os.Rename(inner, final)
}

func extractTarXz(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	xr, err := xz.NewReader(f)
	if err != nil {
		return err
	}
	return extractTar(xr, dest)
}
