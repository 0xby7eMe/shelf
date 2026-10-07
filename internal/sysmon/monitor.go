package sysmon

import (
	"sync"
	"time"
)

// historyLen is how many samples are kept, so a view opened late still has a past.
const historyLen = 120

// Monitor samples on a steady beat for as long as someone is looking. A view
// calls Start to say it's there, and again now and then to say it still is.
// If it stops calling, as when the window goes away without a word, the
// monitor stops by itself rather than sampling forever.
type Monitor struct {
	sampler  *Sampler
	emit     func(Sample)
	interval time.Duration
	leaseTTL time.Duration

	mu      sync.Mutex
	running bool
	lease   time.Time
	stop    chan struct{}
	history []Sample
}

// NewMonitor returns a Monitor that passes every sample to emit.
func NewMonitor(s *Sampler, emit func(Sample)) *Monitor {
	return &Monitor{sampler: s, emit: emit, interval: time.Second, leaseTTL: 15 * time.Second}
}

// Start begins sampling, or renews the lease if it already has.
func (m *Monitor) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lease = time.Now().Add(m.leaseTTL)
	if m.running {
		return
	}
	m.running = true
	m.stop = make(chan struct{})
	go m.run(m.stop)
}

// Stop ends sampling now and forgets the history, which would otherwise show
// a gap the next time the view opens.
func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		close(m.stop)
		m.running = false
	}
	m.history = nil
}

// History returns the samples kept, oldest first.
func (m *Monitor) History() []Sample {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Sample{}, m.history...)
}

// Running reports whether the monitor is sampling.
func (m *Monitor) Running() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

func (m *Monitor) run(stop chan struct{}) {
	m.sampler.Sample() // rates need a first look to compare with
	tick := time.NewTicker(m.interval)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
		}

		m.mu.Lock()
		if !m.running || m.stop != stop {
			m.mu.Unlock()
			return
		}
		if time.Now().After(m.lease) {
			m.running, m.history = false, nil
			m.mu.Unlock()
			return
		}
		m.mu.Unlock()

		s := m.sampler.Sample()
		m.mu.Lock()
		if m.stop == stop && m.running {
			m.history = append(m.history, s)
			if len(m.history) > historyLen {
				m.history = m.history[len(m.history)-historyLen:]
			}
		}
		m.mu.Unlock()
		m.emit(s)
	}
}
