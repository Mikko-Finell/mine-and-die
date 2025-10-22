package combat

import (
	"testing"
	"time"

	internaleffects "mine-and-die/server/internal/effects"
)

func TestStopProjectileTriggersExplosionsAndTelemetry(t *testing.T) {
	now := time.Unix(100, 0)
	impactSpec := &internaleffects.ExplosionSpec{EffectType: "impact"}
	expirySpec := &internaleffects.ExplosionSpec{EffectType: "expiry"}
	effect := &internaleffects.State{
		ExpiresAt: now.Add(time.Second),
		Projectile: &internaleffects.ProjectileState{
			RemainingRange: 10,
			Template: &internaleffects.ProjectileTemplate{
				ImpactRules: internaleffects.ImpactRuleConfig{
					ExplodeOnImpact:    impactSpec,
					ExplodeOnExpiry:    expirySpec,
					ExpiryOnlyIfNoHits: true,
				},
			},
		},
	}

	var remainingCalls []float64
	ids := []string{"impact-id", "expiry-id"}
	var registered []*internaleffects.State
	var recordedSpawns []string
	var recordedReason string

	StopProjectile(ProjectileStopConfig{
		Effect:  effect,
		Now:     now,
		Options: ProjectileStopOptions{TriggerImpact: true, TriggerExpiry: true},
		SetRemainingRange: func(remaining float64) {
			remainingCalls = append(remainingCalls, remaining)
		},
		AreaEffectSpawn: &internaleffects.AreaEffectSpawnConfig{
			Source: effect,
			Now:    now,
			AllocateID: func() string {
				if len(ids) == 0 {
					t.Fatalf("unexpected allocate call after ids exhausted")
					return ""
				}
				id := ids[0]
				ids = ids[1:]
				return id
			},
			Register: func(state *internaleffects.State) bool {
				registered = append(registered, state)
				return true
			},
			RecordSpawn: func(effectType, category string) {
				recordedSpawns = append(recordedSpawns, effectType+":"+category)
			},
		},
		RecordEffectEnd: func(reason string) {
			recordedReason = reason
		},
	})

	if effect.Projectile.RemainingRange != 0 {
		t.Fatalf("expected projectile remaining range to be 0, got %.2f", effect.Projectile.RemainingRange)
	}
	if len(remainingCalls) != 1 || remainingCalls[0] != 0 {
		t.Fatalf("expected SetRemainingRange to be called with 0 once, got %v", remainingCalls)
	}
	if !effect.Projectile.ExpiryResolved {
		t.Fatalf("expected projectile expiry to be resolved")
	}
	if !effect.ExpiresAt.Equal(now) {
		t.Fatalf("expected effect expiry to be clamped to now, got %v", effect.ExpiresAt)
	}
	if recordedReason != "impact" {
		t.Fatalf("expected record reason to be impact, got %q", recordedReason)
	}
	if len(registered) != 2 {
		t.Fatalf("expected both impact and expiry explosions, got %d", len(registered))
	}
	if registered[0].Type != impactSpec.EffectType || registered[1].Type != expirySpec.EffectType {
		t.Fatalf("expected registered effects to match impact then expiry specs, got %q -> %q", registered[0].Type, registered[1].Type)
	}
	if len(recordedSpawns) != 2 || recordedSpawns[0] != "impact:explosion" || recordedSpawns[1] != "expiry:explosion" {
		t.Fatalf("expected spawn telemetry for impact then expiry explosion, got %v", recordedSpawns)
	}
}

func TestStopProjectileSkipsExpiryExplosionAfterHits(t *testing.T) {
	now := time.Unix(200, 0)
	expirySpec := &internaleffects.ExplosionSpec{EffectType: "expiry"}
	effect := &internaleffects.State{
		ExpiresAt: now.Add(time.Second),
		Projectile: &internaleffects.ProjectileState{
			HitCount: 3,
			Template: &internaleffects.ProjectileTemplate{
				ImpactRules: internaleffects.ImpactRuleConfig{
					ExplodeOnExpiry:    expirySpec,
					ExpiryOnlyIfNoHits: true,
				},
			},
		},
	}

	StopProjectile(ProjectileStopConfig{
		Effect:  effect,
		Now:     now,
		Options: ProjectileStopOptions{TriggerExpiry: true},
		AreaEffectSpawn: &internaleffects.AreaEffectSpawnConfig{
			Source: effect,
			Now:    now,
			AllocateID: func() string {
				t.Fatalf("expected expiry explosion to be skipped after hits")
				return ""
			},
		},
	})

	if effect.ExpiresAt != now {
		t.Fatalf("expected expiry timestamp to update to now, got %v", effect.ExpiresAt)
	}
}

func TestStopProjectileClampsWhenAlreadyResolved(t *testing.T) {
	now := time.Unix(300, 0)
	later := now.Add(time.Minute)
	earlier := now.Add(-time.Minute)
	effect := &internaleffects.State{
		ExpiresAt: later,
		Projectile: &internaleffects.ProjectileState{
			ExpiryResolved: true,
		},
	}

	StopProjectile(ProjectileStopConfig{
		Effect:  effect,
		Now:     earlier,
		Options: ProjectileStopOptions{},
		AreaEffectSpawn: &internaleffects.AreaEffectSpawnConfig{
			AllocateID: func() string {
				t.Fatalf("expected no explosions when already resolved")
				return ""
			},
		},
		RecordEffectEnd: func(reason string) {
			t.Fatalf("expected no telemetry when already resolved, got %q", reason)
		},
	})

	if !effect.ExpiresAt.Equal(earlier) {
		t.Fatalf("expected expiry time to clamp to earlier timestamp, got %v", effect.ExpiresAt)
	}
}
