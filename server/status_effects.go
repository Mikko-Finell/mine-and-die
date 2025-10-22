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

type statusEffectInstance struct {
	Definition     *worldpkg.StatusEffectDefinition
	SourceID       string
	AppliedAt      time.Time
	ExpiresAt      time.Time
	NextTick       time.Time
	LastTick       time.Time
	attachedEffect *effectState
	actor          *actorState
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
	return inst.Definition.Type
}

var _ worldpkg.StatusEffectInstance = (*statusEffectInstance)(nil)

const (
	StatusEffectBurning StatusEffectType = "burning"
)

const (
	burningStatusEffectDuration = 3 * time.Second
	burningTickInterval         = 200 * time.Millisecond
)

func newStatusEffectDefinitions(w *World) map[StatusEffectType]worldpkg.ApplyStatusEffectDefinition {
	if w == nil {
		return nil
	}

	defs := worldpkg.NewStatusEffectDefinitions(worldpkg.StatusEffectDefinitionsConfig{
		Burning: worldpkg.BurningStatusEffectDefinitionConfig{
			Type:               string(StatusEffectBurning),
			Duration:           burningStatusEffectDuration,
			TickInterval:       burningTickInterval,
			InitialTick:        true,
			FallbackAttachment: worldpkg.AttachStatusEffectVisual,
			OnApply: func(rt worldpkg.StatusEffectApplyRuntime) {
				w.handleBurningStatusApply(rt)
			},
			OnTick: func(rt worldpkg.StatusEffectTickRuntime) {
				w.handleBurningStatusTick(rt)
			},
		},
	})

	result := make(map[StatusEffectType]worldpkg.ApplyStatusEffectDefinition, len(defs))
	for key, def := range defs {
		result[StatusEffectType(key)] = def
	}
	return result
}

func (w *World) handleBurningStatusApply(rt worldpkg.StatusEffectApplyRuntime) {
	if w == nil {
		return
	}

	handle := rt.Handle
	handle.Attachment.Clear()

	lifetime := burningTickInterval
	expiresAt := rt.Now.Add(lifetime)
	if handle.ExpiresAt != nil {
		expires := handle.ExpiresAt()
		if !expires.IsZero() {
			expiresAt = expires
			lifetime = expires.Sub(rt.Now)
			if lifetime <= 0 {
				lifetime = burningTickInterval
				expiresAt = rt.Now.Add(lifetime)
			}
		}
	}

	var sourceID string
	if handle.SourceID != nil {
		sourceID = handle.SourceID()
	}

	actor, _ := handle.Actor().(*actorState)

	if w.effectManager == nil {
		if actor == nil {
			return
		}
		w.attachStatusEffectVisual(handle, actor, StatusEffectBurning, sourceID, effectTypeBurningVisual, lifetime, expiresAt, rt.Now)
		return
	}

	if actor == nil {
		return
	}

	if intent, ok := NewStatusVisualIntent(actor, sourceID, effectTypeBurningVisual, lifetime); ok {
		w.effectManager.EnqueueIntent(intent)
	}
}

func (w *World) handleBurningStatusTick(rt worldpkg.StatusEffectTickRuntime) {
	if w == nil {
		return
	}

	handle := rt.Handle
	inst, _ := handle.Instance.(*statusEffectInstance)
	if inst == nil {
		return
	}

	actor, _ := handle.Actor().(*actorState)
	if actor == nil {
		return
	}

	interval := burningTickInterval
	if inst.Definition != nil && inst.Definition.TickInterval > 0 {
		interval = inst.Definition.TickInterval
	}

	damage := lavaDamagePerSecond * interval.Seconds()
	w.applyStatusEffectDamage(actor, inst, rt.Now, damage)
}

func (w *World) applyStatusEffect(target *actorState, cond StatusEffectType, source string, now time.Time) bool {
	if w == nil || target == nil || cond == "" {
		return false
	}

	return worldpkg.ApplyStatusEffect(worldpkg.ApplyStatusEffectConfig{
		Now:      now,
		Type:     string(cond),
		SourceID: source,
		LookupDefinition: func() (worldpkg.ApplyStatusEffectDefinition, bool) {
			if w == nil {
				return worldpkg.ApplyStatusEffectDefinition{}, false
			}
			def, ok := w.statusEffectDefs[cond]
			if !ok {
				return worldpkg.ApplyStatusEffectDefinition{}, false
			}
			return def, true
		},
		FindInstance: func() (worldpkg.StatusEffectInstanceHandle, bool) {
			if target.statusEffects == nil {
				return worldpkg.StatusEffectInstanceHandle{}, false
			}
			inst, ok := target.statusEffects[cond]
			if !ok || inst == nil {
				return worldpkg.StatusEffectInstanceHandle{}, false
			}
			handle := newStatusEffectInstanceHandle(inst, target)
			if handle.SetSourceID != nil {
				handle.SetSourceID(source)
			}
			handle.SetActor(target)
			return handle, true
		},
		NewInstance: func() worldpkg.StatusEffectInstanceHandle {
			inst := &statusEffectInstance{}
			handle := newStatusEffectInstanceHandle(inst, target)
			if handle.SetSourceID != nil {
				handle.SetSourceID(source)
			}
			handle.SetActor(target)
			return handle
		},
		StoreInstance: func(handle worldpkg.StatusEffectInstanceHandle) {
			inst, _ := handle.Instance.(*statusEffectInstance)
			if inst == nil {
				return
			}
			if target.statusEffects == nil {
				target.statusEffects = make(map[StatusEffectType]*statusEffectInstance)
			}
			target.statusEffects[cond] = inst
		},
		RecordApplied: func(duration time.Duration) {
			if w == nil {
				return
			}
			actorRef := logging.EntityRef{}
			if source != "" {
				actorRef = w.entityRef(source)
			}
			targetRef := logging.EntityRef{}
			if target != nil {
				targetRef = w.entityRef(target.ID)
			}
			payload := loggingstatuseffects.AppliedPayload{StatusEffect: string(cond), SourceID: source}
			if duration > 0 {
				payload.DurationMs = duration.Milliseconds()
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
		},
	})
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
					handle := newStatusEffectInstanceHandle(inst, actor)
					def.OnTick(worldpkg.StatusEffectTickRuntime{Handle: handle, Now: tickAt})
				}
			}
			if def.OnExpire != nil {
				cfg.Definition.OnExpire = func(expireAt time.Time) {
					handle := newStatusEffectInstanceHandle(inst, actor)
					def.OnExpire(worldpkg.StatusEffectExpireRuntime{Handle: handle, Now: expireAt})
				}
			}

			cfg.Attachment = &worldpkg.StatusEffectAttachmentConfig{
				Extend: func(expiresAt time.Time) {
					handle := newStatusEffectInstanceHandle(inst, actor)
					handle.Attachment.Extend(expiresAt)
				},
				Expire: func(at time.Time) (any, bool) {
					handle := newStatusEffectInstanceHandle(inst, actor)
					return handle.Attachment.Expire(at)
				},
				Clear: func() {
					handle := newStatusEffectInstanceHandle(inst, actor)
					handle.Attachment.Clear()
				},
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
		statusType = StatusEffectType(inst.Definition.Type)
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

func (w *World) attachStatusEffectVisual(handle worldpkg.StatusEffectInstanceHandle, actor *actorState, statusType StatusEffectType, sourceID, effectType string, lifetime time.Duration, expiresAt, now time.Time) *effectState {
	if w == nil || statusType == "" || effectType == "" {
		return nil
	}
	inst, _ := handle.Instance.(*statusEffectInstance)
	if inst == nil {
		return nil
	}

	if actor == nil {
		if handle.Actor != nil {
			cast, _ := handle.Actor().(*actorState)
			actor = cast
		}
	}
	if actor == nil {
		return nil
	}

	handle.SetActor(actor)

	if lifetime <= 0 {
		lifetime = 100 * time.Millisecond
	}
	if expiresAt.IsZero() {
		expiresAt = now.Add(lifetime)
	}
	w.pruneEffects(now)
	w.nextEffectID++
	width := playerHalf * 2
	height := playerHalf * 2
	start := now.UnixMilli()
	instanceID := fmt.Sprintf("effect-%d", w.nextEffectID)
	owner := sourceID
	if owner == "" {
		owner = actor.ID
	}
	duration := expiresAt.Sub(now)
	if duration <= 0 {
		duration = lifetime
		expiresAt = now.Add(duration)
	}
	eff := &effectState{
		ID:                 instanceID,
		Type:               effectType,
		Owner:              owner,
		Start:              start,
		Duration:           duration.Milliseconds(),
		X:                  actor.X - width/2,
		Y:                  actor.Y - height/2,
		Width:              width,
		Height:             height,
		Instance:           effectcontract.EffectInstance{ID: instanceID, DefinitionID: effectType, OwnerActorID: actor.ID, StartTick: effectcontract.Tick(int64(w.currentTick))},
		ExpiresAt:          expiresAt,
		FollowActorID:      actor.ID,
		TelemetrySpawnTick: effectcontract.Tick(int64(w.currentTick)),
	}

	var attach func(worldpkg.AttachStatusEffectVisualConfig)
	if def, ok := w.statusEffectDefs[statusType]; ok {
		if state, _ := def.State.(*worldpkg.StatusEffectDefinition); state != nil && state.AttachVisual != nil {
			attach = state.AttachVisual
		}
	}
	if attach == nil {
		attach = worldpkg.AttachStatusEffectVisual
	}

	attach(worldpkg.AttachStatusEffectVisualConfig{
		Instance:    inst,
		Effect:      statusEffectVisualStateAdapter{state: eff, setStatus: handle.Attachment.SetStatus},
		DefaultType: string(statusType),
	})

	handle.Attachment.SetStatus(string(statusType))

	if !expiresAt.IsZero() {
		handle.Attachment.Extend(expiresAt)
	}

	return eff
}

func newStatusEffectInstanceHandle(inst *statusEffectInstance, actor *actorState) worldpkg.StatusEffectInstanceHandle {
	if inst != nil {
		inst.actor = actor
	}

	return worldpkg.StatusEffectInstanceHandle{
		Instance: inst,
		HasDefinition: func() bool {
			return inst != nil && inst.Definition != nil
		},
		SetDefinition: func(value any) {
			if inst == nil {
				return
			}
			def, _ := value.(*worldpkg.StatusEffectDefinition)
			inst.Definition = def
		},
		SetActor: func(value any) {
			if inst == nil {
				return
			}
			cast, _ := value.(*actorState)
			inst.actor = cast
		},
		Actor: func() any {
			if inst == nil {
				return nil
			}
			return inst.actor
		},
		SetSourceID: func(value string) {
			if inst == nil {
				return
			}
			inst.SourceID = value
		},
		SourceID: func() string {
			if inst == nil {
				return ""
			}
			return inst.SourceID
		},
		SetAppliedAt: func(at time.Time) {
			if inst == nil {
				return
			}
			inst.AppliedAt = at
		},
		SetExpiresAt: func(at time.Time) {
			if inst == nil {
				return
			}
			inst.ExpiresAt = at
		},
		ExpiresAt: func() time.Time {
			if inst == nil {
				return time.Time{}
			}
			return inst.ExpiresAt
		},
		SetNextTick: func(at time.Time) {
			if inst == nil {
				return
			}
			inst.NextTick = at
		},
		NextTick: func() time.Time {
			if inst == nil {
				return time.Time{}
			}
			return inst.NextTick
		},
		SetLastTick: func(at time.Time) {
			if inst == nil {
				return
			}
			inst.LastTick = at
		},
		Attachment: worldpkg.StatusEffectInstanceAttachment{
			SetStatus: func(effectType string) {
				if inst == nil || inst.attachedEffect == nil || effectType == "" {
					return
				}
				inst.attachedEffect.StatusEffect = StatusEffectType(effectType)
			},
			Extend: func(expiresAt time.Time) {
				if inst == nil || inst.attachedEffect == nil {
					return
				}
				worldpkg.ExtendStatusEffectAttachment(statusEffectAttachmentFields(inst.attachedEffect), expiresAt)
			},
			Expire: func(at time.Time) (any, bool) {
				if inst == nil || inst.attachedEffect == nil {
					return nil, false
				}
				shouldRecord := worldpkg.ExpireStatusEffectAttachment(statusEffectAttachmentFields(inst.attachedEffect), at)
				return inst.attachedEffect, shouldRecord
			},
			Clear: func() {
				if inst == nil {
					return
				}
				inst.attachedEffect = nil
			},
		},
	}
}

type statusEffectVisualStateAdapter struct {
	state     *effectState
	setStatus func(string)
}

func (a statusEffectVisualStateAdapter) SetStatusEffect(value string) {
	if value == "" {
		return
	}
	if a.setStatus != nil {
		a.setStatus(value)
		return
	}
	if a.state == nil {
		return
	}
	a.state.StatusEffect = StatusEffectType(value)
}

func (a statusEffectVisualStateAdapter) EffectState() any {
	if a.state == nil {
		return nil
	}
	return a.state
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
