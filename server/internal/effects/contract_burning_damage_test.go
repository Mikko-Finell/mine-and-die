package effects

import (
	"math"
	"testing"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

type stubStatusInstance struct {
	typ string
}

func (stubStatusInstance) AttachEffect(any) {}

func (s stubStatusInstance) DefinitionType() string {
	return s.typ
}

func TestContractBurningDamageHook_UsesBehaviorExtraDelta(t *testing.T) {
	var (
		called    int
		gotOwner  string
		gotStatus StatusEffectType
		gotDelta  float64
		gotNow    time.Time
	)

	cfg := ContractBurningDamageHookConfig{
		StatusEffect:    StatusEffectType("burning"),
		DamagePerSecond: 20,
		TickInterval:    200 * time.Millisecond,
		LookupActor: func(actorID string) *ContractStatusActor {
			if actorID != "target-actor" {
				t.Fatalf("unexpected actor lookup: %q", actorID)
			}
			return &ContractStatusActor{
				ID: "target-actor",
				StatusInstance: &ContractStatusInstance{
					Instance: stubStatusInstance{typ: "burning-custom"},
				},
				ApplyBurningDamage: func(ownerID string, status StatusEffectType, delta float64, now time.Time) {
					called++
					gotOwner = ownerID
					gotStatus = status
					gotDelta = delta
					gotNow = now
				},
			}
		},
	}

	hook := ContractBurningDamageHook(cfg)
	instance := &effectcontract.EffectInstance{
		OwnerActorID:  "lava-source",
		FollowActorID: "target-actor",
		BehaviorState: effectcontract.EffectBehaviorState{
			Extra: map[string]int{"healthDelta": -7},
		},
	}
	now := time.Unix(123, 0)

	hook.OnSpawn(nil, instance, effectcontract.Tick(1), now)

	if called != 1 {
		t.Fatalf("expected ApplyBurningDamage to be called once, got %d", called)
	}
	if gotOwner != "lava-source" {
		t.Fatalf("expected owner %q, got %q", "lava-source", gotOwner)
	}
	if gotStatus != StatusEffectType("burning-custom") {
		t.Fatalf("expected status %q, got %q", "burning-custom", gotStatus)
	}
	if gotDelta != -7 {
		t.Fatalf("expected delta -7, got %.2f", gotDelta)
	}
	if !gotNow.Equal(now) {
		t.Fatalf("expected timestamp %v, got %v", now, gotNow)
	}
}

func TestContractBurningDamageHook_FallsBackToDefaultDelta(t *testing.T) {
	const tol = 1e-9

	var (
		called    int
		gotStatus StatusEffectType
		gotDelta  float64
	)

	cfg := ContractBurningDamageHookConfig{
		StatusEffect:    StatusEffectType("burning"),
		DamagePerSecond: 12.5,
		TickInterval:    400 * time.Millisecond,
		LookupActor: func(actorID string) *ContractStatusActor {
			if actorID != "attached-actor" {
				t.Fatalf("unexpected actor lookup: %q", actorID)
			}
			return &ContractStatusActor{
				ID: "attached-actor",
				ApplyBurningDamage: func(ownerID string, status StatusEffectType, delta float64, now time.Time) {
					called++
					gotStatus = status
					gotDelta = delta
				},
			}
		},
	}

	hook := ContractBurningDamageHook(cfg)
	instance := &effectcontract.EffectInstance{
		OwnerActorID: "owner-id",
		DeliveryState: effectcontract.EffectDeliveryState{
			AttachedActorID: "attached-actor",
		},
	}

	hook.OnSpawn(nil, instance, effectcontract.Tick(5), time.Unix(0, 0))

	if called != 1 {
		t.Fatalf("expected ApplyBurningDamage to be called once, got %d", called)
	}
	if gotStatus != StatusEffectType("burning") {
		t.Fatalf("expected default status %q, got %q", "burning", gotStatus)
	}
	expected := -cfg.DamagePerSecond * cfg.TickInterval.Seconds()
	if math.Abs(gotDelta-expected) > tol {
		t.Fatalf("expected delta %.6f, got %.6f", expected, gotDelta)
	}
}
