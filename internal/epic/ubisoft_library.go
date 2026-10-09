package epic

import (
	"context"
	"encoding/binary"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"shelf/internal/library"
)

// Ubisoft's web login is guarded by a bot check that only a real browser can
// pass, so Shelf never signs in to Ubisoft itself. Ubisoft Connect, running in
// its Proton prefix, is the account: once the user has signed in there, it
// keeps two files that Shelf reads:
//
//	cache/configuration/configurations  every game Connect knows: ids, name, how to start it
//	cache/ownership/<account id>        the launcher ids of the games the account owns
//
// Both are protobuf. The library is the games present in both.

// ubiGame is an owned game as Connect describes it.
type ubiGame struct {
	Name      string
	InstallID int // key of its install in the registry, and of `uplay://install`
	LaunchID  int // `uplay://launch`; usually the same number
	// Platform is the launcher that really installs the game ("Steam"), empty
	// when Connect does. Connect lists such games but can't install them.
	Platform string
	// Cover and Hero are Ubisoft's poster and banner art, as URLs.
	Cover, Hero string
}

// key is the game's id inside Shelf.
func (g ubiGame) key() string { return "uplay-" + strconv.Itoa(g.InstallID) }

var ubiKeyRe = regexp.MustCompile(`^uplay-(\d{1,10})$`)

// connectDir is where Connect itself is installed.
func connectDir() string {
	return filepath.Join(ubisoftPrefix(), "pfx", "drive_c", "Program Files (x86)",
		"Ubisoft", "Ubisoft Game Launcher")
}

// connectDataDir is where Connect keeps what it records about the account.
// Current versions write to the Windows user's AppData; older ones wrote
// next to the program. The folder with the records wins.
func connectDataDir() string {
	users := filepath.Join(ubisoftPrefix(), "pfx", "drive_c", "users")
	matches, _ := filepath.Glob(filepath.Join(users, "*", "AppData", "Local", "Ubisoft Game Launcher"))
	candidates := append(matches, connectDir())
	for _, dir := range candidates {
		if _, err := os.Stat(filepath.Join(dir, "cache", "configuration", "configurations")); err == nil {
			return dir
		}
	}
	return connectDir()
}

func connectConfigPath() string {
	return filepath.Join(connectDataDir(), "cache", "configuration", "configurations")
}

// ownershipFile is the newest file in Connect's ownership folder. They are
// named after the account id, and the newest belongs to whoever signed in last.
func ownershipFile() (path, account string) {
	dir := filepath.Join(connectDataDir(), "cache", "ownership")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", ""
	}
	var best os.FileInfo
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			continue
		}
		if best == nil || info.ModTime().After(best.ModTime()) {
			best = info
		}
	}
	if best == nil {
		return "", ""
	}
	return filepath.Join(dir, best.Name()), best.Name()
}

// --- protobuf ---

// pbField is one field of a protobuf message. Only varints and byte strings
// matter here.
type pbField struct {
	num   uint64
	value uint64 // varint fields
	bytes []byte // length-delimited fields
}

// pbMessage decodes a message's fields. It fails on anything it doesn't
// understand rather than guessing.
func pbMessage(b []byte) ([]pbField, error) {
	var out []pbField
	for len(b) > 0 {
		tag, n := binary.Uvarint(b)
		if n <= 0 {
			return nil, fmt.Errorf("bad tag")
		}
		b = b[n:]
		f := pbField{num: tag >> 3}
		switch tag & 7 {
		case 0:
			v, n := binary.Uvarint(b)
			if n <= 0 {
				return nil, fmt.Errorf("bad varint")
			}
			f.value, b = v, b[n:]
		case 2:
			l, n := binary.Uvarint(b)
			if n <= 0 || l > uint64(len(b)-n) {
				return nil, fmt.Errorf("bad length")
			}
			f.bytes, b = b[n:n+int(l)], b[n+int(l):]
		case 1:
			if len(b) < 8 {
				return nil, fmt.Errorf("short fixed64")
			}
			b = b[8:]
		case 5:
			if len(b) < 4 {
				return nil, fmt.Errorf("short fixed32")
			}
			b = b[4:]
		default:
			return nil, fmt.Errorf("unsupported wire type")
		}
		out = append(out, f)
	}
	return out, nil
}

// pbRecords splits a file into its top-level records: field 1, length-delimited,
// each holding one message. If the data between records isn't a record, it
// skips ahead to the next one that parses, so a changed header can't hide everything.
func pbRecords(data []byte) [][]pbField {
	var out [][]pbField
	for i := 0; i < len(data); {
		if data[i] != 0x0a {
			i++
			continue
		}
		l, n := binary.Uvarint(data[i+1:])
		end := i + 1 + n + int(l)
		if n <= 0 || l == 0 || l > uint64(len(data)) || end > len(data) {
			i++
			continue
		}
		fields, err := pbMessage(data[i+1+n : end])
		if err != nil {
			i++
			continue
		}
		out = append(out, fields)
		i = end
	}
	return out
}

// --- Connect's records ---

var (
	yamlName    = regexp.MustCompile(`(?m)^\s{1,4}name:\s*(.+?)\s*$`)
	yamlIdent   = regexp.MustCompile(`(?m)^\s+game_identifier:\s*(.+?)\s*$`)
	yamlThumb   = regexp.MustCompile(`(?m)^  thumb_image:\s*(\S+)`)
	yamlBack    = regexp.MustCompile(`(?m)^  background_image:\s*(\S+)`)
	yamlLocKey  = regexp.MustCompile(`^l\d+$`)
	yamlLocLine = regexp.MustCompile(`^    (l\d+):\s*(.+?)\s*$`)
	yamlGameKey = regexp.MustCompile(`(?m)^\s+GAMENAME:\s*(.+?)\s*$`)
	yamlThird   = regexp.MustCompile(`(?m)^\s+third_party_platform:\s*\r?\n\s+name:\s*(.+?)\s*$`)
)

// placeholderNames are what Connect writes when a game has no name of its own.
var placeholderNames = map[string]bool{"": true, "gamename": true, "l1": true, "ubisoft game": true, "name": true}

// ubiAssets is Ubisoft's CDN for the launcher's images.
const ubiAssets = "https://static3.cdn.ubi.com/orbit/uplay_launcher_3_0/assets/"

// localized maps a game's l1, l2... keys to their default-language text.
func localized(doc string) map[string]string {
	out := map[string]string{}
	_, rest, ok := strings.Cut(doc, "\n  default:")
	if !ok {
		return out
	}
	for _, line := range strings.Split(rest, "\n")[1:] {
		line = strings.TrimRight(line, "\r")
		if !strings.HasPrefix(line, "    ") {
			break
		}
		if m := yamlLocLine.FindStringSubmatch(line); m != nil {
			out[m[1]] = unquote(m[2])
		}
	}
	return out
}

// artURL finds the image a root-level field names, which may be a
// localization key, and returns its address on the CDN.
func artURL(re *regexp.Regexp, doc string, loc map[string]string) string {
	m := re.FindStringSubmatch(doc)
	if m == nil {
		return ""
	}
	file := unquote(m[1])
	if yamlLocKey.MatchString(file) {
		file = loc[file]
	}
	if file == "" || strings.ContainsAny(file, `/\?#`) {
		return ""
	}
	return ubiAssets + file
}

func platformName(raw string) string {
	switch strings.ToLower(raw) {
	case "steam":
		return "Steam"
	case "origin", "ea app":
		return "the EA app"
	}
	return raw
}

func unquote(s string) string { return strings.Trim(strings.TrimSpace(s), `"'`) }

// parseUbiConfigurations reads the games out of Connect's configurations file.
// Games that run through Steam or another launcher are kept, marked with that
// launcher, so the library is complete.
func parseUbiConfigurations(data []byte) []ubiGame {
	var out []ubiGame
	seen := map[int]bool{}
	for _, rec := range pbRecords(data) {
		var install, launch uint64
		var doc string
		for _, f := range rec {
			switch f.num {
			case 1:
				install = f.value
			case 2:
				launch = f.value
			case 3:
				doc = string(f.bytes)
			}
		}
		if install == 0 || seen[int(install)] || !strings.Contains(doc, "start_game") {
			continue
		}
		if launch == 0 {
			launch = install
		}

		name := ""
		if m := yamlName.FindStringSubmatch(doc); m != nil {
			name = unquote(m[1])
		}
		loc := localized(doc)
		if yamlLocKey.MatchString(name) && loc[name] != "" {
			name = loc[name]
		}
		for _, re := range []*regexp.Regexp{yamlIdent, yamlGameKey} {
			if !placeholderNames[strings.ToLower(name)] {
				break
			}
			if m := re.FindStringSubmatch(doc); m != nil {
				name = unquote(m[1])
			}
		}
		if placeholderNames[strings.ToLower(name)] {
			continue
		}
		platform := ""
		if m := yamlThird.FindStringSubmatch(doc); m != nil {
			platform = platformName(unquote(m[1]))
		}
		seen[int(install)] = true
		out = append(out, ubiGame{
			Name: name, InstallID: int(install), LaunchID: int(launch), Platform: platform,
			Cover: artURL(yamlThumb, doc, loc), Hero: artURL(yamlBack, doc, loc),
		})
	}
	return out
}

// ownershipHeader is how many bytes of the ownership file come before its records.
const ownershipHeader = 0x108

// parseUbiOwnership returns the launcher ids the account owns.
func parseUbiOwnership(data []byte) map[int]bool {
	owned := map[int]bool{}
	if len(data) <= ownershipHeader {
		return owned
	}
	for _, rec := range pbRecords(data[ownershipHeader:]) {
		for _, f := range rec {
			if (f.num == 1 || f.num == 2) && f.value != 0 {
				owned[int(f.value)] = true
			}
		}
	}
	return owned
}

// UbisoftLibrary is what Shelf learned from Connect's files.
type ubiLocal struct {
	account string
	games   []ubiGame
}

type ubiLocalCache struct {
	stamp string
	local ubiLocal
}

func fileStamp(path string) string {
	st, err := os.Stat(path)
	if err != nil {
		return "-"
	}
	return fmt.Sprintf("%d:%d", st.Size(), st.ModTime().UnixNano())
}

// readConnect returns the account and owned games Connect has recorded, read
// again only when its files change.
func (m *Manager) readConnect() ubiLocal {
	own, account := ownershipFile()
	stamp := fileStamp(connectConfigPath()) + "|" + own + "|" + fileStamp(own)

	m.ubiMu.Lock()
	c := m.ubiCache
	m.ubiMu.Unlock()
	if c != nil && c.stamp == stamp {
		return c.local
	}

	local := ubiLocal{account: account}
	if own != "" {
		ownData, err1 := os.ReadFile(own)
		cfgData, err2 := os.ReadFile(connectConfigPath())
		if err1 == nil && err2 == nil {
			owned := parseUbiOwnership(ownData)
			for _, g := range parseUbiConfigurations(cfgData) {
				if owned[g.LaunchID] || owned[g.InstallID] {
					local.games = append(local.games, g)
				}
			}
			sort.Slice(local.games, func(i, j int) bool {
				return strings.ToLower(local.games[i].Name) < strings.ToLower(local.games[j].Name)
			})
			m.logf(ubisoftSource, "", "Connect lists %d owned games for account %s", len(local.games), account)
		}
	}

	m.ubiMu.Lock()
	m.ubiCache = &ubiLocalCache{stamp: stamp, local: local}
	m.ubiMu.Unlock()
	return local
}

// ubiLookup finds an owned game by its Shelf id.
func (m *Manager) ubiLookup(key string) (ubiGame, bool) {
	if !ubiKeyRe.MatchString(key) {
		return ubiGame{}, false
	}
	for _, g := range m.readConnect().games {
		if g.key() == key {
			return g, true
		}
	}
	return ubiGame{}, false
}

// UbisoftSync reads Connect's files again and refreshes the library.
func (m *Manager) UbisoftSync() error {
	if !connectInstalled() {
		return fmt.Errorf("set up Ubisoft Connect first")
	}
	m.ubiMu.Lock()
	m.ubiCache = nil
	m.ubiMu.Unlock()
	if m.readConnect().account == "" {
		return fmt.Errorf("Ubisoft Connect hasn't been signed in yet. Open it and sign in first")
	}
	m.send("library:changed", nil)
	return nil
}

// --- library provider ---

// UbisoftProvider returns the library.Provider for Ubisoft games.
func (m *Manager) UbisoftProvider() library.Provider { return ubiProvider{m} }

type ubiProvider struct{ m *Manager }

func (ubiProvider) Source() library.Source { return library.SourceUbisoft }

func (p ubiProvider) Scan() ([]library.Game, error) {
	if macOS {
		return nil, nil // Ubisoft Connect runs through Proton
	}
	m := p.m
	if !connectInstalled() {
		return nil, nil
	}
	local := m.readConnect()
	installs := m.ubisoftInstalls()

	games := make([]library.Game, 0, len(local.games))
	for _, o := range local.games {
		key := o.key()
		g := library.Game{
			ID:         "ubisoft:" + key,
			Source:     library.SourceUbisoft,
			ExternalID: key,
			Name:       o.Name,
			ThirdParty: o.Platform,
			Cover:      o.Cover,
			Hero:       o.Hero,
		}
		if dir, ok := installs[strconv.Itoa(o.InstallID)]; ok && o.Platform == "" {
			g.Installed = true
			g.InstallPath = dir
			g.AntiCheat = antiCheatOf(dir)
			g.SizeBytes = m.ubiInstallSize(dir)
		}
		g.PlaytimeMinutes, g.LastPlayed = m.hist.Totals(key)
		games = append(games, g)
	}
	return games, nil
}

// --- install sizes ---

type ubiSizeEntry struct {
	bytes int64
	at    time.Time
}

// sizeTTL is how long a measured install size is trusted. Measuring walks the
// whole game folder, so it is done in the background and not on every refresh.
const sizeTTL = 5 * time.Minute

// ubiInstallSize returns the space a game's folder takes, as last measured.
// It never waits: a size that is missing or stale is measured in the
// background, and the library refreshes when the number changes.
func (m *Manager) ubiInstallSize(dir string) int64 {
	m.ubiMu.Lock()
	defer m.ubiMu.Unlock()
	e, known := m.ubiSizes[dir]
	if (!known || time.Since(e.at) > sizeTTL) && !m.ubiSizing[dir] {
		if m.ubiSizing == nil {
			m.ubiSizing = map[string]bool{}
		}
		m.ubiSizing[dir] = true
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			bytes := diskUsage(ctx, dir)
			m.ubiMu.Lock()
			if m.ubiSizes == nil {
				m.ubiSizes = map[string]ubiSizeEntry{}
			}
			prev, had := m.ubiSizes[dir]
			m.ubiSizes[dir] = ubiSizeEntry{bytes: bytes, at: time.Now()}
			delete(m.ubiSizing, dir)
			m.ubiMu.Unlock()
			if !had || prev.bytes != bytes {
				m.send("library:changed", nil)
			}
		}()
	}
	return e.bytes
}

// ubiInstallSizeNow is ubiInstallSize for callers that can wait, such as the
// storage view, which wants the real number.
func (m *Manager) ubiInstallSizeNow(ctx context.Context, dir string) int64 {
	m.ubiMu.Lock()
	e, known := m.ubiSizes[dir]
	m.ubiMu.Unlock()
	if known && time.Since(e.at) <= sizeTTL {
		return e.bytes
	}
	bytes := diskUsage(ctx, dir)
	if ctx.Err() != nil {
		return bytes
	}
	m.ubiMu.Lock()
	if m.ubiSizes == nil {
		m.ubiSizes = map[string]ubiSizeEntry{}
	}
	m.ubiSizes[dir] = ubiSizeEntry{bytes: bytes, at: time.Now()}
	m.ubiMu.Unlock()
	return bytes
}

// UbisoftStoreURL returns Ubisoft's store search for a game.
func (m *Manager) UbisoftStoreURL(key string) (string, error) {
	g, ok := m.ubiLookup(key)
	if !ok {
		return "", fmt.Errorf("unknown game")
	}
	return "https://store.ubisoft.com/search?q=" + url.QueryEscape(g.Name), nil
}
