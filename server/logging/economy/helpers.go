package economy

import (
	"context"

	"mine-and-die/server/logging"
)

const (
	// EventItemGrantFailed is emitted when the server fails to add an item to an inventory.
	EventItemGrantFailed logging.EventType = "economy.item_grant_failed"
)

// ItemGrantFailedPayload describes the attempted item grant.
type ItemGrantFailedPayload struct {
	ItemType string `json:"itemType"`
	Quantity int    `json:"quantity,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// ItemGrantFailed publishes an event for a failed inventory grant.
func ItemGrantFailed(ctx context.Context, pub logging.Publisher, tick uint64, actor logging.EntityRef, payload ItemGrantFailedPayload, extra map[string]any) {
	if pub == nil {
		return
	}
	event := logging.Event{
		Type:     EventItemGrantFailed,
		Tick:     tick,
		Actor:    actor,
		Severity: logging.SeverityWarn,
		Category: "economy",
		Payload:  payload,
		Extra:    extra,
	}
	pub.Publish(ctx, event)
}
