package combat

import (
	"fmt"
	"testing"
	"time"

	worldpkg "mine-and-die/server/internal/world"
)

func TestNewWorldEffectHitDispatcherDelegates(t *testing.T) {
	var extractEffectCalled bool
	var extractActorCalled bool
	var setPlayerHealthTarget ActorRef
	var recordDamageCalled bool

	dispatcher := NewWorldEffectHitDispatcher(WorldEffectHitDispatcherConfig{
		ExtractEffect: func(effect any) (EffectRef, bool) {
			extractEffectCalled = true
			eff, _ := effect.(EffectRef)
			return eff, true
		},
		ExtractActor: func(target any) (ActorRef, bool) {
			extractActorCalled = true
			actor, _ := target.(ActorRef)
			return actor, true
		},
		HealthEpsilon:           1e-6,
		BaselinePlayerMaxHealth: 100,
		SetPlayerHealth: func(target ActorRef, next float64) {
			setPlayerHealthTarget = target
		},
		SetNPCHealth: func(ActorRef, float64) {},
		ApplyGenericHealthDelta: func(target ActorRef, delta float64) (bool, float64, float64) {
			return true, delta, target.Actor.Health + delta
		},
		RecordEffectHitTelemetry: func(EffectRef, ActorRef, float64) {},
		RecordDamageTelemetry: func(EffectRef, ActorRef, float64, float64, string) {
			recordDamageCalled = true
		},
		RecordDefeatTelemetry: func(EffectRef, ActorRef, string) {},
		DropAllInventory:      func(ActorRef, string) {},
		ApplyStatusEffect:     func(EffectRef, ActorRef, string, time.Time) {},
	})

	if dispatcher == nil {
		t.Fatalf("expected dispatcher")
	}

	eff := EffectRef{Effect: Effect{Type: EffectTypeAttack, OwnerID: "caster", Params: map[string]float64{"healthDelta": -5}}}
	actor := ActorRef{Actor: Actor{ID: "target", Health: 20, MaxHealth: 100, Kind: ActorKindPlayer}}

	dispatcher(eff, actor, time.UnixMilli(42))

	if !extractEffectCalled {
		t.Fatalf("expected effect extraction")
	}
	if !extractActorCalled {
		t.Fatalf("expected actor extraction")
	}
	if setPlayerHealthTarget.Actor.ID != actor.Actor.ID {
		t.Fatalf("expected player health update, got %q", setPlayerHealthTarget.Actor.ID)
	}
	if !recordDamageCalled {
		t.Fatalf("expected damage telemetry to fire")
	}
}

func TestNewWorldEffectHitDispatcherIgnoresNilInputs(t *testing.T) {
	var called bool
	dispatcher := NewWorldEffectHitDispatcher(WorldEffectHitDispatcherConfig{
		ExtractEffect: func(effect any) (EffectRef, bool) {
			called = true
			return EffectRef{}, true
		},
		ExtractActor: func(target any) (ActorRef, bool) {
			called = true
			return ActorRef{}, true
		},
	})

	if dispatcher == nil {
		t.Fatalf("expected dispatcher")
	}

	dispatcher(nil, struct{}{}, time.Time{})
	dispatcher(struct{}{}, nil, time.Time{})

	if called {
		t.Fatalf("expected dispatcher to ignore nil inputs")
	}
}

func TestApplyEffectHitGuardsNil(t *testing.T) {
	ApplyEffectHit(nil, struct{}{}, struct{}{}, time.Time{})

	var invoked bool
	ApplyEffectHit(func(effect any, target any, now time.Time) {
		invoked = true
	}, struct{}{}, struct{}{}, time.UnixMilli(1))

	if !invoked {
		t.Fatalf("expected callback invocation")
	}

	invoked = false
	ApplyEffectHit(func(effect any, target any, now time.Time) {
		invoked = true
	}, nil, struct{}{}, time.UnixMilli(1))

	if invoked {
		t.Fatalf("expected nil effect to skip callback")
	}

	invoked = false
	ApplyEffectHit(func(effect any, target any, now time.Time) {
		invoked = true
	}, struct{}{}, nil, time.UnixMilli(1))

	if invoked {
		t.Fatalf("expected nil target to skip callback")
	}
}

func TestNewWorldPlayerEffectHitCallbackDelegates(t *testing.T) {
	var dispatcherCalled bool
	callback := NewWorldPlayerEffectHitCallback(WorldPlayerEffectHitCallbackConfig{
		Dispatcher: func(effect any, target any, now time.Time) {
			dispatcherCalled = true
			if now.UnixMilli() != 7 {
				t.Fatalf("unexpected timestamp %d", now.UnixMilli())
			}
		},
	})

	if callback == nil {
		t.Fatalf("expected callback")
	}

	callback(struct{}{}, struct{}{}, time.UnixMilli(7))

	if !dispatcherCalled {
		t.Fatalf("expected dispatcher to be invoked")
	}
}

func TestNewWorldNPCEffectHitCallbackDelegates(t *testing.T) {
	var sequence []string
	alive := true

	callback := NewWorldNPCEffectHitCallback(WorldNPCEffectHitCallbackConfig{
		Dispatcher: func(effect any, target any, now time.Time) {
			sequence = append(sequence, "hit")
			alive = false
		},
		SpawnBlood: func(effect any, target any, now time.Time) {
			sequence = append(sequence, "blood")
		},
		IsAlive: func(target any) bool {
			sequence = append(sequence, "alive")
			return alive
		},
		HandleDefeat: func(target any) {
			sequence = append(sequence, "defeat")
		},
	})

	if callback == nil {
		t.Fatalf("expected callback")
	}

	callback(struct{}{}, struct{}{}, time.UnixMilli(11))

	expected := []string{"blood", "alive", "hit", "alive", "defeat"}
	if diff := cmpSlice(sequence, expected); diff != "" {
		t.Fatalf("unexpected sequence: %s", diff)
	}
}

func TestNewWorldEffectHitCallbacksRequireDispatcher(t *testing.T) {
	if cb := NewWorldPlayerEffectHitCallback(WorldPlayerEffectHitCallbackConfig{}); cb != nil {
		t.Fatalf("expected nil dispatcher to skip player callback")
	}
	if cb := NewWorldNPCEffectHitCallback(WorldNPCEffectHitCallbackConfig{}); cb != nil {
		t.Fatalf("expected nil dispatcher to skip npc callback")
	}
}

func TestNewWorldBurningDamageCallbackDelegates(t *testing.T) {
	target := struct{ id string }{id: "actor"}
	now := time.UnixMilli(17)

	var (
		dispatcherEffect any
		dispatcherTarget any
		dispatcherNow    time.Time
		builderCalled    bool
		afterCalled      bool
	)

	callback := NewWorldBurningDamageCallback(WorldBurningDamageCallbackConfig{
		Dispatcher: func(effect any, target any, at time.Time) {
			dispatcherEffect = effect
			dispatcherTarget = target
			dispatcherNow = at
		},
		Target: &target,
		Now:    now,
		BuildEffect: func(effect worldpkg.BurningDamageEffect) any {
			builderCalled = true
			if effect.EffectType != "burn" {
				t.Fatalf("unexpected effect type %q", effect.EffectType)
			}
			return &struct{ delta float64 }{delta: effect.HealthDelta}
		},
		AfterApply: func(effect any) {
			afterCalled = true
			if effect != dispatcherEffect {
				t.Fatalf("after apply mismatch")
			}
		},
	})

	if callback == nil {
		t.Fatalf("expected callback")
	}

	payload := worldpkg.BurningDamageEffect{EffectType: "burn", OwnerID: "caster", HealthDelta: -3}
	callback(payload)

	if !builderCalled {
		t.Fatalf("expected builder to run")
	}
	if dispatcherEffect == nil {
		t.Fatalf("expected dispatcher to receive effect")
	}
	if dispatcherTarget != &target {
		t.Fatalf("unexpected dispatcher target: %#v", dispatcherTarget)
	}
	if dispatcherNow != now {
		t.Fatalf("unexpected dispatcher time: %v", dispatcherNow)
	}
	if !afterCalled {
		t.Fatalf("expected after apply hook to fire")
	}
}

func TestNewWorldBurningDamageCallbackAllowsNilDispatcher(t *testing.T) {
	var afterCalled bool

	callback := NewWorldBurningDamageCallback(WorldBurningDamageCallbackConfig{
		Target: struct{}{},
		Now:    time.UnixMilli(23),
		BuildEffect: func(effect worldpkg.BurningDamageEffect) any {
			return effect.EffectType
		},
		AfterApply: func(effect any) {
			afterCalled = true
			if effect != "burn" {
				t.Fatalf("unexpected effect %v", effect)
			}
		},
	})

	if callback == nil {
		t.Fatalf("expected callback")
	}

	callback(worldpkg.BurningDamageEffect{EffectType: "burn"})

	if !afterCalled {
		t.Fatalf("expected after apply hook")
	}
}

func TestNewWorldBurningDamageCallbackSkipsNilBuildResult(t *testing.T) {
	var (
		dispatcherCalled bool
		afterCalled      bool
	)

	callback := NewWorldBurningDamageCallback(WorldBurningDamageCallbackConfig{
		Dispatcher: func(effect any, target any, now time.Time) {
			dispatcherCalled = true
		},
		Target: struct{}{},
		Now:    time.UnixMilli(31),
		BuildEffect: func(effect worldpkg.BurningDamageEffect) any {
			return nil
		},
		AfterApply: func(effect any) {
			afterCalled = true
		},
	})

	if callback == nil {
		t.Fatalf("expected callback")
	}

	callback(worldpkg.BurningDamageEffect{EffectType: "burn"})

	if dispatcherCalled {
		t.Fatalf("expected dispatcher to be skipped")
	}
	if afterCalled {
		t.Fatalf("expected after apply to be skipped")
	}
}

func TestNewWorldBurningDamageCallbackRequiresBuilder(t *testing.T) {
	if callback := NewWorldBurningDamageCallback(WorldBurningDamageCallbackConfig{}); callback != nil {
		t.Fatalf("expected nil callback without builder")
	}
}

func cmpSlice[T comparable](got, want []T) string {
	if len(got) != len(want) {
		return fmt.Sprintf("length mismatch: got %d want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			return fmt.Sprintf("mismatch at %d: got %v want %v", i, got[i], want[i])
		}
	}
	return ""
}
