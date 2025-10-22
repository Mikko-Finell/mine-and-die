package combat

import (
	"math"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

// EffectType identifiers mirror contract definition IDs used by combat effects.
const (
	EffectTypeAttack        = effectcontract.EffectIDAttack
	EffectTypeFireball      = effectcontract.EffectIDFireball
	EffectTypeBloodSplatter = effectcontract.EffectIDBloodSplatter
	EffectTypeBurningTick   = effectcontract.EffectIDBurningTick
	EffectTypeBurningVisual = effectcontract.EffectIDBurningVisual
)

// Status effect identifiers applied by combat behaviors.
const (
	StatusEffectBurning = "burning"
)

// EffectHitCallback applies an effect's hit to a target actor. The effect and
// target parameters remain opaque so callers can adapt their legacy structs
// without exposing them to the combat package.
type EffectHitCallback func(effect any, target any, now time.Time)

// ActorKind identifies the classification of the target actor for hit
// resolution.
type ActorKind int

const (
	// ActorKindUnknown represents an unclassified actor.
	ActorKindUnknown ActorKind = iota
	// ActorKindPlayer identifies a player-controlled actor.
	ActorKindPlayer
	// ActorKindNPC identifies a non-player actor.
	ActorKindNPC
	// ActorKindGeneric identifies an actor with no dedicated setter
	// helpers. The dispatcher will rely on the generic delta adapter.
	ActorKindGeneric
)

// Effect captures the minimal effect metadata required during hit resolution.
type Effect struct {
	Type         string
	OwnerID      string
	Params       map[string]float64
	StatusEffect string
}

// EffectRef wraps the opaque effect reference passed to the dispatcher with the
// extracted metadata required for hit resolution.
type EffectRef struct {
	Effect Effect
	Raw    any
}

// Actor captures the actor metadata required during hit resolution.
type Actor struct {
	ID        string
	Health    float64
	MaxHealth float64
	Kind      ActorKind
}

// ActorRef wraps the opaque actor reference passed to the dispatcher with the
// extracted metadata required for hit resolution.
type ActorRef struct {
	Actor Actor
	Raw   any
}

// EffectHitDispatcherConfig bundles the adapters required to resolve combat
// effect hits without depending on legacy world types.
type EffectHitDispatcherConfig struct {
	ExtractEffect func(effect any) (EffectRef, bool)
	ExtractActor  func(target any) (ActorRef, bool)

	HealthEpsilon            float64
	BaselinePlayerMaxHealth  float64
	SetPlayerHealth          func(target ActorRef, next float64)
	SetNPCHealth             func(target ActorRef, next float64)
	ApplyGenericHealthDelta  func(target ActorRef, delta float64) (changed bool, actualDelta float64, newHealth float64)
	RecordEffectHitTelemetry func(effect EffectRef, target ActorRef, actualDelta float64)
	RecordDamageTelemetry    func(effect EffectRef, target ActorRef, damage float64, targetHealth float64, statusEffect string)
	RecordDefeatTelemetry    func(effect EffectRef, target ActorRef, statusEffect string)
	DropAllInventory         func(target ActorRef, reason string)
	ApplyStatusEffect        func(effect EffectRef, target ActorRef, statusEffect string, now time.Time)
}

type effectBehavior func(d *effectDispatcher, effect EffectRef, target ActorRef, now time.Time)

type effectDispatcher struct {
	cfg       EffectHitDispatcherConfig
	behaviors map[string]effectBehavior
}

// NewEffectHitDispatcher constructs an adapter that mirrors the legacy effect
// hit dispatch semantics using the provided configuration.
func NewEffectHitDispatcher(cfg EffectHitDispatcherConfig) EffectHitCallback {
	dispatcher := &effectDispatcher{
		cfg:       cfg,
		behaviors: newEffectBehaviors(),
	}
	return dispatcher.dispatch
}

func (d *effectDispatcher) dispatch(effect any, target any, now time.Time) {
	if d == nil {
		return
	}
	if d.cfg.ExtractEffect == nil || d.cfg.ExtractActor == nil {
		return
	}

	effRef, ok := d.cfg.ExtractEffect(effect)
	if !ok || effRef.Effect.Type == "" {
		return
	}
	actorRef, ok := d.cfg.ExtractActor(target)
	if !ok || actorRef.Actor.ID == "" {
		return
	}

	behavior, ok := d.behaviors[effRef.Effect.Type]
	if !ok || behavior == nil {
		return
	}

	behavior(d, effRef, actorRef, now)
}

func newEffectBehaviors() map[string]effectBehavior {
	return map[string]effectBehavior{
		EffectTypeAttack:      healthDeltaBehavior("healthDelta", 0),
		EffectTypeFireball:    damageAndStatusEffectBehavior("healthDelta", 0, StatusEffectBurning),
		EffectTypeBurningTick: healthDeltaBehavior("healthDelta", 0),
	}
}

func healthDeltaBehavior(param string, fallback float64) effectBehavior {
	return func(d *effectDispatcher, eff EffectRef, target ActorRef, now time.Time) {
		if d == nil {
			return
		}
		delta := fallback
		if params := eff.Effect.Params; params != nil {
			if value, ok := params[param]; ok {
				delta = value
			}
		}
		if delta == 0 || target.Actor.ID == "" {
			return
		}

		max := target.Actor.MaxHealth
		if max <= 0 && target.Actor.Kind != ActorKindGeneric {
			max = d.cfg.BaselinePlayerMaxHealth
		}

		next := target.Actor.Health + delta
		if math.IsNaN(next) || math.IsInf(next, 0) {
			return
		}
		if next < 0 {
			next = 0
		} else if max > 0 && next > max {
			next = max
		}

		if math.Abs(next-target.Actor.Health) < d.cfg.HealthEpsilon {
			return
		}

		actualDelta := next - target.Actor.Health
		switched := false

		switch target.Actor.Kind {
		case ActorKindPlayer:
			if d.cfg.SetPlayerHealth == nil {
				return
			}
			d.cfg.SetPlayerHealth(target, next)
			switched = true
		case ActorKindNPC:
			if d.cfg.SetNPCHealth == nil {
				return
			}
			d.cfg.SetNPCHealth(target, next)
			switched = true
		default:
			if d.cfg.ApplyGenericHealthDelta == nil {
				return
			}
			changed, deltaApplied, newHealth := d.cfg.ApplyGenericHealthDelta(target, delta)
			if !changed {
				return
			}
			actualDelta = deltaApplied
			next = newHealth
			switched = true
		}

		if !switched {
			return
		}

		if d.cfg.RecordEffectHitTelemetry != nil {
			d.cfg.RecordEffectHitTelemetry(eff, target, actualDelta)
		}

		if delta >= 0 {
			return
		}

		if d.cfg.RecordDamageTelemetry != nil {
			d.cfg.RecordDamageTelemetry(eff, target, -delta, next, eff.Effect.StatusEffect)
		}

		if next > 0 {
			return
		}

		if d.cfg.RecordDefeatTelemetry != nil {
			d.cfg.RecordDefeatTelemetry(eff, target, eff.Effect.StatusEffect)
		}

		if d.cfg.DropAllInventory != nil {
			d.cfg.DropAllInventory(target, "death")
		}
	}
}

func damageAndStatusEffectBehavior(param string, fallback float64, statusEffect string) effectBehavior {
	base := healthDeltaBehavior(param, fallback)
	return func(d *effectDispatcher, eff EffectRef, target ActorRef, now time.Time) {
		if base != nil {
			base(d, eff, target, now)
		}
		if d == nil || d.cfg.ApplyStatusEffect == nil || target.Actor.ID == "" || statusEffect == "" {
			return
		}
		d.cfg.ApplyStatusEffect(eff, target, statusEffect, now)
	}
}
