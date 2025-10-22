package world

import (
	"testing"
	"time"
)

type legacyProjectile struct {
	id              string
	contractManaged bool
	hasProjectile   bool
	expiresAt       time.Time
}

func TestAdvanceLegacyProjectilesStopsExpiredAndAdvancesActive(t *testing.T) {
	now := time.UnixMilli(1_700_000_000)
	effects := []*legacyProjectile{
		{id: "active", hasProjectile: true, expiresAt: now.Add(time.Second)},
		{id: "expired", hasProjectile: true, expiresAt: now.Add(-time.Millisecond)},
		{id: "contract", contractManaged: true, hasProjectile: true, expiresAt: now.Add(time.Second)},
		{id: "noproj", hasProjectile: false, expiresAt: now.Add(time.Second)},
		nil,
	}

	var advanced []string
	var stopped []string
	var stopOptions []ProjectileStopOptions

	AdvanceLegacyProjectiles(LegacyProjectileAdvanceConfig{
		Now:   now,
		Delta: 0.016,
		ForEachEffect: func(visitor func(effect any)) {
			for _, eff := range effects {
				visitor(eff)
			}
		},
		Inspect: func(effect any) LegacyProjectileState {
			eff, _ := effect.(*legacyProjectile)
			if eff == nil {
				return LegacyProjectileState{}
			}
			return LegacyProjectileState{
				ContractManaged: eff.contractManaged,
				HasProjectile:   eff.hasProjectile,
				ExpiresAt:       eff.expiresAt,
			}
		},
		Advance: func(effect any, now time.Time, dt float64) {
			eff, _ := effect.(*legacyProjectile)
			if eff != nil {
				advanced = append(advanced, eff.id)
			}
		},
		StopAdapter: ProjectileStopAdapter{},
		StopProjectile: func(cfg ProjectileStopConfig, opts ProjectileStopOptions) {
			eff, _ := cfg.Effect.(*legacyProjectile)
			if eff != nil {
				stopped = append(stopped, eff.id)
			} else {
				stopped = append(stopped, "<nil>")
			}
			stopOptions = append(stopOptions, opts)
		},
	})

	if len(advanced) != 1 || advanced[0] != "active" {
		t.Fatalf("expected only active projectile to advance, got %v", advanced)
	}

	if len(stopped) != 1 || stopped[0] != "expired" {
		t.Fatalf("expected only expired projectile to stop, got %v", stopped)
	}
	if len(stopOptions) != 1 || !stopOptions[0].TriggerExpiry || stopOptions[0].TriggerImpact {
		t.Fatalf("unexpected stop options %+v", stopOptions)
	}
}

func TestStopLegacyProjectileOnExpiryInvokesStopper(t *testing.T) {
	adapter := NewProjectileStopAdapter(ProjectileStopAdapterConfig{})
	effect := &legacyProjectile{id: "effect-expiry"}
	now := time.UnixMilli(1_600_000_000)

	called := false
	StopLegacyProjectileOnExpiry(adapter, effect, now, func(cfg ProjectileStopConfig, opts ProjectileStopOptions) {
		called = true
		if cfg.Effect != effect {
			t.Fatalf("expected stopper to receive original effect")
		}
		if cfg.Now != now {
			t.Fatalf("expected stopper to receive timestamp %v, got %v", now, cfg.Now)
		}
		if !opts.TriggerExpiry || opts.TriggerImpact {
			t.Fatalf("expected expiry-only stop options, got %+v", opts)
		}
	})

	if !called {
		t.Fatalf("expected stopper to be invoked")
	}
}
