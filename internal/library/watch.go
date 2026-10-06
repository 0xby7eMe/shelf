package library

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

const watchDebounce = 1500 * time.Millisecond

func Watch(ctx context.Context, onChange func()) error {
	root, err := findSteamRoot()
	if err != nil {
		return err
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	addAll := func() int {
		n := 0
		for _, lib := range libraryDirs(root) {
			if w.Add(filepath.Join(lib, "steamapps")) == nil {
				n++
			}
		}
		cfgs, _ := filepath.Glob(filepath.Join(root, "userdata", "*", "config"))
		for _, c := range cfgs {
			if w.Add(c) == nil {
				n++
			}
		}
		return n
	}
	if addAll() == 0 {
		return fmt.Errorf("no steam folders to watch")
	}

	var pending *time.Timer
	trigger := func() {
		if pending != nil {
			pending.Stop()
		}
		pending = time.AfterFunc(watchDebounce, onChange)
	}
	defer func() {
		if pending != nil {
			pending.Stop()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case ev, ok := <-w.Events:
			if !ok {
				return nil
			}
			if ev.Op == fsnotify.Chmod {
				continue
			}
			name := filepath.Base(ev.Name)
			switch {
			case name == "libraryfolders.vdf":
				addAll()
				trigger()
			case name == "localconfig.vdf",
				strings.HasPrefix(name, "appmanifest_") && strings.HasSuffix(name, ".acf"):
				trigger()
			}
		case err, ok := <-w.Errors:
			if !ok {
				return nil
			}
			log.Printf("watch: %v", err)
		}
	}
}