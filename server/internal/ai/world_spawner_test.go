package ai

import (
	"testing"

	worldpkg "mine-and-die/server/internal/world"
)

func TestWorldNPCSpawner_Defaults(t *testing.T) {
	var spawner WorldNPCSpawner

	cfg := spawner.Config()
	if cfg != worldpkg.DefaultConfig() {
		t.Fatalf("expected default config, got %+v", cfg)
	}

	width, height := spawner.Dimensions()
	if width != worldpkg.DefaultWidth || height != worldpkg.DefaultHeight {
		t.Fatalf("expected default dimensions %.1fx%.1f, got %.1fx%.1f", worldpkg.DefaultWidth, worldpkg.DefaultHeight, width, height)
	}

	rng := spawner.SubsystemRNG("npcs.extra")
	if rng == nil {
		t.Fatalf("expected fallback RNG")
	}
	want := worldpkg.NewDeterministicRNG(worldpkg.DefaultSeed, "npcs.extra")
	if rng.Float64() != want.Float64() {
		t.Fatalf("expected deterministic fallback RNG value")
	}
}

func TestWorldNPCSpawner_SpawnCallbacks(t *testing.T) {
	var (
		goblinX, goblinY float64
		goblinGold       int
		goblinPotion     int
		goblinWaypoints  []worldpkg.Vec2
		ratX, ratY       float64
	)

	spawner := WorldNPCSpawner{
		SpawnGoblinFunc: func(x, y float64, waypoints []worldpkg.Vec2, goldQty, potionQty int) {
			goblinX = x
			goblinY = y
			goblinGold = goldQty
			goblinPotion = potionQty
			goblinWaypoints = append([]worldpkg.Vec2(nil), waypoints...)
		},
		SpawnRatFunc: func(x, y float64) {
			ratX = x
			ratY = y
		},
	}

	waypoints := []worldpkg.Vec2{{X: 10, Y: 20}, {X: 30, Y: 40}}
	spawner.SpawnGoblinAt(5, 6, waypoints, 7, 8)
	spawner.SpawnRatAt(11, 12)

	if goblinX != 5 || goblinY != 6 {
		t.Fatalf("unexpected goblin spawn position %.1f, %.1f", goblinX, goblinY)
	}
	if goblinGold != 7 || goblinPotion != 8 {
		t.Fatalf("unexpected goblin inventory %d gold, %d potions", goblinGold, goblinPotion)
	}
	if len(goblinWaypoints) != len(waypoints) {
		t.Fatalf("expected %d waypoints, got %d", len(waypoints), len(goblinWaypoints))
	}
	if goblinWaypoints[0] != waypoints[0] || goblinWaypoints[1] != waypoints[1] {
		t.Fatalf("unexpected waypoint copy: %+v", goblinWaypoints)
	}
	if ratX != 11 || ratY != 12 {
		t.Fatalf("unexpected rat spawn position %.1f, %.1f", ratX, ratY)
	}
}

func TestSeedInitialNPCs_DelegatesToCallbacks(t *testing.T) {
	var goblinCount, ratCount int
	spawner := WorldNPCSpawner{
		ConfigFunc: func() worldpkg.Config {
			cfg := worldpkg.DefaultConfig()
			cfg.NPCs = true
			cfg.GoblinCount = 1
			cfg.RatCount = 1
			return cfg
		},
		SpawnGoblinFunc: func(x, y float64, waypoints []worldpkg.Vec2, goldQty, potionQty int) {
			goblinCount++
		},
		SpawnRatFunc: func(x, y float64) {
			ratCount++
		},
	}

	SeedInitialNPCs(spawner)

	if goblinCount != 1 {
		t.Fatalf("expected 1 goblin spawn, got %d", goblinCount)
	}
	if ratCount != 1 {
		t.Fatalf("expected 1 rat spawn, got %d", ratCount)
	}
}
