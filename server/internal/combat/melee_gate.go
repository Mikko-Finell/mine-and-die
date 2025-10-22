package combat

import "time"

// AbilityOwnerRef captures the minimal owner metadata required by ability gates.
// Callers populate the identifier, cooldown registry, and a legacy reference
// that downstream code can use to enqueue effect intents.
type AbilityOwnerRef struct {
	ActorID   string
	Cooldowns *map[string]time.Time
	Reference any
}

// MeleeAbilityGate provides gating for melee ability triggers. Callers pass in
// the actor identifier and current wall-clock time; the gate consults the
// configured lookup and cooldown tracker to determine whether the ability may
// execute and returns the resolved owner reference when successful.
type MeleeAbilityGate func(actorID string, now time.Time) (AbilityOwnerRef, bool)

// ProjectileAbilityGate provides gating for projectile ability triggers using
// the shared ability owner reference contract.
type ProjectileAbilityGate func(actorID string, now time.Time) (AbilityOwnerRef, bool)

// MeleeAbilityGateConfig bundles the dependencies required to reproduce the
// legacy melee ability gating semantics without importing the server package.
type MeleeAbilityGateConfig struct {
	AbilityID   string
	Cooldown    time.Duration
	LookupOwner func(actorID string) (AbilityOwnerRef, bool)
}

// ProjectileAbilityGateConfig carries the dependencies required to reproduce
// the legacy projectile ability gating semantics.
type ProjectileAbilityGateConfig struct {
	AbilityID   string
	Cooldown    time.Duration
	LookupOwner func(actorID string) (AbilityOwnerRef, bool)
}

type abilityGateConfig struct {
	AbilityID   string
	Cooldown    time.Duration
	LookupOwner func(actorID string) (AbilityOwnerRef, bool)
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

func newAbilityGate(cfg abilityGateConfig) func(actorID string, now time.Time) (AbilityOwnerRef, bool) {
	if cfg.LookupOwner == nil || cfg.AbilityID == "" {
		return nil
	}
	return func(actorID string, now time.Time) (AbilityOwnerRef, bool) {
		if actorID == "" {
			return AbilityOwnerRef{}, false
		}
		owner, ok := cfg.LookupOwner(actorID)
		if !ok || owner.ActorID == "" || owner.Cooldowns == nil {
			return AbilityOwnerRef{}, false
		}
		if !ReadyCooldown(owner.Cooldowns, cfg.AbilityID, cfg.Cooldown, now) {
			return AbilityOwnerRef{}, false
		}
		return owner, true
	}
}

// NewMeleeAbilityGate constructs an adapter that reproduces the legacy melee
// ability gating semantics using the provided configuration.
func NewMeleeAbilityGate(cfg MeleeAbilityGateConfig) MeleeAbilityGate {
	gate := newAbilityGate(abilityGateConfig{
		AbilityID:   cfg.AbilityID,
		Cooldown:    cfg.Cooldown,
		LookupOwner: cfg.LookupOwner,
	})
	if gate == nil {
		return nil
	}
	return MeleeAbilityGate(gate)
}

// NewProjectileAbilityGate constructs an adapter that reproduces the legacy
// projectile ability gating semantics using the provided configuration.
func NewProjectileAbilityGate(cfg ProjectileAbilityGateConfig) ProjectileAbilityGate {
	gate := newAbilityGate(abilityGateConfig{
		AbilityID:   cfg.AbilityID,
		Cooldown:    cfg.Cooldown,
		LookupOwner: cfg.LookupOwner,
	})
	if gate == nil {
		return nil
	}
	return ProjectileAbilityGate(gate)
}
