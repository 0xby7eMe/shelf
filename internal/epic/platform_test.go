package epic

import (
	"runtime"
	"testing"
)

// linuxOnly skips a test of something that runs through Proton, which only Linux has.
func linuxOnly(t *testing.T) {
	t.Helper()
	if !useProton {
		t.Skip("needs Proton, which only Linux has")
	}
}

// unixOnly skips a test that stands in for legendary with a shell script,
// which Windows can't run.
func unixOnly(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in programs are shell scripts")
	}
}
