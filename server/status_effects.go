package server

import (
	"context"
	"math"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	internaleffects "mine-and-die/server/internal/effects"
	worldpkg "mine-and-die/server/internal/world"
	statuspkg "mine-and-die/server/internal/world/status"
	"mine-and-die/server/logging"
	loggingstatuseffects "mine-and-die/server/logging/status_effects"
)

var _ statuspkg.StatusEffectInstance = (*statusEffectInstance)(nil)

const (
	StatusEffectBurning StatusEffectType = "burning"
)

const (
	burningStatusEffectDuration = 3 * time.Second
	burningTickInterval         = 200 * time.Millisecond
)

func newStatusEffectDefinitions(w *World) map[StatusEffectType]statuspkg.ApplyStatusEffectDefinition {
	if w == nil {
		return nil
	}

	defs := statuspkg.NewStatusEffectDefinitions(statuspkg.StatusEffectDefinitionsConfig{
		Burning: statuspkg.BurningStatusEffectDefinitionConfig{
			Type:               string(StatusEffectBurning),
			Duration:           burningStatusEffectDuration,
			TickInterval:       burningTickInterval,
			InitialTick:        true,
			FallbackAttachment: statuspkg.AttachStatusEffectVisual,
			OnApply: func(rt statuspkg.StatusEffectApplyRuntime) {
				w.handleBurningStatusApply(rt)
			},
			OnTick: func(rt statuspkg.StatusEffectTickRuntime) {
				w.handleBurningStatusTick(rt)
			},
		},
	})

	result := make(map[StatusEffectType]statuspkg.ApplyStatusEffectDefinition, len(defs))
	for key, def := range defs {
		result[StatusEffectType(key)] = def
	}
	return result
}

func (w *World) handleBurningStatusApply(rt statuspkg.StatusEffectApplyRuntime) {
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

func (w *World) handleBurningStatusTick(rt statuspkg.StatusEffectTickRuntime) {
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
	if def, ok := inst.Definition.(*statuspkg.StatusEffectDefinition); ok && def != nil && def.TickInterval > 0 {
		interval = def.TickInterval
	}

	damage := lavaDamagePerSecond * interval.Seconds()
	w.applyStatusEffectDamage(actor, inst, rt.Now, damage)
}

func (w *World) applyStatusEffect(target *actorState, cond StatusEffectType, source string, now time.Time) bool {
	if w == nil || target == nil || cond == "" {
		return false
	}

	return statuspkg.ApplyStatusEffect(statuspkg.ApplyStatusEffectConfig{
		Now:      now,
		Type:     string(cond),
		SourceID: source,
		LookupDefinition: func() (statuspkg.ApplyStatusEffectDefinition, bool) {
			if w == nil {
				return statuspkg.ApplyStatusEffectDefinition{}, false
			}
			def, ok := w.statusEffectDefs[cond]
			if !ok {
				return statuspkg.ApplyStatusEffectDefinition{}, false
			}
			return def, true
		},
		FindInstance: func() (statuspkg.StatusEffectInstanceHandle, bool) {
			if target.StatusEffects == nil {
				return statuspkg.StatusEffectInstanceHandle{}, false
			}
			inst, ok := target.StatusEffects[cond]
			if !ok || inst == nil {
				return statuspkg.StatusEffectInstanceHandle{}, false
			}
			handle := newStatusEffectInstanceHandle(inst, target)
			if handle.SetSourceID != nil {
				handle.SetSourceID(source)
			}
			handle.SetActor(target)
			return handle, true
		},
		NewInstance: func() statuspkg.StatusEffectInstanceHandle {
			inst := &statusEffectInstance{}
			handle := newStatusEffectInstanceHandle(inst, target)
			if handle.SetSourceID != nil {
				handle.SetSourceID(source)
			}
			handle.SetActor(target)
			return handle
		},
		StoreInstance: func(handle statuspkg.StatusEffectInstanceHandle) {
			inst, _ := handle.Instance.(*statusEffectInstance)
			if inst == nil {
				return
			}
			if target.StatusEffects == nil {
				target.StatusEffects = make(map[StatusEffectType]*statusEffectInstance)
			}
			target.StatusEffects[cond] = inst
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

	statuspkg.AdvanceStatusEffects(statuspkg.AdvanceStatusEffectsConfig{
		Now: now,
		ForEachPlayer: func(apply func(statuspkg.AdvanceActorStatusEffectsConfig)) {
			if apply == nil {
				return
			}
			for _, player := range w.players {
				if player == nil {
					continue
				}
				apply(w.statusEffectsAdvanceConfig(&player.ActorState))
			}
		},
		ForEachNPC: func(apply func(statuspkg.AdvanceActorStatusEffectsConfig)) {
			if apply == nil {
				return
			}
			for _, npc := range w.npcs {
				if npc == nil {
					continue
				}
				apply(w.statusEffectsAdvanceConfig(&npc.ActorState))
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
	statuspkg.AdvanceActorStatusEffects(cfg)
}

func (w *World) statusEffectsAdvanceConfig(actor *actorState) statuspkg.AdvanceActorStatusEffectsConfig {
	return statuspkg.AdvanceActorStatusEffectsConfig{
		ForEachInstance: func(visitor func(key string, instance any)) {
			if visitor == nil || actor == nil {
				return
			}
			for key, inst := range actor.StatusEffects {
				visitor(string(key), inst)
			}
		},
		Instance: func(value any) (statuspkg.StatusEffectInstanceConfig, bool) {
			inst, _ := value.(*statusEffectInstance)
			if inst == nil {
				return statuspkg.StatusEffectInstanceConfig{}, false
			}
			def, ok := inst.Definition.(*statuspkg.StatusEffectDefinition)
			if !ok || def == nil {
				return statuspkg.StatusEffectInstanceConfig{}, false
			}

			cfg := statuspkg.StatusEffectInstanceConfig{
				Definition: statuspkg.StatusEffectDefinitionCallbacks{
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
					def.OnTick(statuspkg.StatusEffectTickRuntime{Handle: handle, Now: tickAt})
				}
			}
			if def.OnExpire != nil {
				cfg.Definition.OnExpire = func(expireAt time.Time) {
					handle := newStatusEffectInstanceHandle(inst, actor)
					def.OnExpire(statuspkg.StatusEffectExpireRuntime{Handle: handle, Now: expireAt})
				}
			}

			cfg.Attachment = &statuspkg.StatusEffectAttachmentConfig{
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
			if actor == nil || actor.StatusEffects == nil {
				return
			}
			delete(actor.StatusEffects, StatusEffectType(key))
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
	if inst != nil {
		if def, ok := inst.Definition.(*statuspkg.StatusEffectDefinition); ok && def != nil {
			statusType = StatusEffectType(def.Type)
		}
	}
	delta := -amount
	if w.effectManager != nil {
		if intent, ok := statuspkg.NewBurningTickIntent(statuspkg.BurningTickIntentConfig{
			EffectType:    effectTypeBurningTick,
			TargetActorID: actor.ID,
			SourceActorID: owner,
			StatusEffect:  statuspkg.StatusEffectType(statusType),
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

	dispatcher := w.effectHitDispatcher()
	if dispatcher == nil {
		return
	}

	callback := worldpkg.NewWorldBurningDamageCallback(worldpkg.WorldBurningDamageCallbackConfig{
		Dispatcher: dispatcher,
		Target:     actor,
		Now:        now,
		BuildEffect: func(effect statuspkg.BurningDamageEffect) any {
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
				StatusEffect:       internaleffects.StatusEffectType(effect.StatusEffect),
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

	statuspkg.ApplyBurningDamage(statuspkg.ApplyBurningDamageConfig{
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

func (w *World) effectHitDispatcher() worldpkg.EffectHitCallback {
	if w == nil {
		return nil
	}
	if w.effectHitAdapter != nil {
		return w.effectHitAdapter
	}
	if w.internalWorld == nil {
		return nil
	}

	dispatcher := w.internalWorld.EffectHitDispatcher()
	if dispatcher == nil {
		return nil
	}

	w.effectHitAdapter = dispatcher
	return dispatcher
}

func (w *World) attachStatusEffectVisual(handle statuspkg.StatusEffectInstanceHandle, actor *actorState, statusType StatusEffectType, sourceID, effectType string, lifetime time.Duration, expiresAt, now time.Time) *effectState {
	if w == nil || statusType == "" || effectType == "" {
		return nil
	}
	inst, _ := handle.Instance.(*statusEffectInstance)
	if inst == nil {
		return nil
	}

	if actor == nil {
		cast, _ := handle.Actor().(*actorState)
		actor = cast
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
	instanceID := w.allocateEffectID()
	width := playerHalf * 2
	height := playerHalf * 2
	start := now.UnixMilli()
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

	var attach func(statuspkg.AttachStatusEffectVisualConfig)
	if def, ok := w.statusEffectDefs[statusType]; ok {
		if state, _ := def.State.(*statuspkg.StatusEffectDefinition); state != nil && state.AttachVisual != nil {
			attach = state.AttachVisual
		}
	}
	if attach == nil {
		attach = statuspkg.AttachStatusEffectVisual
	}

	attach(statuspkg.AttachStatusEffectVisualConfig{
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

func newStatusEffectInstanceHandle(inst *statusEffectInstance, actor *actorState) statuspkg.StatusEffectInstanceHandle {
	if inst != nil {
		inst.SetActorState(actor)
	}

	return statuspkg.StatusEffectInstanceHandle{
		Instance: inst,
		HasDefinition: func() bool {
			return inst != nil && inst.Definition != nil
		},
		SetDefinition: func(value any) {
			if inst == nil {
				return
			}
			def, _ := value.(*statuspkg.StatusEffectDefinition)
			inst.Definition = def
		},
		SetActor: func(value any) {
			if inst == nil {
				return
			}
			cast, _ := value.(*actorState)
			inst.SetActorState(cast)
		},
		Actor: func() any {
			if inst == nil {
				return nil
			}
			return inst.ActorState()
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
		Attachment: statuspkg.StatusEffectInstanceAttachment{
			SetStatus: func(effectType string) {
				if inst == nil || effectType == "" {
					return
				}
				if eff, ok := inst.AttachedEffect().(*effectState); ok && eff != nil {
					eff.StatusEffect = internaleffects.StatusEffectType(effectType)
				}
			},
			Extend: func(expiresAt time.Time) {
				if inst == nil {
					return
				}
				if eff, ok := inst.AttachedEffect().(*effectState); ok && eff != nil {
					statuspkg.ExtendStatusEffectAttachment(statusEffectAttachmentFields(eff), expiresAt)
				}
			},
			Expire: func(at time.Time) (any, bool) {
				if inst == nil {
					return nil, false
				}
				eff, ok := inst.AttachedEffect().(*effectState)
				if !ok || eff == nil {
					return nil, false
				}
				shouldRecord := statuspkg.ExpireStatusEffectAttachment(statusEffectAttachmentFields(eff), at)
				return eff, shouldRecord
			},
			Clear: func() {
				if inst == nil {
					return
				}
				inst.ClearAttachedEffect()
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
	a.state.StatusEffect = internaleffects.StatusEffectType(value)
}

func (a statusEffectVisualStateAdapter) EffectState() any {
	if a.state == nil {
		return nil
	}
	return a.state
}

func statusEffectAttachmentFields(eff *effectState) statuspkg.StatusEffectAttachmentFields {
	return statuspkg.StatusEffectAttachmentFields{
		StatusEffectLifetimeFields: statuspkg.StatusEffectLifetimeFields{
			ExpiresAt:      &eff.ExpiresAt,
			StartMillis:    eff.Start,
			DurationMillis: &eff.Duration,
		},
		TelemetryEnded: eff.TelemetryEnded,
	}
}
