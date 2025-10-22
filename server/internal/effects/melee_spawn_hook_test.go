package effects

import (
	"testing"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

func TestMeleeSpawnHookDelegatesToResolver(t *testing.T) {
	now := time.UnixMilli(1700)
	instance := &effectcontract.EffectInstance{
		ID:           "effect-melee",
		DefinitionID: "melee",
		OwnerActorID: "actor-1",
		StartTick:    12,
	}
	cfg := MeleeSpawnHookConfig{
		TileSize:        40,
		DefaultWidth:    32,
		DefaultReach:    64,
		DefaultDamage:   15,
		DefaultDuration: 750 * time.Millisecond,
	}
	owner := &MeleeOwner{X: 100, Y: 200, Reference: "owner-ref"}

	var capturedEffect *State
	var capturedOwner *MeleeOwner
	var capturedActorID string
	var capturedTick effectcontract.Tick
	var capturedNow time.Time
	var capturedArea MeleeImpactArea

	cfg.LookupOwner = func(actorID string) *MeleeOwner {
		if actorID != "actor-1" {
			return nil
		}
		return owner
	}
	cfg.ResolveImpact = func(effect *State, ownerArg *MeleeOwner, actorID string, tick effectcontract.Tick, nowArg time.Time, area MeleeImpactArea) {
		capturedEffect = effect
		capturedOwner = ownerArg
		capturedActorID = actorID
		capturedTick = tick
		capturedNow = nowArg
		capturedArea = area
	}

	hook := MeleeSpawnHook(cfg)
	hook.OnSpawn(nil, instance, 45, now)

	if capturedEffect == nil {
		t.Fatalf("expected resolver to receive effect state")
	}
	if capturedOwner != owner {
		t.Fatalf("expected resolver to receive owner pointer")
	}
	if capturedActorID != "actor-1" {
		t.Fatalf("unexpected actor ID: %s", capturedActorID)
	}
	if capturedTick != 45 {
		t.Fatalf("unexpected tick: %v", capturedTick)
	}
	if capturedNow != now {
		t.Fatalf("resolver received incorrect time")
	}

	if capturedEffect.ID != "effect-melee" {
		t.Fatalf("unexpected effect ID: %s", capturedEffect.ID)
	}
	if capturedEffect.Type != "melee" {
		t.Fatalf("unexpected effect type: %s", capturedEffect.Type)
	}
	if capturedEffect.Owner != "actor-1" {
		t.Fatalf("unexpected effect owner: %s", capturedEffect.Owner)
	}
	if capturedEffect.Start != now.UnixMilli() {
		t.Fatalf("unexpected effect start: %d", capturedEffect.Start)
	}
	expectedDuration := cfg.DefaultDuration.Milliseconds()
	if capturedEffect.Duration != expectedDuration {
		t.Fatalf("unexpected effect duration: %d", capturedEffect.Duration)
	}
	if capturedEffect.Width != cfg.DefaultWidth {
		t.Fatalf("expected width %f got %f", cfg.DefaultWidth, capturedEffect.Width)
	}
	if capturedEffect.Height != cfg.DefaultReach {
		t.Fatalf("expected height %f got %f", cfg.DefaultReach, capturedEffect.Height)
	}
	if capturedEffect.X != owner.X-cfg.DefaultWidth/2 {
		t.Fatalf("unexpected effect X: %f", capturedEffect.X)
	}
	if capturedEffect.Y != owner.Y-cfg.DefaultReach/2 {
		t.Fatalf("unexpected effect Y: %f", capturedEffect.Y)
	}

	if value := capturedEffect.Params["healthDelta"]; value != -cfg.DefaultDamage {
		t.Fatalf("expected healthDelta %f got %f", -cfg.DefaultDamage, value)
	}
	if value := capturedEffect.Params["reach"]; value != cfg.DefaultReach {
		t.Fatalf("expected reach %f got %f", cfg.DefaultReach, value)
	}
	if value := capturedEffect.Params["width"]; value != cfg.DefaultWidth {
		t.Fatalf("expected width %f got %f", cfg.DefaultWidth, value)
	}

	motion := capturedEffect.Instance.DeliveryState.Motion
	expectedPos := QuantizeWorldCoord(owner.X, cfg.TileSize)
	expectedPosY := QuantizeWorldCoord(owner.Y, cfg.TileSize)
	if motion.PositionX != expectedPos || motion.PositionY != expectedPosY {
		t.Fatalf("unexpected motion position: (%d,%d)", motion.PositionX, motion.PositionY)
	}
	if motion.VelocityX != 0 || motion.VelocityY != 0 {
		t.Fatalf("expected zero velocities, got (%d,%d)", motion.VelocityX, motion.VelocityY)
	}

	if capturedArea.X != capturedEffect.X || capturedArea.Y != capturedEffect.Y ||
		capturedArea.Width != capturedEffect.Width || capturedArea.Height != capturedEffect.Height {
		t.Fatalf("resolver received mismatched area: %+v vs effect %+v", capturedArea, capturedEffect)
	}
}

func TestMeleeSpawnHookPreservesInstanceGeometry(t *testing.T) {
	tileSize := 40.0
	width := 48.0
	height := 24.0
	offsetX := -10.0
	offsetY := 6.0
	instance := &effectcontract.EffectInstance{
		ID:           "instance-geom",
		DefinitionID: "melee",
		OwnerActorID: "actor-geom",
		StartTick:    8,
		DeliveryState: effectcontract.EffectDeliveryState{
			Geometry: effectcontract.EffectGeometry{
				Width:   QuantizeWorldCoord(width, tileSize),
				Height:  QuantizeWorldCoord(height, tileSize),
				OffsetX: QuantizeWorldCoord(offsetX, tileSize),
				OffsetY: QuantizeWorldCoord(offsetY, tileSize),
			},
		},
		BehaviorState: effectcontract.EffectBehaviorState{
			Extra: map[string]int{
				"healthDelta": -22,
				"reach":       77,
				"width":       31,
			},
		},
	}
	owner := &MeleeOwner{X: 10, Y: 20, Reference: "owner"}
	cfg := MeleeSpawnHookConfig{
		TileSize:        tileSize,
		DefaultWidth:    32,
		DefaultReach:    64,
		DefaultDamage:   15,
		DefaultDuration: time.Second,
		LookupOwner: func(actorID string) *MeleeOwner {
			if actorID != "actor-geom" {
				return nil
			}
			return owner
		},
	}

	var capturedEffect *State
	cfg.ResolveImpact = func(effect *State, _ *MeleeOwner, _ string, _ effectcontract.Tick, _ time.Time, _ MeleeImpactArea) {
		capturedEffect = effect
	}

	hook := MeleeSpawnHook(cfg)
	hook.OnSpawn(nil, instance, 99, time.UnixMilli(5000))

	if capturedEffect == nil {
		t.Fatalf("expected resolver to receive effect state")
	}

	expectedWidth := DequantizeWorldCoord(instance.DeliveryState.Geometry.Width, tileSize)
	expectedHeight := DequantizeWorldCoord(instance.DeliveryState.Geometry.Height, tileSize)
	if capturedEffect.Width != expectedWidth {
		t.Fatalf("expected width %f got %f", expectedWidth, capturedEffect.Width)
	}
	if capturedEffect.Height != expectedHeight {
		t.Fatalf("expected height %f got %f", expectedHeight, capturedEffect.Height)
	}
	expectedX := owner.X + DequantizeWorldCoord(instance.DeliveryState.Geometry.OffsetX, tileSize) - expectedWidth/2
	expectedY := owner.Y + DequantizeWorldCoord(instance.DeliveryState.Geometry.OffsetY, tileSize) - expectedHeight/2
	if capturedEffect.X != expectedX || capturedEffect.Y != expectedY {
		t.Fatalf("unexpected effect position: (%f,%f)", capturedEffect.X, capturedEffect.Y)
	}

	if capturedEffect.Params["healthDelta"] != float64(-22) {
		t.Fatalf("expected custom healthDelta to be preserved")
	}
	if capturedEffect.Params["reach"] != float64(77) {
		t.Fatalf("expected custom reach to be preserved")
	}
	if capturedEffect.Params["width"] != float64(31) {
		t.Fatalf("expected custom width to be preserved")
	}
}

func TestMeleeSpawnHookSkipsWithoutOwner(t *testing.T) {
	cfg := MeleeSpawnHookConfig{
		TileSize:      40,
		DefaultWidth:  32,
		DefaultReach:  64,
		DefaultDamage: 10,
	}
	cfg.LookupOwner = func(actorID string) *MeleeOwner {
		return nil
	}
	called := false
	cfg.ResolveImpact = func(*State, *MeleeOwner, string, effectcontract.Tick, time.Time, MeleeImpactArea) {
		called = true
	}

	hook := MeleeSpawnHook(cfg)
	hook.OnSpawn(nil, &effectcontract.EffectInstance{OwnerActorID: "missing"}, 0, time.Time{})

	if called {
		t.Fatalf("resolver should not be invoked when owner lookup fails")
	}
}
