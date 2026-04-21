// Package resilience provides three-tier degradation wrappers for every
// external dependency in ViewAura.
//
// Degradation order for every component:
//
//	Tier 1  External service / provider     (full capability)
//	Tier 2  Redis-backed in-process queue   (reduced capability)
//	Tier 3  Local JSONL log file            (zero capability, observable)
//
// The only hard startup requirement is *db.Pool.  Everything else —
// Kafka, Redis, email, AI, Temporal, R2, Stripe — degrades gracefully
// down to Tier 3 without crashing the API.
package resilience

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileOutbox is a thread-safe, append-only JSONL writer used as the Tier 3
// fallback when both the primary service and Redis are unavailable.
//
// Each entry is a single JSON line:
//
//	{"ts":"2024-01-01T00:00:00Z","kind":"kafka","topic":"user.events","payload":{...}}
//
// Files rotate daily: logs/events_outbox_2024-01-01.jsonl
// The ops team can replay entries via the kafka-replay tool in tools/.
//
// FileOutbox never panics and never returns an error — if it cannot write
// (e.g. disk full) it emits a stderr line and discards the entry.  The calling
// code should treat a FileOutbox write as best-effort.
type FileOutbox struct {
	mu      sync.Mutex
	dir     string // directory to write files into; defaults to "logs"
	kind    string // label used in the "kind" field, e.g. "kafka", "mail", "push"
	current *os.File
	day     string // YYYY-MM-DD of the currently open file
}

// NewFileOutbox creates a FileOutbox that writes to dir/kind_outbox_YYYY-MM-DD.jsonl.
// dir is created if it does not exist.
func NewFileOutbox(dir, kind string) *FileOutbox {
	if dir == "" {
		dir = "logs"
	}
	return &FileOutbox{dir: dir, kind: kind}
}

// OutboxEntry is the envelope written to the JSONL file.
type OutboxEntry struct {
	Timestamp string          `json:"ts"`
	Kind      string          `json:"kind"`
	Topic     string          `json:"topic,omitempty"`     // Kafka topic or mail template
	Recipient string          `json:"recipient,omitempty"` // email address or user ID
	Payload   json.RawMessage `json:"payload"`
}

// Write appends a single entry to today's log file.
// It is safe to call from multiple goroutines.
// On any I/O error it prints to stderr and returns — it never panics.
func (f *FileOutbox) Write(topic, recipient string, payload any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "file_outbox: marshal payload: %v\n", err)
		return
	}
	entry := OutboxEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Kind:      f.kind,
		Topic:     topic,
		Recipient: recipient,
		Payload:   raw,
	}
	line, err := json.Marshal(entry)
	if err != nil {
		fmt.Fprintf(os.Stderr, "file_outbox: marshal entry: %v\n", err)
		return
	}
	line = append(line, '\n')

	f.mu.Lock()
	defer f.mu.Unlock()

	file, err := f.openFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "file_outbox: open file: %v\n", err)
		return
	}
	if _, err := file.Write(line); err != nil {
		fmt.Fprintf(os.Stderr, "file_outbox: write: %v\n", err)
	}
}

// openFile returns the current log file, rotating when the day changes.
// Must be called with f.mu held.
func (f *FileOutbox) openFile() (*os.File, error) {
	today := time.Now().UTC().Format("2006-01-02")
	if f.current != nil && f.day == today {
		return f.current, nil
	}

	// Close the previous file if rotating.
	if f.current != nil {
		_ = f.current.Close()
		f.current = nil
	}

	if err := os.MkdirAll(f.dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", f.dir, err)
	}

	name := filepath.Join(f.dir, fmt.Sprintf("%s_outbox_%s.jsonl", f.kind, today))
	file, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", name, err)
	}

	f.current = file
	f.day = today
	return file, nil
}

// Close flushes and closes the currently open file.
// Safe to call multiple times.
func (f *FileOutbox) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.current != nil {
		_ = f.current.Close()
		f.current = nil
	}
}
