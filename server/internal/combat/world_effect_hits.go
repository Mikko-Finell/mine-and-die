package combat

import (
	"time"

	worldpkg "mine-and-die/server/internal/world"
)

// WorldEffectHitDispatcherConfig bundles the adapters required to wire the
// world effect hit wrapper into the shared combat dispatcher while preserving
// the legacy telemetry callbacks.
type WorldEffectHitDispatcherConfig struct {
	ExtractEffect func(effect any) (EffectRef, bool)
	ExtractActor  func(target any) (ActorRef, bool)

	HealthEpsilon           float64
	BaselinePlayerMaxHealth float64

	SetPlayerHealth         func(target ActorRef, next float64)
	SetNPCHealth            func(target ActorRef, next float64)
	ApplyGenericHealthDelta func(target ActorRef, delta float64) (changed bool, actualDelta float64, newHealth float64)

	RecordEffectHitTelemetry func(effect EffectRef, target ActorRef, actualDelta float64)
	RecordDamageTelemetry    func(effect EffectRef, target ActorRef, damage float64, targetHealth float64, statusEffect string)
	RecordDefeatTelemetry    func(effect EffectRef, target ActorRef, statusEffect string)

	DropAllInventory  func(target ActorRef, reason string)
	ApplyStatusEffect func(effect EffectRef, target ActorRef, statusEffect string, now time.Time)
}

// NewWorldEffectHitDispatcher constructs a world-scoped effect hit dispatcher
// that reuses the shared combat logic while guarding against nil effect and
// target references.
func NewWorldEffectHitDispatcher(cfg WorldEffectHitDispatcherConfig) EffectHitCallback {
	dispatcher := NewEffectHitDispatcher(EffectHitDispatcherConfig{
		ExtractEffect:            cfg.ExtractEffect,
		ExtractActor:             cfg.ExtractActor,
		HealthEpsilon:            cfg.HealthEpsilon,
		BaselinePlayerMaxHealth:  cfg.BaselinePlayerMaxHealth,
		SetPlayerHealth:          cfg.SetPlayerHealth,
		SetNPCHealth:             cfg.SetNPCHealth,
		ApplyGenericHealthDelta:  cfg.ApplyGenericHealthDelta,
		RecordEffectHitTelemetry: cfg.RecordEffectHitTelemetry,
		RecordDamageTelemetry:    cfg.RecordDamageTelemetry,
		RecordDefeatTelemetry:    cfg.RecordDefeatTelemetry,
		DropAllInventory:         cfg.DropAllInventory,
		ApplyStatusEffect:        cfg.ApplyStatusEffect,
	})
	if dispatcher == nil {
		return nil
	}
	return func(effect any, target any, now time.Time) {
		if effect == nil || target == nil {
			return
		}
		dispatcher(effect, target, now)
	}
}

// ApplyEffectHit invokes the provided effect hit callback after guarding
// against nil adapters or targets, mirroring the legacy world wrapper.
func ApplyEffectHit(callback EffectHitCallback, effect any, target any, now time.Time) {
	if callback == nil || effect == nil || target == nil {
		return
	}
	callback(effect, target, now)
}

// WorldPlayerEffectHitCallbackConfig bundles the dependencies required to
// reproduce the legacy player hit wiring while delegating combat staging to the
// shared dispatcher.
type WorldPlayerEffectHitCallbackConfig struct {
	Dispatcher EffectHitCallback
}

// NewWorldPlayerEffectHitCallback constructs a player hit callback that guards
// nil inputs through the shared dispatcher while preserving the existing world
// adapter contract.
func NewWorldPlayerEffectHitCallback(cfg WorldPlayerEffectHitCallbackConfig) worldpkg.EffectHitCallback {
	if cfg.Dispatcher == nil {
		return nil
	}

	applyActorHit := func(effect any, target any, now time.Time) {
		ApplyEffectHit(cfg.Dispatcher, effect, target, now)
	}

	return worldpkg.EffectHitPlayerCallback(worldpkg.EffectHitPlayerConfig{
		ApplyActorHit: worldpkg.EffectHitCallback(applyActorHit),
	})
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
func NewWorldNPCEffectHitCallback(cfg WorldNPCEffectHitCallbackConfig) worldpkg.EffectHitCallback {
	if cfg.Dispatcher == nil {
		return nil
	}

	applyActorHit := func(effect any, target any, now time.Time) {
		ApplyEffectHit(cfg.Dispatcher, effect, target, now)
	}

	npcCfg := worldpkg.EffectHitNPCConfig{
		ApplyActorHit: worldpkg.EffectHitCallback(applyActorHit),
		SpawnBlood:    worldpkg.EffectHitCallback(cfg.SpawnBlood),
		IsAlive:       cfg.IsAlive,
		HandleDefeat:  cfg.HandleDefeat,
	}

	return worldpkg.EffectHitNPCCallback(npcCfg)
}

// WorldBurningDamageCallbackConfig bundles the adapters required to route world
// burning damage through the shared combat dispatcher while preserving the
// legacy effect construction and telemetry hooks.
type WorldBurningDamageCallbackConfig struct {
	Dispatcher EffectHitCallback
	Target     any
	Now        time.Time

	BuildEffect func(effect worldpkg.BurningDamageEffect) any
	AfterApply  func(effect any)
}

// NewWorldBurningDamageCallback constructs a callback compatible with the
// world burning damage helper that converts the normalized effect payload into
// the legacy effect state, applies the hit through the dispatcher, and invokes
// the optional telemetry hook.
func NewWorldBurningDamageCallback(cfg WorldBurningDamageCallbackConfig) func(worldpkg.BurningDamageEffect) {
	if cfg.BuildEffect == nil {
		return nil
	}

	return func(effect worldpkg.BurningDamageEffect) {
		built := cfg.BuildEffect(effect)
		if built == nil {
			return
		}

		ApplyEffectHit(cfg.Dispatcher, built, cfg.Target, cfg.Now)

		if cfg.AfterApply != nil {
			cfg.AfterApply(built)
		}
	}
}
