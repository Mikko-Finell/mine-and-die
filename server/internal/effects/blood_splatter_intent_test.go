package effects

import (
	"testing"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

func TestNewBloodSplatterIntent(t *testing.T) {
	cfg := BloodSplatterIntentConfig{
		SourceActorID: "player-8",
		TargetActorID: "npc-5",
		Target: &ActorPosition{
			X: 200,
			Y: 260,
		},
		TileSize:  40,
		Footprint: 28,
		Duration:  1200 * time.Millisecond,
		TickRate:  15,
	}

	intent, ok := NewBloodSplatterIntent(cfg)
	if !ok {
		t.Fatal("expected blood splatter intent to be constructed")
	}

	if intent.EntryID != effectcontract.EffectIDBloodSplatter {
		t.Fatalf("expected EntryID %q, got %q", effectcontract.EffectIDBloodSplatter, intent.EntryID)
	}
	if intent.TypeID != effectcontract.EffectIDBloodSplatter {
		t.Fatalf("expected TypeID %q, got %q", effectcontract.EffectIDBloodSplatter, intent.TypeID)
	}
	if intent.SourceActorID != cfg.SourceActorID {
		t.Fatalf("expected SourceActorID %q, got %q", cfg.SourceActorID, intent.SourceActorID)
	}
	if intent.Delivery != effectcontract.DeliveryKindVisual {
		t.Fatalf("expected DeliveryKindVisual, got %q", intent.Delivery)
	}

	expectedDuration := durationToTicks(cfg.Duration, cfg.TickRate)
	if intent.DurationTicks != expectedDuration {
		t.Fatalf("expected DurationTicks %d, got %d", expectedDuration, intent.DurationTicks)
	}

	expectedFootprint := QuantizeWorldCoord(cfg.Footprint, cfg.TileSize)
	if intent.Geometry.Width != expectedFootprint || intent.Geometry.Height != expectedFootprint {
		t.Fatalf("expected footprint %d, got width=%d height=%d", expectedFootprint, intent.Geometry.Width, intent.Geometry.Height)
	}

	expectedCenterX := QuantizeWorldCoord(cfg.Target.X, cfg.TileSize)
	if intent.Params["centerX"] != expectedCenterX {
		t.Fatalf("expected centerX %d, got %d", expectedCenterX, intent.Params["centerX"])
	}

	expectedCenterY := QuantizeWorldCoord(cfg.Target.Y, cfg.TileSize)
	if intent.Params["centerY"] != expectedCenterY {
		t.Fatalf("expected centerY %d, got %d", expectedCenterY, intent.Params["centerY"])
	}
}
