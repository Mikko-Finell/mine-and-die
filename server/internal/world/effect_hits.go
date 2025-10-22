package world

import "time"

// EffectHitCallback applies an effect's hit to a target actor. The effect and
// target parameters remain opaque so callers can adapt their legacy structs
// without exposing them to the world package.
type EffectHitCallback func(effect any, target any, now time.Time)

// EffectHitPlayerConfig bundles the dependencies required to apply a hit to a
// player through a shared callback.
type EffectHitPlayerConfig struct {
	// ApplyActorHit mutates the target actor with the effect's hit logic.
	ApplyActorHit EffectHitCallback
}

// EffectHitPlayerCallback constructs a callback that applies effect hits to
// players while guarding against nil targets and missing adapters.
func EffectHitPlayerCallback(cfg EffectHitPlayerConfig) EffectHitCallback {
	if cfg.ApplyActorHit == nil {
		return nil
	}

	return func(effect any, target any, now time.Time) {
		if target == nil {
			return
		}
		cfg.ApplyActorHit(effect, target, now)
	}
}

// EffectHitNPCConfig bundles the dependencies required to apply a hit to an NPC
// while reproducing the legacy side effects that accompany damage resolution.
type EffectHitNPCConfig struct {
	// ApplyActorHit mutates the target actor with the effect's hit logic.
	ApplyActorHit EffectHitCallback
	// SpawnBlood emits any contract-managed visuals associated with the hit.
	SpawnBlood EffectHitCallback
	// IsAlive reports whether the NPC is currently alive.
	IsAlive func(target any) bool
	// HandleDefeat cleans up NPC state after the actor is defeated.
	HandleDefeat func(target any)
}

// EffectHitNPCCallback constructs a callback that mirrors the legacy NPC hit
// flow, spawning blood visuals before applying damage and invoking the defeat
// handler when the actor transitions from alive to defeated.
func EffectHitNPCCallback(cfg EffectHitNPCConfig) EffectHitCallback {
	if cfg.ApplyActorHit == nil {
		return nil
	}

	return func(effect any, target any, now time.Time) {
		if target == nil {
			return
		}

		if cfg.SpawnBlood != nil {
			cfg.SpawnBlood(effect, target, now)
		}

		wasAlive := false
		if cfg.IsAlive != nil {
			wasAlive = cfg.IsAlive(target)
		}

		cfg.ApplyActorHit(effect, target, now)

		if !wasAlive || cfg.HandleDefeat == nil || cfg.IsAlive == nil {
			return
		}

		if cfg.IsAlive(target) {
			return
		}

		cfg.HandleDefeat(target)
	}
}
