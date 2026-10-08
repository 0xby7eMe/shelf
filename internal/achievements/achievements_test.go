package achievements

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"shelf/internal/steamapi"
)

type fake struct {
	source string
	ready  bool
	data   map[string]Progress
	err    map[string]error
	calls  atomic.Int32
}

func (f *fake) Source() string { return f.source }
func (f *fake) Name() string   { return "Fake" }
func (f *fake) Ready() (bool, string) {
	if !f.ready {
		return false, "not set up"
	}
	return true, ""
}
func (f *fake) Progress(_ context.Context, id string) (Progress, error) {
	f.calls.Add(1)
	if err := f.err[id]; err != nil {
		return Progress{}, err
	}
	p, ok := f.data[id]
	if !ok {
		return Progress{}, ErrNone
	}
	return p, nil
}
func (f *fake) Detail(_ context.Context, id string) (Detail, error) {
	p, err := f.Progress(context.Background(), id)
	return Detail{Source: f.source, GameID: id, Unlocked: p.Unlocked, Total: p.Total}, err
}

// waitIdle waits for a scan to finish.
func waitIdle(t *testing.T, h *Hub) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for h.Overview().Scanning {
		if time.Now().After(deadline) {
			t.Fatal("scan did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func refs(source string, ids ...string) []GameRef {
	out := make([]GameRef, len(ids))
	for i, id := range ids {
		out[i] = GameRef{Source: source, GameID: id, Name: "Game " + id}
	}
	return out
}

func TestScanSumsUpAndSortsByCompletion(t *testing.T) {
	f := &fake{source: "a", ready: true, data: map[string]Progress{
		"1": {Unlocked: 5, Total: 10},
		"2": {Unlocked: 10, Total: 10},
		"3": {Unlocked: 9, Total: 10},
	}}
	var events atomic.Int32
	h := NewHub("", func(string, any) { events.Add(1) }, f)

	h.Scan(context.Background(), refs("a", "1", "2", "3", "4"), false) // 4 has none
	waitIdle(t, h)

	o := h.Overview()
	if o.Unlocked != 24 || o.Total != 30 || o.Perfect != 1 || len(o.Games) != 3 {
		t.Fatalf("overview = %+v", o)
	}
	if o.Games[0].GameID != "2" || o.Games[1].GameID != "3" || o.Games[2].GameID != "1" {
		t.Errorf("order = %+v", o.Games)
	}
	if o.Providers[0].Games != 3 || !o.Providers[0].Ready {
		t.Errorf("providers = %+v", o.Providers)
	}
	if events.Load() == 0 {
		t.Error("no change event")
	}
}

func TestResultsAreRememberedUntilStaleOrForced(t *testing.T) {
	f := &fake{source: "a", ready: true, data: map[string]Progress{"1": {Unlocked: 1, Total: 2}}}
	h := NewHub("", nil, f)
	now := time.Unix(1_000_000, 0)
	h.now = func() time.Time { return now }
	r := refs("a", "1", "2")

	h.Scan(context.Background(), r, false)
	waitIdle(t, h)
	if f.calls.Load() != 2 {
		t.Fatalf("calls = %d", f.calls.Load())
	}
	h.Scan(context.Background(), r, false)
	waitIdle(t, h)
	if f.calls.Load() != 2 {
		t.Errorf("fresh results were fetched again: %d", f.calls.Load())
	}
	h.Scan(context.Background(), r, true)
	waitIdle(t, h)
	if f.calls.Load() != 4 {
		t.Errorf("force: %d", f.calls.Load())
	}
	now = now.Add(freshFor + time.Minute)
	h.Scan(context.Background(), r, false)
	waitIdle(t, h)
	if f.calls.Load() != 6 {
		t.Errorf("stale: %d", f.calls.Load())
	}
}

func TestCacheSurvivesARestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.json")
	f := &fake{source: "a", ready: true, data: map[string]Progress{"1": {Unlocked: 1, Total: 2}}}
	h := NewHub(path, nil, f)
	h.Scan(context.Background(), refs("a", "1"), false)
	waitIdle(t, h)

	again := NewHub(path, nil, f)
	again.Scan(context.Background(), refs("a", "1"), false)
	if o := again.Overview(); o.Total != 2 || o.Scanning {
		t.Errorf("overview = %+v", o)
	}
	if f.calls.Load() != 1 {
		t.Errorf("calls = %d, want 1", f.calls.Load())
	}
}

func TestOnlyReadyProvidersAreAskedAndOtherGamesIgnored(t *testing.T) {
	off := &fake{source: "b"}
	on := &fake{source: "a", ready: true, data: map[string]Progress{"1": {Total: 1}}}
	h := NewHub("", nil, on, off)
	h.Scan(context.Background(), append(refs("a", "1"), append(refs("b", "9"), refs("zzz", "5")...)...), false)
	waitIdle(t, h)
	if off.calls.Load() != 0 {
		t.Error("asked a provider that isn't ready")
	}
	o := h.Overview()
	if o.Providers[1].Ready || o.Providers[1].Message != "not set up" {
		t.Errorf("providers = %+v", o.Providers)
	}
}

func TestPrivateDataStopsThatLauncherAndSaysSo(t *testing.T) {
	// Every lookup says private. A worker's second job always sees that the
	// launcher was stopped, because its own first call is what stopped it, so
	// there can be at most one call per worker however quickly they run.
	ids := make([]string, 40)
	errs := map[string]error{}
	for i := range ids {
		ids[i] = fmt.Sprint(i + 1)
		errs[ids[i]] = ErrPrivate
	}
	f := &fake{source: "a", ready: true, err: errs}
	h := NewHub("", nil, f)
	h.Scan(context.Background(), refs("a", ids...), false)
	waitIdle(t, h)

	if h.Overview().Providers[0].Message == "" {
		t.Error("no explanation")
	}
	if n := int(f.calls.Load()); n > scanWorkers {
		t.Errorf("kept asking after the data was private: %d calls, at most %d expected", n, scanWorkers)
	}
}

func TestOtherErrorsAreNotRemembered(t *testing.T) {
	f := &fake{source: "a", ready: true, err: map[string]error{"1": errors.New("flaky")}}
	h := NewHub("", nil, f)
	h.Scan(context.Background(), refs("a", "1"), false)
	waitIdle(t, h)
	f.err = nil
	f.data = map[string]Progress{"1": {Unlocked: 1, Total: 1}}
	h.Scan(context.Background(), refs("a", "1"), false)
	waitIdle(t, h)
	if o := h.Overview(); o.Total != 1 {
		t.Errorf("a failed lookup was cached as empty: %+v", o)
	}
}

func TestDetailRefreshesTheOverview(t *testing.T) {
	f := &fake{source: "a", ready: true, data: map[string]Progress{"1": {Unlocked: 2, Total: 4}}}
	h := NewHub("", nil, f)
	h.mu.Lock()
	h.refs["a:1"] = GameRef{Source: "a", GameID: "1", Name: "One"}
	h.mu.Unlock()

	d, err := h.Detail(context.Background(), "a", "1")
	if err != nil || d.Total != 4 {
		t.Fatalf("detail = %+v, %v", d, err)
	}
	if o := h.Overview(); o.Total != 4 || o.Unlocked != 2 {
		t.Errorf("overview = %+v", o)
	}
	if d, err := h.Detail(context.Background(), "a", "none"); err != nil || d.Total != 0 || d.Achievements == nil {
		t.Errorf("no achievements should be an empty detail: %+v, %v", d, err)
	}
	if _, err := h.Detail(context.Background(), "nope", "1"); err == nil {
		t.Error("unknown launcher accepted")
	}
}

func TestUnsupportedSaysWhy(t *testing.T) {
	h := NewHub("", nil, NewUnsupported("epic", "Epic Games", "no way in"))
	h.Scan(context.Background(), refs("epic", "x"), false)
	o := h.Overview()
	if o.Providers[0].Ready || o.Providers[0].Message != "no way in" || o.Scanning {
		t.Errorf("providers = %+v", o.Providers)
	}
	if _, err := h.Detail(context.Background(), "epic", "x"); err == nil {
		t.Error("detail from an unsupported launcher")
	}
}

func TestSteamProviderSortsAndMergesTheSchema(t *testing.T) {
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/ISteamUserStats/GetPlayerAchievements/v1/":
			fmt.Fprint(w, `{"playerstats":{"success":true,"achievements":[
				{"apiname":"OLD","name":"Old one","achieved":1,"unlocktime":100},
				{"apiname":"NEW","name":"New one","achieved":1,"unlocktime":900},
				{"apiname":"RARE","name":"Rare","achieved":0},
				{"apiname":"EASY","name":"Easy","achieved":0}]}}`)
		case "/ISteamUserStats/GetSchemaForGame/v2/":
			fmt.Fprint(w, `{"game":{"availableGameStats":{"achievements":[
				{"name":"NEW","icon":"new.png","icongray":"new_g.png"},
				{"name":"RARE","hidden":1,"icon":"rare.png","icongray":"rare_g.png"}]}}}`)
		case "/ISteamUserStats/GetGlobalAchievementPercentagesForApp/v2/":
			fmt.Fprint(w, `{"achievementpercentages":{"achievements":[{"name":"RARE","percent":1.5},{"name":"EASY","percent":80}]}}`)
		}
	}))
	defer srv.Close()
	api := steamapi.New(func() (string, error) { return "me", nil })
	api.BaseURL = srv.URL
	setTestKey(api)

	p := NewSteam(api)
	pr, err := p.Progress(context.Background(), "1")
	if err != nil || pr.Unlocked != 2 || pr.Total != 4 {
		t.Fatalf("progress = %+v, %v", pr, err)
	}
	d, err := p.Detail(context.Background(), "1")
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, a := range d.Achievements {
		ids = append(ids, a.ID)
	}
	if fmt.Sprint(ids) != "[NEW OLD EASY RARE]" {
		t.Errorf("order = %v", ids)
	}
	byID := map[string]Achievement{}
	for _, a := range d.Achievements {
		byID[a.ID] = a
	}
	if byID["NEW"].Icon != "new.png" || byID["RARE"].Icon != "rare_g.png" || !byID["RARE"].Hidden {
		t.Errorf("schema not merged: %+v", byID)
	}
	if byID["RARE"].Percent != 1.5 || byID["OLD"].Percent != -1 {
		t.Errorf("rarity: %+v", byID)
	}
}
