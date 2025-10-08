package combat

import (
	"context"

	"mine-and-die/server/logging"
)

const (
	// EventAttackOverlap is emitted when an attack overlaps multiple actors.
	EventAttackOverlap logging.EventType = "combat.attack_overlap"
)

// AttackOverlapPayload captures the targets affected by an overlapping attack.
type AttackOverlapPayload struct {
	Ability    string   `json:"ability"`
	PlayerHits []string `json:"playerHits,omitempty"`
	NPCHits    []string `json:"npcHits,omitempty"`
}

// AttackOverlap publishes a combat overlap event.
func AttackOverlap(ctx context.Context, pub logging.Publisher, tick uint64, actor logging.EntityRef, targets []logging.EntityRef, payload AttackOverlapPayload, extra map[string]any) {
	if pub == nil {
		return
	}
	event := logging.Event{
		Type:     EventAttackOverlap,
		Tick:     tick,
		Actor:    actor,
		Targets:  targets,
		Severity: logging.SeverityInfo,
		Category: "combat",
		Payload:  payload,
		Extra:    extra,
	}
	pub.Publish(ctx, event)
}
