package world

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
