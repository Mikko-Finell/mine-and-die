package combat

import (
	"context"

	"mine-and-die/server/logging"
)

const (
	// EventAttackOverlap is emitted when an attack overlaps multiple actors.
	EventAttackOverlap logging.EventType = "combat.attack_overlap"
	// EventDamage is emitted when an ability deals damage to a target.
	EventDamage logging.EventType = "combat.damage"
	// EventDefeat is emitted when an actor is defeated.
	EventDefeat logging.EventType = "combat.defeat"
)

// AttackOverlapPayload captures the targets affected by an overlapping attack.
type AttackOverlapPayload struct {
	Ability    string   `json:"ability"`
	PlayerHits []string `json:"playerHits,omitempty"`
	NPCHits    []string `json:"npcHits,omitempty"`
}

// DamagePayload captures the amount dealt to a single target.
type DamagePayload struct {
	Ability      string  `json:"ability,omitempty"`
	Amount       float64 `json:"amount"`
	TargetHealth float64 `json:"targetHealth"`
	StatusEffect string  `json:"statusEffect,omitempty"`
}

// DefeatPayload describes the context for a fatal blow.
type DefeatPayload struct {
	Ability      string `json:"ability,omitempty"`
	StatusEffect string `json:"statusEffect,omitempty"`
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

// Damage publishes a combat damage event for a single target.
func Damage(ctx context.Context, pub logging.Publisher, tick uint64, actor logging.EntityRef, target logging.EntityRef, payload DamagePayload, extra map[string]any) {
	if pub == nil {
		return
	}
	event := logging.Event{
		Type:     EventDamage,
		Tick:     tick,
		Actor:    actor,
		Targets:  []logging.EntityRef{target},
		Severity: logging.SeverityInfo,
		Category: "combat",
		Payload:  payload,
		Extra:    extra,
	}
	pub.Publish(ctx, event)
}

// Defeat publishes a combat defeat event for the eliminated actor.
func Defeat(ctx context.Context, pub logging.Publisher, tick uint64, actor logging.EntityRef, target logging.EntityRef, payload DefeatPayload, extra map[string]any) {
	if pub == nil {
		return
	}
	event := logging.Event{
		Type:     EventDefeat,
		Tick:     tick,
		Actor:    actor,
		Targets:  []logging.EntityRef{target},
		Severity: logging.SeverityInfo,
		Category: "combat",
		Payload:  payload,
		Extra:    extra,
	}
	pub.Publish(ctx, event)
}
