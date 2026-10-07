// Package applog keeps a rolling log of what Shelf and the tools it drives are
// doing, for the log window.
package applog

import (
	"strings"
	"sync"
	"time"
)

// Line is one log entry.
type Line struct {
	Time   int64  `json:"time"`   // unix milliseconds
	Source string `json:"source"` // install, update, launch, saves, ...
	App    string `json:"app,omitempty"`
	Level  string `json:"level"` // info, warn or error
	Text   string `json:"text"`
}

const flushDelay = 80 * time.Millisecond

// Log is a ring buffer that also pushes new lines to a listener in small batches.
type Log struct {
	mu      sync.Mutex
	max     int
	lines   []Line
	pending []Line
	timer   *time.Timer
	emit    func([]Line)
}

// New creates a log that keeps the most recent max lines.
func New(max int) *Log { return &Log{max: max} }

// SetEmitter sets where batches of new lines go.
func (l *Log) SetEmitter(emit func([]Line)) {
	l.mu.Lock()
	l.emit = emit
	l.mu.Unlock()
}

func classify(text string) string {
	switch {
	case strings.Contains(text, "ERROR"), strings.Contains(text, "CRITICAL"),
		strings.Contains(text, "FATAL"), strings.Contains(text, "Traceback"):
		return "error"
	case strings.Contains(text, "WARN"):
		return "warn"
	}
	return "info"
}

// Add records a line. The level is guessed from legendary's usual prefixes.
func (l *Log) Add(source, app, text string) {
	text = strings.TrimRight(text, " \t\r\n")
	if text == "" {
		return
	}
	line := Line{Time: time.Now().UnixMilli(), Source: source, App: app, Level: classify(text), Text: text}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, line)
	if len(l.lines) > l.max {
		l.lines = append([]Line(nil), l.lines[len(l.lines)-l.max:]...)
	}
	if l.emit == nil {
		return
	}
	l.pending = append(l.pending, line)
	if l.timer == nil {
		l.timer = time.AfterFunc(flushDelay, l.flush)
	}
}

func (l *Log) flush() {
	l.mu.Lock()
	batch := l.pending
	l.pending = nil
	l.timer = nil
	emit := l.emit
	l.mu.Unlock()
	if emit != nil && len(batch) > 0 {
		emit(batch)
	}
}

// AddLines records a block of output, one entry per line (\n or \r separated).
func (l *Log) AddLines(source, app, text string) {
	for _, line := range strings.FieldsFunc(text, func(r rune) bool { return r == '\n' || r == '\r' }) {
		l.Add(source, app, line)
	}
}

// Snapshot returns everything currently kept, oldest first.
func (l *Log) Snapshot() []Line {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]Line{}, l.lines...)
}

// Clear forgets all lines.
func (l *Log) Clear() {
	l.mu.Lock()
	l.lines, l.pending = nil, nil
	l.mu.Unlock()
}
