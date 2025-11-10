package world

import (
	"time"

	statuspkg "mine-and-die/server/internal/world/status"
)

// WorldPlayerEffectHitCallbackConfig bundles the dependencies required to
// reproduce the legacy player hit wiring while delegating combat staging to the
// shared dispatcher.
type WorldPlayerEffectHitCallbackConfig struct {
	Dispatcher EffectHitCallback
}

// NewWorldPlayerEffectHitCallback constructs a player hit callback that guards
// nil inputs through the shared dispatcher while preserving the existing world
// adapter contract.
func NewWorldPlayerEffectHitCallback(cfg WorldPlayerEffectHitCallbackConfig) EffectHitCallback {
	if cfg.Dispatcher == nil {
		return nil
	}

	applyActorHit := func(effect any, target any, now time.Time) {
		cfg.Dispatcher(effect, target, now)
	}

	return EffectHitPlayerCallback(EffectHitPlayerConfig{ApplyActorHit: applyActorHit})
}

// WorldNPCEffectHitCallbackConfig bundles the dependencies required to
// reproduce the legacy NPC hit wiring while delegating combat staging to the
// shared dispatcher.
type WorldNPCEffectHitCallbackConfig struct {
	Dispatcher   EffectHitCallback
	SpawnBlood   func(effect any, target any, now time.Time)
	IsAlive      func(target any) bool
	HandleDefeat func(target any)
}

// NewWorldNPCEffectHitCallback constructs an NPC hit callback that mirrors the
// legacy flow — spawning blood visuals, applying damage via the shared
// dispatcher, and invoking defeat handlers when actors transition from alive to
// defeated.
func NewWorldNPCEffectHitCallback(cfg WorldNPCEffectHitCallbackConfig) EffectHitCallback {
	if cfg.Dispatcher == nil {
		return nil
	}

	applyActorHit := func(effect any, target any, now time.Time) {
		cfg.Dispatcher(effect, target, now)
	}

	npcCfg := EffectHitNPCConfig{
		ApplyActorHit: applyActorHit,
		SpawnBlood:    cfg.SpawnBlood,
		IsAlive:       cfg.IsAlive,
		HandleDefeat:  cfg.HandleDefeat,
	}

	return EffectHitNPCCallback(npcCfg)
}

// WorldBurningDamageCallbackConfig bundles the adapters required to route world
// burning damage through the shared combat dispatcher while preserving the
// legacy effect construction and telemetry hooks.
type WorldBurningDamageCallbackConfig struct {
	Dispatcher EffectHitCallback
	Target     any
	Now        time.Time

	BuildEffect func(effect statuspkg.BurningDamageEffect) any
	AfterApply  func(effect any)
}

// NewWorldBurningDamageCallback constructs a callback compatible with the
// world burning damage helper that converts the normalized effect payload into
// the legacy effect state, applies the hit through the dispatcher, and invokes
// the optional telemetry hook.
func NewWorldBurningDamageCallback(cfg WorldBurningDamageCallbackConfig) func(statuspkg.BurningDamageEffect) {
	if cfg.BuildEffect == nil {
		return nil
	}

	return func(effect statuspkg.BurningDamageEffect) {
		built := cfg.BuildEffect(effect)
		if built == nil {
			return
		}

		if cfg.Dispatcher != nil {
			cfg.Dispatcher(built, cfg.Target, cfg.Now)
		}

		if cfg.AfterApply != nil {
			cfg.AfterApply(built)
		}
	}
}
