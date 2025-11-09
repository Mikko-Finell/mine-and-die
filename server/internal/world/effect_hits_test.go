package world

import (
	"testing"
	"time"

	worldeffects "mine-and-die/server/internal/world/effects"
	state "mine-and-die/server/internal/world/state"
	statuspkg "mine-and-die/server/internal/world/status"
	"mine-and-die/server/logging"
)

func TestEffectHitPlayerCallbackInvokesAdapter(t *testing.T) {
	called := false
	callback := EffectHitPlayerCallback(EffectHitPlayerConfig{
		ApplyActorHit: func(effect any, target any, now time.Time) {
			called = true
			if effect != "effect" {
				t.Fatalf("expected effect value passed through")
			}
			if target != "target" {
				t.Fatalf("expected target value passed through")
			}
		},
	})

	if callback == nil {
		t.Fatalf("expected callback to be constructed")
	}

	callback("effect", "target", time.UnixMilli(42))

	if !called {
		t.Fatalf("expected adapter to be invoked")
	}
}

func TestEffectHitPlayerCallbackIgnoresNilTarget(t *testing.T) {
	callback := EffectHitPlayerCallback(EffectHitPlayerConfig{
		ApplyActorHit: func(effect any, target any, now time.Time) {
			t.Fatalf("expected adapter not to be called for nil target")
		},
	})

	if callback == nil {
		t.Fatalf("expected callback to be constructed")
	}

	callback("effect", nil, time.UnixMilli(42))
}

func TestEffectHitPlayerCallbackNilAdapter(t *testing.T) {
	if callback := EffectHitPlayerCallback(EffectHitPlayerConfig{}); callback != nil {
		t.Fatalf("expected nil callback when adapter missing")
	}
}

func TestEffectHitNPCCallbackMirrorsLegacyFlow(t *testing.T) {
	spawnCalled := false
	applyCalled := false
	defeatCalled := false
	alive := true

	callback := EffectHitNPCCallback(EffectHitNPCConfig{
		ApplyActorHit: func(effect any, target any, now time.Time) {
			applyCalled = true
			alive = false
		},
		SpawnBlood: func(effect any, target any, now time.Time) {
			spawnCalled = true
		},
		IsAlive: func(target any) bool {
			return alive
		},
		HandleDefeat: func(target any) {
			defeatCalled = true
		},
	})

	if callback == nil {
		t.Fatalf("expected callback to be constructed")
	}

	callback("effect", "target", time.UnixMilli(42))

	if !spawnCalled {
		t.Fatalf("expected SpawnBlood to be invoked before applying damage")
	}
	if !applyCalled {
		t.Fatalf("expected ApplyActorHit to be invoked")
	}
	if !defeatCalled {
		t.Fatalf("expected HandleDefeat to be invoked after defeat")
	}
}

func TestEffectHitNPCCallbackSkipsDefeatWhenStillAlive(t *testing.T) {
	defeatCalled := false

	callback := EffectHitNPCCallback(EffectHitNPCConfig{
		ApplyActorHit: func(effect any, target any, now time.Time) {},
		IsAlive: func(target any) bool {
			return true
		},
		HandleDefeat: func(target any) {
			defeatCalled = true
		},
	})

	if callback == nil {
		t.Fatalf("expected callback to be constructed")
	}

	callback("effect", "target", time.UnixMilli(42))

	if defeatCalled {
		t.Fatalf("expected defeat handler not to run when actor remains alive")
	}
}

func TestEffectHitNPCCallbackNilAdapter(t *testing.T) {
	if callback := EffectHitNPCCallback(EffectHitNPCConfig{}); callback != nil {
		t.Fatalf("expected nil callback when actor adapter missing")
	}
}

func TestEffectHitNPCCallbackIgnoresNilTarget(t *testing.T) {
	callback := EffectHitNPCCallback(EffectHitNPCConfig{
		ApplyActorHit: func(effect any, target any, now time.Time) {
			t.Fatalf("expected adapter not to be called for nil target")
		},
	})

	if callback == nil {
		t.Fatalf("expected callback to be constructed")
	}

	callback("effect", nil, time.UnixMilli(42))
}

func TestNewEffectHitCombatDispatcherDelegates(t *testing.T) {
	now := time.UnixMilli(64)
	effect := &worldeffects.State{
		ID:           "effect-1",
		Type:         "fireball",
		Owner:        "caster-1",
		Params:       map[string]float64{"healthDelta": -10},
		StatusEffect: statuspkg.StatusEffectType("burning"),
	}
	player := &state.PlayerState{ActorState: state.ActorState{Actor: state.Actor{ID: "player-1", Health: 10, MaxHealth: 10}}}

	var (
		setPlayer struct {
			id   string
			next float64
		}
		recordedTelemetry struct {
			effect   *worldeffects.State
			targetID string
			delta    float64
		}
		drop struct {
			actorID string
			reason  string
		}
		statusApplied struct {
			effectID string
			actorID  string
			status   statuspkg.StatusEffectType
			at       time.Time
		}
		lookedUp []string
	)

	dispatcher := NewEffectHitCombatDispatcher(EffectHitCombatDispatcherConfig{
		BaselinePlayerMaxHealth: 100,
		Publisher:               logging.NopPublisher{},
		LookupEntity: func(id string) logging.EntityRef {
			lookedUp = append(lookedUp, id)
			return logging.EntityRef{ID: id}
		},
		CurrentTick: func() uint64 { return 7 },
		SetPlayerHealth: func(id string, next float64) {
			setPlayer.id = id
			setPlayer.next = next
		},
		RecordEffectHitTelemetry: func(eff *worldeffects.State, targetID string, delta float64) {
			recordedTelemetry = struct {
				effect   *worldeffects.State
				targetID string
				delta    float64
			}{effect: eff, targetID: targetID, delta: delta}
		},
		DropAllInventory: func(actor *state.ActorState, reason string) {
			if actor != nil {
				drop.actorID = actor.ID
			}
			drop.reason = reason
		},
		ApplyStatusEffect: func(eff *worldeffects.State, actor *state.ActorState, status statuspkg.StatusEffectType, at time.Time) {
			statusApplied = struct {
				effectID string
				actorID  string
				status   statuspkg.StatusEffectType
				at       time.Time
			}{effectID: eff.ID, actorID: actor.ID, status: status, at: at}
		},
		BuildLegacyAdapter: func(adapterCfg LegacyEffectHitAdapterConfig) EffectHitCallback {
			return func(effect any, target any, now time.Time) {
				eff, ok := adapterCfg.ExtractEffect(effect)
				if !ok || eff == nil {
					return
				}
				actor, ok := adapterCfg.ExtractActor(target)
				if !ok || actor.State == nil {
					return
				}

				delta := eff.Params["healthDelta"]
				if delta == 0 {
					return
				}

				current := actor.State.Health
				next := current + delta
				if next < 0 {
					next = 0
				}
				actualDelta := next - current

				switch actor.Kind {
				case CombatActorKindPlayer:
					if adapterCfg.SetPlayerHealth != nil {
						adapterCfg.SetPlayerHealth(actor.State.ID, next)
					}
				case CombatActorKindNPC:
					if adapterCfg.SetNPCHealth != nil {
						adapterCfg.SetNPCHealth(actor.State.ID, next)
					}
				default:
					if adapterCfg.ApplyGenericHealthDelta != nil {
						adapterCfg.ApplyGenericHealthDelta(actor, delta)
					}
				}

				actor.State.Health = next

				if adapterCfg.RecordEffectHitTelemetry != nil {
					adapterCfg.RecordEffectHitTelemetry(eff, actor.State.ID, actualDelta)
				}

				if actualDelta < 0 && adapterCfg.RecordDamageTelemetry != nil {
					adapterCfg.RecordDamageTelemetry(eff, actor, -actualDelta, next, string(eff.StatusEffect))
				}

				if next == 0 {
					if adapterCfg.DropAllInventory != nil {
						adapterCfg.DropAllInventory(actor, "death")
					}
					if adapterCfg.RecordDefeatTelemetry != nil {
						adapterCfg.RecordDefeatTelemetry(eff, actor, string(eff.StatusEffect))
					}
				}

				if eff.StatusEffect != "" && adapterCfg.ApplyStatusEffect != nil {
					adapterCfg.ApplyStatusEffect(eff, actor, statuspkg.StatusEffectType(eff.StatusEffect), now)
				}
			}
		},
	})

	if dispatcher == nil {
		t.Fatalf("expected dispatcher to be constructed")
	}

	dispatcher(effect, player, now)

	if setPlayer.id != "player-1" || setPlayer.next != 0 {
		t.Fatalf("expected player health setter to be invoked with defeat, got %+v", setPlayer)
	}

	if recordedTelemetry.effect != effect || recordedTelemetry.targetID != "player-1" || recordedTelemetry.delta != -10 {
		t.Fatalf("unexpected hit telemetry: %+v", recordedTelemetry)
	}

	if drop.actorID != "player-1" || drop.reason != "death" {
		t.Fatalf("expected inventory drop on defeat, got %+v", drop)
	}

	if statusApplied.effectID != "effect-1" || statusApplied.actorID != "player-1" {
		t.Fatalf("expected status application to receive effect and actor, got %+v", statusApplied)
	}
	if statusApplied.status != statuspkg.StatusEffectType("burning") || !statusApplied.at.Equal(now) {
		t.Fatalf("unexpected status application payload: %+v", statusApplied)
	}

	checked := make(map[string]bool)
	for _, id := range lookedUp {
		checked[id] = true
	}
	if !checked["caster-1"] || !checked["player-1"] {
		t.Fatalf("expected telemetry lookup for owner and target, got %v", lookedUp)
	}

	generic := &state.ActorState{Actor: state.Actor{ID: "generic", Health: 5, MaxHealth: 5}}
	effect.Params["healthDelta"] = -2

	dispatcher(effect, generic, now)

	if generic.Health != 3 {
		t.Fatalf("expected generic health delta to be applied, got %.1f", generic.Health)
	}
}
