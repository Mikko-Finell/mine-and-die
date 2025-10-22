package server

import (
	"testing"
	"time"
)

func TestWorldApplyBurningDamageDelegatesAndFlushesTelemetry(t *testing.T) {
	t.Parallel()

	const (
		ownerID = "caster-1"
		delta   = -7.5
	)

	now := time.UnixMilli(1_700_000_000)
	later := now.Add(150 * time.Millisecond)

	world := &World{
		telemetry:   &telemetryCounters{},
		currentTick: 42,
	}

	actor := &actorState{Actor: Actor{ID: "target-1", Health: 80, MaxHealth: 120}}

	var (
		dispatcherCalls   int
		dispatchedTimes   []time.Time
		dispatchedTargets []any
		dispatchedEffects []*effectState
	)

	world.effectHitAdapter = func(effect any, target any, at time.Time) {
		dispatcherCalls++
		dispatchedTimes = append(dispatchedTimes, at)
		dispatchedTargets = append(dispatchedTargets, target)

		eff, _ := effect.(*effectState)
		dispatchedEffects = append(dispatchedEffects, eff)
		if eff != nil {
			world.recordEffectHitTelemetry(eff, actor.ID, delta)
		}
	}

	world.applyBurningDamage(ownerID, actor, StatusEffectBurning, delta, now)
	world.applyBurningDamage(ownerID, actor, StatusEffectBurning, delta, later)

	if dispatcherCalls != 2 {
		t.Fatalf("expected dispatcher to run twice, got %d", dispatcherCalls)
	}
	if len(dispatchedEffects) != 2 {
		t.Fatalf("expected two dispatched effects, got %d", len(dispatchedEffects))
	}
	if len(dispatchedTargets) != 2 {
		t.Fatalf("expected two dispatched targets, got %d", len(dispatchedTargets))
	}
	if len(dispatchedTimes) != 2 {
		t.Fatalf("expected two dispatch timestamps, got %d", len(dispatchedTimes))
	}

	for i, eff := range dispatchedEffects {
		if eff == nil {
			t.Fatalf("effect %d was nil", i)
		}
		if eff.Type != effectTypeBurningTick {
			t.Fatalf("effect %d type mismatch: %q", i, eff.Type)
		}
		if eff.Owner != ownerID {
			t.Fatalf("effect %d owner mismatch: %q", i, eff.Owner)
		}
		if eff.StatusEffect != StatusEffectBurning {
			t.Fatalf("effect %d status effect mismatch: %q", i, eff.StatusEffect)
		}
		if eff.Params["healthDelta"] != delta {
			t.Fatalf("effect %d health delta mismatch: %v", i, eff.Params["healthDelta"])
		}
	}

	if dispatchedTargets[0] != actor || dispatchedTargets[1] != actor {
		t.Fatalf("dispatcher received unexpected targets: %#v", dispatchedTargets)
	}
	if !dispatchedTimes[0].Equal(now) || !dispatchedTimes[1].Equal(later) {
		t.Fatalf("dispatcher received unexpected times: %v", dispatchedTimes)
	}

	summary := world.telemetry.effectParity.snapshot(0)
	entry, ok := summary[effectTypeBurningTick]
	if !ok {
		t.Fatalf("missing telemetry entry for %q", effectTypeBurningTick)
	}
	if entry.Hits != 2 {
		t.Fatalf("expected two hits recorded, got %d", entry.Hits)
	}
	expectedDamage := -2 * delta
	if entry.Damage != expectedDamage {
		t.Fatalf("expected damage %f, got %f", expectedDamage, entry.Damage)
	}
	victims := entry.VictimBuckets["1"]
	if victims != 2 {
		t.Fatalf("expected victim bucket count 2, got %d", victims)
	}
}
