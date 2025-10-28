package status

import (
	"testing"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	worldeffects "mine-and-die/server/internal/world/effects"
)

func TestNewBurningTickIntent_BuildsIntentFromWorldHelper(t *testing.T) {
	cfg := BurningTickIntentConfig{
		EffectType:    "burning-tick",
		TargetActorID: "target-actor",
		SourceActorID: "lava-source",
		StatusEffect:  StatusEffectType("burning"),
		Delta:         -4,
		TileSize:      40,
		Footprint:     80,
		Now:           time.Unix(123, 0),
		CurrentTick:   7,
	}

	intent, ok := NewBurningTickIntent(cfg)
	if !ok {
		t.Fatal("expected burning tick intent to be constructed")
	}

	if intent.EntryID != cfg.EffectType {
		t.Fatalf("expected EntryID %q, got %q", cfg.EffectType, intent.EntryID)
	}
	if intent.TypeID != cfg.EffectType {
		t.Fatalf("expected TypeID %q, got %q", cfg.EffectType, intent.TypeID)
	}
	if intent.SourceActorID != cfg.SourceActorID {
		t.Fatalf("expected SourceActorID %q, got %q", cfg.SourceActorID, intent.SourceActorID)
	}
	if intent.TargetActorID != cfg.TargetActorID {
		t.Fatalf("expected TargetActorID %q, got %q", cfg.TargetActorID, intent.TargetActorID)
	}
	if intent.Delivery != effectcontract.DeliveryKindTarget {
		t.Fatalf("expected DeliveryKindTarget, got %q", intent.Delivery)
	}
	if intent.DurationTicks != 1 {
		t.Fatalf("expected DurationTicks 1, got %d", intent.DurationTicks)
	}
	expectedFootprint := worldeffects.QuantizeWorldCoord(cfg.Footprint, cfg.TileSize)
	if intent.Geometry.Width != expectedFootprint || intent.Geometry.Height != expectedFootprint {
		t.Fatalf("expected footprint %d, got width=%d height=%d", expectedFootprint, intent.Geometry.Width, intent.Geometry.Height)
	}
	if intent.Params["healthDelta"] != int(cfg.Delta) {
		t.Fatalf("expected healthDelta %d, got %d", int(cfg.Delta), intent.Params["healthDelta"])
	}
}

func TestNewBurningTickIntent_FallsBackToTargetWhenOwnerMissing(t *testing.T) {
	cfg := BurningTickIntentConfig{
		EffectType:    "burning-tick",
		TargetActorID: "target-actor",
		Delta:         -3,
		TileSize:      40,
		Footprint:     80,
	}

	intent, ok := NewBurningTickIntent(cfg)
	if !ok {
		t.Fatal("expected burning tick intent to be constructed")
	}
	if intent.SourceActorID != cfg.TargetActorID {
		t.Fatalf("expected SourceActorID to fall back to %q, got %q", cfg.TargetActorID, intent.SourceActorID)
	}
}

func TestNewBurningTickIntent_IgnoresRoundedZeroDelta(t *testing.T) {
	cfg := BurningTickIntentConfig{
		EffectType:    "burning-tick",
		TargetActorID: "target-actor",
		Delta:         -0.4,
	}

	if _, ok := NewBurningTickIntent(cfg); ok {
		t.Fatal("expected burning tick intent construction to fail for rounded zero delta")
	}
}

func TestNewBurningTickIntent_RequiresTargetAndEffectType(t *testing.T) {
	if _, ok := NewBurningTickIntent(BurningTickIntentConfig{TargetActorID: "target-actor"}); ok {
		t.Fatal("expected construction to fail without effect type")
	}
	if _, ok := NewBurningTickIntent(BurningTickIntentConfig{EffectType: "burning-tick"}); ok {
		t.Fatal("expected construction to fail without target actor")
	}
}
