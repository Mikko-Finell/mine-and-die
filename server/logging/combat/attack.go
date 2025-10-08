package combat

import (
	"context"

	"mine-and-die/server/logging"
)

const AttackOverlapEventType logging.EventType = "combat.attack_overlap"

type AttackOverlapPayload struct {
	Ability       string              `json:"ability"`
	PlayerTargets []logging.EntityRef `json:"playerTargets,omitempty"`
	NPCTargets    []logging.EntityRef `json:"npcTargets,omitempty"`
}

func AttackOverlap(ctx context.Context, pub logging.Publisher, tick uint64, actor logging.EntityRef, ability string, players []logging.EntityRef, npcs []logging.EntityRef) {
	if pub == nil {
		return
	}
	targets := make([]logging.EntityRef, 0, len(players)+len(npcs))
	targets = append(targets, players...)
	targets = append(targets, npcs...)
	payload := AttackOverlapPayload{
		Ability:       ability,
		PlayerTargets: nil,
		NPCTargets:    nil,
	}
	if len(players) > 0 {
		payload.PlayerTargets = append([]logging.EntityRef(nil), players...)
	}
	if len(npcs) > 0 {
		payload.NPCTargets = append([]logging.EntityRef(nil), npcs...)
	}
	event := logging.Event{
		Type:     AttackOverlapEventType,
		Tick:     tick,
		Actor:    actor,
		Targets:  targets,
		Severity: logging.SeverityInfo,
		Category: logging.CategoryCombat,
		Payload:  payload,
	}
	pub.Publish(ctx, event)
}
