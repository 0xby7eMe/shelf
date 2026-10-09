package library

import "strings"

// runningAppIDFromRegistry reads RunningAppID from Steam's registry.vdf: the
// game Steam is running, or "" when none is. Steam writes 0 when idle.
func runningAppIDFromRegistry(data []byte) string {
	doc, err := parseVDF(data)
	if err != nil {
		return ""
	}
	id := strings.TrimSpace(doc.path("Registry", "HKCU", "Software", "Valve", "Steam").str("RunningAppID"))
	if id == "0" || !appIDRe.MatchString(id) {
		return ""
	}
	return id
}
