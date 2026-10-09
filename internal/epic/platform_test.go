package epic

import "testing"

// linuxOnly skips a test of something that runs through Proton, which macOS doesn't have.
func linuxOnly(t *testing.T) {
	t.Helper()
	if macOS {
		t.Skip("needs Proton, which macOS doesn't have")
	}
}
