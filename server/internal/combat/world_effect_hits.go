package combat

import (
	"time"

	internaleffects "mine-and-die/server/internal/effects"
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

// WorldActorAdapter captures the metadata required to adapt legacy world actor
// references into combat actor refs while preserving access to the original
// struct for mutation helpers.
type WorldActorAdapter struct {
	ID        string
	Health    float64
	MaxHealth float64
	KindHint  ActorKind
	Raw       any
}

// LegacyWorldEffectHitAdapterConfig bundles the dependencies required to wire
// the legacy world effect hit callbacks into the shared combat dispatcher while
// keeping the server package's wrappers thin.
type LegacyWorldEffectHitAdapterConfig struct {
	HealthEpsilon           float64
	BaselinePlayerMaxHealth float64

	ExtractEffect func(effect any) (*internaleffects.State, bool)
	ExtractActor  func(target any) (WorldActorAdapter, bool)
	IsPlayer      func(id string) bool
	IsNPC         func(id string) bool

	SetPlayerHealth         func(id string, next float64)
	SetNPCHealth            func(id string, next float64)
	ApplyGenericHealthDelta func(actor WorldActorAdapter, delta float64) (changed bool, actualDelta float64, newHealth float64)

	RecordEffectHitTelemetry func(effect *internaleffects.State, targetID string, actualDelta float64)
	RecordDamageTelemetry    func(effect EffectRef, target ActorRef, damage float64, targetHealth float64, statusEffect string)
	RecordDefeatTelemetry    func(effect EffectRef, target ActorRef, statusEffect string)

	DropAllInventory  func(actor WorldActorAdapter, reason string)
	ApplyStatusEffect func(effect *internaleffects.State, actor WorldActorAdapter, statusEffect string, now time.Time)
}

// NewLegacyWorldEffectHitAdapter constructs the world-scoped dispatcher using
// the provided legacy adapters, returning a callback compatible with the
// existing world wrappers.
func NewLegacyWorldEffectHitAdapter(cfg LegacyWorldEffectHitAdapterConfig) EffectHitCallback {
	dispatcherCfg := WorldEffectHitDispatcherConfig{
		HealthEpsilon:           cfg.HealthEpsilon,
		BaselinePlayerMaxHealth: cfg.BaselinePlayerMaxHealth,
		ExtractEffect: func(effect any) (EffectRef, bool) {
			if cfg.ExtractEffect == nil {
				return EffectRef{}, false
			}
			state, ok := cfg.ExtractEffect(effect)
			if !ok || state == nil {
				return EffectRef{}, false
			}
			status := ""
			if state.StatusEffect != "" {
				status = string(state.StatusEffect)
			}
			return EffectRef{
				Effect: Effect{
					Type:         state.Type,
					OwnerID:      state.Owner,
					Params:       state.Params,
					StatusEffect: status,
				},
				Raw: state,
			}, true
		},
		ExtractActor: func(target any) (ActorRef, bool) {
			if cfg.ExtractActor == nil {
				return ActorRef{}, false
			}
			adapter, ok := cfg.ExtractActor(target)
			if !ok || adapter.ID == "" {
				return ActorRef{}, false
			}

			kind := adapter.KindHint
			if kind == ActorKindGeneric {
				if cfg.IsPlayer != nil && cfg.IsPlayer(adapter.ID) {
					kind = ActorKindPlayer
				} else if cfg.IsNPC != nil && cfg.IsNPC(adapter.ID) {
					kind = ActorKindNPC
				}
			}

			return ActorRef{
				Actor: Actor{
					ID:        adapter.ID,
					Health:    adapter.Health,
					MaxHealth: adapter.MaxHealth,
					Kind:      kind,
				},
				Raw: adapter,
			}, true
		},
		SetPlayerHealth: func(target ActorRef, next float64) {
			if cfg.SetPlayerHealth == nil || target.Actor.ID == "" {
				return
			}
			cfg.SetPlayerHealth(target.Actor.ID, next)
		},
		SetNPCHealth: func(target ActorRef, next float64) {
			if cfg.SetNPCHealth == nil || target.Actor.ID == "" {
				return
			}
			cfg.SetNPCHealth(target.Actor.ID, next)
		},
		ApplyGenericHealthDelta: func(target ActorRef, delta float64) (bool, float64, float64) {
			if cfg.ApplyGenericHealthDelta == nil {
				return false, 0, target.Actor.Health
			}
			adapter, _ := target.Raw.(WorldActorAdapter)
			return cfg.ApplyGenericHealthDelta(adapter, delta)
		},
		RecordEffectHitTelemetry: func(effect EffectRef, target ActorRef, actualDelta float64) {
			if cfg.RecordEffectHitTelemetry == nil || target.Actor.ID == "" {
				return
			}
			state, _ := effect.Raw.(*internaleffects.State)
			if state == nil {
				return
			}
			cfg.RecordEffectHitTelemetry(state, target.Actor.ID, actualDelta)
		},
		RecordDamageTelemetry: func(effect EffectRef, target ActorRef, damage float64, targetHealth float64, statusEffect string) {
			if cfg.RecordDamageTelemetry == nil || damage <= 0 || target.Actor.ID == "" {
				return
			}
			cfg.RecordDamageTelemetry(effect, target, damage, targetHealth, statusEffect)
		},
		RecordDefeatTelemetry: func(effect EffectRef, target ActorRef, statusEffect string) {
			if cfg.RecordDefeatTelemetry == nil || target.Actor.ID == "" {
				return
			}
			cfg.RecordDefeatTelemetry(effect, target, statusEffect)
		},
		DropAllInventory: func(target ActorRef, reason string) {
			if cfg.DropAllInventory == nil {
				return
			}
			adapter, _ := target.Raw.(WorldActorAdapter)
			cfg.DropAllInventory(adapter, reason)
		},
		ApplyStatusEffect: func(effect EffectRef, target ActorRef, statusEffect string, now time.Time) {
			if cfg.ApplyStatusEffect == nil || statusEffect == "" {
				return
			}
			adapter, _ := target.Raw.(WorldActorAdapter)
			state, _ := effect.Raw.(*internaleffects.State)
			if state == nil {
				return
			}
			cfg.ApplyStatusEffect(state, adapter, statusEffect, now)
		},
	}

	return NewWorldEffectHitDispatcher(dispatcherCfg)
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

// World-specific callback helpers live in the world package to avoid import cycles.
