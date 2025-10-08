package sinks

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"sync"
	"time"

	"mine-and-die/server/logging"
)

// JSON emits newline-delimited structured events.
type JSON struct {
	mu        sync.Mutex
	writer    *bufio.Writer
	encoder   *json.Encoder
	autoFlush bool
}

// NewJSON constructs a JSON sink writing to the provided io.Writer.
func NewJSON(w io.Writer, flushInterval time.Duration) *JSON {
	if w == nil {
		w = io.Discard
	}
	buf := bufio.NewWriter(w)
	sink := &JSON{writer: buf, encoder: json.NewEncoder(buf), autoFlush: flushInterval <= 0}
	if flushInterval > 0 {
		go sink.periodicFlush(flushInterval)
	}
	return sink
}

// Write satisfies logging.Sink.
func (s *JSON) Write(event logging.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	wire := map[string]any{
		"type":      event.Type,
		"tick":      event.Tick,
		"time":      event.Time.Format(time.RFC3339Nano),
		"severity":  event.Severity,
		"category":  event.Category,
		"actor":     event.Actor,
		"targets":   event.Targets,
		"payload":   event.Payload,
		"extra":     event.Extra,
		"traceId":   event.TraceID,
		"commandId": event.CommandID,
	}
	if err := s.encoder.Encode(wire); err != nil {
		return err
	}
	if s.autoFlush {
		return s.writer.Flush()
	}
	return nil
}

// Close flushes buffers.
func (s *JSON) Close(context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writer.Flush()
}

func (s *JSON) periodicFlush(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		s.mu.Lock()
		s.writer.Flush()
		s.mu.Unlock()
	}
}
