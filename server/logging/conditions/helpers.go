package conditions

import (
	"context"

	"mine-and-die/server/logging"
)

const (
	// EventApplied is emitted when a condition is applied to an actor.
	EventApplied logging.EventType = "conditions.applied"
)

// AppliedPayload captures details about a condition application.
type AppliedPayload struct {
	Condition  string `json:"condition"`
	SourceID   string `json:"sourceId,omitempty"`
	DurationMs int64  `json:"durationMs,omitempty"`
}

// Applied publishes a condition application event.
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
		Category: "conditions",
		Payload:  payload,
		Extra:    extra,
	}
	pub.Publish(ctx, event)
}
