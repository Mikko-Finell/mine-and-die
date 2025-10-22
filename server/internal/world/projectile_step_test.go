package world

import (
	"testing"
	"time"
)

type legacyProjectileEffect struct {
	hasProjectile bool
	position      struct {
		x float64
		y float64
	}
	area Obstacle
}

func TestAdvanceLegacyProjectileDelegatesConfig(t *testing.T) {
	effect := &legacyProjectileEffect{
		hasProjectile: true,
		area:          Obstacle{X: 1, Y: 2, Width: 3, Height: 4},
	}

	adapterCalled := false
	adapter := NewProjectileStopAdapter(ProjectileStopAdapterConfig{})

	var captured LegacyProjectileStepAdvanceConfig
	advancerCalled := false

	result := AdvanceLegacyProjectile(LegacyProjectileStepConfig{
		Effect: effect,
		Now:    time.UnixMilli(1_700_000_000),
		Delta:  0.016,
		HasProjectile: func(effect any) bool {
			eff, _ := effect.(*legacyProjectileEffect)
			return eff != nil && eff.hasProjectile
		},
		Dimensions: func() (float64, float64) {
			return 1280, 720
		},
		ComputeArea: func(effect any) Obstacle {
			eff, _ := effect.(*legacyProjectileEffect)
			if eff == nil {
				return Obstacle{}
			}
			return eff.area
		},
		AnyObstacleOverlap: func(obstacle Obstacle) bool {
			return obstacle.X > 0
		},
		SetPosition: func(effect any, x, y float64) {
			eff, _ := effect.(*legacyProjectileEffect)
			if eff == nil {
				return
			}
			eff.position.x = x
			eff.position.y = y
		},
		StopAdapter: adapter,
		BindStopConfig: func(bindings ProjectileStopConfig, effect any, now time.Time) any {
			adapterCalled = true
			if bindings.Effect != effect {
				t.Fatalf("expected stop bindings to carry the effect reference")
			}
			if bindings.Now != time.UnixMilli(1_700_000_000) {
				t.Fatalf("expected stop timestamp to be forwarded")
			}
			return struct{}{}
		},
		RecordAttackOverlap: func(string, uint64, string, []string, []string, map[string]any) {},
		CurrentTick:         func() uint64 { return 42 },
		VisitPlayers: func(visitor LegacyProjectileOverlapVisitor) {
			visitor(LegacyProjectileOverlapTarget{ID: "player-1", X: 5, Y: 6, Radius: 7})
		},
		VisitNPCs: func(visitor LegacyProjectileOverlapVisitor) {
			visitor(LegacyProjectileOverlapTarget{ID: "npc-1", X: 8, Y: 9, Radius: 10})
		},
		OnPlayerHit: func(target LegacyProjectileOverlapTarget) {
			if target.ID != "player-hit" {
				t.Fatalf("unexpected player hit target %q", target.ID)
			}
		},
		OnNPCHit: func(target LegacyProjectileOverlapTarget) {
			if target.ID != "npc-hit" {
				t.Fatalf("unexpected npc hit target %q", target.ID)
			}
		},
		Advance: func(cfg LegacyProjectileStepAdvanceConfig) LegacyProjectileStepAdvanceResult {
			advancerCalled = true
			captured = cfg

			if cfg.Effect != effect {
				t.Fatalf("expected effect pointer to be forwarded")
			}
			if cfg.Delta != 0.016 {
				t.Fatalf("expected delta 0.016, got %f", cfg.Delta)
			}
			if cfg.WorldWidth != 1280 || cfg.WorldHeight != 720 {
				t.Fatalf("unexpected world dimensions %.1f x %.1f", cfg.WorldWidth, cfg.WorldHeight)
			}

			area := cfg.ComputeArea()
			if area != effect.area {
				t.Fatalf("expected compute area to return effect area, got %#v", area)
			}
			if !cfg.AnyObstacleOverlap(area) {
				t.Fatalf("expected obstacle overlap callback to return true")
			}

			cfg.SetPosition(11, 12)
			if effect.position.x != 11 || effect.position.y != 12 {
				t.Fatalf("expected position to update through callback")
			}

			if cfg.BindStopConfig == nil {
				t.Fatalf("expected bind stop config to be forwarded")
			}
			cfg.BindStopConfig(cfg.StopBindings, cfg.Effect, cfg.Now)

			if cfg.VisitPlayers == nil || cfg.VisitNPCs == nil {
				t.Fatalf("expected visitor callbacks to be forwarded")
			}
			playerVisited := false
			cfg.VisitPlayers(func(target LegacyProjectileOverlapTarget) bool {
				playerVisited = true
				if target.ID != "player-1" {
					t.Fatalf("unexpected player visitor target %q", target.ID)
				}
				return true
			})
			if !playerVisited {
				t.Fatalf("expected player visitor to be invoked")
			}

			npcVisited := false
			cfg.VisitNPCs(func(target LegacyProjectileOverlapTarget) bool {
				npcVisited = true
				if target.ID != "npc-1" {
					t.Fatalf("unexpected npc visitor target %q", target.ID)
				}
				return true
			})
			if !npcVisited {
				t.Fatalf("expected npc visitor to be invoked")
			}

			if cfg.OnPlayerHit == nil || cfg.OnNPCHit == nil {
				t.Fatalf("expected hit callbacks to be forwarded")
			}
			cfg.OnPlayerHit(LegacyProjectileOverlapTarget{ID: "player-hit"})
			cfg.OnNPCHit(LegacyProjectileOverlapTarget{ID: "npc-hit"})

			if cfg.CurrentTick != 42 {
				t.Fatalf("expected current tick 42, got %d", cfg.CurrentTick)
			}

			return LegacyProjectileStepAdvanceResult{Stopped: true, Raw: "combat-result"}
		},
	})

	if !advancerCalled {
		t.Fatalf("expected advancer to be invoked")
	}
	if !adapterCalled {
		t.Fatalf("expected bind stop config callback to be invoked")
	}
	if !result.Stopped {
		t.Fatalf("expected result to report stopped")
	}
	if result.Raw != "combat-result" {
		t.Fatalf("expected raw result to be forwarded")
	}

	if captured.Effect != effect {
		t.Fatalf("expected captured config to retain effect pointer")
	}
}

func TestAdvanceLegacyProjectileSkipsWhenNoProjectile(t *testing.T) {
	effect := &legacyProjectileEffect{}

	called := false
	result := AdvanceLegacyProjectile(LegacyProjectileStepConfig{
		Effect: effect,
		Now:    time.UnixMilli(0),
		Delta:  0.1,
		HasProjectile: func(effect any) bool {
			return false
		},
		Advance: func(cfg LegacyProjectileStepAdvanceConfig) LegacyProjectileStepAdvanceResult {
			called = true
			return LegacyProjectileStepAdvanceResult{Stopped: true}
		},
	})

	if called {
		t.Fatalf("expected advancer not to be invoked when projectile missing")
	}
	if result.Stopped {
		t.Fatalf("expected result to remain zero value when skipped")
	}
}
