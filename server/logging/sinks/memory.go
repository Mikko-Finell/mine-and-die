package sinks

import (
	"context"
	"sync"

	"mine-and-die/server/logging"
)

type MemorySink struct {
	mu     sync.RWMutex
	events []logging.Event
}

func NewMemorySink() *MemorySink {
	return &MemorySink{events: make([]logging.Event, 0)}
}

func (s *MemorySink) Write(event logging.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, cloneForMemory(event))
	return nil
}

func (s *MemorySink) Events() []logging.Event {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copied := make([]logging.Event, len(s.events))
	copy(copied, s.events)
	return copied
}

func (s *MemorySink) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = s.events[:0]
}

func (s *MemorySink) Close(context.Context) error {
	return nil
}

func cloneForMemory(event logging.Event) logging.Event {
	cloned := event
	if len(event.Targets) > 0 {
		cloned.Targets = append([]logging.EntityRef(nil), event.Targets...)
	}
	if event.Extra != nil {
		copied := make(map[string]any, len(event.Extra))
		for k, v := range event.Extra {
			copied[k] = v
		}
		cloned.Extra = copied
	}
	return cloned
}
