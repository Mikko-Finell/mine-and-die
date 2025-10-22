package world

import "time"

// LegacyProjectileState captures the metadata required to determine whether a
// legacy projectile should advance or stop.
type LegacyProjectileState struct {
	ContractManaged bool
	HasProjectile   bool
	ExpiresAt       time.Time
}

// LegacyProjectileAdvanceConfig bundles the callbacks required to iterate over
// legacy effects and advance their projectiles without importing the legacy
// world types.
type LegacyProjectileAdvanceConfig struct {
	Now   time.Time
	Delta float64

	ForEachEffect func(func(effect any))
	Inspect       func(effect any) LegacyProjectileState
	Advance       func(effect any, now time.Time, dt float64)

	StopAdapter    ProjectileStopAdapter
	StopProjectile ProjectileStopper
}

// AdvanceLegacyProjectiles walks legacy projectile effects, stopping expired
// instances through the provided stop callback and advancing the remaining
// projectiles via the supplied advance helper. Contract-managed and
// non-projectile effects are skipped to mirror the legacy loop behaviour.
func AdvanceLegacyProjectiles(cfg LegacyProjectileAdvanceConfig) {
	if cfg.ForEachEffect == nil || cfg.Inspect == nil || cfg.Advance == nil {
		return
	}

	cfg.ForEachEffect(func(effect any) {
		if effect == nil {
			return
		}

		state := cfg.Inspect(effect)
		if state.ContractManaged || !state.HasProjectile {
			return
		}

		if !cfg.Now.Before(state.ExpiresAt) {
			if cfg.StopProjectile != nil {
				bindings := cfg.StopAdapter.StopConfig(effect, cfg.Now)
				cfg.StopProjectile(bindings, ProjectileStopOptions{TriggerExpiry: true})
			}
			return
		}

		cfg.Advance(effect, cfg.Now, cfg.Delta)
	})
}

// StopLegacyProjectileOnExpiry applies the expiry stop semantics for the
// provided effect using the shared projectile stop adapter.
func StopLegacyProjectileOnExpiry(adapter ProjectileStopAdapter, effect any, now time.Time, stop ProjectileStopper) {
	if stop == nil || effect == nil {
		return
	}

	bindings := adapter.StopConfig(effect, now)
	stop(bindings, ProjectileStopOptions{TriggerExpiry: true})
}
