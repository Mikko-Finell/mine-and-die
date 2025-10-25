package world

import "time"

// ApplyStatusEffectDefinition describes the runtime behavior for a single
// status effect when applying it to an actor. The legacy world wrapper provides
// closures that already capture its dependencies so the shared helper can
// invoke them without importing server types.
type ApplyStatusEffectDefinition struct {
	Duration     time.Duration
	TickInterval time.Duration
	InitialTick  bool

	State any

	OnApply func(StatusEffectInstanceHandle, time.Time)
	OnTick  func(StatusEffectInstanceHandle, time.Time)
}

// StatusEffectInstanceAttachment exposes the operations required to keep a
// status effect's attached visual in sync with the instance state.
type StatusEffectInstanceAttachment struct {
	SetStatus func(string)
	Extend    func(time.Time)
	Expire    func(time.Time) (any, bool)
	Clear     func()
}

// StatusEffectInstanceHandle bundles field mutators for a single status effect
// instance. The legacy world constructs handles around its concrete instance
// type so the shared helper can mutate state without a direct dependency.
type StatusEffectInstanceHandle struct {
	Instance any

	HasDefinition func() bool
	SetDefinition func(any)

	SetActor func(any)
	Actor    func() any

	SetSourceID  func(string)
	SourceID     func() string
	SetAppliedAt func(time.Time)
	SetExpiresAt func(time.Time)
	ExpiresAt    func() time.Time

	SetNextTick func(time.Time)
	NextTick    func() time.Time
	SetLastTick func(time.Time)

	Attachment StatusEffectInstanceAttachment
}

// StatusEffectApplyRuntime bundles the context supplied to apply callbacks when a
// status effect instance is first created.
type StatusEffectApplyRuntime struct {
	Handle StatusEffectInstanceHandle
	Now    time.Time
}

// StatusEffectTickRuntime bundles the runtime context for status effect tick
// callbacks.
type StatusEffectTickRuntime struct {
	Handle StatusEffectInstanceHandle
	Now    time.Time
}

// StatusEffectExpireRuntime bundles the runtime context for status effect
// expiry callbacks.
type StatusEffectExpireRuntime struct {
	Handle StatusEffectInstanceHandle
	Now    time.Time
}

// StatusEffectDefinition captures the runtime callbacks required to advance a
// status effect instance while hiding the legacy world types behind adapters.
// AttachVisual delegates fallback visual attachments when the contract effect
// manager is unavailable.
type StatusEffectDefinition struct {
	Type         string
	TickInterval time.Duration

	OnTick       func(StatusEffectTickRuntime)
	OnExpire     func(StatusEffectExpireRuntime)
	AttachVisual func(AttachStatusEffectVisualConfig)
}

// StatusEffectType implements state.StatusEffectDefinitionView so shared state
// can report the registered status effect identifier without importing this
// package.
func (def *StatusEffectDefinition) StatusEffectType() string {
	if def == nil {
		return ""
	}
	return def.Type
}

// StatusEffectDefinitionsConfig enumerates the status effects that should be
// registered along with the callbacks required to drive their runtime
// behaviour.
type StatusEffectDefinitionsConfig struct {
	Burning BurningStatusEffectDefinitionConfig
}

// BurningStatusEffectDefinitionConfig carries the configuration required to
// construct the burning status effect definition while keeping the legacy world
// dependencies behind adapters.
type BurningStatusEffectDefinitionConfig struct {
	Type         string
	Duration     time.Duration
	TickInterval time.Duration
	InitialTick  bool

	OnApply  func(StatusEffectApplyRuntime)
	OnTick   func(StatusEffectTickRuntime)
	OnExpire func(StatusEffectExpireRuntime)

	FallbackAttachment func(AttachStatusEffectVisualConfig)
}

// NewStatusEffectDefinitions constructs the registered status effect
// definitions using the provided configuration.
func NewStatusEffectDefinitions(cfg StatusEffectDefinitionsConfig) map[string]ApplyStatusEffectDefinition {
	defs := make(map[string]ApplyStatusEffectDefinition)

	if cfg.Burning.Type != "" {
		defs[cfg.Burning.Type] = newBurningStatusEffectDefinition(cfg.Burning)
	}

	return defs
}

func newBurningStatusEffectDefinition(cfg BurningStatusEffectDefinitionConfig) ApplyStatusEffectDefinition {
	state := &StatusEffectDefinition{
		Type:         cfg.Type,
		TickInterval: cfg.TickInterval,
	}

	if cfg.OnTick != nil {
		state.OnTick = cfg.OnTick
	}
	if cfg.OnExpire != nil {
		state.OnExpire = cfg.OnExpire
	}
	if cfg.FallbackAttachment != nil {
		state.AttachVisual = cfg.FallbackAttachment
	}

	def := ApplyStatusEffectDefinition{
		Duration:     cfg.Duration,
		TickInterval: cfg.TickInterval,
		InitialTick:  cfg.InitialTick,
		State:        state,
	}

	if cfg.OnApply != nil {
		def.OnApply = func(handle StatusEffectInstanceHandle, at time.Time) {
			cfg.OnApply(StatusEffectApplyRuntime{Handle: handle, Now: at})
		}
	}
	if cfg.OnTick != nil {
		def.OnTick = func(handle StatusEffectInstanceHandle, at time.Time) {
			cfg.OnTick(StatusEffectTickRuntime{Handle: handle, Now: at})
		}
	}

	return def
}

// ApplyStatusEffectConfig carries the dependencies required to apply or refresh
// a status effect instance for an actor while keeping the legacy world wrapper
// thin.
type ApplyStatusEffectConfig struct {
	Now      time.Time
	Type     string
	SourceID string

	LookupDefinition func() (ApplyStatusEffectDefinition, bool)
	FindInstance     func() (StatusEffectInstanceHandle, bool)
	NewInstance      func() StatusEffectInstanceHandle
	StoreInstance    func(StatusEffectInstanceHandle)

	RecordApplied func(time.Duration)
}

// ApplyStatusEffect applies or refreshes the requested status effect using the
// provided adapters. The helper mirrors the legacy behavior by performing
// initial tick handling, attachment bookkeeping, and telemetry logging when the
// effect is newly applied.
func ApplyStatusEffect(cfg ApplyStatusEffectConfig) bool {
	if cfg.Type == "" || cfg.LookupDefinition == nil {
		return false
	}

	def, ok := cfg.LookupDefinition()
	if !ok {
		return false
	}
	if def.Duration <= 0 {
		return false
	}

	if cfg.FindInstance == nil || cfg.NewInstance == nil || cfg.StoreInstance == nil {
		return false
	}

	if inst, exists := cfg.FindInstance(); exists {
		if inst.SetSourceID != nil {
			inst.SetSourceID(cfg.SourceID)
		}

		expiresAt := cfg.Now.Add(def.Duration)
		if inst.SetExpiresAt != nil {
			inst.SetExpiresAt(expiresAt)
		}

		if inst.SetDefinition != nil {
			if inst.HasDefinition == nil || !inst.HasDefinition() {
				inst.SetDefinition(def.State)
			}
		}

		if def.TickInterval > 0 && inst.SetNextTick != nil && inst.NextTick != nil {
			if inst.NextTick().IsZero() {
				inst.SetNextTick(cfg.Now.Add(def.TickInterval))
			}
		}

		if inst.Attachment.Extend != nil {
			inst.Attachment.Extend(expiresAt)
		}

		return false
	}

	inst := cfg.NewInstance()
	if inst.SetSourceID != nil {
		inst.SetSourceID(cfg.SourceID)
	}
	if inst.SetAppliedAt != nil {
		inst.SetAppliedAt(cfg.Now)
	}

	expiresAt := cfg.Now.Add(def.Duration)
	if inst.SetExpiresAt != nil {
		inst.SetExpiresAt(expiresAt)
	}

	if inst.SetDefinition != nil {
		inst.SetDefinition(def.State)
	}

	if def.TickInterval > 0 && inst.SetNextTick != nil {
		nextTick := cfg.Now.Add(def.TickInterval)
		if def.InitialTick {
			nextTick = cfg.Now
		}
		inst.SetNextTick(nextTick)
	}

	cfg.StoreInstance(inst)

	if def.OnApply != nil {
		def.OnApply(inst, cfg.Now)
	}

	if def.InitialTick && def.OnTick != nil {
		def.OnTick(inst, cfg.Now)
		if inst.SetLastTick != nil {
			inst.SetLastTick(cfg.Now)
		}
		if def.TickInterval > 0 && inst.SetNextTick != nil {
			inst.SetNextTick(cfg.Now.Add(def.TickInterval))
		}
	}

	if inst.Attachment.SetStatus != nil {
		inst.Attachment.SetStatus(cfg.Type)
	}

	if cfg.RecordApplied != nil {
		cfg.RecordApplied(def.Duration)
	}

	return true
}

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
