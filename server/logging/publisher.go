package logging

import (
	"context"
	"time"
)

type EventType string

type Severity int

const (
	SeverityDebug Severity = iota
	SeverityInfo
	SeverityWarn
	SeverityError
)

type EntityKind string

const (
	EntityKindUnknown EntityKind = "unknown"
	EntityKindPlayer  EntityKind = "player"
	EntityKindNPC     EntityKind = "npc"
	EntityKindEffect  EntityKind = "effect"
	EntityKindWorld   EntityKind = "world"
)

type Event struct {
	Type      EventType      `json:"type"`
	Tick      uint64         `json:"tick"`
	Time      time.Time      `json:"time"`
	Actor     EntityRef      `json:"actor"`
	Targets   []EntityRef    `json:"targets,omitempty"`
	Severity  Severity       `json:"severity"`
	Category  string         `json:"category,omitempty"`
	Payload   any            `json:"payload,omitempty"`
	Extra     map[string]any `json:"extra,omitempty"`
	TraceID   string         `json:"traceId,omitempty"`
	CommandID string         `json:"commandId,omitempty"`
}

type EntityRef struct {
	ID   string     `json:"id"`
	Kind EntityKind `json:"kind"`
}

const (
	CategoryGameplay = "gameplay"
	CategoryCombat   = "combat"
	CategorySystem   = "system"
)

type Publisher interface {
	Publish(ctx context.Context, event Event)
}

type PublisherFunc func(ctx context.Context, event Event)

func (f PublisherFunc) Publish(ctx context.Context, event Event) {
	if f == nil {
		return
	}
	f(ctx, event)
}

type nopPublisher struct{}

func (nopPublisher) Publish(context.Context, Event) {}

func NopPublisher() Publisher {
	return nopPublisher{}
}

type fieldPublisher struct {
	next   Publisher
	fields map[string]any
}

func (p *fieldPublisher) Publish(ctx context.Context, event Event) {
	if p.next == nil {
		return
	}
	if len(p.fields) > 0 {
		event = cloneForFields(event)
		if event.Extra == nil {
			event.Extra = make(map[string]any, len(p.fields))
		}
		for k, v := range p.fields {
			if _, exists := event.Extra[k]; !exists {
				event.Extra[k] = v
			}
		}
	}
	p.next.Publish(ctx, event)
}

func cloneForFields(event Event) Event {
	cloned := event
	if len(event.Targets) > 0 {
		cloned.Targets = append([]EntityRef(nil), event.Targets...)
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

func WithFields(p Publisher, fields map[string]any) Publisher {
	if p == nil {
		return NopPublisher()
	}
	if len(fields) == 0 {
		return p
	}
	copied := make(map[string]any, len(fields))
	for k, v := range fields {
		copied[k] = v
	}
	return &fieldPublisher{next: p, fields: copied}
}

func (e Event) WithExtra(key string, value any) Event {
	if e.Extra == nil {
		e.Extra = make(map[string]any, 1)
	}
	e.Extra[key] = value
	return e
}
