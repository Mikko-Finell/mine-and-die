package combat

import (
	"math"

	internaleffects "mine-and-die/server/internal/effects"
	worldpkg "mine-and-die/server/internal/world"
)

// ProjectileAdvanceConfig bundles the adapters required to advance a legacy
// projectile without importing the server package. Callers provide the effect
// state alongside callbacks that update position, persist remaining range, and
// a stop configuration that applies expiry/impact semantics when travel or
// collision rules demand it.
type ProjectileAdvanceConfig struct {
	Effect *internaleffects.State
	Delta  float64

	WorldWidth  float64
	WorldHeight float64

	ComputeArea        func() worldpkg.Obstacle
	AnyObstacleOverlap func(worldpkg.Obstacle) bool

	SetPosition       func(x, y float64)
	SetRemainingRange func(remaining float64)
	Stop              ProjectileStopConfig

	AreaEffectSpawn *internaleffects.AreaEffectSpawnConfig

	OverlapConfig ProjectileOverlapResolutionConfig
}

// ProjectileAdvanceResult reports the outcome of advancing a projectile for a
// single tick, including whether motion stopped and the overlap resolution
// result from scanning nearby actors.
type ProjectileAdvanceResult struct {
	Stopped          bool
	StoppedForImpact bool
	StoppedForExpiry bool

	OverlapResult ProjectileOverlapResolutionResult
}

// AdvanceProjectile applies travel, range, boundary, and obstacle rules to the
// provided projectile before resolving overlaps against nearby actors. The
// helper updates the projectile state through the supplied callbacks and applies
// stop semantics through the provided configuration so callers can handle
// telemetry and teardown without wiring their own stop logic.
func AdvanceProjectile(cfg ProjectileAdvanceConfig) ProjectileAdvanceResult {
	result := ProjectileAdvanceResult{}

	effect := cfg.Effect
	if effect == nil {
		return result
	}
	projectile := effect.Projectile
	if projectile == nil {
		return result
	}

	template := projectile.Template
	stopWithOptions := func(options ProjectileStopOptions) {
		stopCfg := cfg.Stop
		if stopCfg.Effect == nil {
			stopCfg.Effect = effect
		}
		stopCfg.Options = options
		StopProjectile(stopCfg)
	}

	if template == nil {
		stopWithOptions(ProjectileStopOptions{})
		result.Stopped = true
		return result
	}

	if template.TravelMode.StraightLine && template.Speed > 0 && cfg.Delta > 0 {
		distance := template.Speed * cfg.Delta
		if projectile.RemainingRange > 0 && distance > projectile.RemainingRange {
			distance = projectile.RemainingRange
		}
		if distance > 0 {
			if cfg.SetPosition != nil {
				newX := effect.X + projectile.VelocityUnitX*distance
				newY := effect.Y + projectile.VelocityUnitY*distance
				cfg.SetPosition(newX, newY)
			}
			if projectile.RemainingRange > 0 {
				previous := projectile.RemainingRange
				projectile.RemainingRange -= distance
				if projectile.RemainingRange < 0 {
					projectile.RemainingRange = 0
				}
				if cfg.SetRemainingRange != nil && math.Abs(previous-projectile.RemainingRange) > 1e-9 {
					cfg.SetRemainingRange(projectile.RemainingRange)
				}
			}
		}
	}

	if template.MaxDistance > 0 && projectile.RemainingRange <= 0 {
		stopWithOptions(ProjectileStopOptions{TriggerExpiry: true})
		result.Stopped = true
		result.StoppedForExpiry = true
		return result
	}

	if cfg.WorldWidth > 0 || cfg.WorldHeight > 0 {
		if effect.X < 0 || effect.Y < 0 ||
			(cfg.WorldWidth > 0 && effect.X+effect.Width > cfg.WorldWidth) ||
			(cfg.WorldHeight > 0 && effect.Y+effect.Height > cfg.WorldHeight) {
			stopWithOptions(ProjectileStopOptions{TriggerExpiry: true})
			result.Stopped = true
			result.StoppedForExpiry = true
			return result
		}
	}

	var area worldpkg.Obstacle
	if cfg.ComputeArea != nil {
		area = cfg.ComputeArea()
	}

	if cfg.AnyObstacleOverlap != nil && cfg.AnyObstacleOverlap(area) {
		stopWithOptions(ProjectileStopOptions{TriggerImpact: true})
		result.Stopped = true
		result.StoppedForImpact = true
		return result
	}

	overlapCfg := cfg.OverlapConfig
	overlapCfg.Projectile = projectile
	overlapCfg.Area = area

	result.OverlapResult = ResolveProjectileOverlaps(overlapCfg)

	if result.OverlapResult.ShouldStop {
		stopWithOptions(ProjectileStopOptions{})
		result.Stopped = true
	}

	if result.OverlapResult.HitsApplied > 0 && template != nil {
		if spec := template.ImpactRules.ExplodeOnImpact; spec != nil {
			if cfg.AreaEffectSpawn != nil {
				spawnCfg := *cfg.AreaEffectSpawn
				if spawnCfg.Source == nil {
					spawnCfg.Source = effect
				}
				spawnCfg.Spec = spec
				internaleffects.SpawnAreaEffect(spawnCfg)
			}
		}
	}

	return result
}
