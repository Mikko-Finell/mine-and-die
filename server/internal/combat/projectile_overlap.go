package combat

import (
	internaleffects "mine-and-die/server/internal/effects"
)

// ProjectileImpactRules captures the subset of projectile impact policies needed
// to resolve multi-target overlaps for legacy projectile instances.
type ProjectileImpactRules struct {
	StopOnHit    bool
	MaxTargets   int
	AffectsOwner bool
}

// ProjectileOverlapTarget carries the metadata required to evaluate projectile
// overlap against a potential target while preserving access to the original
// reference for hit callbacks.
type ProjectileOverlapTarget struct {
	ID     string
	X      float64
	Y      float64
	Radius float64
	Raw    any
}

// ProjectileOverlapVisitor consumes a candidate target and returns true when
// iteration should continue. Returning false stops the scan early.
type ProjectileOverlapVisitor func(target ProjectileOverlapTarget) bool

// ProjectileOverlapResolutionConfig bundles the adapters required to process
// projectile overlaps without importing the legacy world package.
type ProjectileOverlapResolutionConfig struct {
	Projectile *internaleffects.ProjectileState
	Impact     ProjectileImpactRules

	OwnerID  string
	Ability  string
	Tick     uint64
	Metadata map[string]any

	Area Rectangle

	RecordAttackOverlap func(ownerID string, tick uint64, ability string, playerHits []string, npcHits []string, metadata map[string]any)

	VisitPlayers func(visitor ProjectileOverlapVisitor)
	VisitNPCs    func(visitor ProjectileOverlapVisitor)

	OnPlayerHit func(target ProjectileOverlapTarget)
	OnNPCHit    func(target ProjectileOverlapTarget)
}

// ProjectileOverlapResolutionResult reports the outcome of resolving projectile
// overlaps for the provided configuration.
type ProjectileOverlapResolutionResult struct {
	HitsApplied int
	ShouldStop  bool
}

// ResolveProjectileOverlaps scans the provided player and NPC iterators,
// applying projectile impact rules and invoking the supplied callbacks for each
// newly registered hit.
func ResolveProjectileOverlaps(cfg ProjectileOverlapResolutionConfig) ProjectileOverlapResolutionResult {
	result := ProjectileOverlapResolutionResult{}

	projectile := cfg.Projectile
	if projectile == nil {
		return result
	}

	hitCountAtStart := projectile.HitCount
	playerHits := make([]string, 0)
	npcHits := make([]string, 0)

	processTarget := func(target ProjectileOverlapTarget, onHit func(ProjectileOverlapTarget), hits *[]string) bool {
		if target.ID == "" {
			return true
		}
		if !cfg.Impact.AffectsOwner && target.ID == cfg.OwnerID {
			return true
		}
		if !CircleRectOverlap(target.X, target.Y, target.Radius, cfg.Area) {
			return true
		}
		if !projectile.MarkHit(target.ID) {
			return true
		}

		if onHit != nil {
			onHit(target)
		}
		*hits = append(*hits, target.ID)

		if cfg.Impact.StopOnHit || (cfg.Impact.MaxTargets > 0 && projectile.HitCount >= cfg.Impact.MaxTargets) {
			result.ShouldStop = true
			return false
		}
		return true
	}

	if cfg.VisitPlayers != nil {
		cfg.VisitPlayers(func(target ProjectileOverlapTarget) bool {
			return processTarget(target, cfg.OnPlayerHit, &playerHits)
		})
	}

	if !result.ShouldStop && cfg.VisitNPCs != nil {
		cfg.VisitNPCs(func(target ProjectileOverlapTarget) bool {
			return processTarget(target, cfg.OnNPCHit, &npcHits)
		})
	}

	result.HitsApplied = projectile.HitCount - hitCountAtStart

	if result.HitsApplied > 0 && cfg.RecordAttackOverlap != nil && (len(playerHits) > 0 || len(npcHits) > 0) {
		cfg.RecordAttackOverlap(cfg.OwnerID, cfg.Tick, cfg.Ability, playerHits, npcHits, cfg.Metadata)
	}

	return result
}
