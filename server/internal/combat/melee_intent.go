package combat

import (
	"math"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

const (
	// MeleeAttackCooldown mirrors the legacy swing rate between melee triggers.
	MeleeAttackCooldown = 400 * time.Millisecond
	// MeleeAttackDuration captures how long the melee area remains active.
	MeleeAttackDuration = 150 * time.Millisecond
	// MeleeAttackReach defines how far the melee area extends from the owner.
	MeleeAttackReach = 56.0
	// MeleeAttackWidth defines the lateral thickness of the melee area.
	MeleeAttackWidth = 40.0
	// MeleeAttackDamage is the default damage applied by melee swings.
	MeleeAttackDamage = 10.0
)

// MeleeAttackGeometryConfig carries the spatial parameters required to build the
// melee attack rectangle relative to the owner.
type MeleeAttackGeometryConfig struct {
	PlayerHalf    float64
	Reach         float64
	Width         float64
	DefaultFacing string
}

// MeleeIntentConfig bundles the dependencies required to reproduce the legacy
// melee intent construction without importing the server package.
type MeleeIntentConfig struct {
	Geometry        MeleeAttackGeometryConfig
	TileSize        float64
	Damage          float64
	Duration        time.Duration
	QuantizeCoord   func(float64) int
	DurationToTicks func(time.Duration) int
}

// MeleeIntentOwner captures the minimal actor metadata required to stage a
// melee intent.
type MeleeIntentOwner struct {
	ID     string
	X      float64
	Y      float64
	Facing string
}

// NewMeleeIntent bridges the melee trigger into a contract EffectIntent so the
// effect manager observes the same swing geometry and payload metadata.
func NewMeleeIntent(cfg MeleeIntentConfig, owner MeleeIntentOwner) (effectcontract.EffectIntent, bool) {
	if owner.ID == "" || cfg.QuantizeCoord == nil || cfg.DurationToTicks == nil {
		return effectcontract.EffectIntent{}, false
	}
	if cfg.TileSize == 0 {
		return effectcontract.EffectIntent{}, false
	}

	facing := owner.Facing
	if facing == "" {
		facing = cfg.Geometry.DefaultFacing
	}

	rectX, rectY, rectW, rectH := MeleeAttackRectangle(cfg.Geometry, owner.X, owner.Y, facing)
	centerX := rectX + rectW/2
	centerY := rectY + rectH/2

	quantizeWorldCoord := func(value float64) int {
		return cfg.QuantizeCoord(value / cfg.TileSize)
	}

	geometry := effectcontract.EffectGeometry{
		Shape:   effectcontract.GeometryShapeRect,
		Width:   quantizeWorldCoord(rectW),
		Height:  quantizeWorldCoord(rectH),
		OffsetX: quantizeWorldCoord(centerX - owner.X),
		OffsetY: quantizeWorldCoord(centerY - owner.Y),
	}

	params := map[string]int{
		"healthDelta": int(math.Round(-cfg.Damage)),
		"reach":       int(math.Round(cfg.Geometry.Reach)),
		"width":       int(math.Round(cfg.Geometry.Width)),
	}

	intent := effectcontract.EffectIntent{
		EntryID:       EffectTypeAttack,
		TypeID:        EffectTypeAttack,
		Delivery:      effectcontract.DeliveryKindArea,
		SourceActorID: owner.ID,
		Geometry:      geometry,
		DurationTicks: cfg.DurationToTicks(cfg.Duration),
		Params:        params,
	}

	return intent, true
}

// MeleeAttackRectangle builds the hitbox in front of a player for a melee swing.
func MeleeAttackRectangle(cfg MeleeAttackGeometryConfig, x, y float64, facing string) (float64, float64, float64, float64) {
	reach := cfg.Reach
	thickness := cfg.Width
	half := cfg.PlayerHalf

	if facing == "" {
		facing = cfg.DefaultFacing
	}

	switch facing {
	case "up":
		return x - thickness/2, y - half - reach, thickness, reach
	case "down":
		return x - thickness/2, y + half, thickness, reach
	case "left":
		return x - half - reach, y - thickness/2, reach, thickness
	case "right":
		return x + half, y - thickness/2, reach, thickness
	default:
		return x - thickness/2, y + half, thickness, reach
	}
}
