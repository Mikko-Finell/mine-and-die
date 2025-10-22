package combat

import (
	"testing"
	"time"
)

func TestDispatcherAppliesPlayerDamage(t *testing.T) {
	var setHealthTarget ActorRef
	var setHealthValue float64
	var telemetryDelta float64
	var damageAmount float64
	var damageHealth float64
	dispatcher := NewEffectHitDispatcher(EffectHitDispatcherConfig{
		ExtractEffect: func(effect any) (EffectRef, bool) {
			eff, _ := effect.(EffectRef)
			return eff, true
		},
		ExtractActor: func(target any) (ActorRef, bool) {
			actor, _ := target.(ActorRef)
			return actor, true
		},
		HealthEpsilon:           1e-6,
		BaselinePlayerMaxHealth: 100,
		SetPlayerHealth: func(target ActorRef, next float64) {
			setHealthTarget = target
			setHealthValue = next
		},
		SetNPCHealth: func(ActorRef, float64) {},
		ApplyGenericHealthDelta: func(target ActorRef, delta float64) (bool, float64, float64) {
			return false, 0, target.Actor.Health
		},
		RecordEffectHitTelemetry: func(effect EffectRef, target ActorRef, actualDelta float64) {
			telemetryDelta = actualDelta
		},
		RecordDamageTelemetry: func(effect EffectRef, target ActorRef, damage float64, targetHealth float64, statusEffect string) {
			damageAmount = damage
			damageHealth = targetHealth
		},
		RecordDefeatTelemetry: func(EffectRef, ActorRef, string) {},
		DropAllInventory:      func(ActorRef, string) {},
		ApplyStatusEffect:     func(EffectRef, ActorRef, string, time.Time) {},
	})

	eff := EffectRef{Effect: Effect{Type: EffectTypeAttack, OwnerID: "caster", Params: map[string]float64{"healthDelta": -15}}}
	actor := ActorRef{Actor: Actor{ID: "player-1", Health: 50, MaxHealth: 100, Kind: ActorKindPlayer}}

	dispatcher(eff, actor, time.UnixMilli(42))

	if setHealthTarget.Actor.ID != actor.Actor.ID {
		t.Fatalf("expected player health update, got %q", setHealthTarget.Actor.ID)
	}
	if setHealthValue != 35 {
		t.Fatalf("expected health to clamp to 35, got %.2f", setHealthValue)
	}
	if telemetryDelta != -15 {
		t.Fatalf("expected telemetry delta -15, got %.2f", telemetryDelta)
	}
	if damageAmount != 15 {
		t.Fatalf("expected damage amount 15, got %.2f", damageAmount)
	}
	if damageHealth != 35 {
		t.Fatalf("expected damage target health 35, got %.2f", damageHealth)
	}
}

func TestDispatcherTriggersDefeatAndDrop(t *testing.T) {
	var defeatCalled bool
	var dropCalled bool
	dispatcher := NewEffectHitDispatcher(EffectHitDispatcherConfig{
		ExtractEffect: func(effect any) (EffectRef, bool) {
			eff, _ := effect.(EffectRef)
			return eff, true
		},
		ExtractActor: func(target any) (ActorRef, bool) {
			actor, _ := target.(ActorRef)
			return actor, true
		},
		HealthEpsilon:           1e-6,
		BaselinePlayerMaxHealth: 100,
		SetNPCHealth:            func(ActorRef, float64) {},
		SetPlayerHealth:         func(target ActorRef, next float64) {},
		ApplyGenericHealthDelta: func(target ActorRef, delta float64) (bool, float64, float64) {
			return true, delta, target.Actor.Health + delta
		},
		RecordEffectHitTelemetry: func(effect EffectRef, target ActorRef, actualDelta float64) {},
		RecordDamageTelemetry:    func(effect EffectRef, target ActorRef, damage float64, targetHealth float64, statusEffect string) {},
		RecordDefeatTelemetry: func(EffectRef, ActorRef, string) {
			defeatCalled = true
		},
		DropAllInventory: func(ActorRef, string) {
			dropCalled = true
		},
		ApplyStatusEffect: func(EffectRef, ActorRef, string, time.Time) {},
	})

	eff := EffectRef{Effect: Effect{Type: EffectTypeBurningTick, OwnerID: "caster", Params: map[string]float64{"healthDelta": -10}}}
	actor := ActorRef{Actor: Actor{ID: "target", Health: 5, MaxHealth: 5, Kind: ActorKindGeneric}}

	dispatcher(eff, actor, time.UnixMilli(99))

	if !defeatCalled {
		t.Fatalf("expected defeat telemetry to fire")
	}
	if !dropCalled {
		t.Fatalf("expected drop inventory to fire")
	}
}
