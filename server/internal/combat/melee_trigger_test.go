package combat

import (
	"math"
	"testing"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

func TestStageMeleeIntentReturnsIntent(t *testing.T) {
	tileSize := 40.0
	tickRate := 15.0

	quantize := func(value float64) int {
		return int(math.Round(value * effectcontract.CoordScale))
	}
	durationToTicks := func(duration time.Duration) int {
		if duration <= 0 {
			return 0
		}
		ticks := int(math.Ceil(duration.Seconds() * tickRate))
		if ticks < 1 {
			ticks = 1
		}
		return ticks
	}

	cfg := MeleeIntentConfig{
		Geometry: MeleeAttackGeometryConfig{
			PlayerHalf:    20,
			Reach:         MeleeAttackReach,
			Width:         MeleeAttackWidth,
			DefaultFacing: "down",
		},
		TileSize:        tileSize,
		Damage:          MeleeAttackDamage,
		Duration:        MeleeAttackDuration,
		QuantizeCoord:   quantize,
		DurationToTicks: durationToTicks,
	}

	expectedOwner := MeleeIntentOwner{ID: "player-1", X: 200, Y: 180, Facing: "down"}

	gate := func(actorID string, now time.Time) (MeleeIntentOwner, bool) {
		if actorID != expectedOwner.ID {
			t.Fatalf("expected gate actor %q, got %q", expectedOwner.ID, actorID)
		}
		_ = now
		return expectedOwner, true
	}

	intent, ok := StageMeleeIntent(MeleeAbilityTriggerConfig{
		AbilityGate:  gate,
		IntentConfig: cfg,
	}, expectedOwner.ID, time.Unix(1, 0))
	if !ok {
		t.Fatalf("expected melee intent to be staged")
	}

	if intent.TypeID != EffectTypeAttack || intent.SourceActorID != expectedOwner.ID {
		t.Fatalf("expected staged melee intent owned by %q", expectedOwner.ID)
	}
}

func TestStageMeleeIntentRequiresValidOwner(t *testing.T) {
	intent, ok := StageMeleeIntent(MeleeAbilityTriggerConfig{
		AbilityGate: func(string, time.Time) (MeleeIntentOwner, bool) {
			return MeleeIntentOwner{}, true
		},
	}, "player-1", time.Unix(0, 0))
	if ok {
		t.Fatalf("expected invalid owner failure, got %#v", intent)
	}
}

func TestStageMeleeIntentRequiresGate(t *testing.T) {
	intent, ok := StageMeleeIntent(MeleeAbilityTriggerConfig{}, "player-1", time.Unix(0, 0))
	if ok {
		t.Fatalf("expected staging to fail without gate, got %#v", intent)
	}
}
