package epic

import "runtime"

// macOS has no Proton, so Windows games can't run there. Epic games use their
// native Mac builds instead, and what needs Proton (Ubisoft Connect, GOG's
// Windows installers, prefixes, BattlEye) is switched off.
var macOS = runtime.GOOS == "darwin"

// epicPlatform is the build legendary installs.
func epicPlatform() string {
	if macOS {
		return "Mac"
	}
	return "Windows"
}

// errNeedsProton refuses something that runs Windows programs, on macOS.
func errNeedsProton(what string) error {
	return &protonError{what}
}

type protonError struct{ what string }

func (e *protonError) Error() string {
	return e.what + " needs Proton, which only exists on Linux, so it isn't available on macOS yet"
}
