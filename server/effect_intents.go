package server

import (
	"math"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	combat "mine-and-die/server/internal/combat"
)

// durationToTicks converts a wall-clock duration into the number of simulation
// ticks using the shared tickRate. Durations shorter than a single tick still
// return at least one tick so short-lived effects remain visible.
func durationToTicks(duration time.Duration) int {
	if duration <= 0 {
		return 0
	}
	ticks := int(math.Ceil(duration.Seconds() * float64(tickRate)))
	if ticks < 1 {
		ticks = 1
	}
	return ticks
}

var meleeIntentConfig = combat.MeleeIntentConfig{
	Geometry: combat.MeleeAttackGeometryConfig{
		PlayerHalf:    playerHalf,
		Reach:         combat.MeleeAttackReach,
		Width:         combat.MeleeAttackWidth,
		DefaultFacing: string(defaultFacing),
	},
	TileSize:        tileSize,
	Damage:          combat.MeleeAttackDamage,
	Duration:        combat.MeleeAttackDuration,
	QuantizeCoord:   QuantizeCoord,
	DurationToTicks: durationToTicks,
}

var projectileIntentConfig = combat.ProjectileIntentConfig{
	TileSize:      tileSize,
	DefaultFacing: string(defaultFacing),
	QuantizeCoord: QuantizeCoord,
	FacingVector: func(facing string) (float64, float64) {
		return facingToVector(FacingDirection(facing))
	},
	OwnerHalfExtent: func(combat.ProjectileIntentOwner) float64 {
		return playerHalf
	},
}

// quantizeWorldCoord translates a world-space measurement (expressed in the
// same units as actor positions) into the fixed-point coordinate system used by
// the unified effect contract. World units are normalised to tile units so the
// client can dequantize using its tile size.
func quantizeWorldCoord(value float64) int {
	return QuantizeCoord(value / tileSize)
}

func newMeleeIntent(owner *actorState) (effectcontract.EffectIntent, bool) {
	if owner == nil {
		return effectcontract.EffectIntent{}, false
	}

	return combat.NewMeleeIntent(meleeIntentConfig, combat.MeleeIntentOwner{
		ID:     owner.ID,
		X:      owner.X,
		Y:      owner.Y,
		Facing: string(owner.Facing),
	})
}

// NewProjectileIntent converts a projectile template and owner into an
// EffectIntent that mirrors the spawn metadata used by the legacy projectile
// systems.
func NewProjectileIntent(owner *actorState, tpl *ProjectileTemplate) (effectcontract.EffectIntent, bool) {
	if owner == nil || tpl == nil {
		return effectcontract.EffectIntent{}, false
	}

	return combat.NewProjectileIntent(
		projectileIntentConfig,
		combat.ProjectileIntentOwner{
			ID:     owner.ID,
			X:      owner.X,
			Y:      owner.Y,
			Facing: string(owner.Facing),
		},
		combat.ProjectileIntentTemplate{
			Type:        tpl.Type,
			Speed:       tpl.Speed,
			MaxDistance: tpl.MaxDistance,
			SpawnRadius: tpl.SpawnRadius,
			SpawnOffset: tpl.SpawnOffset,
			CollisionShape: combat.ProjectileIntentCollisionShape{
				UseRect: tpl.CollisionShape.UseRect,
				Width:   tpl.CollisionShape.RectWidth,
				Height:  tpl.CollisionShape.RectHeight,
			},
			Params: tpl.Params,
		},
	)
}

// NewStatusVisualIntent converts a status-effect attachment into an
// EffectIntent that follows the target actor for the duration of the status.
func NewStatusVisualIntent(target *actorState, sourceID, effectType string, lifetime time.Duration) (effectcontract.EffectIntent, bool) {
	if target == nil || target.ID == "" || effectType == "" {
		return effectcontract.EffectIntent{}, false
	}

	if lifetime <= 0 {
		lifetime = 100 * time.Millisecond
	}

	width := quantizeWorldCoord(playerHalf * 2)
	height := quantizeWorldCoord(playerHalf * 2)

	geometry := effectcontract.EffectGeometry{
		Shape:   effectcontract.GeometryShapeRect,
		Width:   width,
		Height:  height,
		OffsetX: 0,
		OffsetY: 0,
	}

	intent := effectcontract.EffectIntent{
		EntryID:       effectType,
		TypeID:        effectType,
		Delivery:      effectcontract.DeliveryKindTarget,
		SourceActorID: sourceID,
		TargetActorID: target.ID,
		Geometry:      geometry,
		DurationTicks: durationToTicks(lifetime),
	}

	if intent.DurationTicks < 1 {
		intent.DurationTicks = 1
	}

	if intent.SourceActorID == "" {
		intent.SourceActorID = target.ID
	}

	return intent, true
}
