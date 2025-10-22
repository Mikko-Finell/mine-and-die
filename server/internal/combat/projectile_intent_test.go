package combat

import (
	"math"
	"testing"

	effectcontract "mine-and-die/server/effects/contract"
)

func TestNewProjectileIntentConstructsIntent(t *testing.T) {
	tileSize := 40.0

	quantize := func(value float64) int {
		return int(math.Round(value * effectcontract.CoordScale))
	}

	cfg := ProjectileIntentConfig{
		TileSize:      tileSize,
		DefaultFacing: "down",
		QuantizeCoord: quantize,
		FacingVector: func(facing string) (float64, float64) {
			switch facing {
			case "up":
				return 0, -1
			case "down":
				return 0, 1
			case "left":
				return -1, 0
			case "right":
				return 1, 0
			default:
				return 0, 1
			}
		},
		OwnerHalfExtent: func(ProjectileIntentOwner) float64 {
			return 20
		},
	}

	owner := ProjectileIntentOwner{ID: "player-2", X: 160, Y: 160, Facing: "right"}
	tpl := ProjectileIntentTemplate{
		Type:        EffectTypeFireball,
		Speed:       320,
		MaxDistance: 160,
		SpawnRadius: 12,
		SpawnOffset: 46,
		CollisionShape: ProjectileIntentCollisionShape{
			UseRect: false,
		},
		Params: map[string]float64{
			"radius": 12,
			"speed":  320,
			"range":  160,
		},
	}

	intent, ok := NewProjectileIntent(cfg, owner, tpl)
	if !ok {
		t.Fatalf("expected projectile intent to be constructed")
	}

	if intent.EntryID != tpl.Type || intent.TypeID != tpl.Type {
		t.Fatalf("expected projectile type %q, got entry=%q type=%q", tpl.Type, intent.EntryID, intent.TypeID)
	}
	if intent.SourceActorID != owner.ID {
		t.Fatalf("expected source %q, got %q", owner.ID, intent.SourceActorID)
	}

	quantizeWorld := func(value float64) int {
		return quantize(value / tileSize)
	}

	expectedOffsetX := quantizeWorld(tpl.SpawnOffset)
	if intent.Geometry.OffsetX != expectedOffsetX {
		t.Fatalf("expected offsetX %d, got %d", expectedOffsetX, intent.Geometry.OffsetX)
	}
	if intent.Geometry.OffsetY != 0 {
		t.Fatalf("expected offsetY 0, got %d", intent.Geometry.OffsetY)
	}

	expectedRadius := quantizeWorld(tpl.SpawnRadius)
	if intent.Geometry.Radius != expectedRadius {
		t.Fatalf("expected radius %d, got %d", expectedRadius, intent.Geometry.Radius)
	}

	if intent.Params["dx"] != 1 || intent.Params["dy"] != 0 {
		t.Fatalf("expected direction (1,0), got (%d,%d)", intent.Params["dx"], intent.Params["dy"])
	}
	if intent.Params["range"] != int(math.Round(tpl.MaxDistance)) {
		t.Fatalf("expected range %d, got %d", int(math.Round(tpl.MaxDistance)), intent.Params["range"])
	}
}

func TestNewProjectileIntentDefaultsSpawnOffset(t *testing.T) {
	tileSize := 40.0

	cfg := ProjectileIntentConfig{
		TileSize:      tileSize,
		DefaultFacing: "down",
		QuantizeCoord: func(value float64) int { return int(math.Round(value * effectcontract.CoordScale)) },
		FacingVector:  func(string) (float64, float64) { return 0, 1 },
		OwnerHalfExtent: func(ProjectileIntentOwner) float64 {
			return 18
		},
	}

	owner := ProjectileIntentOwner{ID: "caster", X: 120, Y: 160, Facing: ""}
	tpl := ProjectileIntentTemplate{Type: EffectTypeFireball, SpawnRadius: 5}

	intent, ok := NewProjectileIntent(cfg, owner, tpl)
	if !ok {
		t.Fatalf("expected projectile intent to be constructed")
	}

	quantizeWorld := func(value float64) int { return cfg.QuantizeCoord(value / cfg.TileSize) }

	expectedOffsetY := quantizeWorld(cfg.OwnerHalfExtent(owner) + sanitizeSpawnRadius(tpl.SpawnRadius))
	if intent.Geometry.OffsetY != expectedOffsetY {
		t.Fatalf("expected offsetY %d, got %d", expectedOffsetY, intent.Geometry.OffsetY)
	}
	if intent.Params["radius"] != int(math.Round(sanitizeSpawnRadius(tpl.SpawnRadius))) {
		t.Fatalf("expected radius param %d, got %d", int(math.Round(sanitizeSpawnRadius(tpl.SpawnRadius))), intent.Params["radius"])
	}
}
