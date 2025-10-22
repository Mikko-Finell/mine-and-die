package combat

import (
	"fmt"
	"testing"
	"time"

	internaleffects "mine-and-die/server/internal/effects"
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

func TestNewLegacyWorldEffectHitAdapterDelegates(t *testing.T) {
	state := &internaleffects.State{
		Type:         EffectTypeFireball,
		Owner:        "caster",
		Params:       map[string]float64{"healthDelta": -5},
		StatusEffect: internaleffects.StatusEffectType(StatusEffectBurning),
	}

	actorRaw := struct{ id string }{id: "target"}
	adapterInput := WorldActorAdapter{
		ID:        "target",
		Health:    2,
		MaxHealth: 100,
		KindHint:  ActorKindGeneric,
		Raw:       &actorRaw,
	}

	var (
		appliedHealthDelta  WorldActorAdapter
		telemetryEffect     *internaleffects.State
		telemetryTarget     string
		hitTelemetryCallers []float64
		damageTelemetry     struct {
			effect       EffectRef
			target       ActorRef
			amount       float64
			targetHealth float64
			status       string
		}
		defeatTelemetry struct {
			effect EffectRef
			target ActorRef
			status string
		}
		droppedInventory WorldActorAdapter
		appliedStatus    struct {
			effect *internaleffects.State
			actor  WorldActorAdapter
			status string
			at     time.Time
		}
		playerHealthUpdates []struct {
			id   string
			next float64
		}
	)

	dispatcher := NewLegacyWorldEffectHitAdapter(LegacyWorldEffectHitAdapterConfig{
		HealthEpsilon:           1e-6,
		BaselinePlayerMaxHealth: 100,
		ExtractEffect: func(effect any) (*internaleffects.State, bool) {
			got, _ := effect.(*internaleffects.State)
			return got, got != nil
		},
		ExtractActor: func(target any) (WorldActorAdapter, bool) {
			adapter, _ := target.(WorldActorAdapter)
			return adapter, true
		},
		IsPlayer: func(id string) bool {
			return id == "target"
		},
		SetPlayerHealth: func(id string, next float64) {
			playerHealthUpdates = append(playerHealthUpdates, struct {
				id   string
				next float64
			}{id: id, next: next})
		},
		ApplyGenericHealthDelta: func(actor WorldActorAdapter, delta float64) (bool, float64, float64) {
			appliedHealthDelta = actor
			return true, delta, actor.Health + delta
		},
		RecordEffectHitTelemetry: func(effect *internaleffects.State, targetID string, actualDelta float64) {
			telemetryEffect = effect
			telemetryTarget = targetID
			hitTelemetryCallers = append(hitTelemetryCallers, actualDelta)
		},
		RecordDamageTelemetry: func(effect EffectRef, target ActorRef, damage float64, targetHealth float64, status string) {
			damageTelemetry = struct {
				effect       EffectRef
				target       ActorRef
				amount       float64
				targetHealth float64
				status       string
			}{effect: effect, target: target, amount: damage, targetHealth: targetHealth, status: status}
		},
		RecordDefeatTelemetry: func(effect EffectRef, target ActorRef, status string) {
			defeatTelemetry = struct {
				effect EffectRef
				target ActorRef
				status string
			}{effect: effect, target: target, status: status}
		},
		DropAllInventory: func(actor WorldActorAdapter, reason string) {
			if reason != "death" {
				t.Fatalf("unexpected drop reason %q", reason)
			}
			droppedInventory = actor
		},
		ApplyStatusEffect: func(effect *internaleffects.State, actor WorldActorAdapter, status string, at time.Time) {
			appliedStatus = struct {
				effect *internaleffects.State
				actor  WorldActorAdapter
				status string
				at     time.Time
			}{effect: effect, actor: actor, status: status, at: at}
		},
	})

	if dispatcher == nil {
		t.Fatalf("expected dispatcher")
	}

	now := time.UnixMilli(42)
	dispatcher(state, adapterInput, now)

	if len(playerHealthUpdates) == 0 || playerHealthUpdates[0].id != "target" {
		t.Fatalf("expected player health update for target")
	}
	if telemetryEffect != state || telemetryTarget != "target" {
		t.Fatalf("expected hit telemetry for target")
	}
	if damageTelemetry.effect.Effect.OwnerID != "caster" || damageTelemetry.effect.Effect.Type != EffectTypeFireball {
		t.Fatalf("unexpected damage effect telemetry: %+v", damageTelemetry.effect)
	}
	if damageTelemetry.target.Actor.ID != "target" {
		t.Fatalf("unexpected damage target telemetry: %+v", damageTelemetry.target)
	}
	if damageTelemetry.amount != 5 || damageTelemetry.targetHealth != 0 {
		t.Fatalf("unexpected damage values: %+v", damageTelemetry)
	}
	if damageTelemetry.status != StatusEffectBurning {
		t.Fatalf("expected burning status in damage telemetry, got %q", damageTelemetry.status)
	}
	if defeatTelemetry.effect.Effect.OwnerID != "caster" || defeatTelemetry.effect.Effect.Type != EffectTypeFireball {
		t.Fatalf("unexpected defeat effect telemetry: %+v", defeatTelemetry.effect)
	}
	if defeatTelemetry.target.Actor.ID != "target" {
		t.Fatalf("unexpected defeat target telemetry: %+v", defeatTelemetry.target)
	}
	if droppedInventory.Raw != adapterInput.Raw {
		t.Fatalf("expected inventory drop to receive raw actor")
	}
	if appliedStatus.effect != state || appliedStatus.actor.Raw != adapterInput.Raw || appliedStatus.status != StatusEffectBurning {
		t.Fatalf("unexpected status effect application: %+v", appliedStatus)
	}
	if !appliedStatus.at.Equal(now) {
		t.Fatalf("expected status effect time to match now")
	}

	if len(hitTelemetryCallers) != 1 || hitTelemetryCallers[0] != -2 {
		t.Fatalf("unexpected hit telemetry sequence: %v", hitTelemetryCallers)
	}

	appliedHealthDelta = WorldActorAdapter{}
	genericRaw := struct{ id string }{id: "generic"}
	genericAdapter := WorldActorAdapter{
		ID:        "generic",
		Health:    10,
		MaxHealth: 10,
		KindHint:  ActorKindGeneric,
		Raw:       &genericRaw,
	}

	dispatcher(state, genericAdapter, now)

	if appliedHealthDelta.Raw != genericAdapter.Raw {
		t.Fatalf("expected generic health delta to receive raw reference")
	}
	if len(hitTelemetryCallers) != 2 || hitTelemetryCallers[1] != -5 {
		t.Fatalf("unexpected hit telemetry updates after generic call: %v", hitTelemetryCallers)
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
