//go:build !windows

package library

import "os/exec"

// openExternal hands a link or folder to the desktop's default program.
func openExternal(target string) error {
	cmd := exec.Command(openCommand, target)
	cmd.Env = ChildEnv()
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

// pathKey is how two spellings of one folder are told apart.
func pathKey(p string) string { return p }
