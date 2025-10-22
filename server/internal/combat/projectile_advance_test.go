package combat

import (
	"testing"

	internaleffects "mine-and-die/server/internal/effects"
	worldpkg "mine-and-die/server/internal/world"
)

func TestAdvanceProjectileMovesAndResolvesOverlaps(t *testing.T) {
	effect := &internaleffects.State{
		X:      0,
		Y:      0,
		Width:  2,
		Height: 2,
		Owner:  "owner",
		Type:   "fireball",
		Projectile: &internaleffects.ProjectileState{
			VelocityUnitX:  1,
			VelocityUnitY:  0,
			RemainingRange: 15,
			Template: &internaleffects.ProjectileTemplate{
				Speed:       10,
				MaxDistance: 15,
				TravelMode:  internaleffects.TravelModeConfig{StraightLine: true},
				ImpactRules: internaleffects.ImpactRuleConfig{
					StopOnHit:    false,
					MaxTargets:   0,
					AffectsOwner: false,
				},
			},
		},
	}

	var recordedPositions [][2]float64
	var remainingValues []float64
	telemetryStops := 0
	stopImpact := false
	stopExpiry := false
	overlapCount := 0
	var recordedArea worldpkg.Obstacle

	cfg := ProjectileAdvanceConfig{
		Effect:      effect,
		Delta:       0.5,
		WorldWidth:  100,
		WorldHeight: 100,
		ComputeArea: func() worldpkg.Obstacle {
			area := worldpkg.Obstacle{X: effect.X, Y: effect.Y, Width: effect.Width, Height: effect.Height}
			recordedArea = area
			return area
		},
		AnyObstacleOverlap: func(area worldpkg.Obstacle) bool {
			if area != recordedArea {
				t.Fatalf("expected obstacle check to use computed area %+v, got %+v", recordedArea, area)
			}
			return false
		},
		SetPosition: func(x, y float64) {
			effect.X = x
			effect.Y = y
			recordedPositions = append(recordedPositions, [2]float64{x, y})
		},
		SetRemainingRange: func(remaining float64) {
			remainingValues = append(remainingValues, remaining)
		},
		Stop: func(triggerImpact, triggerExpiry bool) {
			telemetryStops++
			stopImpact = triggerImpact
			stopExpiry = triggerExpiry
		},
		OverlapConfig: ProjectileOverlapResolutionConfig{
			Impact: ProjectileImpactRules{
				StopOnHit:    false,
				MaxTargets:   0,
				AffectsOwner: false,
			},
			OwnerID:  effect.Owner,
			Ability:  effect.Type,
			Tick:     42,
			Metadata: map[string]any{"projectile": effect.Type},
			RecordAttackOverlap: func(ownerID string, tick uint64, ability string, playerHits []string, npcHits []string, metadata map[string]any) {
				overlapCount++
				if ownerID != effect.Owner || ability != effect.Type || tick != 42 {
					t.Fatalf("unexpected telemetry payload owner=%s ability=%s tick=%d", ownerID, ability, tick)
				}
				if metadata["projectile"] != effect.Type {
					t.Fatalf("expected telemetry metadata to include projectile type, got %v", metadata)
				}
			},
			VisitPlayers: func(visitor ProjectileOverlapVisitor) {
				target := ProjectileOverlapTarget{
					ID:     "player-1",
					X:      effect.X + effect.Width/2,
					Y:      effect.Y + effect.Height/2,
					Radius: 1,
					Raw:    "player-1",
				}
				if !visitor(target) {
					t.Fatalf("expected visitor to continue")
				}
			},
		},
	}

	result := AdvanceProjectile(cfg)

	if len(recordedPositions) != 1 {
		t.Fatalf("expected projectile position to update once, got %d", len(recordedPositions))
	}
	pos := recordedPositions[0]
	if pos[0] != 5 || pos[1] != 0 {
		t.Fatalf("expected projectile to move to (5,0), got (%.2f, %.2f)", pos[0], pos[1])
	}
	if len(remainingValues) != 1 || remainingValues[0] != 10 {
		t.Fatalf("expected remaining range to update to 10, got %v", remainingValues)
	}
	if telemetryStops != 0 {
		t.Fatalf("expected projectile to continue, but Stop called %d times", telemetryStops)
	}
	if stopImpact || stopExpiry {
		t.Fatalf("expected no stop flags, got impact=%t expiry=%t", stopImpact, stopExpiry)
	}
	if overlapCount != 1 {
		t.Fatalf("expected telemetry overlap recorded once, got %d", overlapCount)
	}
	if result.Stopped {
		t.Fatalf("expected projectile to continue, got stopped result")
	}
	if result.OverlapResult.HitsApplied != 1 {
		t.Fatalf("expected exactly one hit applied, got %d", result.OverlapResult.HitsApplied)
	}
}

func TestAdvanceProjectileStopsOnExpiry(t *testing.T) {
	effect := &internaleffects.State{
		X:      0,
		Y:      0,
		Width:  2,
		Height: 2,
		Projectile: &internaleffects.ProjectileState{
			VelocityUnitX:  1,
			VelocityUnitY:  0,
			RemainingRange: 2,
			Template: &internaleffects.ProjectileTemplate{
				Speed:       10,
				MaxDistance: 2,
				TravelMode:  internaleffects.TravelModeConfig{StraightLine: true},
			},
		},
	}

	stops := 0
	lastImpact := false
	lastExpiry := false
	cfg := ProjectileAdvanceConfig{
		Effect:      effect,
		Delta:       0.5,
		WorldWidth:  100,
		WorldHeight: 100,
		ComputeArea: func() worldpkg.Obstacle { return worldpkg.Obstacle{} },
		SetPosition: func(x, y float64) {
			effect.X = x
			effect.Y = y
		},
		SetRemainingRange: func(remaining float64) {},
		Stop: func(triggerImpact, triggerExpiry bool) {
			stops++
			lastImpact = triggerImpact
			lastExpiry = triggerExpiry
		},
		OverlapConfig: ProjectileOverlapResolutionConfig{},
	}

	result := AdvanceProjectile(cfg)

	if stops != 1 {
		t.Fatalf("expected stop callback to fire once, got %d", stops)
	}
	if !lastExpiry || lastImpact {
		t.Fatalf("expected expiry stop, got impact=%t expiry=%t", lastImpact, lastExpiry)
	}
	if !result.Stopped || !result.StoppedForExpiry {
		t.Fatalf("expected stopped-for-expiry result, got %+v", result)
	}
	if result.OverlapResult.HitsApplied != 0 {
		t.Fatalf("expected no overlaps processed on expiry stop, got %d", result.OverlapResult.HitsApplied)
	}
}

func TestAdvanceProjectileStopsOnObstacle(t *testing.T) {
	effect := &internaleffects.State{
		X:      1,
		Y:      1,
		Width:  2,
		Height: 2,
		Projectile: &internaleffects.ProjectileState{
			VelocityUnitX:  1,
			VelocityUnitY:  0,
			RemainingRange: 10,
			Template: &internaleffects.ProjectileTemplate{
				Speed:       10,
				MaxDistance: 10,
				TravelMode:  internaleffects.TravelModeConfig{StraightLine: true},
			},
		},
	}

	stops := 0
	lastImpact := false
	lastExpiry := false
	cfg := ProjectileAdvanceConfig{
		Effect:      effect,
		Delta:       0.1,
		WorldWidth:  100,
		WorldHeight: 100,
		ComputeArea: func() worldpkg.Obstacle {
			return worldpkg.Obstacle{X: effect.X, Y: effect.Y, Width: effect.Width, Height: effect.Height}
		},
		AnyObstacleOverlap: func(area worldpkg.Obstacle) bool { return true },
		SetPosition: func(x, y float64) {
			effect.X = x
			effect.Y = y
		},
		SetRemainingRange: func(float64) {},
		Stop: func(triggerImpact, triggerExpiry bool) {
			stops++
			lastImpact = triggerImpact
			lastExpiry = triggerExpiry
		},
		OverlapConfig: ProjectileOverlapResolutionConfig{},
	}

	result := AdvanceProjectile(cfg)

	if stops != 1 {
		t.Fatalf("expected stop callback once, got %d", stops)
	}
	if !lastImpact || lastExpiry {
		t.Fatalf("expected impact stop, got impact=%t expiry=%t", lastImpact, lastExpiry)
	}
	if !result.Stopped || !result.StoppedForImpact {
		t.Fatalf("expected stopped-for-impact result, got %+v", result)
	}
	if result.OverlapResult.HitsApplied != 0 {
		t.Fatalf("expected no overlaps processed on impact stop, got %d", result.OverlapResult.HitsApplied)
	}
}

func TestAdvanceProjectileStopsWhenTemplateMissing(t *testing.T) {
	effect := &internaleffects.State{
		Projectile: &internaleffects.ProjectileState{},
	}

	stops := 0
	cfg := ProjectileAdvanceConfig{
		Effect: effect,
		Delta:  0.5,
		Stop: func(triggerImpact, triggerExpiry bool) {
			stops++
			if triggerImpact || triggerExpiry {
				t.Fatalf("expected generic stop, got impact=%t expiry=%t", triggerImpact, triggerExpiry)
			}
		},
	}

	result := AdvanceProjectile(cfg)

	if stops != 1 {
		t.Fatalf("expected stop callback once, got %d", stops)
	}
	if !result.Stopped || result.StoppedForImpact || result.StoppedForExpiry {
		t.Fatalf("expected generic stop result, got %+v", result)
	}
}
