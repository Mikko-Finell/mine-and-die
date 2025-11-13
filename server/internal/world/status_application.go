package world

import (
	"context"
	"time"

	internaleffects "mine-and-die/server/internal/effects"
	worldeffects "mine-and-die/server/internal/world/effects"
	state "mine-and-die/server/internal/world/state"
	statuspkg "mine-and-die/server/internal/world/status"
	"mine-and-die/server/logging"
	loggingstatuseffects "mine-and-die/server/logging/status_effects"
)

// ApplyStatusEffect applies or refreshes the requested status effect on the actor.
func (w *World) ApplyStatusEffect(target *state.ActorState, cond statuspkg.StatusEffectType, source string, now time.Time) bool {
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
			def, ok := w.statusEffectDefinitions[string(cond)]
			if !ok {
				return statuspkg.ApplyStatusEffectDefinition{}, false
			}
			return def, true
		},
		FindInstance: func() (statuspkg.StatusEffectInstanceHandle, bool) {
			if target.StatusEffects == nil {
				return statuspkg.StatusEffectInstanceHandle{}, false
			}
			inst, ok := target.StatusEffects[state.StatusEffectType(cond)]
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
			inst := &state.StatusEffectInstance{}
			handle := newStatusEffectInstanceHandle(inst, target)
			if handle.SetSourceID != nil {
				handle.SetSourceID(source)
			}
			handle.SetActor(target)
			return handle
		},
		StoreInstance: func(handle statuspkg.StatusEffectInstanceHandle) {
			inst, _ := handle.Instance.(*state.StatusEffectInstance)
			if inst == nil {
				return
			}
			if target.StatusEffects == nil {
				target.StatusEffects = make(map[state.StatusEffectType]*state.StatusEffectInstance)
			}
			target.StatusEffects[state.StatusEffectType(cond)] = inst
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
				w.currentTick(),
				actorRef,
				targetRef,
				payload,
				nil,
			)
		},
	})
}

func newStatusEffectInstanceHandle(inst *state.StatusEffectInstance, actor *state.ActorState) statuspkg.StatusEffectInstanceHandle {
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
			cast, _ := value.(*state.ActorState)
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
				if eff, ok := inst.AttachedEffect().(*worldeffects.State); ok && eff != nil {
					eff.StatusEffect = internaleffects.StatusEffectType(effectType)
				}
			},
			Extend: func(expiresAt time.Time) {
				if inst == nil {
					return
				}
				if eff, ok := inst.AttachedEffect().(*worldeffects.State); ok && eff != nil {
					statuspkg.ExtendStatusEffectAttachment(statusEffectAttachmentFields(eff), expiresAt)
				}
			},
			Expire: func(at time.Time) (any, bool) {
				if inst == nil {
					return nil, false
				}
				eff, ok := inst.AttachedEffect().(*worldeffects.State)
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

func statusEffectAttachmentFields(eff *worldeffects.State) statuspkg.StatusEffectAttachmentFields {
	if eff == nil {
		return statuspkg.StatusEffectAttachmentFields{}
	}
	return statuspkg.StatusEffectAttachmentFields{
		StatusEffectLifetimeFields: statuspkg.StatusEffectLifetimeFields{
			ExpiresAt:      &eff.ExpiresAt,
			StartMillis:    eff.Start,
			DurationMillis: &eff.Duration,
		},
		TelemetryEnded: eff.TelemetryEnded,
	}
}

func (w *World) entityRef(actorID string) logging.EntityRef {
	if actorID == "" {
		return logging.EntityRef{}
	}
	if _, ok := w.players[actorID]; ok {
		return logging.EntityRef{ID: actorID, Kind: logging.EntityKind("player")}
	}
	if _, ok := w.npcs[actorID]; ok {
		return logging.EntityRef{ID: actorID, Kind: logging.EntityKind("npc")}
	}
	return logging.EntityRef{ID: actorID, Kind: logging.EntityKind("unknown")}
}
