package library

import (
	"fmt"
	"os/exec"
)

func openExternal(target string) error {
	cmd := exec.Command("xdg-open", target)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
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

func OpenFolder(path string) error {
	return openExternal(path)
}