package combat

import (
	"math"
	"testing"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

func TestStageProjectileIntentReturnsIntent(t *testing.T) {
	tileSize := 40.0
	cfg := ProjectileIntentConfig{
		TileSize:      tileSize,
		DefaultFacing: "down",
		QuantizeCoord: func(value float64) int { return int(math.Round(value * effectcontract.CoordScale)) },
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
	}

	expectedOwner := ProjectileIntentOwner{ID: "caster", X: 120, Y: 160, Facing: "right"}

	gate := func(actorID string, now time.Time) (ProjectileIntentOwner, bool) {
		if actorID != expectedOwner.ID {
			t.Fatalf("expected gate actor %q, got %q", expectedOwner.ID, actorID)
		}
		_ = now
		return expectedOwner, true
	}

	tpl := ProjectileIntentTemplate{
		Type:        EffectTypeFireball,
		Speed:       320,
		MaxDistance: 200,
		SpawnRadius: 12,
		SpawnOffset: 46,
		Params: map[string]float64{
			"radius": 12,
			"speed":  320,
			"range":  200,
		},
	}

	intent, ok := StageProjectileIntent(ProjectileAbilityTriggerConfig{
		AbilityGate:  gate,
		IntentConfig: cfg,
		Template:     tpl,
	}, expectedOwner.ID, time.Unix(1, 0))
	if !ok {
		t.Fatalf("expected projectile intent to be staged")
	}

	if intent.TypeID != tpl.Type || intent.SourceActorID != expectedOwner.ID {
		t.Fatalf("expected staged intent for %q owned by %q", tpl.Type, expectedOwner.ID)
	}
}

func TestStageProjectileIntentRequiresValidOwner(t *testing.T) {
	intent, ok := StageProjectileIntent(ProjectileAbilityTriggerConfig{
		AbilityGate: func(string, time.Time) (ProjectileIntentOwner, bool) {
			return ProjectileIntentOwner{}, true
		},
		Template: ProjectileIntentTemplate{Type: EffectTypeFireball},
	}, "caster", time.Unix(0, 0))
	if ok {
		t.Fatalf("expected invalid owner failure, got %#v", intent)
	}
}

func TestStageProjectileIntentRequiresGate(t *testing.T) {
	intent, ok := StageProjectileIntent(ProjectileAbilityTriggerConfig{
		Template: ProjectileIntentTemplate{Type: EffectTypeFireball},
	}, "caster", time.Unix(0, 0))
	if ok {
		t.Fatalf("expected staging to fail without gate, got %#v", intent)
	}
}
