package combat

import (
	"math"

	internaleffects "mine-and-die/server/internal/effects"
	worldpkg "mine-and-die/server/internal/world"
)

// ProjectileAdvanceStopFunc applies stop semantics for projectile motion when
// the helper decides the instance should end due to impact or expiry.
type ProjectileAdvanceStopFunc func(triggerImpact bool, triggerExpiry bool)

// ProjectileAdvanceConfig bundles the adapters required to advance a legacy
// projectile without importing the server package. Callers provide the effect
// state alongside callbacks that update position, persist remaining range, and
// stop the projectile when travel or collision rules demand it.
type ProjectileAdvanceConfig struct {
	Effect *internaleffects.State
	Delta  float64

	WorldWidth  float64
	WorldHeight float64

	ComputeArea        func() worldpkg.Obstacle
	AnyObstacleOverlap func(worldpkg.Obstacle) bool

	SetPosition       func(x, y float64)
	SetRemainingRange func(remaining float64)
	Stop              ProjectileAdvanceStopFunc

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
// helper updates the projectile state through the supplied callbacks and
// signals stop conditions via the Stop callback so callers can handle telemetry
// and teardown.
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
	if template == nil {
		if cfg.Stop != nil {
			cfg.Stop(false, false)
		}
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
		if cfg.Stop != nil {
			cfg.Stop(false, true)
		}
		result.Stopped = true
		result.StoppedForExpiry = true
		return result
	}

	if cfg.WorldWidth > 0 || cfg.WorldHeight > 0 {
		if effect.X < 0 || effect.Y < 0 ||
			(cfg.WorldWidth > 0 && effect.X+effect.Width > cfg.WorldWidth) ||
			(cfg.WorldHeight > 0 && effect.Y+effect.Height > cfg.WorldHeight) {
			if cfg.Stop != nil {
				cfg.Stop(false, true)
			}
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
		if cfg.Stop != nil {
			cfg.Stop(true, false)
		}
		result.Stopped = true
		result.StoppedForImpact = true
		return result
	}

	overlapCfg := cfg.OverlapConfig
	overlapCfg.Projectile = projectile
	overlapCfg.Area = area

	result.OverlapResult = ResolveProjectileOverlaps(overlapCfg)

	if result.OverlapResult.ShouldStop {
		if cfg.Stop != nil {
			cfg.Stop(false, false)
		}
		result.Stopped = true
	}

	return result
}
