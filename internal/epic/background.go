package epic

import (
	"context"
	"log"
	"time"
)

const (
	firstCheckDelay = 20 * time.Second
	checkEvery      = 6 * time.Hour
)

// UpdatesEvent is sent as "epic:updates" when new updates were found.
type UpdatesEvent struct {
	Titles []string `json:"titles"`
	// Auto is true when Shelf is going to install them itself.
	Auto bool `json:"auto"`
}

// Start runs the periodic update checks, for Epic and GOG, until ctx ends.
func (m *Manager) Start(ctx context.Context) {
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(firstCheckDelay):
		}
		m.announceImportable()
		for {
			m.autoUpdateRound()
			m.gogAutoUpdateRound()
			select {
			case <-ctx.Done():
				return
			case <-time.After(checkEvery):
			}
		}
	}()
}

// announceImportable tells the UI once per session when installed games can be adopted.
func (m *Manager) announceImportable() {
	m.mu.Lock()
	done := m.announcedImport
	m.announcedImport = true
	m.mu.Unlock()
	if done || !m.Account().LoggedIn {
		return
	}
	if found, err := m.FindImportable(); err == nil && len(found) > 0 {
		m.send("epic:importable", len(found))
	}
}

func (m *Manager) autoUpdateRound() {
	s := m.settings.get()
	if !s.AutoCheckUpdates || !m.Account().LoggedIn {
		return
	}
	updates, err := m.CheckUpdates()
	if err != nil {
		log.Printf("epic: update check: %v", err)
		return
	}

	var fresh []string
	var toInstall []string
	m.mu.Lock()
	for _, u := range updates {
		if m.announcedUpdates[u.AppName] != u.Latest {
			m.announcedUpdates[u.AppName] = u.Latest
			fresh = append(fresh, u.Title)
		}
	}
	m.mu.Unlock()

	for _, u := range updates {
		if choose(m.games.get(u.AppName).AutoUpdate, s.AutoUpdate) && !m.isRunningGame(u.AppName) {
			toInstall = append(toInstall, u.AppName)
		}
	}
	if len(fresh) > 0 {
		m.send("epic:updates", UpdatesEvent{Titles: fresh, Auto: len(toInstall) > 0})
	}
	for _, name := range toInstall {
		if err := m.Update(name); err != nil {
			log.Printf("epic: auto update %s: %v", name, err)
		}
	}
}
