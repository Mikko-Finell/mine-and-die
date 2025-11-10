package combat

import (
	"testing"
	"time"

	internaleffects "mine-and-die/server/internal/effects"
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
					ExplodeOnImpact: &internaleffects.ExplosionSpec{
						EffectType: "impact-explosion",
						Radius:     3,
						Duration:   750 * time.Millisecond,
						Params: map[string]float64{
							"damage": 12,
						},
					},
				},
			},
		},
	}

	var recordedPositions [][2]float64
	var remainingValues []float64
	telemetryStops := 0
	overlapCount := 0
	var recordedArea Rectangle
	spawnCount := 0
	spawnTelemetry := 0
	var registeredExplosion *internaleffects.State
	now := time.Unix(0, 123*int64(time.Millisecond))

	spawnCfg := internaleffects.AreaEffectSpawnConfig{
		Now: now,
		AllocateID: func() string {
			spawnCount++
			return "explosion-id"
		},
		Register: func(effect *internaleffects.State) bool {
			registeredExplosion = effect
			return true
		},
		RecordSpawn: func(effectType, category string) {
			spawnTelemetry++
			if effectType != "impact-explosion" || category != "explosion" {
				t.Fatalf("unexpected spawn telemetry payload type=%s category=%s", effectType, category)
			}
		},
	}

	var stopReasons []string
	setRemainingRange := func(remaining float64) {
		remainingValues = append(remainingValues, remaining)
	}

	cfg := ProjectileAdvanceConfig{
		Effect:      effect,
		Delta:       0.5,
		WorldWidth:  100,
		WorldHeight: 100,
		ComputeArea: func() Rectangle {
			area := Rectangle{X: effect.X, Y: effect.Y, Width: effect.Width, Height: effect.Height}
			recordedArea = area
			return area
		},
		AnyObstacleOverlap: func(area Rectangle) bool {
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
		SetRemainingRange: setRemainingRange,
		Stop: ProjectileStopConfig{
			Effect:            effect,
			Now:               now,
			SetRemainingRange: setRemainingRange,
			AreaEffectSpawn:   &spawnCfg,
			RecordEffectEnd: func(reason string) {
				telemetryStops++
				stopReasons = append(stopReasons, reason)
			},
		},
		AreaEffectSpawn: &spawnCfg,
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
	if len(stopReasons) != 0 {
		t.Fatalf("expected no stop reasons recorded, got %v", stopReasons)
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
	if spawnCount != 1 {
		t.Fatalf("expected explosion to spawn once, got %d", spawnCount)
	}
	if spawnTelemetry != 1 {
		t.Fatalf("expected spawn telemetry once, got %d", spawnTelemetry)
	}
	if registeredExplosion == nil {
		t.Fatalf("expected explosion to register")
	}
	if registeredExplosion.ID != "explosion-id" {
		t.Fatalf("expected explosion ID to propagate, got %s", registeredExplosion.ID)
	}
	if registeredExplosion.Type != "impact-explosion" {
		t.Fatalf("expected explosion type to propagate, got %s", registeredExplosion.Type)
	}
	if registeredExplosion.Owner != effect.Owner {
		t.Fatalf("expected explosion owner %s, got %s", effect.Owner, registeredExplosion.Owner)
	}
	if registeredExplosion.Width != 6 || registeredExplosion.Height != 6 {
		t.Fatalf("expected explosion dimensions 6x6, got %.1fx%.1f", registeredExplosion.Width, registeredExplosion.Height)
	}
	if registeredExplosion.X != 3 || registeredExplosion.Y != -2 {
		t.Fatalf("expected explosion positioned at (3,-2), got (%.1f, %.1f)", registeredExplosion.X, registeredExplosion.Y)
	}
	if registeredExplosion.Duration != (750 * time.Millisecond).Milliseconds() {
		t.Fatalf("expected explosion duration 750ms, got %d", registeredExplosion.Duration)
	}
	if radius, ok := registeredExplosion.Params["radius"]; !ok || radius != 3 {
		t.Fatalf("expected explosion radius param 3, got %v", registeredExplosion.Params)
	}
	if damage, ok := registeredExplosion.Params["damage"]; !ok || damage != 12 {
		t.Fatalf("expected explosion to include damage param 12, got %v", registeredExplosion.Params)
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

	now := time.Unix(0, 200*int64(time.Millisecond))
	stops := 0
	var recordedReasons []string
	var remainingValues []float64
	setRemainingRange := func(remaining float64) {
		remainingValues = append(remainingValues, remaining)
	}
	cfg := ProjectileAdvanceConfig{
		Effect:      effect,
		Delta:       0.5,
		WorldWidth:  100,
		WorldHeight: 100,
		ComputeArea: func() Rectangle { return Rectangle{} },
		SetPosition: func(x, y float64) {
			effect.X = x
			effect.Y = y
		},
		SetRemainingRange: setRemainingRange,
		Stop: ProjectileStopConfig{
			Effect:            effect,
			Now:               now,
			SetRemainingRange: setRemainingRange,
			RecordEffectEnd: func(reason string) {
				stops++
				recordedReasons = append(recordedReasons, reason)
			},
		},
		OverlapConfig: ProjectileOverlapResolutionConfig{},
	}

	result := AdvanceProjectile(cfg)

	if stops != 1 {
		t.Fatalf("expected stop telemetry once, got %d", stops)
	}
	if len(recordedReasons) != 1 || recordedReasons[0] != "expiry" {
		t.Fatalf("expected expiry reason, got %v", recordedReasons)
	}
	if len(remainingValues) != 1 || remainingValues[0] != 0 {
		t.Fatalf("expected range update to record 0, got %v", remainingValues)
	}
	if !effect.Projectile.ExpiryResolved {
		t.Fatalf("expected projectile expiry resolved")
	}
	if !effect.ExpiresAt.Equal(now) {
		t.Fatalf("expected expiry timestamp to clamp to now, got %v", effect.ExpiresAt)
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

	now := time.Unix(0, 300*int64(time.Millisecond))
	stops := 0
	var recordedReasons []string
	var remainingValues []float64
	setRemainingRange := func(remaining float64) {
		remainingValues = append(remainingValues, remaining)
	}
	cfg := ProjectileAdvanceConfig{
		Effect:      effect,
		Delta:       0.1,
		WorldWidth:  100,
		WorldHeight: 100,
		ComputeArea: func() Rectangle {
			return Rectangle{X: effect.X, Y: effect.Y, Width: effect.Width, Height: effect.Height}
		},
		AnyObstacleOverlap: func(area Rectangle) bool { return true },
		SetPosition: func(x, y float64) {
			effect.X = x
			effect.Y = y
		},
		SetRemainingRange: setRemainingRange,
		Stop: ProjectileStopConfig{
			Effect:            effect,
			Now:               now,
			SetRemainingRange: setRemainingRange,
			RecordEffectEnd: func(reason string) {
				stops++
				recordedReasons = append(recordedReasons, reason)
			},
		},
		OverlapConfig: ProjectileOverlapResolutionConfig{},
	}

	result := AdvanceProjectile(cfg)

	if stops != 1 {
		t.Fatalf("expected stop telemetry once, got %d", stops)
	}
	if len(recordedReasons) != 1 || recordedReasons[0] != "impact" {
		t.Fatalf("expected impact reason, got %v", recordedReasons)
	}
	if len(remainingValues) != 2 || remainingValues[0] != 9 || remainingValues[1] != 0 {
		t.Fatalf("expected range updates [9 0], got %v", remainingValues)
	}
	if !effect.Projectile.ExpiryResolved {
		t.Fatalf("expected projectile expiry resolved")
	}
	if !effect.ExpiresAt.Equal(now) {
		t.Fatalf("expected expiry timestamp to clamp to now, got %v", effect.ExpiresAt)
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

	now := time.Unix(0, 400*int64(time.Millisecond))
	stops := 0
	var recordedReasons []string
	cfg := ProjectileAdvanceConfig{
		Effect: effect,
		Delta:  0.5,
		Stop: ProjectileStopConfig{
			Effect: effect,
			Now:    now,
			RecordEffectEnd: func(reason string) {
				stops++
				recordedReasons = append(recordedReasons, reason)
			},
		},
	}

	result := AdvanceProjectile(cfg)

	if stops != 1 {
		t.Fatalf("expected stop callback once, got %d", stops)
	}
	if len(recordedReasons) != 1 || recordedReasons[0] != "stopped" {
		t.Fatalf("expected generic stop reason, got %v", recordedReasons)
	}
	if !effect.Projectile.ExpiryResolved {
		t.Fatalf("expected projectile expiry resolved")
	}
	if !effect.ExpiresAt.Equal(now) {
		t.Fatalf("expected expiry timestamp to clamp to now, got %v", effect.ExpiresAt)
	}
	if !result.Stopped || result.StoppedForImpact || result.StoppedForExpiry {
		t.Fatalf("expected generic stop result, got %+v", result)
	}
}
