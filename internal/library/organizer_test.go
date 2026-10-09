package library

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func newTestOrganizer(t *testing.T) *Organizer {
	t.Helper()
	return &Organizer{data: emptyOrganization(), path: filepath.Join(t.TempDir(), "organizer.json")}
}

func TestCollections(t *testing.T) {
	o := newTestOrganizer(t)

	d, err := o.CreateCollection("  Co-op   nights ")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Collections) != 1 || d.Collections[0].Name != "Co-op nights" {
		t.Fatalf("name not cleaned: %+v", d.Collections)
	}
	id := d.Collections[0].ID

	if _, err := o.CreateCollection("co-op NIGHTS"); err == nil {
		t.Error("duplicate name (ignoring case) accepted")
	}
	if _, err := o.CreateCollection("   "); err == nil {
		t.Error("blank name accepted")
	}
	if _, err := o.CreateCollection(strings.Repeat("x", maxCollectionName+1)); err == nil {
		t.Error("overlong name accepted")
	}

	if _, err := o.SetGameCollections("steam:620", []string{id}); err != nil {
		t.Fatal(err)
	}
	if _, err := o.SetGameCollections("epic:Fortnite", []string{id}); err != nil {
		t.Fatal(err)
	}
	d, _ = o.SetGameCollections("steam:620", []string{id}) // again: no duplicate
	if got := d.Collections[0].Games; !reflect.DeepEqual(got, []string{"steam:620", "epic:Fortnite"}) {
		t.Fatalf("games = %v", got)
	}

	d, _ = o.SetGameCollections("steam:620", nil)
	if got := d.Collections[0].Games; !reflect.DeepEqual(got, []string{"epic:Fortnite"}) {
		t.Fatalf("after removing: %v", got)
	}

	if _, err := o.SetGameCollections("steam:620", []string{"nope"}); err == nil {
		t.Error("unknown collection accepted")
	}
	if _, err := o.SetGameCollections("../etc", []string{id}); err == nil {
		t.Error("invalid game id accepted")
	}
	if _, err := o.SetGameCollections("itch:1", []string{id}); err == nil {
		t.Error("unknown store accepted")
	}

	d, err = o.RenameCollection(id, "Friday")
	if err != nil || d.Collections[0].Name != "Friday" {
		t.Fatalf("rename: %v %+v", err, d.Collections)
	}

	d, err = o.DeleteCollection(id)
	if err != nil || len(d.Collections) != 0 {
		t.Fatalf("delete: %v %+v", err, d.Collections)
	}
	if _, err := o.DeleteCollection(id); err == nil {
		t.Error("deleting twice succeeded")
	}
}

func TestTags(t *testing.T) {
	o := newTestOrganizer(t)

	d, err := o.SetTags("steam:620", []string{" Puzzle ", "puzzle", "Co-op", "", strings.Repeat("y", maxTagLength+1)})
	if err != nil {
		t.Fatal(err)
	}
	if got := d.Tags["steam:620"]; !reflect.DeepEqual(got, []string{"co-op", "puzzle"}) {
		t.Fatalf("tags = %v", got)
	}

	o.SetTags("epic:Fortnite", []string{"puzzle", "battle royale"})
	d, _ = o.DeleteTag("Puzzle")
	if got := d.Tags["steam:620"]; !reflect.DeepEqual(got, []string{"co-op"}) {
		t.Errorf("after delete: %v", got)
	}
	if got := d.Tags["epic:Fortnite"]; !reflect.DeepEqual(got, []string{"battle royale"}) {
		t.Errorf("after delete: %v", got)
	}

	d, _ = o.SetTags("steam:620", nil)
	if _, ok := d.Tags["steam:620"]; ok {
		t.Error("empty tag list should remove the entry")
	}

	many := make([]string, 0, maxTagsPerGame+5)
	for i := 0; i < maxTagsPerGame+5; i++ {
		many = append(many, string(rune('a'+i)))
	}
	d, _ = o.SetTags("steam:1", many)
	if len(d.Tags["steam:1"]) != maxTagsPerGame {
		t.Errorf("tag cap: got %d", len(d.Tags["steam:1"]))
	}
}

func TestOrganizerPersists(t *testing.T) {
	o := newTestOrganizer(t)
	d, _ := o.CreateCollection("RPGs")
	o.SetGameCollections("steam:1", []string{d.Collections[0].ID})
	o.SetTags("steam:1", []string{"long"})

	again := &Organizer{data: emptyOrganization(), path: o.path}
	again.load()
	if !reflect.DeepEqual(again.Get(), o.Get()) {
		t.Fatalf("reloaded %+v, want %+v", again.Get(), o.Get())
	}
}

func TestOrganizerLoadDropsBadEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "organizer.json")
	os.WriteFile(path, []byte(`{
		"collections": [
			{"id": "a", "name": "Good", "games": ["steam:1", "../x", "steam:1"]},
			{"id": "a", "name": "Dup id", "games": []},
			{"id": "b", "name": "", "games": []}
		],
		"tags": {"steam:1": ["Fun", "fun"], "bad id": ["x"]}
	}`), 0o644)

	o := &Organizer{data: emptyOrganization(), path: path}
	o.load()
	d := o.Get()
	if len(d.Collections) != 1 || !reflect.DeepEqual(d.Collections[0].Games, []string{"steam:1"}) {
		t.Errorf("collections = %+v", d.Collections)
	}
	if len(d.Tags) != 1 || !reflect.DeepEqual(d.Tags["steam:1"], []string{"fun"}) {
		t.Errorf("tags = %+v", d.Tags)
	}
}

func TestOrganizerRollsBackWhenSaveFails(t *testing.T) {
	o := &Organizer{data: emptyOrganization(), path: filepath.Join(t.TempDir(), "missing", "organizer.json")}
	if _, err := o.CreateCollection("Nope"); err == nil {
		t.Fatal("expected save error")
	}
	if n := len(o.Get().Collections); n != 0 {
		t.Errorf("state kept after failed save: %d collections", n)
	}
}
