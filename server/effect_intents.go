package main

import (
	"math"
	"time"
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

// quantizeWorldCoord translates a world-space measurement (expressed in the
// same units as actor positions) into the fixed-point coordinate system used by
// the unified effect contract. World units are normalised to tile units so the
// client can dequantize using its tile size.
func quantizeWorldCoord(value float64) int {
	return QuantizeCoord(value / tileSize)
}

// copyFloatParams rounds a float64 map into the integer Params representation
// expected by the contract helpers. Values are rounded to the nearest integer
// because legacy gameplay data is stored in world units that already align to a
// whole-number grid.
func copyFloatParams(source map[string]float64) map[string]int {
	if len(source) == 0 {
		return nil
	}
	params := make(map[string]int, len(source))
	for key, value := range source {
		if key == "" {
			continue
		}
		params[key] = int(math.Round(value))
	}
	return params
}

// NewMeleeIntent bridges the legacy melee trigger into a contract EffectIntent
// so the EffectManager observes the same swing geometry and payload metadata.
func NewMeleeIntent(owner *actorState) (EffectIntent, bool) {
	if owner == nil || owner.ID == "" {
		return EffectIntent{}, false
	}

	facing := owner.Facing
	if facing == "" {
		facing = defaultFacing
	}

	rectX, rectY, rectW, rectH := meleeAttackRectangle(owner.X, owner.Y, facing)
	centerX := rectX + rectW/2
	centerY := rectY + rectH/2

	geometry := EffectGeometry{
		Shape:   GeometryShapeRect,
		Width:   quantizeWorldCoord(rectW),
		Height:  quantizeWorldCoord(rectH),
		OffsetX: quantizeWorldCoord(centerX - owner.X),
		OffsetY: quantizeWorldCoord(centerY - owner.Y),
	}

	params := map[string]int{
		"healthDelta": int(math.Round(-meleeAttackDamage)),
		"reach":       int(math.Round(meleeAttackReach)),
		"width":       int(math.Round(meleeAttackWidth)),
	}

	intent := EffectIntent{
		TypeID:        effectTypeAttack,
		Delivery:      DeliveryKindArea,
		SourceActorID: owner.ID,
		Geometry:      geometry,
		DurationTicks: durationToTicks(meleeAttackDuration),
		Params:        params,
	}

	return intent, true
}

// NewProjectileIntent converts a projectile template and owner into an
// EffectIntent that mirrors the spawn metadata used by the legacy projectile
// systems.
func NewProjectileIntent(owner *actorState, tpl *ProjectileTemplate) (EffectIntent, bool) {
	if owner == nil || owner.ID == "" || tpl == nil || tpl.Type == "" {
		return EffectIntent{}, false
	}

	facing := owner.Facing
	if facing == "" {
		facing = defaultFacing
	}
	dirX, dirY := facingToVector(facing)
	if dirX == 0 && dirY == 0 {
		dirX, dirY = 0, 1
	}

	spawnRadius := sanitizedSpawnRadius(tpl.SpawnRadius)
	spawnOffset := tpl.SpawnOffset
	if spawnOffset == 0 {
		spawnOffset = ownerHalfExtent(owner) + spawnRadius
	}

	centerX := owner.X + dirX*spawnOffset
	centerY := owner.Y + dirY*spawnOffset

	width, height := spawnSizeFromShape(tpl)

	geometry := EffectGeometry{
		Shape:   GeometryShapeCircle,
		OffsetX: quantizeWorldCoord(centerX - owner.X),
		OffsetY: quantizeWorldCoord(centerY - owner.Y),
	}

	if tpl.CollisionShape.UseRect {
		geometry.Shape = GeometryShapeRect
		geometry.Width = quantizeWorldCoord(width)
		geometry.Height = quantizeWorldCoord(height)
	} else {
		geometry.Radius = quantizeWorldCoord(spawnRadius)
		if width > 0 {
			geometry.Width = quantizeWorldCoord(width)
		}
		if height > 0 {
			geometry.Height = quantizeWorldCoord(height)
		}
	}

	params := copyFloatParams(tpl.Params)
	if params == nil {
		params = make(map[string]int)
	}
	params["dx"] = int(math.Round(dirX))
	params["dy"] = int(math.Round(dirY))
	if _, ok := params["radius"]; !ok {
		params["radius"] = int(math.Round(spawnRadius))
	}
	if _, ok := params["speed"]; !ok {
		params["speed"] = int(math.Round(tpl.Speed))
	}
	if _, ok := params["range"]; !ok {
		params["range"] = int(math.Round(tpl.MaxDistance))
	}

	intent := EffectIntent{
		TypeID:        tpl.Type,
		Delivery:      DeliveryKindArea,
		SourceActorID: owner.ID,
		Geometry:      geometry,
		Params:        params,
	}

	return intent, true
}
