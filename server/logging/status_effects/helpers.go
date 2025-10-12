package status_effects

import (
	"context"

	"mine-and-die/server/logging"
)

const (
	// EventApplied is emitted when a status effect is applied to an actor.
	EventApplied logging.EventType = "status_effects.applied"
)

// AppliedPayload captures details about a status effect application.
type AppliedPayload struct {
	StatusEffect string `json:"statusEffect"`
	SourceID     string `json:"sourceId,omitempty"`
	DurationMs   int64  `json:"durationMs,omitempty"`
}

// Applied publishes a status effect application event.
func Applied(ctx context.Context, pub logging.Publisher, tick uint64, actor logging.EntityRef, target logging.EntityRef, payload AppliedPayload, extra map[string]any) {
	if pub == nil {
		return
	}
	event := logging.Event{
		Type:     EventApplied,
		Tick:     tick,
		Actor:    actor,
		Targets:  []logging.EntityRef{target},
		Severity: logging.SeverityInfo,
		Category: "status_effects",
		Payload:  payload,
		Extra:    extra,
	}
	pub.Publish(ctx, event)
}
