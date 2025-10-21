package effects

import (
	"testing"

	effectcontract "mine-and-die/server/effects/contract"
)

func TestSyncContractStatusVisualInstance_WithActor(t *testing.T) {
	instance := &effectcontract.EffectInstance{
		DeliveryState: effectcontract.EffectDeliveryState{
			Geometry: effectcontract.EffectGeometry{},
			Motion: effectcontract.EffectMotionState{
				VelocityX: 5,
				VelocityY: -3,
			},
		},
	}
	effect := &State{X: 120, Y: 220, Width: 30, Height: 34}

	actor := &ActorPosition{X: 150, Y: 240}

	cfg := StatusVisualSyncConfig{
		Instance:         instance,
		Effect:           effect,
		Actor:            actor,
		TileSize:         40,
		DefaultFootprint: 28,
	}

	SyncContractStatusVisualInstance(cfg)

	geometry := instance.DeliveryState.Geometry
	expectedWidth := QuantizeWorldCoord(effect.Width, cfg.TileSize)
	if geometry.Width != expectedWidth {
		t.Fatalf("expected width %d, got %d", expectedWidth, geometry.Width)
	}
	expectedHeight := QuantizeWorldCoord(effect.Height, cfg.TileSize)
	if geometry.Height != expectedHeight {
		t.Fatalf("expected height %d, got %d", expectedHeight, geometry.Height)
	}

	effectCenterX := effect.X + effect.Width/2
	effectCenterY := effect.Y + effect.Height/2
	expectedOffsetX := QuantizeWorldCoord(effectCenterX-actor.X, cfg.TileSize)
	if geometry.OffsetX != expectedOffsetX {
		t.Fatalf("expected offsetX %d, got %d", expectedOffsetX, geometry.OffsetX)
	}
	expectedOffsetY := QuantizeWorldCoord(effectCenterY-actor.Y, cfg.TileSize)
	if geometry.OffsetY != expectedOffsetY {
		t.Fatalf("expected offsetY %d, got %d", expectedOffsetY, geometry.OffsetY)
	}

	motion := instance.DeliveryState.Motion
	expectedPosX := QuantizeWorldCoord(actor.X, cfg.TileSize)
	if motion.PositionX != expectedPosX {
		t.Fatalf("expected positionX %d, got %d", expectedPosX, motion.PositionX)
	}
	expectedPosY := QuantizeWorldCoord(actor.Y, cfg.TileSize)
	if motion.PositionY != expectedPosY {
		t.Fatalf("expected positionY %d, got %d", expectedPosY, motion.PositionY)
	}
	if motion.VelocityX != 0 || motion.VelocityY != 0 {
		t.Fatalf("expected zero velocity, got (%d,%d)", motion.VelocityX, motion.VelocityY)
	}
}

func TestSyncContractStatusVisualInstance_WithoutActor(t *testing.T) {
	initialOffsetX := 7
	initialOffsetY := -11
	instance := &effectcontract.EffectInstance{
		DeliveryState: effectcontract.EffectDeliveryState{
			Geometry: effectcontract.EffectGeometry{
				OffsetX: initialOffsetX,
				OffsetY: initialOffsetY,
			},
			Motion: effectcontract.EffectMotionState{
				VelocityX: 15,
				VelocityY: 20,
			},
		},
	}
	effect := &State{X: 100, Y: 80}

	cfg := StatusVisualSyncConfig{
		Instance:         instance,
		Effect:           effect,
		TileSize:         40,
		DefaultFootprint: 28,
	}

	SyncContractStatusVisualInstance(cfg)

	geometry := instance.DeliveryState.Geometry
	expectedWidth := QuantizeWorldCoord(cfg.DefaultFootprint, cfg.TileSize)
	if geometry.Width != expectedWidth {
		t.Fatalf("expected width %d, got %d", expectedWidth, geometry.Width)
	}
	expectedHeight := QuantizeWorldCoord(cfg.DefaultFootprint, cfg.TileSize)
	if geometry.Height != expectedHeight {
		t.Fatalf("expected height %d, got %d", expectedHeight, geometry.Height)
	}
	if geometry.OffsetX != initialOffsetX || geometry.OffsetY != initialOffsetY {
		t.Fatalf("expected offsets (%d,%d), got (%d,%d)", initialOffsetX, initialOffsetY, geometry.OffsetX, geometry.OffsetY)
	}

	motion := instance.DeliveryState.Motion
	centerX := effect.X + effect.Width/2
	centerY := effect.Y + effect.Height/2
	expectedPosX := QuantizeWorldCoord(centerX, cfg.TileSize)
	if motion.PositionX != expectedPosX {
		t.Fatalf("expected positionX %d, got %d", expectedPosX, motion.PositionX)
	}
	expectedPosY := QuantizeWorldCoord(centerY, cfg.TileSize)
	if motion.PositionY != expectedPosY {
		t.Fatalf("expected positionY %d, got %d", expectedPosY, motion.PositionY)
	}
	if motion.VelocityX != 0 || motion.VelocityY != 0 {
		t.Fatalf("expected zero velocity, got (%d,%d)", motion.VelocityX, motion.VelocityY)
	}
}
