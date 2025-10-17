package main

import (
	"context"
	"fmt"
	"math"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	"mine-and-die/server/logging"
	loggingstatuseffects "mine-and-die/server/logging/status_effects"
)

type StatusEffectType string

type statusEffectHandler func(w *World, actor *actorState, inst *statusEffectInstance, now time.Time)

type StatusEffectDefinition struct {
	Type         StatusEffectType
	Duration     time.Duration
	TickInterval time.Duration
	InitialTick  bool
	OnApply      statusEffectHandler
	OnTick       statusEffectHandler
	OnExpire     statusEffectHandler
}

type statusEffectInstance struct {
	Definition     *StatusEffectDefinition
	SourceID       string
	AppliedAt      time.Time
	ExpiresAt      time.Time
	NextTick       time.Time
	LastTick       time.Time
	attachedEffect *effectState
}

const (
	StatusEffectBurning StatusEffectType = "burning"
)

const (
	burningStatusEffectDuration = 3 * time.Second
	burningTickInterval         = 200 * time.Millisecond
)

func newStatusEffectDefinitions() map[StatusEffectType]*StatusEffectDefinition {
	return map[StatusEffectType]*StatusEffectDefinition{
		StatusEffectBurning: {
			Type:         StatusEffectBurning,
			Duration:     burningStatusEffectDuration,
			TickInterval: burningTickInterval,
			InitialTick:  true,
			OnApply: func(w *World, actor *actorState, inst *statusEffectInstance, now time.Time) {
				if w == nil || actor == nil || inst == nil {
					return
				}
				lifetime := inst.ExpiresAt.Sub(now)
				if lifetime <= 0 {
					lifetime = burningTickInterval
				}
				inst.attachedEffect = nil
				if w.effectManager != nil {
					if intent, ok := NewStatusVisualIntent(actor, inst.SourceID, effectTypeBurningVisual, lifetime); ok {
						w.effectManager.EnqueueIntent(intent)
					}
				}
			},
			OnTick: func(w *World, actor *actorState, inst *statusEffectInstance, now time.Time) {
				if w == nil || actor == nil || inst == nil {
					return
				}
				interval := inst.Definition.TickInterval
				if interval <= 0 {
					interval = burningTickInterval
				}
				damage := lavaDamagePerSecond * interval.Seconds()
				w.applyStatusEffectDamage(actor, inst, now, damage)
			},
			OnExpire: func(w *World, actor *actorState, inst *statusEffectInstance, now time.Time) {
				if w == nil || inst == nil {
					return
				}
				if inst.attachedEffect != nil {
					w.expireAttachedEffect(inst.attachedEffect, now)
					inst.attachedEffect = nil
				}
			},
		},
	}
}

func (w *World) applyStatusEffect(target *actorState, cond StatusEffectType, source string, now time.Time) bool {
	if w == nil || target == nil || cond == "" {
		return false
	}
	def, ok := w.statusEffectDefs[cond]
	if !ok || def == nil {
		return false
	}
	if def.Duration <= 0 {
		return false
	}
	if target.statusEffects == nil {
		target.statusEffects = make(map[StatusEffectType]*statusEffectInstance)
	}
	inst, exists := target.statusEffects[cond]
	if !exists {
		inst = &statusEffectInstance{
			Definition: def,
			SourceID:   source,
			AppliedAt:  now,
			ExpiresAt:  now.Add(def.Duration),
		}
		if def.TickInterval > 0 {
			if def.InitialTick {
				inst.NextTick = now
			} else {
				inst.NextTick = now.Add(def.TickInterval)
			}
		}
		target.statusEffects[cond] = inst
		if def.OnApply != nil {
			def.OnApply(w, target, inst, now)
		}
		if def.InitialTick && def.OnTick != nil {
			def.OnTick(w, target, inst, now)
			inst.LastTick = now
			if def.TickInterval > 0 {
				inst.NextTick = now.Add(def.TickInterval)
			}
		}
		if inst.attachedEffect != nil {
			inst.attachedEffect.StatusEffect = cond
		}
		if w != nil {
			actorRef := logging.EntityRef{}
			if source != "" {
				actorRef = w.entityRef(source)
			}
			targetRef := logging.EntityRef{}
			if target != nil {
				targetRef = w.entityRef(target.ID)
			}
			payload := loggingstatuseffects.AppliedPayload{StatusEffect: string(cond), SourceID: source}
			if def.Duration > 0 {
				payload.DurationMs = def.Duration.Milliseconds()
			}
			loggingstatuseffects.Applied(
				context.Background(),
				w.publisher,
				w.currentTick,
				actorRef,
				targetRef,
				payload,
				nil,
			)
		}
		return true
	}

	inst.SourceID = source
	inst.ExpiresAt = now.Add(def.Duration)
	if inst.Definition == nil {
		inst.Definition = def
	}
	if def.TickInterval > 0 && inst.NextTick.IsZero() {
		inst.NextTick = now.Add(def.TickInterval)
	}
	if inst.attachedEffect != nil {
		w.extendAttachedEffect(inst.attachedEffect, inst.ExpiresAt)
	}
	return false
}

func (w *World) advanceStatusEffects(now time.Time) {
	if w == nil {
		return
	}
	for _, player := range w.players {
		w.advanceActorStatusEffects(&player.actorState, now)
	}
	for _, npc := range w.npcs {
		w.advanceActorStatusEffects(&npc.actorState, now)
	}
}

func (w *World) advanceActorStatusEffects(actor *actorState, now time.Time) {
	if actor == nil || len(actor.statusEffects) == 0 {
		return
	}
	for key, inst := range actor.statusEffects {
		if inst == nil || inst.Definition == nil {
			delete(actor.statusEffects, key)
			continue
		}
		def := inst.Definition
		if def.TickInterval > 0 && !inst.NextTick.IsZero() {
			for !now.Before(inst.NextTick) {
				if inst.NextTick.After(inst.ExpiresAt) {
					break
				}
				tickAt := inst.NextTick
				if def.OnTick != nil {
					def.OnTick(w, actor, inst, tickAt)
				}
				inst.LastTick = tickAt
				inst.NextTick = inst.NextTick.Add(def.TickInterval)
				if inst.NextTick.Equal(inst.LastTick) {
					break
				}
			}
		}
		if !now.Before(inst.ExpiresAt) {
			if def.OnExpire != nil {
				def.OnExpire(w, actor, inst, now)
			} else if inst.attachedEffect != nil {
				w.expireAttachedEffect(inst.attachedEffect, now)
				inst.attachedEffect = nil
			}
			delete(actor.statusEffects, key)
			continue
		}
		if inst.attachedEffect != nil {
			w.extendAttachedEffect(inst.attachedEffect, inst.ExpiresAt)
		}
	}
}

func (w *World) applyStatusEffectDamage(actor *actorState, inst *statusEffectInstance, now time.Time, amount float64) {
	if w == nil || actor == nil || inst == nil {
		return
	}
	if amount <= 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
		return
	}
	owner := inst.SourceID
	if owner == "" {
		owner = actor.ID
	}
	delta := -amount
	if w.effectManager != nil {
		if intent, ok := NewBurningTickIntent(actor, owner, delta); ok {
			w.effectManager.EnqueueIntent(intent)
		}
		return
	}
	statusType := StatusEffectBurning
	if inst != nil && inst.Definition != nil {
		statusType = inst.Definition.Type
	}
	w.applyBurningDamage(owner, actor, statusType, delta, now)
}

func (w *World) applyBurningDamage(owner string, actor *actorState, status StatusEffectType, delta float64, now time.Time) {
	if w == nil || actor == nil {
		return
	}
	if delta >= 0 || math.IsNaN(delta) || math.IsInf(delta, 0) {
		return
	}
	eff := &effectState{
		Effect: Effect{
			Type:   effectTypeBurningTick,
			Owner:  owner,
			Start:  now.UnixMilli(),
			Params: map[string]float64{"healthDelta": delta},
		},
		StatusEffect:       status,
		telemetrySpawnTick: effectcontract.Tick(int64(w.currentTick)),
	}
	if eff.Effect.Owner == "" {
		eff.Effect.Owner = actor.ID
	}
	w.applyEffectHitActor(eff, actor, now)
	w.flushEffectTelemetry(eff)
}

func (w *World) attachStatusEffectVisual(actor *actorState, effectType string, lifetime time.Duration, now time.Time) *effectState {
	if w == nil || actor == nil || effectType == "" {
		return nil
	}
	if lifetime <= 0 {
		lifetime = 100 * time.Millisecond
	}
	w.pruneEffects(now)
	w.nextEffectID++
	width := playerHalf * 2
	height := playerHalf * 2
	start := now.UnixMilli()
	eff := &effectState{
		Effect: Effect{
			ID:       fmt.Sprintf("effect-%d", w.nextEffectID),
			Type:     effectType,
			Owner:    actor.ID,
			Start:    start,
			Duration: lifetime.Milliseconds(),
			X:        actor.X - width/2,
			Y:        actor.Y - height/2,
			Width:    width,
			Height:   height,
		},
		expiresAt:          now.Add(lifetime),
		FollowActorID:      actor.ID,
		telemetrySpawnTick: effectcontract.Tick(int64(w.currentTick)),
	}
	if !w.registerEffect(eff) {
		return nil
	}
	w.recordEffectSpawn(effectType, "status-effect")
	return eff
}

func (w *World) extendAttachedEffect(eff *effectState, expiresAt time.Time) {
	if eff == nil {
		return
	}
	if expiresAt.Before(eff.expiresAt) {
		return
	}
	eff.expiresAt = expiresAt
	start := time.UnixMilli(eff.Effect.Start)
	if eff.Effect.Start == 0 {
		start = expiresAt
	}
	duration := expiresAt.Sub(start)
	if duration < 0 {
		duration = 0
	}
	eff.Effect.Duration = duration.Milliseconds()
}

func (w *World) expireAttachedEffect(eff *effectState, now time.Time) {
	if eff == nil {
		return
	}
	shouldRecord := !eff.telemetryEnded
	if now.Before(eff.expiresAt) {
		eff.expiresAt = now
	}
	start := time.UnixMilli(eff.Effect.Start)
	if eff.Effect.Start == 0 {
		start = now
	}
	duration := now.Sub(start)
	if duration < 0 {
		duration = 0
	}
	eff.Effect.Duration = duration.Milliseconds()
	if shouldRecord {
		w.recordEffectEnd(eff, "status-effect-expire")
	}
}
