package library

import (
	"fmt"
	"os/exec"
	"strings"
)

func openExternal(target string) error {
	cmd := exec.Command(openCommand, target)
	cmd.Env = ChildEnv()
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

// InstallSteam asks Steam to install a game, which opens its own install dialog.
func InstallSteam(appID string) error {
	if !appIDRe.MatchString(appID) {
		return fmt.Errorf("invalid app id")
	}
	return openExternal("steam://install/" + appID)
}

func Launch(appID string) error {
	if !appIDRe.MatchString(appID) {
		return fmt.Errorf("invalid app id")
	}
	return openExternal("steam://rungameid/" + appID)
}

func OpenStore(appID string) error {
	if !appIDRe.MatchString(appID) {
		return fmt.Errorf("invalid app id")
	}
	return openExternal("https://store.steampowered.com/app/" + appID)
}

// OpenURL opens an http(s) link in the default browser.
func OpenURL(u string) error {
	if !strings.HasPrefix(u, "https://") {
		return fmt.Errorf("refusing to open %q", u)
	}
	return openExternal(u)
}

func OpenFolder(path string) error {
	return openExternal(path)
}
