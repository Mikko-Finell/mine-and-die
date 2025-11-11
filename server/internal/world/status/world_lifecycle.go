package status

import (
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	worldeffects "mine-and-die/server/internal/world/effects"
	worldstate "mine-and-die/server/internal/world/state"
)

// BurningLifecycleConfig wires the callbacks required to operate the burning
// status effect using internal world state. Callers supply the contract visual
// adapters, fallback attachment helpers, and damage application closures while
// this package drives the lifecycle sequencing.
type BurningLifecycleConfig struct {
	StatusEffect     StatusEffectType
	TickInterval     time.Duration
	DefaultLifetime  time.Duration
	VisualEffectType string
	VisualFootprint  float64
	DamagePerSecond  float64

	BuildContractVisualIntent func(BurningContractVisualConfig) (effectcontract.EffectIntent, bool)
	EnqueueIntent             func(effectcontract.EffectIntent)

	PruneEffects     func(time.Time)
	AllocateEffectID func() string
	CurrentTick      func() effectcontract.Tick

	ApplyDamage func(BurningDamageConfig)
}

// BurningContractVisualConfig describes the request to build a contract-managed
// burning visual intent.
type BurningContractVisualConfig struct {
	Actor    *worldstate.ActorState
	SourceID string
	Lifetime time.Duration
	Now      time.Time
}

// BurningFallbackVisualConfig captures the data required to attach a fallback
// status visual when the contract manager is unavailable.
type BurningFallbackVisualConfig struct {
	Handle          StatusEffectInstanceHandle
	Actor           *worldstate.ActorState
	Status          StatusEffectType
	EffectType      string
	SourceID        string
	Lifetime        time.Duration
	DefaultLifetime time.Duration
	ExpiresAt       time.Time
	Now             time.Time
	Footprint       float64
	Definition      *StatusEffectDefinition

	AllocateEffectID func() string
	CurrentTick      func() effectcontract.Tick
	PruneEffects     func(time.Time)
}

// BurningDamageConfig carries the information required to apply burning damage
// during the status effect tick hook.
type BurningDamageConfig struct {
	Handle     StatusEffectInstanceHandle
	Actor      *worldstate.ActorState
	Instance   *worldstate.StatusEffectInstance
	Status     StatusEffectType
	Damage     float64
	Now        time.Time
	Definition *StatusEffectDefinition
}

func (cfg BurningLifecycleConfig) normalized(def BurningStatusEffectDefinitionConfig, state *StatusEffectDefinition) BurningLifecycleConfig {
	result := cfg

	if result.StatusEffect == "" && def.Type != "" {
		result.StatusEffect = StatusEffectType(def.Type)
	}
	if result.TickInterval <= 0 {
		result.TickInterval = def.TickInterval
	}
	if result.DefaultLifetime <= 0 {
		result.DefaultLifetime = def.TickInterval
	}
	if result.DefaultLifetime <= 0 {
		result.DefaultLifetime = 100 * time.Millisecond
	}
	if result.VisualEffectType == "" {
		result.VisualEffectType = def.Type
	}
	if result.VisualFootprint <= 0 {
		result.VisualFootprint = 1
	}
	if result.DamagePerSecond < 0 {
		result.DamagePerSecond = 0
	}
	return result
}

func newBurningLifecycleApplyHook(cfg BurningLifecycleConfig, state *StatusEffectDefinition) func(StatusEffectApplyRuntime) {
	return func(rt StatusEffectApplyRuntime) {
		handle := rt.Handle
		if handle.Attachment.Clear != nil {
			handle.Attachment.Clear()
		}

		lifetime := cfg.TickInterval
		if lifetime <= 0 {
			lifetime = cfg.DefaultLifetime
		}

		expiresAt := time.Time{}
		if handle.ExpiresAt != nil {
			expiresAt = handle.ExpiresAt()
		}
		if expiresAt.IsZero() {
			expiresAt = rt.Now.Add(lifetime)
		}

		sourceID := ""
		if handle.SourceID != nil {
			sourceID = handle.SourceID()
		}

		actor, _ := handle.Actor().(*worldstate.ActorState)
		if actor == nil {
			return
		}

		if cfg.BuildContractVisualIntent != nil && cfg.EnqueueIntent != nil {
			if intent, ok := cfg.BuildContractVisualIntent(BurningContractVisualConfig{
				Actor:    actor,
				SourceID: sourceID,
				Lifetime: expiresAt.Sub(rt.Now),
				Now:      rt.Now,
			}); ok {
				cfg.EnqueueIntent(intent)
				return
			}
		}

		attachFallbackStatusEffectVisual(BurningFallbackVisualConfig{
			Handle:           handle,
			Actor:            actor,
			Status:           cfg.StatusEffect,
			EffectType:       cfg.VisualEffectType,
			SourceID:         sourceID,
			Lifetime:         lifetime,
			DefaultLifetime:  cfg.DefaultLifetime,
			ExpiresAt:        expiresAt,
			Now:              rt.Now,
			Footprint:        cfg.VisualFootprint,
			Definition:       state,
			AllocateEffectID: cfg.AllocateEffectID,
			CurrentTick:      cfg.CurrentTick,
			PruneEffects:     cfg.PruneEffects,
		})
	}
}

func newBurningLifecycleTickHook(cfg BurningLifecycleConfig, state *StatusEffectDefinition) func(StatusEffectTickRuntime) {
	return func(rt StatusEffectTickRuntime) {
		if cfg.ApplyDamage == nil || cfg.DamagePerSecond <= 0 {
			return
		}

		handle := rt.Handle
		inst, _ := handle.Instance.(*worldstate.StatusEffectInstance)
		if inst == nil {
			return
		}

		actor, _ := handle.Actor().(*worldstate.ActorState)
		if actor == nil {
			return
		}

		interval := cfg.TickInterval
		if definition, _ := inst.Definition.(*StatusEffectDefinition); definition != nil && definition.TickInterval > 0 {
			interval = definition.TickInterval
		}
		if interval <= 0 {
			interval = cfg.TickInterval
		}
		if interval <= 0 {
			interval = cfg.DefaultLifetime
		}
		if interval <= 0 {
			return
		}

		damage := cfg.DamagePerSecond * interval.Seconds()
		if damage <= 0 {
			return
		}

		cfg.ApplyDamage(BurningDamageConfig{
			Handle:     handle,
			Actor:      actor,
			Instance:   inst,
			Status:     cfg.StatusEffect,
			Damage:     damage,
			Now:        rt.Now,
			Definition: state,
		})
	}
}

func attachFallbackStatusEffectVisual(cfg BurningFallbackVisualConfig) *worldeffects.State {
	handle := cfg.Handle
	if handle.Instance == nil || cfg.Status == "" || cfg.EffectType == "" {
		return nil
	}

	inst, _ := handle.Instance.(*worldstate.StatusEffectInstance)
	if inst == nil {
		return nil
	}

	actor := cfg.Actor
	if actor == nil {
		candidate, _ := handle.Actor().(*worldstate.ActorState)
		actor = candidate
	}
	if actor == nil {
		return nil
	}

	if handle.SetActor != nil {
		handle.SetActor(actor)
	}
	if cfg.PruneEffects != nil {
		cfg.PruneEffects(cfg.Now)
	}

	lifetime := cfg.Lifetime
	if lifetime <= 0 {
		lifetime = cfg.DefaultLifetime
	}
	if lifetime <= 0 {
		lifetime = 100 * time.Millisecond
	}

	expiresAt := cfg.ExpiresAt
	if expiresAt.IsZero() {
		expiresAt = cfg.Now.Add(lifetime)
	}

	var id string
	if cfg.AllocateEffectID != nil {
		id = cfg.AllocateEffectID()
	}

	footprint := cfg.Footprint
	if footprint <= 0 {
		footprint = 1
	}

	startTick := effectcontract.Tick(0)
	if cfg.CurrentTick != nil {
		startTick = cfg.CurrentTick()
	}

	duration := expiresAt.Sub(cfg.Now)
	if duration <= 0 {
		duration = lifetime
		expiresAt = cfg.Now.Add(duration)
	}

	owner := cfg.SourceID
	if owner == "" {
		owner = actor.ID
	}

	effect := &worldeffects.State{
		ID:        id,
		Type:      cfg.EffectType,
		Owner:     owner,
		Start:     cfg.Now.UnixMilli(),
		Duration:  duration.Milliseconds(),
		X:         actor.X - footprint/2,
		Y:         actor.Y - footprint/2,
		Width:     footprint,
		Height:    footprint,
		Instance:  effectcontract.EffectInstance{ID: id, DefinitionID: cfg.EffectType, OwnerActorID: actor.ID, StartTick: startTick},
		ExpiresAt: expiresAt,

		FollowActorID:      actor.ID,
		StatusEffect:       worldeffects.StatusEffectType(cfg.Status),
		TelemetrySpawnTick: startTick,
	}

	attach := cfg.Definition.AttachVisual
	if attach == nil {
		attach = AttachStatusEffectVisual
	}

	attach(AttachStatusEffectVisualConfig{
		Instance:    inst,
		Effect:      statusEffectVisualAdapter{state: effect},
		DefaultType: string(cfg.Status),
	})

	if handle.Attachment.SetStatus != nil {
		handle.Attachment.SetStatus(string(cfg.Status))
	}
	if handle.Attachment.Extend != nil {
		handle.Attachment.Extend(expiresAt)
	}

	return effect
}
