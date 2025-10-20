package server

import (
	"math"
	"testing"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

func TestSyncProjectileInstanceQuantizesDirection(t *testing.T) {
	owner := &actorState{Actor: Actor{ID: "owner", X: 10, Y: 20, Facing: FacingUp}}
	instance := &effectcontract.EffectInstance{
		ID:           "effect-1",
		OwnerActorID: owner.ID,
		BehaviorState: effectcontract.EffectBehaviorState{
			Extra: make(map[string]int),
		},
		DeliveryState: effectcontract.EffectDeliveryState{Geometry: effectcontract.EffectGeometry{}},
		Params:        make(map[string]int),
	}
	tpl := &ProjectileTemplate{Type: effectTypeFireball, MaxDistance: 30}
	effect := &effectState{
		Params: map[string]float64{"radius": 1},
		Projectile: &ProjectileState{
			Template:       tpl,
			VelocityUnitX:  math.Sqrt(0.5),
			VelocityUnitY:  math.Sqrt(0.5),
			RemainingRange: 18,
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

	if math.Abs(spawned.Projectile.RemainingRange-18) > 1e-6 {
		t.Fatalf("expected projectile remaining range to persist, got %.2f", spawned.Projectile.RemainingRange)
	}

	if val := spawned.Params["remainingRange"]; math.Abs(val-18) > 1e-6 {
		t.Fatalf("expected effect params remainingRange to persist, got %.2f", val)
	}

	paramDX := spawned.Params["dx"]
	paramDY := spawned.Params["dy"]
	if math.Abs(paramDX-math.Sqrt(0.5)) > 0.05 {
		t.Fatalf("expected effect params dx to approximate diagonal, got %.4f", paramDX)
	}
	if math.Abs(paramDY-math.Sqrt(0.5)) > 0.05 {
		t.Fatalf("expected effect params dy to approximate diagonal, got %.4f", paramDY)
	}
}

func TestSpawnContractProjectileFromInstancePreservesLifetime(t *testing.T) {
	owner := &actorState{Actor: Actor{ID: "owner", Facing: FacingRight}}
	instance := &effectcontract.EffectInstance{
		ID:           "projectile-life",
		OwnerActorID: owner.ID,
		DefinitionID: effectTypeFireball,
		BehaviorState: effectcontract.EffectBehaviorState{
			TicksRemaining: 5,
			Extra:          make(map[string]int),
		},
	}
	tpl := &ProjectileTemplate{
		Type:        effectTypeFireball,
		Speed:       12,
		MaxDistance: 30,
		Lifetime:    3 * time.Second,
	}

	world := &World{effectsByID: make(map[string]*effectState)}
	now := time.Unix(0, 0)

	spawned := world.spawnContractProjectileFromInstance(instance, owner, tpl, now)
	if spawned == nil {
		t.Fatalf("expected projectile to spawn")
	}

	expectedLifetime := ticksToDuration(instance.BehaviorState.TicksRemaining)
	if expectedLifetime <= 0 {
		t.Fatalf("expected positive lifetime from ticks, got %v", expectedLifetime)
	}

	if spawned.Duration != expectedLifetime.Milliseconds() {
		t.Fatalf("expected duration %dms, got %dms", expectedLifetime.Milliseconds(), spawned.Duration)
	}

	actualLifetime := spawned.expiresAt.Sub(now)
	if actualLifetime != expectedLifetime {
		t.Fatalf("expected expiresAt lifetime %v, got %v", expectedLifetime, actualLifetime)
	}
}
