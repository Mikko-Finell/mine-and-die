package economy

import (
	"context"

	"mine-and-die/server/logging"
)

const (
        // EventItemGrantFailed is emitted when the server fails to add an item to an inventory.
        EventItemGrantFailed logging.EventType = "economy.item_grant_failed"
        EventGoldDropped     logging.EventType = "economy.gold_dropped"
        EventGoldPickedUp    logging.EventType = "economy.gold_picked_up"
        EventGoldPickupFail  logging.EventType = "economy.gold_pickup_failed"
)

// ItemGrantFailedPayload describes the attempted item grant.
type ItemGrantFailedPayload struct {
        ItemType string `json:"itemType"`
        Quantity int    `json:"quantity,omitempty"`
        Reason   string `json:"reason,omitempty"`
}

// GoldDroppedPayload describes a gold drop event.
type GoldDroppedPayload struct {
        Quantity int    `json:"quantity"`
        Reason   string `json:"reason"`
}

// GoldPickedUpPayload describes a successful gold pickup.
type GoldPickedUpPayload struct {
        Quantity int `json:"quantity"`
}

// GoldPickupFailedPayload captures failed pickup attempts.
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

// GoldDropped logs a gold drop event with the provided reason.
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

// GoldPickedUp logs a successful pickup of ground gold.
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

// GoldPickupFailed logs a failed attempt to pick up ground gold.
func GoldPickupFailed(ctx context.Context, pub logging.Publisher, tick uint64, actor logging.EntityRef, payload GoldPickupFailedPayload, extra map[string]any) {
        if pub == nil {
                return
        }
        event := logging.Event{
                Type:     EventGoldPickupFail,
                Tick:     tick,
                Actor:    actor,
                Severity: logging.SeverityWarn,
                Category: "economy",
                Payload:  payload,
                Extra:    extra,
        }
        pub.Publish(ctx, event)
}
