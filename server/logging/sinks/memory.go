package sinks

import (
	"context"
	"sync"

	"mine-and-die/server/logging"
)

// Memory collects events for assertions in tests.
type Memory struct {
	mu     sync.Mutex
	events []logging.Event
}

// NewMemory constructs an empty in-memory sink.
func NewMemory() *Memory {
	return &Memory{events: make([]logging.Event, 0)}
}

// Write satisfies logging.Sink.
func (m *Memory) Write(event logging.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := event
	if event.Extra != nil {
		copied.Extra = make(map[string]any, len(event.Extra))
		for k, v := range event.Extra {
			copied.Extra[k] = v
		}
	}
	if event.Targets != nil {
		copied.Targets = append([]logging.EntityRef(nil), event.Targets...)
	}
	m.events = append(m.events, copied)
	return nil
}

// Close satisfies logging.Sink.
func (m *Memory) Close(context.Context) error { return nil }

// Events returns a snapshot of collected events.
func (m *Memory) Events() []logging.Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := make([]logging.Event, len(m.events))
	copy(copied, m.events)
	return copied
}
