package epic

// GOG has no update channel for offline installers: a new version is a new
// installer. Shelf asks GOG's product API which version its installer has now,
// and an update runs that installer over the game's folder.

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

var gogLatestMu sync.Mutex

func gogLatestPath() string {
	if dir := configDir(); dir != "" {
		return filepath.Join(dir, "gog-updates.json")
	}
	return ""
}

// readGogLatest is the installer version GOG last named for each game, by key.
func readGogLatest() map[string]string {
	gogLatestMu.Lock()
	defer gogLatestMu.Unlock()
	out := map[string]string{}
	if data, err := os.ReadFile(gogLatestPath()); err == nil {
		_ = json.Unmarshal(data, &out)
	}
	return out
}

// setGogLatest records the version GOG names for a game. An empty version is
// GOG not saying, which tells nothing, so the old answer stays.
func setGogLatest(key, version string) {
	if version == "" {
		return
	}
	writeGogLatest(map[string]string{key: version})
}

func writeGogLatest(found map[string]string) {
	gogLatestMu.Lock()
	defer gogLatestMu.Unlock()
	all := map[string]string{}
	if data, err := os.ReadFile(gogLatestPath()); err == nil {
		_ = json.Unmarshal(data, &all)
	}
	for k, v := range found {
		all[k] = v
	}
	_ = writeJSON(gogLatestPath(), all)
}

// pendingGogUpdates maps key to the newer version for every installed game
// that is behind. It works from files on disk, so it is cheap to call.
func pendingGogUpdates(installed map[string]gogInstall) map[string]string {
	latest := readGogLatest()
	out := map[string]string{}
	for key, g := range installed {
		if v := latest[key]; v != "" && g.Version != "" && v != g.Version {
			out[key] = v
		}
	}
	return out
}

// GogUpdates lists the installed GOG games that have a newer version.
func (m *Manager) GogUpdates() []UpdateInfo {
	installed := m.gogInstalled()
	out := []UpdateInfo{}
	for key, latest := range pendingGogUpdates(installed) {
		g := installed[key]
		out = append(out, UpdateInfo{AppName: key, Title: g.Title, Installed: g.Version, Latest: latest})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out
}

// GogCheckUpdates asks GOG for the current installer of every installed game
// and returns the games that are behind.
func (m *Manager) GogCheckUpdates() ([]UpdateInfo, error) {
	if !m.GogAccount().LoggedIn {
		return nil, nil
	}
	installed := m.gogInstalled()
	if len(installed) == 0 {
		return []UpdateInfo{}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	m.logf("updates", "", "checking %d GOG game(s) for updates", len(installed))

	var (
		mu     sync.Mutex
		found  = map[string]string{}
		failed int
		last   error
		wg     sync.WaitGroup
		sem    = make(chan struct{}, 4)
	)
	for key := range installed {
		var id int64
		if _, err := fmt.Sscanf(key, "gog-%d", &id); err != nil {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			inst, err := m.gogCurrentInstaller(ctx, id)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				failed++
				last = err
				m.logf("updates", key, "ERROR: %v", err)
				return
			}
			if inst.Version != "" {
				found[key] = inst.Version
			}
		}()
	}
	wg.Wait()
	writeGogLatest(found)

	// Only a check that learnt nothing at all is a failure.
	if failed == len(installed) && last != nil {
		return nil, last
	}
	m.send("library:changed", nil)
	updates := m.GogUpdates()
	m.logf("updates", "", "%d GOG update(s) available", len(updates))
	return updates, nil
}

// GogUpdate downloads a game's current installer and runs it over the
// installed game. It waits its turn in the downloads queue.
func (m *Manager) GogUpdate(key string) error {
	if macOS {
		return errNeedsProton("Updating GOG games from their Windows installers")
	}
	if !gogKeyRe.MatchString(key) {
		return fmt.Errorf("invalid game id")
	}
	if !m.GogAccount().LoggedIn {
		return fmt.Errorf("not signed in to GOG")
	}
	g, ok := m.gogInstalled()[key]
	if !ok {
		return fmt.Errorf("game is not installed")
	}
	if m.isRunningGame(key) {
		return fmt.Errorf("close the game first")
	}
	game, ok := m.gogProductByKey(key)
	if !ok {
		return fmt.Errorf("this game isn't in your GOG library")
	}
	if _, ok := resolveProton(m.settings.get().ProtonPath); !ok {
		return fmt.Errorf("no Proton installation found. Install Proton through Steam or ProtonUp-Qt")
	}
	// The download goes next to the game's folder, where an install would have put it.
	base := filepath.Dir(g.InstallPath)

	ctx, cancel := context.WithCancel(context.Background())
	job := &installJob{cancel: cancel, p: Progress{AppName: key, Kind: KindUpdate, State: StateQueued}}
	job.run = func() {
		job.mu.Lock()
		job.p.State = StateInstalling
		job.mu.Unlock()
		m.send("epic:install", job.snapshot())

		m.logf(KindUpdate, key, "updating from version %s", g.Version)
		err := m.runGogInstaller(ctx, job, KindUpdate, game, base, g.InstallPath, g.InstalledAt)
		switch {
		case ctx.Err() != nil:
			m.finishJob(job, StateCancelled, "", false)
		case err != nil:
			m.finishJob(job, StateFailed, err.Error(), false)
		default:
			m.finishJob(job, StateDone, "", false)
		}
	}
	return m.enqueue(job, true)
}

// gogAutoUpdateRound checks GOG for updates, tells the user about new ones
// and, when asked to, installs them.
func (m *Manager) gogAutoUpdateRound() {
	s := m.settings.get()
	if !s.GogAutoCheckUpdates || !m.GogAccount().LoggedIn {
		return
	}
	updates, err := m.GogCheckUpdates()
	if err != nil {
		log.Printf("gog: update check: %v", err)
		return
	}

	var fresh, toInstall []string
	m.mu.Lock()
	for _, u := range updates {
		if m.announcedUpdates[u.AppName] != u.Latest {
			m.announcedUpdates[u.AppName] = u.Latest
			fresh = append(fresh, u.Title)
		}
	}
	m.mu.Unlock()

	if s.GogAutoUpdate && !macOS {
		for _, u := range updates {
			if choose(m.games.get(u.AppName).AutoUpdate, true) && !m.isRunningGame(u.AppName) {
				toInstall = append(toInstall, u.AppName)
			}
		}
	}
	if len(fresh) > 0 {
		m.send("epic:updates", UpdatesEvent{Titles: fresh, Auto: len(toInstall) > 0})
	}
	for _, key := range toInstall {
		if err := m.GogUpdate(key); err != nil {
			log.Printf("gog: auto update %s: %v", key, err)
		}
	}
}
