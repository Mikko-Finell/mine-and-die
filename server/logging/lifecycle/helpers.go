package lifecycle

import (
	"context"

	"mine-and-die/server/logging"
)

const (
	// EventPlayerJoined is emitted when a player joins the world.
	EventPlayerJoined logging.EventType = "lifecycle.player_joined"
	// EventPlayerDisconnected is emitted when a player leaves the world.
	EventPlayerDisconnected logging.EventType = "lifecycle.player_disconnected"
)

// PlayerJoinedPayload captures spawn metadata for a new player.
type PlayerJoinedPayload struct {
	SpawnX float64 `json:"spawnX"`
	SpawnY float64 `json:"spawnY"`
}

// PlayerDisconnectedPayload captures the reason a player left.
type PlayerDisconnectedPayload struct {
	Reason string `json:"reason"`
}

// PlayerJoined publishes a player join event.
func PlayerJoined(ctx context.Context, pub logging.Publisher, tick uint64, actor logging.EntityRef, payload PlayerJoinedPayload, extra map[string]any) {
	if pub == nil {
		return
	}
	event := logging.Event{
		Type:     EventPlayerJoined,
		Tick:     tick,
		Actor:    actor,
		Severity: logging.SeverityInfo,
		Category: "lifecycle",
		Payload:  payload,
		Extra:    extra,
	}
	pub.Publish(ctx, event)
}

// PlayerDisconnected publishes a player disconnect event.
func PlayerDisconnected(ctx context.Context, pub logging.Publisher, tick uint64, actor logging.EntityRef, payload PlayerDisconnectedPayload, extra map[string]any) {
	if pub == nil {
		return
	}
	event := logging.Event{
		Type:     EventPlayerDisconnected,
		Tick:     tick,
		Actor:    actor,
		Severity: logging.SeverityInfo,
		Category: "lifecycle",
		Payload:  payload,
		Extra:    extra,
	}
	pub.Publish(ctx, event)
}
