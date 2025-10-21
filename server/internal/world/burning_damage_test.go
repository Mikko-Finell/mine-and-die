package world

import (
	"math"
	"testing"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

func TestApplyBurningDamageInvokesCallbackWithNormalizedEffect(t *testing.T) {
	now := time.Date(2024, 7, 1, 2, 3, 4, 0, time.UTC)
	called := 0
	var captured BurningDamageEffect

	ApplyBurningDamage(ApplyBurningDamageConfig{
		EffectType:   "burning-tick",
		ActorID:      "actor-1",
		StatusEffect: "burning",
		Delta:        -12.5,
		Now:          now,
		CurrentTick:  42,
		Apply: func(effect BurningDamageEffect) {
			called++
			captured = effect
		},
	})

	if called != 1 {
		t.Fatalf("expected callback to be invoked once, got %d", called)
	}
	if captured.OwnerID != "actor-1" {
		t.Fatalf("expected owner to fall back to actor ID, got %q", captured.OwnerID)
	}
	if captured.StatusEffect != "burning" {
		t.Fatalf("expected status effect 'burning', got %q", captured.StatusEffect)
	}
	if captured.HealthDelta != -12.5 {
		t.Fatalf("expected health delta -12.5, got %f", captured.HealthDelta)
	}
	if captured.StartMillis != now.UnixMilli() {
		t.Fatalf("expected start millis %d, got %d", now.UnixMilli(), captured.StartMillis)
	}
	expectedTick := effectcontract.Tick(42)
	if captured.SpawnTick != expectedTick {
		t.Fatalf("expected spawn tick %d, got %d", expectedTick, captured.SpawnTick)
	}
}

func TestApplyBurningDamageUsesExplicitOwnerWhenProvided(t *testing.T) {
	called := 0
	ApplyBurningDamage(ApplyBurningDamageConfig{
		EffectType:   "burning-tick",
		OwnerID:      "owner-1",
		ActorID:      "actor-1",
		StatusEffect: "burning",
		Delta:        -3,
		Now:          time.Now(),
		Apply: func(effect BurningDamageEffect) {
			called++
			if effect.OwnerID != "owner-1" {
				t.Fatalf("expected owner ID 'owner-1', got %q", effect.OwnerID)
			}
		},
	})
	if called != 1 {
		t.Fatalf("expected callback to be invoked once, got %d", called)
	}
}

func TestApplyBurningDamageIgnoresNonDamageDeltas(t *testing.T) {
	tests := []float64{0, 5, math.NaN(), math.Inf(1), math.Inf(-1)}

	for _, delta := range tests {
		called := 0
		ApplyBurningDamage(ApplyBurningDamageConfig{
			EffectType:   "burning-tick",
			ActorID:      "actor-1",
			StatusEffect: "burning",
			Delta:        delta,
			Now:          time.Now(),
			Apply: func(BurningDamageEffect) {
				called++
			},
		})
		if called != 0 {
			t.Fatalf("expected no callback for delta %v", delta)
		}
	}
}

func TestApplyBurningDamageNoopWithoutCallback(t *testing.T) {
	ApplyBurningDamage(ApplyBurningDamageConfig{
		EffectType:   "burning-tick",
		ActorID:      "actor-1",
		StatusEffect: "burning",
		Delta:        -1,
		Now:          time.Now(),
	})
}
