package combat

import (
	"time"

	internaleffects "mine-and-die/server/internal/effects"
)

// ProjectileStopOptions capture the stop triggers that should be applied when
// ending a projectile.
type ProjectileStopOptions struct {
	TriggerImpact bool
	TriggerExpiry bool
}

// ProjectileStopConfig bundles the adapters required to stop a legacy
// projectile while delegating world-specific hooks to the caller.
type ProjectileStopConfig struct {
	Effect  *internaleffects.State
	Now     time.Time
	Options ProjectileStopOptions

	SetRemainingRange func(float64)
	AreaEffectSpawn   *internaleffects.AreaEffectSpawnConfig
	RecordEffectEnd   func(reason string)
}

// StopProjectile applies the stop semantics for the provided projectile,
// updating range and expiry bookkeeping while invoking the supplied callbacks
// for telemetry and explosion handling.
func StopProjectile(cfg ProjectileStopConfig) {
	effect := cfg.Effect
	if effect == nil {
		return
	}
	projectile := effect.Projectile
	if projectile == nil {
		return
	}

	if projectile.RemainingRange != 0 {
		projectile.RemainingRange = 0
		if cfg.SetRemainingRange != nil {
			cfg.SetRemainingRange(0)
		}
	}

	if projectile.ExpiryResolved {
		if effect.ExpiresAt.After(cfg.Now) {
			effect.ExpiresAt = cfg.Now
		}
		return
	}

	template := projectile.Template
	opts := cfg.Options

	spawnExplosion := func(spec *internaleffects.ExplosionSpec) {
		if spec == nil {
			return
		}
		if cfg.AreaEffectSpawn == nil {
			return
		}
		spawnCfg := *cfg.AreaEffectSpawn
		if spawnCfg.Source == nil {
			spawnCfg.Source = effect
		}
		spawnCfg.Spec = spec
		if spawnCfg.Now.IsZero() {
			spawnCfg.Now = cfg.Now
		}
		internaleffects.SpawnAreaEffect(spawnCfg)
	}

	if opts.TriggerImpact && template != nil {
		if spec := template.ImpactRules.ExplodeOnImpact; spec != nil {
			spawnExplosion(spec)
		}
	}

	if opts.TriggerExpiry && template != nil {
		if spec := template.ImpactRules.ExplodeOnExpiry; spec != nil {
			if !template.ImpactRules.ExpiryOnlyIfNoHits || projectile.HitCount == 0 {
				spawnExplosion(spec)
			}
		}
	}

	reason := "stopped"
	if opts.TriggerImpact {
		reason = "impact"
	} else if opts.TriggerExpiry {
		reason = "expiry"
	}

	projectile.ExpiryResolved = true
	effect.ExpiresAt = cfg.Now
	if cfg.RecordEffectEnd != nil {
		cfg.RecordEffectEnd(reason)
	}
}
