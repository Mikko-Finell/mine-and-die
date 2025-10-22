package server

import (
	"context"
	"fmt"
	"math"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	combat "mine-and-die/server/internal/combat"
	internaleffects "mine-and-die/server/internal/effects"
	worldpkg "mine-and-die/server/internal/world"
	"mine-and-die/server/logging"
	loggingstatuseffects "mine-and-die/server/logging/status_effects"
)

type StatusEffectType = internaleffects.StatusEffectType

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

func (inst *statusEffectInstance) AttachEffect(value any) {
	if inst == nil {
		return
	}
	eff, ok := value.(*effectState)
	if !ok || eff == nil {
		return
	}
	inst.attachedEffect = eff
}

func (inst *statusEffectInstance) DefinitionType() string {
	if inst == nil || inst.Definition == nil {
		return ""
	}
	return string(inst.Definition.Type)
}

var _ worldpkg.StatusEffectInstance = (*statusEffectInstance)(nil)

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
					if worldpkg.ExpireStatusEffectAttachment(statusEffectAttachmentFields(inst.attachedEffect), now) {
						w.recordEffectEnd(inst.attachedEffect, "status-effect-expire")
					}
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
		worldpkg.ExtendStatusEffectAttachment(statusEffectAttachmentFields(inst.attachedEffect), inst.ExpiresAt)
	}
	return false
}

func (w *World) advanceStatusEffects(now time.Time) {
	if w == nil {
		return
	}

	worldpkg.AdvanceStatusEffects(worldpkg.AdvanceStatusEffectsConfig{
		Now: now,
		ForEachPlayer: func(apply func(worldpkg.AdvanceActorStatusEffectsConfig)) {
			if apply == nil {
				return
			}
			for _, player := range w.players {
				if player == nil {
					continue
				}
				apply(w.statusEffectsAdvanceConfig(&player.actorState))
			}
		},
		ForEachNPC: func(apply func(worldpkg.AdvanceActorStatusEffectsConfig)) {
			if apply == nil {
				return
			}
			for _, npc := range w.npcs {
				if npc == nil {
					continue
				}
				apply(w.statusEffectsAdvanceConfig(&npc.actorState))
			}
		},
	})
}

func (w *World) advanceActorStatusEffects(actor *actorState, now time.Time) {
	if w == nil || actor == nil {
		return
	}
	cfg := w.statusEffectsAdvanceConfig(actor)
	cfg.Now = now
	worldpkg.AdvanceActorStatusEffects(cfg)
}

func (w *World) statusEffectsAdvanceConfig(actor *actorState) worldpkg.AdvanceActorStatusEffectsConfig {
	return worldpkg.AdvanceActorStatusEffectsConfig{
		ForEachInstance: func(visitor func(key string, instance any)) {
			if visitor == nil || actor == nil {
				return
			}
			for key, inst := range actor.statusEffects {
				visitor(string(key), inst)
			}
		},
		Instance: func(value any) (worldpkg.StatusEffectInstanceConfig, bool) {
			inst, _ := value.(*statusEffectInstance)
			if inst == nil {
				return worldpkg.StatusEffectInstanceConfig{}, false
			}
			def := inst.Definition
			if def == nil {
				return worldpkg.StatusEffectInstanceConfig{}, false
			}

			cfg := worldpkg.StatusEffectInstanceConfig{
				Definition: worldpkg.StatusEffectDefinitionCallbacks{
					TickInterval: def.TickInterval,
				},
				NextTick: func() time.Time { return inst.NextTick },
				SetNextTick: func(value time.Time) {
					inst.NextTick = value
				},
				LastTick: func() time.Time { return inst.LastTick },
				SetLastTick: func(value time.Time) {
					inst.LastTick = value
				},
				ExpiresAt: func() time.Time { return inst.ExpiresAt },
			}

			if def.OnTick != nil {
				cfg.Definition.OnTick = func(tickAt time.Time) {
					def.OnTick(w, actor, inst, tickAt)
				}
			}
			if def.OnExpire != nil {
				cfg.Definition.OnExpire = func(expireAt time.Time) {
					def.OnExpire(w, actor, inst, expireAt)
				}
			}

			if inst.attachedEffect != nil {
				cfg.Attachment = &worldpkg.StatusEffectAttachmentConfig{
					Extend: func(expiresAt time.Time) {
						if inst.attachedEffect == nil {
							return
						}
						worldpkg.ExtendStatusEffectAttachment(statusEffectAttachmentFields(inst.attachedEffect), expiresAt)
					},
					Expire: func(at time.Time) (any, bool) {
						if inst.attachedEffect == nil {
							return nil, false
						}
						shouldRecord := worldpkg.ExpireStatusEffectAttachment(statusEffectAttachmentFields(inst.attachedEffect), at)
						return inst.attachedEffect, shouldRecord
					},
					Clear: func() {
						inst.attachedEffect = nil
					},
				}
			}

			return cfg, true
		},
		Remove: func(key string) {
			if actor == nil || actor.statusEffects == nil {
				return
			}
			delete(actor.statusEffects, StatusEffectType(key))
		},
		RecordEffectEnd: func(value any) {
			if w == nil {
				return
			}
			eff, _ := value.(*effectState)
			if eff == nil {
				return
			}
			w.recordEffectEnd(eff, "status-effect-expire")
		},
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
	statusType := StatusEffectBurning
	if inst != nil && inst.Definition != nil {
		statusType = inst.Definition.Type
	}
	delta := -amount
	if w.effectManager != nil {
		if intent, ok := internaleffects.NewBurningTickIntent(internaleffects.BurningTickIntentConfig{
			EffectType:    effectTypeBurningTick,
			TargetActorID: actor.ID,
			SourceActorID: owner,
			StatusEffect:  internaleffects.StatusEffectType(statusType),
			Delta:         delta,
			TileSize:      tileSize,
			Footprint:     playerHalf * 2,
			Now:           now,
			CurrentTick:   w.currentTick,
		}); ok {
			w.effectManager.EnqueueIntent(intent)
		}
		return
	}
	w.applyBurningDamage(owner, actor, statusType, delta, now)
}

func (w *World) applyBurningDamage(owner string, actor *actorState, status StatusEffectType, delta float64, now time.Time) {
	if w == nil || actor == nil {
		return
	}

	callback := combat.NewWorldBurningDamageCallback(combat.WorldBurningDamageCallbackConfig{
		Dispatcher: w.effectHitAdapter,
		Target:     actor,
		Now:        now,
		BuildEffect: func(effect worldpkg.BurningDamageEffect) any {
			return &effectState{
				Type:   effect.EffectType,
				Owner:  effect.OwnerID,
				Start:  effect.StartMillis,
				Params: map[string]float64{"healthDelta": effect.HealthDelta},
				Instance: effectcontract.EffectInstance{
					DefinitionID: effect.EffectType,
					OwnerActorID: effect.OwnerID,
					StartTick:    effect.SpawnTick,
				},
				StatusEffect:       StatusEffectType(effect.StatusEffect),
				TelemetrySpawnTick: effect.SpawnTick,
			}
		},
		AfterApply: func(value any) {
			eff, _ := value.(*effectState)
			if eff == nil {
				return
			}
			w.flushEffectTelemetry(eff)
		},
	})
	if callback == nil {
		return
	}

	worldpkg.ApplyBurningDamage(worldpkg.ApplyBurningDamageConfig{
		EffectType:   effectTypeBurningTick,
		OwnerID:      owner,
		ActorID:      actor.ID,
		StatusEffect: string(status),
		Delta:        delta,
		Now:          now,
		CurrentTick:  w.currentTick,
		Apply:        callback,
	})
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
	instanceID := fmt.Sprintf("effect-%d", w.nextEffectID)
	eff := &effectState{
		ID:                 instanceID,
		Type:               effectType,
		Owner:              actor.ID,
		Start:              start,
		Duration:           lifetime.Milliseconds(),
		X:                  actor.X - width/2,
		Y:                  actor.Y - height/2,
		Width:              width,
		Height:             height,
		Instance:           effectcontract.EffectInstance{ID: instanceID, DefinitionID: effectType, OwnerActorID: actor.ID, StartTick: effectcontract.Tick(int64(w.currentTick))},
		ExpiresAt:          now.Add(lifetime),
		FollowActorID:      actor.ID,
		TelemetrySpawnTick: effectcontract.Tick(int64(w.currentTick)),
	}
	return eff
}

func statusEffectAttachmentFields(eff *effectState) worldpkg.StatusEffectAttachmentFields {
	return worldpkg.StatusEffectAttachmentFields{
		StatusEffectLifetimeFields: worldpkg.StatusEffectLifetimeFields{
			ExpiresAt:      &eff.ExpiresAt,
			StartMillis:    eff.Start,
			DurationMillis: &eff.Duration,
		},
		TelemetryEnded: eff.TelemetryEnded,
	}
}
