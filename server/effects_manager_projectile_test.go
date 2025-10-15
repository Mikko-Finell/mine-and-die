package main

import (
	"math"
	"testing"
	"time"
)

func TestSyncProjectileInstanceQuantizesDirection(t *testing.T) {
	owner := &actorState{Actor: Actor{ID: "owner", X: 10, Y: 20, Facing: FacingUp}}
	instance := &EffectInstance{
		ID:           "effect-1",
		OwnerActorID: owner.ID,
		BehaviorState: EffectBehaviorState{
			Extra: make(map[string]int),
		},
		DeliveryState: EffectDeliveryState{Geometry: EffectGeometry{}},
		Params:        make(map[string]int),
	}
	tpl := &ProjectileTemplate{Type: effectTypeFireball, MaxDistance: 30}
	effect := &effectState{
		Effect: Effect{Params: map[string]float64{"radius": 1}},
		Projectile: &ProjectileState{
			Template:       tpl,
			VelocityUnitX:  math.Sqrt(0.5),
			VelocityUnitY:  math.Sqrt(0.5),
			RemainingRange: 12.5,
		},
	}

	manager := &EffectManager{}
	manager.syncProjectileInstance(instance, owner, effect)

	rawDX, ok := instance.BehaviorState.Extra["dx"]
	if !ok {
		t.Fatalf("expected dx to be stored in BehaviorState.Extra")
	}
	rawDY, ok := instance.BehaviorState.Extra["dy"]
	if !ok {
		t.Fatalf("expected dy to be stored in BehaviorState.Extra")
	}
	if rawDX == 0 || rawDY == 0 {
		t.Fatalf("expected quantized diagonal components, got dx=%d dy=%d", rawDX, rawDY)
	}

	decodedX := DequantizeCoord(rawDX)
	decodedY := DequantizeCoord(rawDY)
	if math.Abs(decodedX-math.Sqrt(0.5)) > 0.05 {
		t.Fatalf("expected decodedX to approximate diagonal, got %.4f", decodedX)
	}
	if math.Abs(decodedY-math.Sqrt(0.5)) > 0.05 {
		t.Fatalf("expected decodedY to approximate diagonal, got %.4f", decodedY)
	}

	world := &World{effectsByID: make(map[string]*effectState)}
	spawned := world.spawnContractProjectileFromInstance(instance, owner, tpl, time.Unix(0, 0))
	if spawned == nil {
		t.Fatalf("expected projectile to spawn")
	}

	if diff := math.Abs(spawned.Projectile.VelocityUnitX - math.Sqrt(0.5)); diff > 0.05 {
		t.Fatalf("expected spawned Projectile.VelocityUnitX to approximate diagonal, diff=%.4f", diff)
	}
	if diff := math.Abs(spawned.Projectile.VelocityUnitY - math.Sqrt(0.5)); diff > 0.05 {
		t.Fatalf("expected spawned Projectile.VelocityUnitY to approximate diagonal, diff=%.4f", diff)
	}

	paramDX := spawned.Effect.Params["dx"]
	paramDY := spawned.Effect.Params["dy"]
	if math.Abs(paramDX-math.Sqrt(0.5)) > 0.05 {
		t.Fatalf("expected effect params dx to approximate diagonal, got %.4f", paramDX)
	}
	if math.Abs(paramDY-math.Sqrt(0.5)) > 0.05 {
		t.Fatalf("expected effect params dy to approximate diagonal, got %.4f", paramDY)
	}
}

func TestSpawnContractProjectileFromInstanceRestoresRemainingRangeAndLifetime(t *testing.T) {
	now := time.Unix(120, 0)
	owner := &actorState{Actor: Actor{ID: "owner", X: 3, Y: 5, Facing: FacingRight}}
	instance := &EffectInstance{
		ID:           "rehydrate-1",
		OwnerActorID: owner.ID,
		BehaviorState: EffectBehaviorState{
			TicksRemaining: tickRate,
			Extra: map[string]int{
				"remainingRange": 15,
				"range":          90,
				"dx":             QuantizeCoord(1),
				"dy":             QuantizeCoord(0),
			},
		},
		DeliveryState: EffectDeliveryState{
			Geometry: EffectGeometry{
				OffsetX: quantizeWorldCoord(1),
				OffsetY: quantizeWorldCoord(0),
			},
		},
	}
	tpl := &ProjectileTemplate{Type: effectTypeFireball, MaxDistance: 90, Speed: 30}
	world := &World{effectsByID: make(map[string]*effectState)}

	spawned := world.spawnContractProjectileFromInstance(instance, owner, tpl, now)
	if spawned == nil {
		t.Fatalf("expected projectile to spawn")
	}

	expectedLifetime := ticksToDuration(instance.BehaviorState.TicksRemaining)
	if expectedLifetime <= 0 {
		t.Fatalf("expected positive lifetime for test")
	}
	if spawned.Duration != expectedLifetime.Milliseconds() {
		t.Fatalf("expected duration %dms, got %dms", expectedLifetime.Milliseconds(), spawned.Duration)
	}
	if spawned.expiresAt.Sub(now) != expectedLifetime {
		t.Fatalf("expected expiresAt to be %s after now, got %s", expectedLifetime, spawned.expiresAt.Sub(now))
	}

	if spawned.Projectile == nil {
		t.Fatalf("expected projectile state to be populated")
	}
	if math.Abs(spawned.Projectile.RemainingRange-15) > 1e-9 {
		t.Fatalf("expected remaining range to be restored, got %.4f", spawned.Projectile.RemainingRange)
	}
	if val := spawned.Params["remainingRange"]; math.Abs(val-15) > 1e-9 {
		t.Fatalf("expected params remainingRange to be 15, got %.4f", val)
	}
}
