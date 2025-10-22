package world

import "time"

// StatusEffectDefinitionCallbacks exposes the tick interval and lifecycle
// handlers required to advance a status effect instance without importing the
// legacy world types.
type StatusEffectDefinitionCallbacks struct {
	TickInterval time.Duration
	OnTick       func(time.Time)
	OnExpire     func(time.Time)
}

// StatusEffectAttachmentConfig wires the callbacks required to keep an attached
// visual effect in sync with the status effect instance.
type StatusEffectAttachmentConfig struct {
	Extend func(time.Time)
	Expire func(time.Time) (any, bool)
	Clear  func()
}

// StatusEffectInstanceConfig bundles the closures required to drive a single
// status effect instance through its tick and expiry lifecycle.
type StatusEffectInstanceConfig struct {
	Definition StatusEffectDefinitionCallbacks

	NextTick    func() time.Time
	SetNextTick func(time.Time)

	LastTick    func() time.Time
	SetLastTick func(time.Time)

	ExpiresAt func() time.Time

	Attachment *StatusEffectAttachmentConfig
}

// AdvanceActorStatusEffectsConfig carries the dependencies required to progress
// every status effect instance attached to a single actor.
type AdvanceActorStatusEffectsConfig struct {
	Now time.Time

	ForEachInstance func(func(key string, instance any))
	Instance        func(instance any) (StatusEffectInstanceConfig, bool)
	Remove          func(key string)

	RecordEffectEnd func(any)
}

// AdvanceStatusEffectsActorIterator enumerates actors that have status effect
// state to advance.
type AdvanceStatusEffectsActorIterator func(func(AdvanceActorStatusEffectsConfig))

// AdvanceStatusEffectsConfig bundles the iterators required to advance status
// effects for all active players and NPCs.
type AdvanceStatusEffectsConfig struct {
	Now time.Time

	ForEachPlayer AdvanceStatusEffectsActorIterator
	ForEachNPC    AdvanceStatusEffectsActorIterator
}

// AdvanceStatusEffects walks the provided actors and advances their status
// effect instances using the supplied iterators.
func AdvanceStatusEffects(cfg AdvanceStatusEffectsConfig) {
	advance := func(iter AdvanceStatusEffectsActorIterator) {
		if iter == nil {
			return
		}

		iter(func(actorCfg AdvanceActorStatusEffectsConfig) {
			actorCfg.Now = cfg.Now
			AdvanceActorStatusEffects(actorCfg)
		})
	}

	advance(cfg.ForEachPlayer)
	advance(cfg.ForEachNPC)
}

// AdvanceActorStatusEffects progresses each status effect instance for a
// single actor, executing tick handlers, extending attachments, and removing
// expired entries.
func AdvanceActorStatusEffects(cfg AdvanceActorStatusEffectsConfig) {
	if cfg.ForEachInstance == nil || cfg.Instance == nil || cfg.Remove == nil {
		return
	}

	cfg.ForEachInstance(func(key string, instance any) {
		if instance == nil {
			cfg.Remove(key)
			return
		}

		instCfg, ok := cfg.Instance(instance)
		if !ok {
			cfg.Remove(key)
			return
		}

		interval := instCfg.Definition.TickInterval
		nextTick := time.Time{}
		if instCfg.NextTick != nil {
			nextTick = instCfg.NextTick()
		}
		expiresAt := time.Time{}
		if instCfg.ExpiresAt != nil {
			expiresAt = instCfg.ExpiresAt()
		}

		if interval > 0 && !nextTick.IsZero() {
			for !cfg.Now.Before(nextTick) {
				if !expiresAt.IsZero() && nextTick.After(expiresAt) {
					break
				}

				tickAt := nextTick
				if instCfg.Definition.OnTick != nil {
					instCfg.Definition.OnTick(tickAt)
				}
				if instCfg.SetLastTick != nil {
					instCfg.SetLastTick(tickAt)
				}

				nextTick = nextTick.Add(interval)
				if instCfg.SetNextTick != nil {
					instCfg.SetNextTick(nextTick)
				}

				if nextTick.Equal(tickAt) {
					break
				}
			}
		}

		if instCfg.ExpiresAt != nil {
			expiresAt = instCfg.ExpiresAt()
		}

		if !cfg.Now.Before(expiresAt) {
			if instCfg.Definition.OnExpire != nil {
				instCfg.Definition.OnExpire(cfg.Now)
			} else if instCfg.Attachment != nil {
				var (
					effect       any
					shouldRecord bool
				)
				if instCfg.Attachment.Expire != nil {
					effect, shouldRecord = instCfg.Attachment.Expire(cfg.Now)
				}
				if shouldRecord && cfg.RecordEffectEnd != nil {
					cfg.RecordEffectEnd(effect)
				}
				if instCfg.Attachment.Clear != nil {
					instCfg.Attachment.Clear()
				}
			}

			cfg.Remove(key)
			return
		}

		if instCfg.Attachment != nil && instCfg.Attachment.Extend != nil {
			instCfg.Attachment.Extend(expiresAt)
		}
	})
}

// StatusEffectInstance exposes the minimal API required to associate a
// contract-managed visual effect with a status effect instance.
type StatusEffectInstance interface {
	// AttachEffect records the provided effect state so the caller can
	// synchronize lifetime updates with the status effect instance.
	AttachEffect(any)
	// DefinitionType returns the status effect type associated with the
	// instance, or an empty string when the type is unknown.
	DefinitionType() string
}

// StatusEffectVisual exposes the minimal API required to update the
// contract-managed visual effect when attaching it to a status effect.
type StatusEffectVisual interface {
	// SetStatusEffect updates the effect's status effect identifier so the
	// client can render it with the correct visual treatment.
	SetStatusEffect(string)
	// EffectState returns the underlying effect state payload so the
	// attachment helper can persist it on the status effect instance.
	EffectState() any
}

// AttachStatusEffectVisualConfig bundles the inputs required to associate a
// visual effect with a status effect instance while keeping the legacy world
// structures behind thin adapters.
type AttachStatusEffectVisualConfig struct {
	Instance    StatusEffectInstance
	Effect      StatusEffectVisual
	DefaultType string
}

// AttachStatusEffectVisual links the provided visual effect to the status
// effect instance described in the configuration. When the instance does not
// expose a definition type the helper falls back to the supplied default type
// so the effect remains tagged for client rendering.
func AttachStatusEffectVisual(cfg AttachStatusEffectVisualConfig) {
	if cfg.Effect == nil {
		return
	}

	state := cfg.Effect.EffectState()
	if state == nil {
		return
	}

	if cfg.Instance == nil {
		return
	}

	cfg.Instance.AttachEffect(state)
	typ := cfg.Instance.DefinitionType()
	if typ == "" {
		typ = cfg.DefaultType
	}
	if typ == "" {
		return
	}

	cfg.Effect.SetStatusEffect(typ)
}

// StatusEffectLifetimeFields captures the pieces of state required to update an
// attached status visual's expiry metadata without exposing the legacy effect
// struct directly.
type StatusEffectLifetimeFields struct {
	ExpiresAt      *time.Time
	StartMillis    int64
	DurationMillis *int64
}

// StatusEffectAttachmentFields extends the shared lifetime fields with telemetry
// bookkeeping so helpers can signal when the caller should record an effect end.
type StatusEffectAttachmentFields struct {
	StatusEffectLifetimeFields
	TelemetryEnded bool
}

// ExtendStatusEffectLifetime updates the expiration timestamp and duration
// metadata for an attached status visual when the new expiry is not earlier
// than the current value. Duration is derived from the effect's start time and
// clamped to zero to mirror the legacy world helper.
func ExtendStatusEffectLifetime(fields StatusEffectLifetimeFields, expiresAt time.Time) {
	if fields.ExpiresAt == nil || fields.DurationMillis == nil {
		return
	}
	if expiresAt.Before(*fields.ExpiresAt) {
		return
	}

	*fields.ExpiresAt = expiresAt
	start := time.UnixMilli(fields.StartMillis)
	if fields.StartMillis == 0 {
		start = expiresAt
	}

	duration := expiresAt.Sub(start)
	if duration < 0 {
		duration = 0
	}
	*fields.DurationMillis = duration.Milliseconds()
}

// ExpireStatusEffectLifetime finalizes an attached status visual's expiration
// timestamp and duration when the instance ends. The expiry is clamped to the
// provided time and the duration derived from the effect's start, mirroring the
// legacy world bookkeeping.
func ExpireStatusEffectLifetime(fields StatusEffectLifetimeFields, now time.Time) {
	if fields.ExpiresAt == nil || fields.DurationMillis == nil {
		return
	}

	if now.Before(*fields.ExpiresAt) {
		*fields.ExpiresAt = now
	}

	start := time.UnixMilli(fields.StartMillis)
	if fields.StartMillis == 0 {
		start = now
	}

	duration := now.Sub(start)
	if duration < 0 {
		duration = 0
	}
	*fields.DurationMillis = duration.Milliseconds()
}

// ExtendStatusEffectAttachment updates the supplied attachment lifetime fields
// to reflect the provided expiry while preserving existing bookkeeping. The
// helper mirrors the legacy world behavior by delegating to the shared
// lifetime updater.
func ExtendStatusEffectAttachment(fields StatusEffectAttachmentFields, expiresAt time.Time) {
	ExtendStatusEffectLifetime(fields.StatusEffectLifetimeFields, expiresAt)
}

// ExpireStatusEffectAttachment finalizes the attachment's expiry metadata and
// reports whether the caller should record a telemetry end event. The helper
// mirrors the legacy world behavior by keeping telemetry logging in the caller
// while centralizing the lifetime update.
func ExpireStatusEffectAttachment(fields StatusEffectAttachmentFields, now time.Time) bool {
	ExpireStatusEffectLifetime(fields.StatusEffectLifetimeFields, now)
	return !fields.TelemetryEnded
}
