package epic

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"shelf/internal/library"
)

const fakeLegendary2 = `#!/bin/sh
echo "$@" >> "$GAMES/calls.log"
case "$*" in
*verify*)
  printf 'Verification progress: 1/2 (50.0%%) [1.0 MiB/s]\r'
  echo "[cli] ERROR: Verification failed, 1 file(s) corrupted, 2 file(s) are missing."
  ;;
*sync-saves*)
  echo "[cli] INFO: Uploading local savegame..."
  ;;
*"--update-only"*)
  echo "[DLManager] INFO: = Progress: 100.00% (2/2), Running for 00:00:01, ETA: 00:00:00" >&2
  ;;
*" import "*)
  printf '{"Sugar":{"app_name":"Sugar","title":"Sugar Game","install_path":"%s","version":"1","platform":"Windows"}}' "$4" > "$LEGENDARY_CONFIG_PATH/installed.json"
  ;;
esac
`

type recorder struct {
	mu     sync.Mutex
	events []Progress
	saves  []SaveEvent
}

func (r *recorder) emit(name string, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	switch v := data.(type) {
	case Progress:
		r.events = append(r.events, v)
	case SaveEvent:
		r.saves = append(r.saves, v)
	}
}

func (r *recorder) waitFinal(t *testing.T, kind string) Progress {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		for _, e := range r.events {
			if e.Kind == kind && e.State != StateInstalling && e.State != StateQueued {
				r.mu.Unlock()
				return e
			}
		}
		r.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no final %s event; got %+v", kind, r.events)
	return Progress{}
}

func TestJobsAndSaves(t *testing.T) {
	root := t.TempDir()
	games := filepath.Join(root, "games")
	bin := filepath.Join(root, "bin")
	os.MkdirAll(bin, 0o755)
	os.MkdirAll(filepath.Join(games, "Sugar"), 0o755)
	os.WriteFile(filepath.Join(bin, "legendary"), []byte(fakeLegendary2), 0o755)

	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "cfg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	t.Setenv("GAMES", games)

	proton := filepath.Join(root, "home", ".local", "share", "Steam", "compatibilitytools.d", "GE-Proton9-1")
	os.MkdirAll(filepath.Join(root, "home", ".local", "share", "Steam", "steamapps"), 0o755)
	os.MkdirAll(proton, 0o755)
	os.WriteFile(filepath.Join(proton, "proton"), []byte("#!/bin/sh\n"), 0o755)

	rec := &recorder{}
	m := New(library.NewHistory(), rec.emit)
	m.settings.set(Settings{InstallDir: games})

	os.MkdirAll(m.cfgDir, 0o755)
	os.WriteFile(filepath.Join(m.cfgDir, "user.json"), []byte(`{"displayName":"t","account_id":"a"}`), 0o600)
	os.WriteFile(filepath.Join(m.cfgDir, "installed.json"),
		[]byte(`{"Sugar":{"app_name":"Sugar","title":"Sugar Game","install_path":"`+filepath.Join(games, "Sugar")+`","version":"1","platform":"Windows"}}`), 0o644)
	os.WriteFile(filepath.Join(m.cfgDir, "assets.json"), []byte(`{"Windows":[{"app_name":"Sugar","build_version":"2"}]}`), 0o644)
	m.owned = []ownedGame{{AppName: "Sugar", AppTitle: "Sugar Game"}}
	m.owned[0].Metadata.CustomAttributes = map[string]struct {
		Value string `json:"value"`
	}{"CloudSaveFolder": {Value: "{UserSavedGames}/Sugar"}}

	// Update flagged on the game.
	list, _ := m.Provider().Scan()
	if len(list) != 1 || !list[0].UpdateAvailable || !list[0].CloudSaves || list[0].Version != "1" {
		t.Fatalf("scan: %+v", list)
	}

	// An outdated game refuses to launch, unless allowed.
	if err := m.Launch("Sugar"); err == nil || !strings.Contains(err.Error(), "needs an update") {
		t.Fatalf("launch of outdated game: %v", err)
	}

	// Update job.
	if err := m.Update("Sugar"); err != nil {
		t.Fatal(err)
	}
	if e := rec.waitFinal(t, KindUpdate); e.State != StateDone {
		t.Fatalf("update: %+v", e)
	}

	// Verify finds damage and says so.
	if err := m.Verify("Sugar"); err != nil {
		t.Fatal(err)
	}
	if e := rec.waitFinal(t, KindVerify); e.State != StateFailed || !e.Damaged || !strings.Contains(e.Error, "1 corrupted and 2 missing") {
		t.Fatalf("verify: %+v", e)
	}

	// Jobs on one game can't overlap, and queued ones are visible and orderable.
	m.SetQueuePaused(true) // nothing starts while paused
	if err := m.Repair("Sugar"); err != nil {
		t.Fatal(err)
	}
	if err := m.Repair("Sugar"); err == nil {
		t.Error("second job on the same game should be refused")
	}
	if st := m.QueueState(); !st.Paused || len(st.Jobs) != 1 || st.Jobs[0].State != StateQueued || st.Jobs[0].Position != 1 {
		t.Errorf("state: %+v", st)
	}
	m.CancelInstall("Sugar")
	if e := rec.waitFinal(t, KindRepair); e.State != StateCancelled {
		t.Errorf("cancelled queued job: %+v", e)
	}
	if st := m.QueueState(); len(st.Jobs) != 0 {
		t.Errorf("cancelled job still listed: %+v", st)
	}
	m.SetQueuePaused(false)

	// Cloud saves: not before Proton made the prefix, then through legendary.
	if err := m.SyncSaves("Sugar"); err == nil {
		t.Error("sync before the first launch should be refused")
	}
	os.MkdirAll(filepath.Join(prefixDir("Sugar"), "pfx", "drive_c"), 0o755)
	if err := m.SyncSaves("Sugar"); err != nil {
		t.Fatal(err)
	}
	if len(rec.saves) != 1 || rec.saves[0].State != "uploaded" {
		t.Errorf("saves: %+v", rec.saves)
	}
	cfg, _ := os.ReadFile(filepath.Join(m.cfgDir, "config.ini"))
	if !strings.Contains(string(cfg), "[Sugar.env]") || !strings.Contains(string(cfg), "STEAM_COMPAT_DATA_PATH = "+prefixDir("Sugar")) {
		t.Errorf("legendary config not pointed at the prefix:\n%s", cfg)
	}
	calls, _ := os.ReadFile(filepath.Join(games, "calls.log"))
	if !strings.Contains(string(calls), "-y sync-saves Sugar --accept-path") {
		t.Errorf("calls:\n%s", calls)
	}
}

func TestImportFlow(t *testing.T) {
	root := t.TempDir()
	games := filepath.Join(root, "games")
	bin := filepath.Join(root, "bin")
	os.MkdirAll(bin, 0o755)
	os.WriteFile(filepath.Join(bin, "legendary"), []byte(fakeLegendary2), 0o755)

	home := filepath.Join(root, "home")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "cfg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	t.Setenv("GAMES", games)
	os.MkdirAll(games, 0o755)

	// An existing Epic Games Launcher style install in Shelf's own folder.
	found := filepath.Join(games, "SugarGame")
	os.MkdirAll(filepath.Join(found, ".egstore"), 0o755)
	os.WriteFile(filepath.Join(found, ".egstore", "X.mancpn"), []byte(`{"AppName":"Sugar"}`), 0o644)
	// One that isn't on the account, and one Heroic already has.
	os.MkdirAll(filepath.Join(games, "Foreign", ".egstore"), 0o755)
	os.WriteFile(filepath.Join(games, "Foreign", ".egstore", "Y.mancpn"), []byte(`{"AppName":"NotMine"}`), 0o644)

	rec := &recorder{}
	m := New(library.NewHistory(), rec.emit)
	m.settings.set(Settings{InstallDir: games})
	os.MkdirAll(m.cfgDir, 0o755)
	os.WriteFile(filepath.Join(m.cfgDir, "user.json"), []byte(`{"displayName":"t"}`), 0o600)
	m.owned = []ownedGame{{AppName: "Sugar", AppTitle: "Sugar Game"}}

	got, err := m.FindImportable()
	if err != nil || len(got) != 1 || got[0].AppName != "Sugar" || got[0].Path != found || got[0].Source != "Epic Games Launcher" {
		t.Fatalf("importable: %+v %v", got, err)
	}

	if _, err := m.ImportAll(); err != nil {
		t.Fatal(err)
	}
	if e := rec.waitFinal(t, KindImport); e.State != StateDone {
		t.Fatalf("import: %+v", e)
	}
	if again, _ := m.FindImportable(); len(again) != 0 {
		t.Errorf("already imported game still offered: %+v", again)
	}
	if m.Import("Sugar", "relative/path") == nil {
		t.Error("relative paths must be refused")
	}
}

func TestQueueOrder(t *testing.T) {
	m := &Manager{installs: map[string]*installJob{}, emit: func(string, any) {}}
	m.paused = true // keep the scheduler from starting anything
	for _, name := range []string{"A", "B", "C"} {
		j := &installJob{p: Progress{AppName: name, State: StateQueued}}
		m.installs[name] = j
		m.queue = append(m.queue, j)
	}
	order := func() string {
		var out []string
		for _, j := range m.QueueState().Jobs {
			out = append(out, j.AppName)
		}
		return strings.Join(out, "")
	}

	if order() != "ABC" {
		t.Fatal(order())
	}
	m.QueueMove("C", -1000) // to the front
	if order() != "CAB" {
		t.Errorf("front: %s", order())
	}
	m.QueueMove("C", 1)
	if order() != "ACB" {
		t.Errorf("down: %s", order())
	}
	m.QueueMove("B", -1)
	m.QueueMove("B", -5) // clamps at the front
	if order() != "BAC" {
		t.Errorf("clamp: %s", order())
	}
	if m.QueueMove("Nope", 1) == nil {
		t.Error("unknown game should error")
	}
	for i, j := range m.QueueState().Jobs {
		if j.Position != i+1 {
			t.Errorf("position of %s = %d", j.AppName, j.Position)
		}
	}
}
