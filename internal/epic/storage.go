package epic

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// PrefixUsage is one game's Proton prefix and the space it takes.
type PrefixUsage struct {
	AppName string `json:"appName"`
	Title   string `json:"title"`
	Path    string `json:"path"`
	Bytes   int64  `json:"bytes"`
	// Installed is false for prefixes left behind by uninstalled games.
	Installed bool `json:"installed"`
	// Shared marks a prefix several games live inside, such as Ubisoft Connect's.
	// Its Bytes leave out those games, which are counted on their own.
	Shared bool `json:"shared,omitempty"`
}

// diskUsage sums the space files really occupy, like du, without following symlinks.
func diskUsage(ctx context.Context, root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil || d.IsDir() || d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if st, ok := info.Sys().(*syscall.Stat_t); ok {
			total += st.Blocks * 512
		} else {
			total += info.Size()
		}
		return nil
	})
	return total
}

// PrefixDir is where all game prefixes live.
func prefixRoot() string { return filepath.Join(dataDir(), "prefixes") }

// Prefixes lists every game prefix with its size, largest first.
func (m *Manager) Prefixes() []PrefixUsage {
	entries, err := os.ReadDir(prefixRoot())
	if err != nil {
		return []PrefixUsage{}
	}
	installed := m.readInstalled()
	m.mu.Lock()
	owned := m.owned
	m.mu.Unlock()
	titles := make(map[string]string, len(owned))
	for _, o := range owned {
		titles[o.AppName] = o.AppTitle
	}
	m.gogMu.Lock()
	for _, g := range m.gogOwned {
		titles[g.key()] = g.Title
	}
	m.gogMu.Unlock()
	gogInstalled := m.gogInstalled()

	out := make([]PrefixUsage, len(entries))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	n := 0
	for _, e := range entries {
		if !e.IsDir() || !appNameRe.MatchString(e.Name()) {
			continue
		}
		name := e.Name()
		i := n
		n++
		title := titles[name]
		if g, ok := installed[name]; ok && g.Title != "" {
			title = g.Title
		}
		if title == "" {
			title = name
		}
		_, isInstalled := installed[name]
		if g, ok := gogInstalled[name]; ok {
			isInstalled = true
			if g.Title != "" {
				title = g.Title
			}
		}
		shared := name == ubisoftPrefixName
		if shared {
			title, isInstalled = "Ubisoft Connect", connectInstalled()
		}
		out[i] = PrefixUsage{AppName: name, Title: title, Path: filepath.Join(prefixRoot(), name), Installed: isInstalled, Shared: shared}

		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			out[i].Bytes = diskUsage(ctx, out[i].Path)
			if out[i].Shared {
				out[i].Bytes = m.withoutUbisoftGames(ctx, out[i].Path, out[i].Bytes)
			}
		}()
	}
	wg.Wait()

	out = out[:n]
	sort.Slice(out, func(i, j int) bool { return out[i].Bytes > out[j].Bytes })
	return out
}

// withoutUbisoftGames takes the games installed inside Connect's prefix off its
// size, so the prefix shows what Connect itself takes and the games show their own.
func (m *Manager) withoutUbisoftGames(ctx context.Context, prefix string, total int64) int64 {
	complete, partial := m.ubisoftRegistry()
	for _, set := range []map[string]string{complete, partial} {
		for _, dir := range set {
			if strings.HasPrefix(dir, prefix+string(filepath.Separator)) {
				total -= m.ubiInstallSizeNow(ctx, dir)
			}
		}
	}
	if total < 0 {
		total = 0
	}
	return total
}

// InstallRoots lists folders that hold Epic data, for free space reporting.
func (m *Manager) InstallRoots() []string {
	return []string{m.settings.get().InstallDir, prefixRoot()}
}

// DeletePrefix removes a game's Proton prefix: wine registry, shader caches and
// any saves kept inside it. Cloud saves are not affected.
func (m *Manager) DeletePrefix(appName string) error {
	if !appNameRe.MatchString(appName) {
		return fmt.Errorf("invalid game id")
	}
	if m.isRunningGame(appName) {
		return fmt.Errorf("close the game first")
	}
	m.mu.Lock()
	_, busy := m.installs[appName]
	m.mu.Unlock()
	if busy {
		return fmt.Errorf("game is busy")
	}
	return os.RemoveAll(filepath.Join(prefixRoot(), appName))
}
