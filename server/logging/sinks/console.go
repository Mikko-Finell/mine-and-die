package sinks

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"mine-and-die/server/logging"
)

// Console emits events using the legacy log.Printf format for parity.
type Console struct {
	mu     sync.Mutex
	logger *log.Logger
}

// NewConsole returns a console sink writing to the provided writer.
func NewConsole(w io.Writer) *Console {
	if w == nil {
		w = io.Discard
	}
	return &Console{logger: log.New(w, "", log.LstdFlags)}
}

// Write satisfies logging.Sink.
func (c *Console) Write(event logging.Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	payload := event.Payload
	if payload == nil {
		payload = event.Extra
	}

	c.logger.Printf("[%s] tick=%d actor=%s payload=%v extra=%v", event.Type, event.Tick, formatActor(event.Actor), payload, event.Extra)
	return nil
}

// Close satisfies logging.Sink.
func (c *Console) Close(context.Context) error { return nil }

func formatActor(ref logging.EntityRef) string {
	if ref.ID == "" {
		return "<none>"
	}
	if ref.Kind == "" {
		return ref.ID
	}
	return fmt.Sprintf("%s(%s)", ref.ID, ref.Kind)
}

// TimestampFromTime provides a helper to match legacy formatting when needed.
func TimestampFromTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339Nano)
}
