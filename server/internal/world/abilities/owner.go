package abilities

import "time"

// AbilityOwnerStateLookup returns the actor state and cooldown registry for the
// requested ability owner. Callers provide lookup functions for each actor
// category so the helper can hide the legacy world maps while keeping the API
// generic.
type AbilityOwnerStateLookup[State any] func(actorID string) (State, *map[string]time.Time, bool)

// AbilityOwnerStateLookupConfig bundles the callbacks required to resolve the
// actor state for a given ability owner while hiding the legacy world maps from
// consumers. Callers may provide lookups for any combination of actor
// categories.
type AbilityOwnerStateLookupConfig[State any] struct {
	FindPlayer func(actorID string) (State, *map[string]time.Time, bool)
	FindNPC    func(actorID string) (State, *map[string]time.Time, bool)
}

// NewAbilityOwnerStateLookup constructs a state lookup using the provided
// configuration. The returned adapter mirrors the legacy behaviour by scanning
// player lookups before NPC lookups and only reporting success when both the
// actor state and cooldown registry are available.
func NewAbilityOwnerStateLookup[State any](cfg AbilityOwnerStateLookupConfig[State]) AbilityOwnerStateLookup[State] {
	return func(actorID string) (State, *map[string]time.Time, bool) {
		var zero State
		if actorID == "" {
			return zero, nil, false
		}

		if cfg.FindPlayer != nil {
			if state, cooldowns, ok := cfg.FindPlayer(actorID); ok && cooldowns != nil {
				return state, cooldowns, true
			}
		}

		if cfg.FindNPC != nil {
			if state, cooldowns, ok := cfg.FindNPC(actorID); ok && cooldowns != nil {
				return state, cooldowns, true
			}
		}

		return zero, nil, false
	}
}

// AbilityOwnerLookup resolves an ability owner and returns a sanitized snapshot
// alongside the cooldown registry. The snapshot type is generic so callers can
// expose domain-specific structs without importing the legacy world package.
type AbilityOwnerLookup[State any, Owner any] func(actorID string) (*Owner, *map[string]time.Time, bool)

// AbilityOwnerLookupConfig bundles the dependencies required to convert raw
// actor state into a sanitized ability owner snapshot. Callers provide a state
// lookup alongside a snapshot function that mirrors the legacy
// abilityActorSnapshot helper.
type AbilityOwnerLookupConfig[State any, Owner any] struct {
	LookupState AbilityOwnerStateLookup[State]
	Snapshot    func(State) *Owner
}

// NewAbilityOwnerLookup constructs an ability owner lookup using the provided
// configuration. The returned adapter mirrors the legacy semantics by returning
// nil when the actor is missing, lacks a cooldown registry, or the snapshot
// converter declines to produce an owner.
func NewAbilityOwnerLookup[State any, Owner any](cfg AbilityOwnerLookupConfig[State, Owner]) AbilityOwnerLookup[State, Owner] {
	if cfg.LookupState == nil || cfg.Snapshot == nil {
		return nil
	}

	return func(actorID string) (*Owner, *map[string]time.Time, bool) {
		if actorID == "" {
			return nil, nil, false
		}

		state, cooldowns, ok := cfg.LookupState(actorID)
		if !ok || cooldowns == nil {
			return nil, nil, false
		}

		owner := cfg.Snapshot(state)
		if owner == nil {
			return nil, nil, false
		}

		return owner, cooldowns, true
	}
}

// AbilityGateConfig captures the ability identifier, cooldown, and lookup
// closure required to construct an ability gate without importing the combat
// package. Callers pass the returned closure into their gate factory.
type AbilityGateConfig[Owner any] struct {
	AbilityID   string
	Cooldown    time.Duration
	LookupOwner func(actorID string) (*Owner, *map[string]time.Time, bool)
}

// AbilityGateOptions bundles the ability metadata and lookup adapter required to
// construct an ability gate configuration.
type AbilityGateOptions[State any, Owner any] struct {
	AbilityID string
	Cooldown  time.Duration
	Lookup    AbilityOwnerLookup[State, Owner]
}

// NewMeleeAbilityGateConfig constructs a melee ability gate configuration using
// the provided lookup adapter.
func NewMeleeAbilityGateConfig[State any, Owner any](opts AbilityGateOptions[State, Owner]) (AbilityGateConfig[Owner], bool) {
	return newAbilityGateConfig(opts)
}

// NewProjectileAbilityGateConfig constructs a projectile ability gate
// configuration using the provided lookup adapter.
func NewProjectileAbilityGateConfig[State any, Owner any](opts AbilityGateOptions[State, Owner]) (AbilityGateConfig[Owner], bool) {
	return newAbilityGateConfig(opts)
}

func newAbilityGateConfig[State any, Owner any](opts AbilityGateOptions[State, Owner]) (AbilityGateConfig[Owner], bool) {
	if opts.AbilityID == "" || opts.Lookup == nil {
		return AbilityGateConfig[Owner]{}, false
	}

	lookup := opts.Lookup
	cfg := AbilityGateConfig[Owner]{
		AbilityID: opts.AbilityID,
		Cooldown:  opts.Cooldown,
		LookupOwner: func(actorID string) (*Owner, *map[string]time.Time, bool) {
			if lookup == nil {
				return nil, nil, false
			}
			return lookup(actorID)
		},
	}

	return cfg, true
}
