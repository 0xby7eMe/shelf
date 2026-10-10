package epic

import "runtime"

// Epic, GOG and Ubisoft games are Windows programs. Linux runs them through
// Proton, each in a prefix of its own. Windows runs them as they are, so
// prefixes, Proton builds and the BattlEye runtime don't apply there. macOS
// has no Proton: Epic games use their native Mac builds instead, and what
// needs a Windows program (Ubisoft Connect, GOG's installers) is switched off.
var (
	macOS     = runtime.GOOS == "darwin"
	onWindows = runtime.GOOS == "windows"
	// useProton is true where Windows games run through Proton: Linux.
	useProton = !macOS && !onWindows
)

// epicPlatform is the build legendary installs.
func epicPlatform() string {
	if macOS {
		return "Mac"
	}
	return "Windows"
}

// errNeedsProton refuses something that runs Windows programs, on macOS, or
// something that only Proton needs, on Windows.
func errNeedsProton(what string) error {
	return &protonError{what}
}

type protonError struct{ what string }

func (e *protonError) Error() string {
	if onWindows {
		return e.what + " is only for running Windows games on Linux. Windows runs them without it"
	}
	return e.what + " needs Proton, which only exists on Linux, so it isn't available on macOS yet"
}
