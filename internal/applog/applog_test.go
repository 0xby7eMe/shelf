package applog

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestRingAndLevels(t *testing.T) {
	l := New(3)
	for i := 0; i < 5; i++ {
		l.Add("install", "A", fmt.Sprintf("line %d", i))
	}
	got := l.Snapshot()
	if len(got) != 3 || got[0].Text != "line 2" || got[2].Text != "line 4" {
		t.Errorf("ring: %+v", got)
	}

	l.Clear()
	l.Add("x", "", "[cli] ERROR: boom")
	l.Add("x", "", "[cli] WARNING: hmm")
	l.Add("x", "", "plain")
	l.Add("x", "", "   \r\n") // blank lines are dropped
	s := l.Snapshot()
	if len(s) != 3 || s[0].Level != "error" || s[1].Level != "warn" || s[2].Level != "info" {
		t.Errorf("levels: %+v", s)
	}
}

func TestBatching(t *testing.T) {
	l := New(100)
	var mu sync.Mutex
	var batches [][]Line
	l.SetEmitter(func(b []Line) {
		mu.Lock()
		batches = append(batches, b)
		mu.Unlock()
	})
	for i := 0; i < 10; i++ {
		l.Add("x", "", fmt.Sprintf("n%d", i))
	}
	time.Sleep(250 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	total := 0
	for _, b := range batches {
		total += len(b)
	}
	if total != 10 || len(batches) > 2 {
		t.Errorf("want 10 lines in one or two batches, got %d in %d", total, len(batches))
	}
}
