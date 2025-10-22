package combat

import (
	"context"

	"mine-and-die/server/logging"
	loggingcombat "mine-and-die/server/logging/combat"
)

// DamageTelemetryRecorderConfig captures the dependencies required to publish
// combat damage telemetry events from within the shared combat package.
type DamageTelemetryRecorderConfig struct {
	Publisher    logging.Publisher
	LookupEntity func(id string) logging.EntityRef
	CurrentTick  func() uint64
}

// NewDamageTelemetryRecorder constructs a dispatcher hook that emits combat
// damage telemetry using the provided publisher and entity lookup.
func NewDamageTelemetryRecorder(cfg DamageTelemetryRecorderConfig) func(effect EffectRef, target ActorRef, damage float64, targetHealth float64, statusEffect string) {
	if cfg.Publisher == nil {
		return nil
	}

	lookup := cfg.LookupEntity
	if lookup == nil {
		lookup = func(string) logging.EntityRef { return logging.EntityRef{} }
	}

	tick := cfg.CurrentTick
	if tick == nil {
		tick = func() uint64 { return 0 }
	}

	return func(effect EffectRef, target ActorRef, damage float64, targetHealth float64, statusEffect string) {
		ownerRef := logging.EntityRef{}
		if effect.Effect.OwnerID != "" {
			ownerRef = lookup(effect.Effect.OwnerID)
		}

		targetRef := logging.EntityRef{}
		if target.Actor.ID != "" {
			targetRef = lookup(target.Actor.ID)
		}

		payload := loggingcombat.DamagePayload{
			Ability:      effect.Effect.Type,
			Amount:       damage,
			TargetHealth: targetHealth,
			StatusEffect: statusEffect,
		}

		loggingcombat.Damage(
			context.Background(),
			cfg.Publisher,
			tick(),
			ownerRef,
			targetRef,
			payload,
			nil,
		)
	}
}

// DefeatTelemetryRecorderConfig captures the dependencies required to publish
// combat defeat telemetry events from within the shared combat package.
type DefeatTelemetryRecorderConfig struct {
	Publisher    logging.Publisher
	LookupEntity func(id string) logging.EntityRef
	CurrentTick  func() uint64
}

// NewDefeatTelemetryRecorder constructs a dispatcher hook that emits combat
// defeat telemetry using the provided publisher and entity lookup.
func NewDefeatTelemetryRecorder(cfg DefeatTelemetryRecorderConfig) func(effect EffectRef, target ActorRef, statusEffect string) {
	if cfg.Publisher == nil {
		return nil
	}

	lookup := cfg.LookupEntity
	if lookup == nil {
		lookup = func(string) logging.EntityRef { return logging.EntityRef{} }
	}

	tick := cfg.CurrentTick
	if tick == nil {
		tick = func() uint64 { return 0 }
	}

	return func(effect EffectRef, target ActorRef, statusEffect string) {
		ownerRef := logging.EntityRef{}
		if effect.Effect.OwnerID != "" {
			ownerRef = lookup(effect.Effect.OwnerID)
		}

		targetRef := logging.EntityRef{}
		if target.Actor.ID != "" {
			targetRef = lookup(target.Actor.ID)
		}

		payload := loggingcombat.DefeatPayload{
			Ability:      effect.Effect.Type,
			StatusEffect: statusEffect,
		}

		loggingcombat.Defeat(
			context.Background(),
			cfg.Publisher,
			tick(),
			ownerRef,
			targetRef,
			payload,
			nil,
		)
	}
}

// AttackOverlapTelemetryRecorderConfig captures the dependencies required to
// publish combat attack-overlap telemetry events from within the shared combat
// package.
type AttackOverlapTelemetryRecorderConfig struct {
	Publisher    logging.Publisher
	LookupEntity func(id string) logging.EntityRef
}

// NewAttackOverlapTelemetryRecorder constructs a telemetry helper that emits
// combat attack-overlap events using the provided publisher and entity lookup.
func NewAttackOverlapTelemetryRecorder(cfg AttackOverlapTelemetryRecorderConfig) func(ownerID string, tick uint64, ability string, playerHits []string, npcHits []string, metadata map[string]any) {
	if cfg.Publisher == nil {
		return nil
	}

	lookup := cfg.LookupEntity
	if lookup == nil {
		lookup = func(string) logging.EntityRef { return logging.EntityRef{} }
	}

	return func(ownerID string, tick uint64, ability string, playerHits []string, npcHits []string, metadata map[string]any) {
		if len(playerHits) == 0 && len(npcHits) == 0 {
			return
		}

		ownerRef := logging.EntityRef{}
		if ownerID != "" {
			ownerRef = lookup(ownerID)
		}

		totalTargets := len(playerHits) + len(npcHits)
		targets := make([]logging.EntityRef, 0, totalTargets)

		for _, id := range playerHits {
			if id == "" {
				continue
			}
			targets = append(targets, lookup(id))
		}

		for _, id := range npcHits {
			if id == "" {
				continue
			}
			targets = append(targets, lookup(id))
		}

		payload := loggingcombat.AttackOverlapPayload{Ability: ability}
		if len(playerHits) > 0 {
			payload.PlayerHits = append(payload.PlayerHits, playerHits...)
		}
		if len(npcHits) > 0 {
			payload.NPCHits = append(payload.NPCHits, npcHits...)
		}

		loggingcombat.AttackOverlap(
			context.Background(),
			cfg.Publisher,
			tick,
			ownerRef,
			targets,
			payload,
			metadata,
		)
	}
}
