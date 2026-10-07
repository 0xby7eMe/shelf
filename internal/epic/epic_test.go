package epic

import (
	"sort"
	"testing"
)

func TestExtractCode(t *testing.T) {
	code := "0123456789abcdef0123456789abcdef"
	ok := []string{
		code,
		`"` + code + `"`,
		"  " + code + "\n",
		`{"redirectUrl":"x","authorizationCode":"` + code + `","sid":null}`,
	}
	for _, in := range ok {
		got, err := ExtractCode(in)
		if err != nil || got != code {
			t.Errorf("ExtractCode(%q) = %q, %v", in, got, err)
		}
	}
	for _, in := range []string{"", "short", `{"authorizationCode":null}`, "{bad", code + "x"} {
		if _, err := ExtractCode(in); err == nil {
			t.Errorf("ExtractCode(%q) should fail", in)
		}
	}
}

func TestParseProgress(t *testing.T) {
	var p Progress
	line := "[DLManager] INFO: = Progress: 12.34% (123/456), Running for 00:00:10, ETA: 00:01:40"
	if !parseProgress(line, &p) || p.Percent != 12.34 || p.ETA != "00:01:40" {
		t.Errorf("progress: %+v", p)
	}
	if !parseProgress("[DLManager] INFO: + Download\t- 25.50 MiB/s (raw) / 12.00 MiB/s (decompressed)", &p) || p.Speed != "25.50 MiB/s" {
		t.Errorf("speed: %+v", p)
	}
	if parseProgress("unrelated", &p) {
		t.Error("unrelated line matched")
	}
}

func TestNaturalOrder(t *testing.T) {
	names := []string{"GE-Proton9-9", "GE-Proton10-1", "GE-Proton9-10"}
	sort.Slice(names, func(i, j int) bool { return naturalLess(names[j], names[i]) })
	want := []string{"GE-Proton10-1", "GE-Proton9-10", "GE-Proton9-9"}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("got %v, want %v", names, want)
		}
	}
}

func TestProtonRank(t *testing.T) {
	if !(protonRank("GE-Proton9-1") < protonRank("Proton - Experimental") &&
		protonRank("Proton - Experimental") < protonRank("Proton 9.0")) {
		t.Error("unexpected family order")
	}
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote("/a b/it's"); got != `'/a b/it'"'"'s'` {
		t.Errorf("got %s", got)
	}
}

func TestAppNameRe(t *testing.T) {
	for _, n := range []string{"Fortnite", "9d2d0eb64d5c44529cece33fe2a46482", "a_b-c"} {
		if !appNameRe.MatchString(n) {
			t.Errorf("%q rejected", n)
		}
	}
	for _, n := range []string{"", "-rf", "a b", "../x", "a/b"} {
		if appNameRe.MatchString(n) {
			t.Errorf("%q accepted", n)
		}
	}
}
