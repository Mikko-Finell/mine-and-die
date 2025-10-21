package world

import "time"

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
