package abilities

// AbilityGateFactory constructs an ability gate using the sanitized owner lookup
// configuration provided by the world package. The returned boolean indicates
// whether the gate was constructed successfully.
type AbilityGateFactory[Owner any, Gate any] func(AbilityGateConfig[Owner]) (Gate, bool)

// AbilityGateBindingOptions captures the metadata, lookup adapter, and factory
// required to bind an ability gate without importing combat types.
type AbilityGateBindingOptions[State any, Owner any, Gate any] struct {
	AbilityGateOptions[State, Owner]
	Factory AbilityGateFactory[Owner, Gate]
}

// BindMeleeAbilityGate constructs a melee ability gate using the provided
// options and factory. The helper hides the legacy world wiring while allowing
// callers to supply their own combat gate implementation.
func BindMeleeAbilityGate[State any, Owner any, Gate any](opts AbilityGateBindingOptions[State, Owner, Gate]) (Gate, bool) {
	return bindAbilityGate(NewMeleeAbilityGateConfig[State, Owner], opts)
}

// BindProjectileAbilityGate constructs a projectile ability gate using the
// provided options and factory. Callers receive the constructed gate when the
// configuration and factory are valid.
func BindProjectileAbilityGate[State any, Owner any, Gate any](opts AbilityGateBindingOptions[State, Owner, Gate]) (Gate, bool) {
	return bindAbilityGate(NewProjectileAbilityGateConfig[State, Owner], opts)
}

func bindAbilityGate[State any, Owner any, Gate any](
	builder func(AbilityGateOptions[State, Owner]) (AbilityGateConfig[Owner], bool),
	opts AbilityGateBindingOptions[State, Owner, Gate],
) (Gate, bool) {
	var zero Gate

	if builder == nil || opts.Factory == nil {
		return zero, false
	}

	cfg, ok := builder(opts.AbilityGateOptions)
	if !ok {
		return zero, false
	}

	return opts.Factory(cfg)
}
