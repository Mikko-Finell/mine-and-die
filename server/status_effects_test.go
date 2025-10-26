package server

import (
	"testing"
	"time"

	statuspkg "mine-and-die/server/internal/world/status"
	"mine-and-die/server/logging"
)

func baseStatusEffectDefinitions() map[string]statuspkg.ApplyStatusEffectDefinition {
	return statuspkg.NewStatusEffectDefinitions(statuspkg.StatusEffectDefinitionsConfig{
		Burning: statuspkg.BurningStatusEffectDefinitionConfig{
			Type:               string(StatusEffectBurning),
			Duration:           burningStatusEffectDuration,
			TickInterval:       burningTickInterval,
			InitialTick:        true,
			FallbackAttachment: statuspkg.AttachStatusEffectVisual,
		},
	})
}

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
		if StatusEffectType(eff.StatusEffect) != StatusEffectBurning {
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

func TestApplyStatusEffectAttachesFallbackVisualWhenManagerMissing(t *testing.T) {
	t.Parallel()

	now := time.UnixMilli(1_700_000_000)

	world := &World{
		publisher:   logging.NopPublisher{},
		telemetry:   &telemetryCounters{},
		currentTick: 12,
	}
	world.statusEffectDefs = newStatusEffectDefinitions(baseStatusEffectDefinitions(), world)

	actor := &actorState{Actor: Actor{ID: "target-1"}}

	applied := world.applyStatusEffect(actor, StatusEffectBurning, "caster-1", now)
	if !applied {
		t.Fatalf("expected status effect to be applied")
	}

	inst, ok := actor.StatusEffects[StatusEffectBurning]
	if !ok || inst == nil {
		t.Fatalf("expected status effect instance to be stored")
	}

	effect, ok := inst.AttachedEffect().(*effectState)
	if !ok || effect == nil {
		t.Fatalf("expected fallback visual effect to be attached")
	}
	if effect.Type != effectTypeBurningVisual {
		t.Fatalf("expected visual type %q, got %q", effectTypeBurningVisual, effect.Type)
	}
	if effect.Owner != "caster-1" {
		t.Fatalf("expected effect owner %q, got %q", "caster-1", effect.Owner)
	}
	if StatusEffectType(effect.StatusEffect) != StatusEffectBurning {
		t.Fatalf("expected effect status %q, got %q", StatusEffectBurning, effect.StatusEffect)
	}
	if effect.ExpiresAt.Before(now) {
		t.Fatalf("expected effect expiration after apply time, got %v", effect.ExpiresAt)
	}
}

func TestAttachStatusEffectVisualResolvesActorFromHandle(t *testing.T) {
	t.Parallel()

	const (
		actorID = "target-42"
	)

	now := time.UnixMilli(1_700_000_123)
	lifetime := 350 * time.Millisecond

	world := &World{
		effects:     make([]*effectState, 0),
		effectsByID: make(map[string]*effectState),
		currentTick: 64,
	}
	world.statusEffectDefs = newStatusEffectDefinitions(baseStatusEffectDefinitions(), world)

	actor := &actorState{Actor: Actor{ID: actorID, X: 320, Y: 440}}
	inst := &statusEffectInstance{}
	handle := newStatusEffectInstanceHandle(inst, actor)

	effect := world.attachStatusEffectVisual(handle, nil, StatusEffectBurning, "", effectTypeBurningVisual, lifetime, time.Time{}, now)
	if effect == nil {
		t.Fatalf("expected fallback visual effect to be constructed")
	}

	if attached, ok := inst.AttachedEffect().(*effectState); !ok || attached != effect {
		t.Fatalf("expected status effect instance to hold attached effect")
	}
	if effect.Owner != actorID {
		t.Fatalf("expected effect owner %q, got %q", actorID, effect.Owner)
	}
	if effect.FollowActorID != actorID {
		t.Fatalf("expected effect to follow actor %q, got %q", actorID, effect.FollowActorID)
	}
	if StatusEffectType(effect.StatusEffect) != StatusEffectBurning {
		t.Fatalf("expected status effect %q, got %q", StatusEffectBurning, effect.StatusEffect)
	}

	expectedWidth := playerHalf * 2
	if effect.Width != expectedWidth || effect.Height != expectedWidth {
		t.Fatalf("expected effect footprint %.1f x %.1f, got %.1f x %.1f", expectedWidth, expectedWidth, effect.Width, effect.Height)
	}
	expectedX := actor.X - expectedWidth/2
	expectedY := actor.Y - expectedWidth/2
	if effect.X != expectedX || effect.Y != expectedY {
		t.Fatalf("expected effect position (%.1f, %.1f), got (%.1f, %.1f)", expectedX, expectedY, effect.X, effect.Y)
	}

	expectedExpiry := now.Add(lifetime)
	if !effect.ExpiresAt.Equal(expectedExpiry) {
		t.Fatalf("expected expiration %v, got %v", expectedExpiry, effect.ExpiresAt)
	}
}
