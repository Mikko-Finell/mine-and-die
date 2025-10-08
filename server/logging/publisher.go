package logging

import (
	"context"
	"time"
)

// EventType provides a namespaced identifier for simulation telemetry.
type EventType string

// Severity expresses the importance of a telemetry event.
type Severity int

const (
	// SeverityDebug is verbose information for diagnostics.
	SeverityDebug Severity = iota
	// SeverityInfo is routine operational telemetry.
	SeverityInfo
	// SeverityWarn indicates a recoverable anomaly.
	SeverityWarn
	// SeverityError indicates a failure that likely needs attention.
	SeverityError
)

// Category groups events by subsystem for filtering.
type Category string

// Event describes a semantic occurrence within the simulation loop.
type Event struct {
	Type      EventType
	Tick      uint64
	Time      time.Time
	Actor     EntityRef
	Targets   []EntityRef
	Severity  Severity
	Category  Category
	Payload   any
	Extra     map[string]any
	TraceID   string
	CommandID string
}

// EntityKind differentiates actors within the simulation.
type EntityKind string

// EntityRef identifies actors involved in an event.
type EntityRef struct {
	ID   string
	Kind EntityKind
}

// Publisher emits telemetry events without blocking the simulation loop.
type Publisher interface {
	Publish(ctx context.Context, event Event)
}

// NopPublisher is a Publisher that drops all events.
type NopPublisher struct{}

// Publish implements Publisher.
func (NopPublisher) Publish(context.Context, Event) {}

// WithFields attaches static metadata to every event emitted by the Publisher.
func WithFields(base Publisher, fields map[string]any) Publisher {
	if base == nil {
		return NopPublisher{}
	}
	copied := make(map[string]any, len(fields))
	for k, v := range fields {
		copied[k] = v
	}
	return &fieldsPublisher{base: base, fields: copied}
}

type fieldsPublisher struct {
	base   Publisher
	fields map[string]any
}

func (p *fieldsPublisher) Publish(ctx context.Context, event Event) {
	if len(p.fields) > 0 {
		if event.Extra == nil {
			event.Extra = make(map[string]any, len(p.fields))
		}
		for k, v := range p.fields {
			if _, exists := event.Extra[k]; !exists {
				event.Extra[k] = v
			}
		}
	}
	p.base.Publish(ctx, event)
}
