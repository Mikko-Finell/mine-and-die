package main

import (
	"context"
	"fmt"
	"math"
	"time"

	"mine-and-die/server/logging"
	loggingconditions "mine-and-die/server/logging/conditions"
)

type ConditionType string

type conditionHandler func(w *World, actor *actorState, inst *conditionInstance, now time.Time)

type ConditionDefinition struct {
	Type         ConditionType
	Duration     time.Duration
	TickInterval time.Duration
	InitialTick  bool
	OnApply      conditionHandler
	OnTick       conditionHandler
	OnExpire     conditionHandler
}

type conditionInstance struct {
	Definition     *ConditionDefinition
	SourceID       string
	AppliedAt      time.Time
	ExpiresAt      time.Time
	NextTick       time.Time
	LastTick       time.Time
	attachedEffect *effectState
}

const (
	ConditionBurning ConditionType = "burning"
)

const (
	burningConditionDuration = 3 * time.Second
	burningTickInterval      = 200 * time.Millisecond
)

func newConditionDefinitions() map[ConditionType]*ConditionDefinition {
	return map[ConditionType]*ConditionDefinition{
		ConditionBurning: {
			Type:         ConditionBurning,
			Duration:     burningConditionDuration,
			TickInterval: burningTickInterval,
			InitialTick:  true,
			OnApply: func(w *World, actor *actorState, inst *conditionInstance, now time.Time) {
				if w == nil || actor == nil || inst == nil {
					return
				}
				lifetime := inst.ExpiresAt.Sub(now)
				if lifetime <= 0 {
					lifetime = burningTickInterval
				}
				inst.attachedEffect = w.attachConditionEffect(actor, "fire", lifetime, now)
			},
			OnTick: func(w *World, actor *actorState, inst *conditionInstance, now time.Time) {
				if w == nil || actor == nil || inst == nil {
					return
				}
				interval := inst.Definition.TickInterval
				if interval <= 0 {
					interval = burningTickInterval
				}
				damage := lavaDamagePerSecond * interval.Seconds()
				w.applyConditionDamage(actor, inst, now, damage)
			},
			OnExpire: func(w *World, actor *actorState, inst *conditionInstance, now time.Time) {
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

func (w *World) applyCondition(target *actorState, cond ConditionType, source string, now time.Time) bool {
	if w == nil || target == nil || cond == "" {
		return false
	}
	def, ok := w.conditionDefs[cond]
	if !ok || def == nil {
		return false
	}
	if def.Duration <= 0 {
		return false
	}
	if target.conditions == nil {
		target.conditions = make(map[ConditionType]*conditionInstance)
	}
	inst, exists := target.conditions[cond]
	if !exists {
		inst = &conditionInstance{
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
		target.conditions[cond] = inst
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
			inst.attachedEffect.Condition = cond
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
			payload := loggingconditions.AppliedPayload{Condition: string(cond), SourceID: source}
			if def.Duration > 0 {
				payload.DurationMs = def.Duration.Milliseconds()
			}
			loggingconditions.Applied(
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

func (w *World) advanceConditions(now time.Time) {
	if w == nil {
		return
	}
	for _, player := range w.players {
		w.advanceActorConditions(&player.actorState, now)
	}
	for _, npc := range w.npcs {
		w.advanceActorConditions(&npc.actorState, now)
	}
}

func (w *World) advanceActorConditions(actor *actorState, now time.Time) {
	if actor == nil || len(actor.conditions) == 0 {
		return
	}
	for key, inst := range actor.conditions {
		if inst == nil || inst.Definition == nil {
			delete(actor.conditions, key)
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
			delete(actor.conditions, key)
			continue
		}
		if inst.attachedEffect != nil {
			w.extendAttachedEffect(inst.attachedEffect, inst.ExpiresAt)
		}
	}
}

func (w *World) applyConditionDamage(actor *actorState, inst *conditionInstance, now time.Time, amount float64) {
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
	eff := &effectState{
		Effect: Effect{
			Type:   effectTypeBurningTick,
			Owner:  owner,
			Start:  now.UnixMilli(),
			Params: map[string]float64{"healthDelta": -amount},
		},
		Condition: inst.Definition.Type,
	}
	w.applyEffectHitActor(eff, actor, now)
}

func (w *World) attachConditionEffect(actor *actorState, effectType string, lifetime time.Duration, now time.Time) *effectState {
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
		expiresAt:     now.Add(lifetime),
		FollowActorID: actor.ID,
	}
	w.effects = append(w.effects, eff)
	w.recordEffectSpawn(effectType, "condition")
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
		w.recordEffectEnd(eff, "condition-expire")
	}
}
