package world

import (
	"context"
	"time"

	internaleffects "mine-and-die/server/internal/effects"
	worldeffects "mine-and-die/server/internal/world/effects"
	state "mine-and-die/server/internal/world/state"
	statuspkg "mine-and-die/server/internal/world/status"
	"mine-and-die/server/logging"
	loggingcombat "mine-and-die/server/logging/combat"
)

// EffectHitCallback applies an effect's hit to a target actor. The effect and
// target parameters remain opaque so callers can adapt their legacy structs
// without exposing them to the world package.
type EffectHitCallback func(effect any, target any, now time.Time)

// EffectHitPlayerConfig bundles the dependencies required to apply a hit to a
// player through a shared callback.
type EffectHitPlayerConfig struct {
	// ApplyActorHit mutates the target actor with the effect's hit logic.
	ApplyActorHit EffectHitCallback
}

// EffectHitPlayerCallback constructs a callback that applies effect hits to
// players while guarding against nil targets and missing adapters.
func EffectHitPlayerCallback(cfg EffectHitPlayerConfig) EffectHitCallback {
	if cfg.ApplyActorHit == nil {
		return nil
	}

	return func(effect any, target any, now time.Time) {
		if target == nil {
			return
		}
		cfg.ApplyActorHit(effect, target, now)
	}
}

// EffectHitNPCConfig bundles the dependencies required to apply a hit to an NPC
// while reproducing the legacy side effects that accompany damage resolution.
type EffectHitNPCConfig struct {
	// ApplyActorHit mutates the target actor with the effect's hit logic.
	ApplyActorHit EffectHitCallback
	// SpawnBlood emits any contract-managed visuals associated with the hit.
	SpawnBlood EffectHitCallback
	// IsAlive reports whether the NPC is currently alive.
	IsAlive func(target any) bool
	// HandleDefeat cleans up NPC state after the actor is defeated.
	HandleDefeat func(target any)
}

// EffectHitNPCCallback constructs a callback that mirrors the legacy NPC hit
// flow, spawning blood visuals before applying damage and invoking the defeat
// handler when the actor transitions from alive to defeated.
func EffectHitNPCCallback(cfg EffectHitNPCConfig) EffectHitCallback {
	if cfg.ApplyActorHit == nil {
		return nil
	}

	return func(effect any, target any, now time.Time) {
		if target == nil {
			return
		}

		if cfg.SpawnBlood != nil {
			cfg.SpawnBlood(effect, target, now)
		}

		wasAlive := false
		if cfg.IsAlive != nil {
			wasAlive = cfg.IsAlive(target)
		}

		cfg.ApplyActorHit(effect, target, now)

		if !wasAlive || cfg.HandleDefeat == nil || cfg.IsAlive == nil {
			return
		}

		if cfg.IsAlive(target) {
			return
		}

		cfg.HandleDefeat(target)
	}
}

// EffectHitAdaptersConfig bundles the dispatcher and callback options required to
// wire the effect hit adapters to world state.
type EffectHitAdaptersConfig struct {
	Combat EffectHitCombatDispatcherConfig
	NPC    EffectHitNPCConfig
}

// ConfigureEffectHitAdapters wires the effect hit dispatcher and NPC/player
// callbacks using the provided configuration. Returns false when the dispatcher
// could not be constructed.
func (w *World) ConfigureEffectHitAdapters(cfg EffectHitAdaptersConfig) bool {
	if w == nil {
		return false
	}

	dispatcher := NewEffectHitCombatDispatcher(cfg.Combat)
	if dispatcher == nil {
		w.effectHitDispatcher = nil
		w.playerEffectHitCallback = nil
		w.npcEffectHitCallback = nil
		return false
	}

	w.effectHitDispatcher = dispatcher
	w.playerEffectHitCallback = EffectHitPlayerCallback(EffectHitPlayerConfig{
		ApplyActorHit: dispatcher,
	})

	npcCfg := cfg.NPC
	npcCfg.ApplyActorHit = dispatcher
	w.npcEffectHitCallback = EffectHitNPCCallback(npcCfg)
	return true
}

// EffectHitCombatDispatcherConfig bundles the adapters required to construct the
// combat dispatcher backed by world state. Callers provide the telemetry and
// mutation helpers so the dispatcher can mirror the legacy effect hit flow while
// running inside the internal world package.
type EffectHitCombatDispatcherConfig struct {
	HealthEpsilon           float64
	BaselinePlayerMaxHealth float64

	Publisher    logging.Publisher
	LookupEntity func(id string) logging.EntityRef
	CurrentTick  func() uint64

	SetPlayerHealth func(id string, next float64)
	SetNPCHealth    func(id string, next float64)

	ApplyGenericHealthDelta func(actor *state.ActorState, delta float64) (changed bool, actualDelta float64, newHealth float64)

	RecordEffectHitTelemetry func(effect *worldeffects.State, targetID string, actualDelta float64)
	DropAllInventory         func(actor *state.ActorState, reason string)
	ApplyStatusEffect        func(effect *worldeffects.State, actor *state.ActorState, status statuspkg.StatusEffectType, now time.Time)

	BuildLegacyAdapter LegacyEffectHitAdapterBuilder
}

// LegacyEffectHitAdapterBuilder constructs a dispatcher using the supplied
// configuration. Callers wrap existing combat helpers to bridge the world
// adapters without creating an import cycle.
type LegacyEffectHitAdapterBuilder func(LegacyEffectHitAdapterConfig) EffectHitCallback

// LegacyEffectHitAdapterConfig enumerates the closures required by the legacy
// combat dispatcher. The configuration is intentionally world-centric so
// callers can translate it into their preferred combat bindings.
type LegacyEffectHitAdapterConfig struct {
	HealthEpsilon           float64
	BaselinePlayerMaxHealth float64

	ExtractEffect func(effect any) (*worldeffects.State, bool)
	ExtractActor  func(target any) (CombatActorData, bool)

	SetPlayerHealth         func(id string, next float64)
	SetNPCHealth            func(id string, next float64)
	ApplyGenericHealthDelta func(actor CombatActorData, delta float64) (changed bool, actualDelta float64, newHealth float64)

	RecordEffectHitTelemetry func(effect *worldeffects.State, targetID string, actualDelta float64)
	RecordDamageTelemetry    func(effect *worldeffects.State, target CombatActorData, damage float64, targetHealth float64, statusEffect string)
	RecordDefeatTelemetry    func(effect *worldeffects.State, target CombatActorData, statusEffect string)

	DropAllInventory  func(actor CombatActorData, reason string)
	ApplyStatusEffect func(effect *worldeffects.State, actor CombatActorData, status statuspkg.StatusEffectType, now time.Time)
}

// CombatActorKind identifies the classification of the target actor for hit
// resolution.
type CombatActorKind int

const (
	CombatActorKindUnknown CombatActorKind = iota
	CombatActorKindPlayer
	CombatActorKindNPC
	CombatActorKindGeneric
)

// CombatActorData captures the metadata required to adapt world actors into the
// combat dispatcher while retaining access to the original target reference.
type CombatActorData struct {
	State  *state.ActorState
	Kind   CombatActorKind
	Target any
}

// NewEffectHitCombatDispatcher constructs the combat dispatcher bound to world
// state by wiring the provided adapters into the shared combat helper. The
// returned callback matches the contract used by the effect manager hooks.
func NewEffectHitCombatDispatcher(cfg EffectHitCombatDispatcherConfig) EffectHitCallback {
	if cfg.BuildLegacyAdapter == nil {
		return nil
	}

	adapterCfg := LegacyEffectHitAdapterConfig{
		HealthEpsilon:           normalizedHealthEpsilon(cfg.HealthEpsilon),
		BaselinePlayerMaxHealth: cfg.BaselinePlayerMaxHealth,
		ExtractEffect:           extractWorldEffect,
		ExtractActor:            extractCombatActor,
		SetPlayerHealth: func(id string, next float64) {
			if cfg.SetPlayerHealth == nil || id == "" {
				return
			}
			cfg.SetPlayerHealth(id, next)
		},
		SetNPCHealth: func(id string, next float64) {
			if cfg.SetNPCHealth == nil || id == "" {
				return
			}
			cfg.SetNPCHealth(id, next)
		},
		ApplyGenericHealthDelta: func(actor CombatActorData, delta float64) (bool, float64, float64) {
			if actor.State == nil {
				return false, 0, 0
			}
			if cfg.ApplyGenericHealthDelta != nil {
				return cfg.ApplyGenericHealthDelta(actor.State, delta)
			}
			before := actor.State.Health
			if !actor.State.ApplyHealthDelta(delta) {
				return false, 0, before
			}
			return true, actor.State.Health - before, actor.State.Health
		},
		RecordEffectHitTelemetry: func(effect *worldeffects.State, targetID string, actualDelta float64) {
			if cfg.RecordEffectHitTelemetry == nil {
				return
			}
			cfg.RecordEffectHitTelemetry(effect, targetID, actualDelta)
		},
		RecordDamageTelemetry: newDamageTelemetryRecorder(cfg.Publisher, cfg.LookupEntity, cfg.CurrentTick),
		RecordDefeatTelemetry: newDefeatTelemetryRecorder(cfg.Publisher, cfg.LookupEntity, cfg.CurrentTick),
		DropAllInventory: func(actor CombatActorData, reason string) {
			if cfg.DropAllInventory == nil || actor.State == nil {
				return
			}
			cfg.DropAllInventory(actor.State, reason)
		},
		ApplyStatusEffect: func(effect *worldeffects.State, actor CombatActorData, status statuspkg.StatusEffectType, now time.Time) {
			if cfg.ApplyStatusEffect == nil || actor.State == nil {
				return
			}
			cfg.ApplyStatusEffect(effect, actor.State, status, now)
		},
	}

	return cfg.BuildLegacyAdapter(adapterCfg)
}

func normalizedHealthEpsilon(epsilon float64) float64 {
	if epsilon > 0 {
		return epsilon
	}
	return HealthEpsilon
}

func extractWorldEffect(effect any) (*worldeffects.State, bool) {
	if state, ok := effect.(*worldeffects.State); ok && state != nil {
		return state, true
	}
	if state, ok := effect.(*internaleffects.State); ok && state != nil {
		return (*worldeffects.State)(state), true
	}
	return nil, false
}

func extractCombatActor(target any) (CombatActorData, bool) {
	actor, kind, raw := resolveActorData(target)
	if actor.State == nil || actor.State.ID == "" {
		return CombatActorData{}, false
	}
	actor.Kind = kind
	actor.Target = raw
	return actor, true
}

func resolveActorData(target any) (CombatActorData, CombatActorKind, any) {
	switch typed := target.(type) {
	case *state.PlayerState:
		if typed == nil {
			return CombatActorData{}, CombatActorKindUnknown, nil
		}
		return CombatActorData{State: &typed.ActorState}, CombatActorKindPlayer, typed
	case *state.NPCState:
		if typed == nil {
			return CombatActorData{}, CombatActorKindUnknown, nil
		}
		return CombatActorData{State: &typed.ActorState}, CombatActorKindNPC, typed
	case *state.ActorState:
		if typed == nil {
			return CombatActorData{}, CombatActorKindUnknown, nil
		}
		return CombatActorData{State: typed}, CombatActorKindGeneric, typed
	default:
		return CombatActorData{}, CombatActorKindUnknown, nil
	}
}

func newDamageTelemetryRecorder(publisher logging.Publisher, lookup func(id string) logging.EntityRef, currentTick func() uint64) func(effect *worldeffects.State, target CombatActorData, damage float64, targetHealth float64, statusEffect string) {
	if publisher == nil {
		return nil
	}

	entityLookup := lookup
	if entityLookup == nil {
		entityLookup = func(string) logging.EntityRef { return logging.EntityRef{} }
	}

	tick := currentTick
	if tick == nil {
		tick = func() uint64 { return 0 }
	}

	return func(effect *worldeffects.State, target CombatActorData, damage float64, targetHealth float64, statusEffect string) {
		if damage <= 0 || target.State == nil {
			return
		}

		ownerRef := logging.EntityRef{}
		if effect != nil && effect.Owner != "" {
			ownerRef = entityLookup(effect.Owner)
		}

		targetRef := logging.EntityRef{}
		if id := target.State.ID; id != "" {
			targetRef = entityLookup(id)
		}

		payload := loggingcombat.DamagePayload{
			Ability:      "",
			Amount:       damage,
			TargetHealth: targetHealth,
			StatusEffect: statusEffect,
		}
		if effect != nil {
			payload.Ability = effect.Type
		}

		loggingcombat.Damage(
			context.Background(),
			publisher,
			tick(),
			ownerRef,
			targetRef,
			payload,
			nil,
		)
	}
}

func newDefeatTelemetryRecorder(publisher logging.Publisher, lookup func(id string) logging.EntityRef, currentTick func() uint64) func(effect *worldeffects.State, target CombatActorData, statusEffect string) {
	if publisher == nil {
		return nil
	}

	entityLookup := lookup
	if entityLookup == nil {
		entityLookup = func(string) logging.EntityRef { return logging.EntityRef{} }
	}

	tick := currentTick
	if tick == nil {
		tick = func() uint64 { return 0 }
	}

	return func(effect *worldeffects.State, target CombatActorData, statusEffect string) {
		if target.State == nil {
			return
		}

		ownerRef := logging.EntityRef{}
		if effect != nil && effect.Owner != "" {
			ownerRef = entityLookup(effect.Owner)
		}

		targetRef := logging.EntityRef{}
		if id := target.State.ID; id != "" {
			targetRef = entityLookup(id)
		}

		ability := ""
		if effect != nil {
			ability = effect.Type
		}

		payload := loggingcombat.DefeatPayload{
			Ability:      ability,
			StatusEffect: statusEffect,
		}

		loggingcombat.Defeat(
			context.Background(),
			publisher,
			tick(),
			ownerRef,
			targetRef,
			payload,
			nil,
		)
	}
}
