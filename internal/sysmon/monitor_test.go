package sysmon

import (
	"sync"
	"testing"
	"time"
)

type collector struct {
	mu sync.Mutex
	n  int
}

func (c *collector) emit(Sample) { c.mu.Lock(); c.n++; c.mu.Unlock() }
func (c *collector) count() int  { c.mu.Lock(); defer c.mu.Unlock(); return c.n }

func fastMonitor(t *testing.T) (*Monitor, *collector) {
	f := newFixture(t)
	c := &collector{}
	m := NewMonitor(f.sampler, c.emit)
	m.interval = 10 * time.Millisecond
	return m, c
}

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for i := 0; i < 300; i++ {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestMonitorSamplesWhileStarted(t *testing.T) {
	m, c := fastMonitor(t)
	if m.Running() {
		t.Fatal("it must not sample before it's asked to")
	}
	m.Start()
	defer m.Stop()
	eventually(t, "samples", func() bool { return c.count() >= 3 })
	if h := m.History(); len(h) < 3 {
		t.Errorf("history: %d", len(h))
	}

	m.Stop()
	if m.Running() || len(m.History()) != 0 {
		t.Error("stopping clears the history, so the next view doesn't show a gap")
	}
	n := c.count()
	time.Sleep(60 * time.Millisecond)
	if c.count() > n+1 {
		t.Errorf("it kept sampling after stopping: %d -> %d", n, c.count())
	}
}

func TestMonitorStopsWhenTheViewVanishes(t *testing.T) {
	m, c := fastMonitor(t)
	m.leaseTTL = 60 * time.Millisecond
	m.Start()
	eventually(t, "samples", func() bool { return c.count() >= 1 })
	eventually(t, "the lease to run out", func() bool { return !m.Running() })

	n := c.count()
	time.Sleep(60 * time.Millisecond)
	if c.count() != n {
		t.Error("an abandoned monitor must stop sampling")
	}
}

func TestMonitorLeaseCanBeRenewed(t *testing.T) {
	m, c := fastMonitor(t)
	m.leaseTTL = 80 * time.Millisecond
	m.Start()
	defer m.Stop()
	for i := 0; i < 8; i++ {
		time.Sleep(30 * time.Millisecond)
		m.Start() // the view says it is still there
	}
	if !m.Running() || c.count() < 5 {
		t.Errorf("a renewed lease keeps it going: running=%v samples=%d", m.Running(), c.count())
	}
}

func TestMonitorRestartsAfterStop(t *testing.T) {
	m, c := fastMonitor(t)
	m.Start()
	eventually(t, "samples", func() bool { return c.count() >= 2 })
	m.Stop()
	n := c.count()
	m.Start()
	defer m.Stop()
	eventually(t, "samples after a restart", func() bool { return c.count() >= n+2 })
}

func TestMonitorHistoryIsBounded(t *testing.T) {
	m, c := fastMonitor(t)
	m.interval = time.Millisecond
	m.Start()
	defer m.Stop()
	eventually(t, "more samples than the history holds", func() bool { return c.count() > historyLen+10 })
	if n := len(m.History()); n > historyLen {
		t.Errorf("history holds %d, at most %d", n, historyLen)
	}
}
