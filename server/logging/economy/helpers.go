package economy

import (
	"context"

	"mine-and-die/server/logging"
)

const (
	// EventItemGrantFailed is emitted when the server fails to add an item to an inventory.
	EventItemGrantFailed logging.EventType = "economy.item_grant_failed"
	// EventGoldDropped is emitted whenever gold is dropped on the ground.
	EventGoldDropped logging.EventType = "economy.gold_dropped"
	// EventGoldPickedUp is emitted whenever ground gold is picked up.
	EventGoldPickedUp logging.EventType = "economy.gold_picked_up"
	// EventGoldPickupFailed is emitted when a pickup attempt fails.
	EventGoldPickupFailed logging.EventType = "economy.gold_pickup_failed"
)

// ItemGrantFailedPayload describes the attempted item grant.
type ItemGrantFailedPayload struct {
	ItemType string `json:"itemType"`
	Quantity int    `json:"quantity,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// GoldDroppedPayload describes a gold drop action.
type GoldDroppedPayload struct {
	Quantity int    `json:"quantity"`
	Reason   string `json:"reason"`
}

// GoldPickedUpPayload describes a successful pickup.
type GoldPickedUpPayload struct {
	Quantity int `json:"quantity"`
}

// GoldPickupFailedPayload describes why a pickup failed.
type GoldPickupFailedPayload struct {
	Reason string `json:"reason"`
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

// GoldDropped publishes a gold drop event.
func GoldDropped(ctx context.Context, pub logging.Publisher, tick uint64, actor logging.EntityRef, payload GoldDroppedPayload, extra map[string]any) {
	if pub == nil {
		return
	}
	event := logging.Event{
		Type:     EventGoldDropped,
		Tick:     tick,
		Actor:    actor,
		Severity: logging.SeverityInfo,
		Category: "economy",
		Payload:  payload,
		Extra:    extra,
	}
	pub.Publish(ctx, event)
}

// GoldPickedUp publishes a gold pickup event.
func GoldPickedUp(ctx context.Context, pub logging.Publisher, tick uint64, actor logging.EntityRef, payload GoldPickedUpPayload, extra map[string]any) {
	if pub == nil {
		return
	}
	event := logging.Event{
		Type:     EventGoldPickedUp,
		Tick:     tick,
		Actor:    actor,
		Severity: logging.SeverityInfo,
		Category: "economy",
		Payload:  payload,
		Extra:    extra,
	}
	pub.Publish(ctx, event)
}

// GoldPickupFailed publishes a failed gold pickup event.
func GoldPickupFailed(ctx context.Context, pub logging.Publisher, tick uint64, actor logging.EntityRef, payload GoldPickupFailedPayload, extra map[string]any) {
	if pub == nil {
		return
	}
	event := logging.Event{
		Type:     EventGoldPickupFailed,
		Tick:     tick,
		Actor:    actor,
		Severity: logging.SeverityWarn,
		Category: "economy",
		Payload:  payload,
		Extra:    extra,
	}
	pub.Publish(ctx, event)
}
