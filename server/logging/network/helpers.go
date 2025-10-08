package network

import (
	"context"

	"mine-and-die/server/logging"
)

const (
	// EventAckAdvanced is emitted when a client acknowledges a newer tick.
	EventAckAdvanced logging.EventType = "network.ack_advanced"
	// EventAckRegression is emitted when a client reports an older acknowledgement than previously recorded.
	EventAckRegression logging.EventType = "network.ack_regression"
)

// AckPayload captures acknowledgement progression details.
type AckPayload struct {
	Previous uint64 `json:"previous"`
	Ack      uint64 `json:"ack"`
}

// AckAdvanced publishes a debug event when a client acknowledgement advances.
func AckAdvanced(ctx context.Context, pub logging.Publisher, tick uint64, actor logging.EntityRef, payload AckPayload, extra map[string]any) {
	if pub == nil {
		return
	}
	event := logging.Event{
		Type:     EventAckAdvanced,
		Tick:     tick,
		Actor:    actor,
		Severity: logging.SeverityDebug,
		Category: "network",
		Payload:  payload,
		Extra:    extra,
	}
	pub.Publish(ctx, event)
}

// AckRegression publishes a warning event when a client acknowledgement regresses.
func AckRegression(ctx context.Context, pub logging.Publisher, tick uint64, actor logging.EntityRef, payload AckPayload, extra map[string]any) {
	if pub == nil {
		return
	}
	event := logging.Event{
		Type:     EventAckRegression,
		Tick:     tick,
		Actor:    actor,
		Severity: logging.SeverityWarn,
		Category: "network",
		Payload:  payload,
		Extra:    extra,
	}
	pub.Publish(ctx, event)
}
