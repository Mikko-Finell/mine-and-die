package world

import "time"

// LegacyProjectileOverlapTarget mirrors the overlap visitor payload used when
// scanning players and NPCs for projectile collisions. Callers forward the
// legacy actor metadata through this struct so the advancer can convert it into
// the combat overlap target format without importing legacy types.
type LegacyProjectileOverlapTarget struct {
	ID     string
	X      float64
	Y      float64
	Radius float64
	Raw    any
}

// LegacyProjectileOverlapVisitor consumes a potential overlap target and
// returns true when iteration should continue. Returning false stops the
// traversal early.
type LegacyProjectileOverlapVisitor func(target LegacyProjectileOverlapTarget) bool

// LegacyProjectileStepAdvancer applies the combat projectile step for the provided
// configuration and reports whether the projectile stopped during the tick. The
// advancer implementation lives outside the world package so callers can
// delegate to combat without introducing an import cycle.
type LegacyProjectileStepAdvancer func(LegacyProjectileStepAdvanceConfig) LegacyProjectileStepAdvanceResult

// LegacyProjectileStepAdvanceResult captures the advancer's outcome, including an
// opaque payload for callers that need to observe the raw combat result.
type LegacyProjectileStepAdvanceResult struct {
	Stopped bool
	Raw     any
}

// LegacyProjectileStepAdvanceConfig bundles the world-driven dependencies required
// to advance a legacy projectile while delegating the combat call through the
// supplied advancer.
type LegacyProjectileStepAdvanceConfig struct {
	Effect any
	Delta  float64
	Now    time.Time

	WorldWidth  float64
	WorldHeight float64

	ComputeArea        func() Obstacle
	AnyObstacleOverlap func(Obstacle) bool
	SetPosition        func(x, y float64)

	StopBindings   ProjectileStopConfig
	BindStopConfig func(ProjectileStopConfig, any, time.Time) any

	RecordAttackOverlap func(ownerID string, tick uint64, ability string, playerHits []string, npcHits []string, metadata map[string]any)
	CurrentTick         uint64

	VisitPlayers func(visitor LegacyProjectileOverlapVisitor)
	VisitNPCs    func(visitor LegacyProjectileOverlapVisitor)

	OnPlayerHit func(target LegacyProjectileOverlapTarget)
	OnNPCHit    func(target LegacyProjectileOverlapTarget)
}

// LegacyProjectileStepConfig carries the world-specific callbacks required to
// wire the legacy projectile step without importing the server package.
type LegacyProjectileStepConfig struct {
	Effect any
	Now    time.Time
	Delta  float64

	HasProjectile func(effect any) bool

	Dimensions         func() (float64, float64)
	ComputeArea        func(effect any) Obstacle
	AnyObstacleOverlap func(Obstacle) bool
	SetPosition        func(effect any, x, y float64)

	StopAdapter         ProjectileStopAdapter
	BindStopConfig      func(ProjectileStopConfig, any, time.Time) any
	RecordAttackOverlap func(ownerID string, tick uint64, ability string, playerHits []string, npcHits []string, metadata map[string]any)
	CurrentTick         func() uint64

	VisitPlayers func(visitor LegacyProjectileOverlapVisitor)
	VisitNPCs    func(visitor LegacyProjectileOverlapVisitor)

	OnPlayerHit func(target LegacyProjectileOverlapTarget)
	OnNPCHit    func(target LegacyProjectileOverlapTarget)

	Advance LegacyProjectileStepAdvancer
}

// AdvanceLegacyProjectile orchestrates the world-facing portions of the legacy
// projectile step and delegates the combat integration to the supplied
// advancer. Callers provide the legacy effect reference alongside geometry and
// overlap helpers so the world package owns the wiring while combat retains the
// physics logic.
func AdvanceLegacyProjectile(cfg LegacyProjectileStepConfig) LegacyProjectileStepAdvanceResult {
	if cfg.Effect == nil || cfg.HasProjectile == nil || cfg.Advance == nil {
		return LegacyProjectileStepAdvanceResult{}
	}

	if !cfg.HasProjectile(cfg.Effect) {
		return LegacyProjectileStepAdvanceResult{}
	}

	var worldWidth, worldHeight float64
	if cfg.Dimensions != nil {
		worldWidth, worldHeight = cfg.Dimensions()
	}

	stopBindings := cfg.StopAdapter.StopConfig(cfg.Effect, cfg.Now)

	advanceCfg := LegacyProjectileStepAdvanceConfig{
		Effect:      cfg.Effect,
		Delta:       cfg.Delta,
		Now:         cfg.Now,
		WorldWidth:  worldWidth,
		WorldHeight: worldHeight,
		ComputeArea: func() Obstacle {
			if cfg.ComputeArea == nil {
				return Obstacle{}
			}
			return cfg.ComputeArea(cfg.Effect)
		},
		AnyObstacleOverlap: func(obstacle Obstacle) bool {
			if cfg.AnyObstacleOverlap == nil {
				return false
			}
			return cfg.AnyObstacleOverlap(obstacle)
		},
		SetPosition: func(x, y float64) {
			if cfg.SetPosition == nil {
				return
			}
			cfg.SetPosition(cfg.Effect, x, y)
		},
		StopBindings:        stopBindings,
		BindStopConfig:      cfg.BindStopConfig,
		RecordAttackOverlap: cfg.RecordAttackOverlap,
		VisitPlayers:        cfg.VisitPlayers,
		VisitNPCs:           cfg.VisitNPCs,
		OnPlayerHit:         cfg.OnPlayerHit,
		OnNPCHit:            cfg.OnNPCHit,
	}

	if cfg.CurrentTick != nil {
		advanceCfg.CurrentTick = cfg.CurrentTick()
	}

	return cfg.Advance(advanceCfg)
}
