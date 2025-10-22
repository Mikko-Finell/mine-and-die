package combat

import "time"

// MeleeAbilityGate provides gating for melee ability triggers. Callers pass in
// the actor identifier and current wall-clock time; the gate consults the
// configured lookup and cooldown tracker to determine whether the ability may
// execute and returns the resolved owner reference when successful.
type MeleeAbilityGate func(actorID string, now time.Time) (MeleeIntentOwner, bool)

// ProjectileAbilityGate provides gating for projectile ability triggers using
// the shared intent owner contract.
type ProjectileAbilityGate func(actorID string, now time.Time) (ProjectileIntentOwner, bool)

// MeleeAbilityGateConfig bundles the dependencies required to reproduce the
// legacy melee ability gating semantics without importing the server package.
type MeleeAbilityGateConfig struct {
	AbilityID   string
	Cooldown    time.Duration
	LookupOwner func(actorID string) (*AbilityActor, *map[string]time.Time, bool)
}

// ProjectileAbilityGateConfig carries the dependencies required to reproduce
// the legacy projectile ability gating semantics.
type ProjectileAbilityGateConfig struct {
	AbilityID   string
	Cooldown    time.Duration
	LookupOwner func(actorID string) (*AbilityActor, *map[string]time.Time, bool)
}

type abilityGateConfig[T any] struct {
	AbilityID    string
	Cooldown     time.Duration
	LookupOwner  func(actorID string) (*AbilityActor, *map[string]time.Time, bool)
	ConvertOwner func(*AbilityActor) (T, bool)
}

// ReadyCooldown mirrors the legacy cooldown bookkeeping: it lazily allocates
// the registry map, refuses to trigger when the ability is still on cooldown,
// and records the trigger timestamp when the ability is ready.
func ReadyCooldown(cooldowns *map[string]time.Time, ability string, cooldown time.Duration, now time.Time) bool {
	if cooldowns == nil {
		return false
	}
	if *cooldowns == nil {
		*cooldowns = make(map[string]time.Time)
	}
	if cooldown > 0 {
		if last, ok := (*cooldowns)[ability]; ok {
			if now.Sub(last) < cooldown {
				return false
			}
		}
	}
	(*cooldowns)[ability] = now
	return true
}

func newAbilityGate[T any](cfg abilityGateConfig[T]) func(actorID string, now time.Time) (T, bool) {
	var zero T
	if cfg.LookupOwner == nil || cfg.ConvertOwner == nil || cfg.AbilityID == "" {
		return nil
	}
	return func(actorID string, now time.Time) (T, bool) {
		if actorID == "" {
			return zero, false
		}
		actor, cooldowns, ok := cfg.LookupOwner(actorID)
		if !ok || cooldowns == nil {
			return zero, false
		}
		owner, ok := cfg.ConvertOwner(actor)
		if !ok {
			return zero, false
		}
		if !ReadyCooldown(cooldowns, cfg.AbilityID, cfg.Cooldown, now) {
			return zero, false
		}
		return owner, true
	}
}

// NewMeleeAbilityGate constructs an adapter that reproduces the legacy melee
// ability gating semantics using the provided configuration.
func NewMeleeAbilityGate(cfg MeleeAbilityGateConfig) MeleeAbilityGate {
	gate := newAbilityGate[MeleeIntentOwner](abilityGateConfig[MeleeIntentOwner]{
		AbilityID:    cfg.AbilityID,
		Cooldown:     cfg.Cooldown,
		LookupOwner:  cfg.LookupOwner,
		ConvertOwner: NewMeleeIntentOwnerFromActor,
	})
	if gate == nil {
		return nil
	}
	return MeleeAbilityGate(gate)
}

// NewProjectileAbilityGate constructs an adapter that reproduces the legacy
// projectile ability gating semantics using the provided configuration.
func NewProjectileAbilityGate(cfg ProjectileAbilityGateConfig) ProjectileAbilityGate {
	gate := newAbilityGate[ProjectileIntentOwner](abilityGateConfig[ProjectileIntentOwner]{
		AbilityID:    cfg.AbilityID,
		Cooldown:     cfg.Cooldown,
		LookupOwner:  cfg.LookupOwner,
		ConvertOwner: NewProjectileIntentOwnerFromActor,
	})
	if gate == nil {
		return nil
	}
	return ProjectileAbilityGate(gate)
}
