package world

import (
	"math"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	combat "mine-and-die/server/internal/combat"
	internaleffects "mine-and-die/server/internal/effects"
	worldeffects "mine-and-die/server/internal/world/effects"
	state "mine-and-die/server/internal/world/state"
	statuspkg "mine-and-die/server/internal/world/status"
)

const (
	// TickRate defines the simulation ticks per second for the internal world.
	TickRate = 15

	// BurningStatusEffectDuration mirrors the legacy burning status duration.
	BurningStatusEffectDuration = 3 * time.Second
	// BurningTickInterval mirrors the legacy burning status tick cadence.
	BurningTickInterval = 200 * time.Millisecond
	// LavaDamagePerSecond represents the base lava damage applied per second.
	LavaDamagePerSecond = 20.0
)

const statusVisualTileSize = 40.0

func (w *World) configureStatusEffects() {
	if w == nil {
		return
	}

	defs := statuspkg.NewStatusEffectDefinitions(statuspkg.StatusEffectDefinitionsConfig{
		Burning: statuspkg.BurningStatusEffectDefinitionConfig{
			Type:         string(statuspkg.StatusEffectBurning),
			Duration:     BurningStatusEffectDuration,
			TickInterval: BurningTickInterval,
			InitialTick:  true,
			Lifecycle: &statuspkg.BurningLifecycleConfig{
				StatusEffect:              statuspkg.StatusEffectBurning,
				TickInterval:              BurningTickInterval,
				DefaultLifetime:           BurningTickInterval,
				VisualEffectType:          combat.EffectTypeBurningVisual,
				VisualFootprint:           PlayerHalf * 2,
				DamagePerSecond:           LavaDamagePerSecond,
				BuildContractVisualIntent: w.buildBurningVisualIntent,
				EnqueueIntent:             w.enqueueStatusEffectIntent,
				AllocateEffectID:          w.allocateEffectID,
				CurrentTick: func() effectcontract.Tick {
					return effectcontract.Tick(int64(w.currentTick()))
				},
				ApplyDamage: w.applyBurningStatusDamage,
			},
		},
	})

	if len(defs) == 0 {
		return
	}

	for key, def := range defs {
		w.statusEffectDefinitions[key] = def
	}
}

func (w *World) buildBurningVisualIntent(cfg statuspkg.BurningContractVisualConfig) (effectcontract.EffectIntent, bool) {
	actor := cfg.Actor
	if actor == nil || actor.ID == "" {
		return effectcontract.EffectIntent{}, false
	}

	effectType := combat.EffectTypeBurningVisual
	if effectType == "" {
		return effectcontract.EffectIntent{}, false
	}

	lifetime := cfg.Lifetime
	if lifetime <= 0 {
		lifetime = BurningTickInterval
	}

	width := worldeffects.QuantizeWorldCoord(PlayerHalf*2, statusVisualTileSize)
	height := width

	intent := effectcontract.EffectIntent{
		EntryID:       effectType,
		TypeID:        effectType,
		Delivery:      effectcontract.DeliveryKindTarget,
		SourceActorID: cfg.SourceID,
		TargetActorID: actor.ID,
		Geometry: effectcontract.EffectGeometry{
			Shape:   effectcontract.GeometryShapeRect,
			Width:   width,
			Height:  height,
			OffsetX: 0,
			OffsetY: 0,
		},
		DurationTicks: durationToTicks(lifetime, TickRate),
	}

	if intent.DurationTicks < 1 {
		intent.DurationTicks = 1
	}
	if intent.SourceActorID == "" {
		intent.SourceActorID = actor.ID
	}

	return intent, true
}

func (w *World) enqueueStatusEffectIntent(intent effectcontract.EffectIntent) {
	if w == nil || intent.TypeID == "" {
		return
	}
	if w.effectManager == nil {
		return
	}
	w.effectManager.EnqueueIntent(intent)
}

func (w *World) applyBurningStatusDamage(cfg statuspkg.BurningDamageConfig) {
	if w == nil || cfg.Actor == nil || cfg.Instance == nil {
		return
	}
	if cfg.Damage <= 0 || math.IsNaN(cfg.Damage) || math.IsInf(cfg.Damage, 0) {
		return
	}

	owner := cfg.Instance.SourceID
	if owner == "" {
		owner = cfg.Actor.ID
	}

	statusType := cfg.Status
	if statusType == "" {
		if cfg.Definition != nil && cfg.Definition.Type != "" {
			statusType = statuspkg.StatusEffectType(cfg.Definition.Type)
		} else {
			statusType = statuspkg.StatusEffectBurning
		}
	}

	delta := -cfg.Damage
	if w.effectManager != nil {
		if intent, ok := statuspkg.NewBurningTickIntent(statuspkg.BurningTickIntentConfig{
			EffectType:    combat.EffectTypeBurningTick,
			TargetActorID: cfg.Actor.ID,
			SourceActorID: owner,
			StatusEffect:  statusType,
			Delta:         delta,
			TileSize:      statusVisualTileSize,
			Footprint:     PlayerHalf * 2,
			Now:           cfg.Now,
			CurrentTick:   w.currentTick(),
		}); ok {
			w.effectManager.EnqueueIntent(intent)
			return
		}
	}

	w.applyBurningDamage(owner, cfg.Actor, statusType, delta, cfg.Now)
}

func (w *World) applyBurningDamage(owner string, actor *state.ActorState, status statuspkg.StatusEffectType, delta float64, now time.Time) {
	if w == nil || actor == nil {
		return
	}
	if delta >= 0 || math.IsNaN(delta) || math.IsInf(delta, 0) {
		return
	}

	dispatcher := w.EffectHitDispatcher()
	if dispatcher == nil {
		return
	}

	callback := NewWorldBurningDamageCallback(WorldBurningDamageCallbackConfig{
		Dispatcher: dispatcher,
		Target:     actor,
		Now:        now,
		BuildEffect: func(effect statuspkg.BurningDamageEffect) any {
			return &worldeffects.State{
				Type:   effect.EffectType,
				Owner:  effect.OwnerID,
				Start:  effect.StartMillis,
				Params: map[string]float64{"healthDelta": effect.HealthDelta},
				Instance: effectcontract.EffectInstance{
					DefinitionID: effect.EffectType,
					OwnerActorID: effect.OwnerID,
					StartTick:    effect.SpawnTick,
				},
				StatusEffect:       internaleffects.StatusEffectType(effect.StatusEffect),
				TelemetrySpawnTick: effect.SpawnTick,
			}
		},
		AfterApply: func(value any) {
			effect, _ := value.(*worldeffects.State)
			if effect == nil {
				return
			}
			w.flushEffectTelemetry(effect)
		},
	})
	if callback == nil {
		return
	}

	statuspkg.ApplyBurningDamage(statuspkg.ApplyBurningDamageConfig{
		EffectType:   combat.EffectTypeBurningTick,
		OwnerID:      owner,
		ActorID:      actor.ID,
		StatusEffect: string(status),
		Delta:        delta,
		Now:          now,
		CurrentTick:  w.currentTick(),
		Apply:        callback,
	})
}

// ApplyBurningDamage exposes the burning damage helper for legacy callers.
func (w *World) ApplyBurningDamage(owner string, actor *state.ActorState, status statuspkg.StatusEffectType, delta float64, now time.Time) {
	w.applyBurningDamage(owner, actor, status, delta, now)
}

func durationToTicks(duration time.Duration, tickRate int) int {
	if duration <= 0 || tickRate <= 0 {
		return 0
	}
	ticks := int(math.Ceil(duration.Seconds() * float64(tickRate)))
	if ticks < 1 {
		return 1
	}
	return ticks
}
