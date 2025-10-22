package effects

import (
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

// MeleeOwner captures the owner position and legacy reference required to
// resolve melee impact events while the hook lives inside the effects package.
// Callers populate the reference with their world-specific actor pointer so the
// resolver can continue to access inventories and telemetry helpers without the
// package importing server internals.
type MeleeOwner struct {
	X         float64
	Y         float64
	Reference any
}

// MeleeImpactArea describes the rectangular region covered by a melee swing in
// world coordinates. The hook reports the computed footprint to the impact
// resolver so legacy collision and telemetry helpers can run unchanged.
type MeleeImpactArea struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// MeleeSpawnHookConfig bundles the dependencies required to translate contract
// melee spawn events into legacy world impact resolution. Callers supply owner
// lookups and impact callbacks so the hook can operate without importing the
// server package.
type MeleeSpawnHookConfig struct {
	TileSize        float64
	DefaultWidth    float64
	DefaultReach    float64
	DefaultDamage   float64
	DefaultDuration time.Duration
	LookupOwner     func(actorID string) *MeleeOwner
	ResolveImpact   func(effect *State, owner *MeleeOwner, actorID string, tick effectcontract.Tick, now time.Time, area MeleeImpactArea)
}

// MeleeSpawnHook returns the spawn handler that converts contract melee
// instances into legacy effect state and delegates impact resolution through the
// provided callback.
func MeleeSpawnHook(cfg MeleeSpawnHookConfig) HookSet {
	return HookSet{
		OnSpawn: func(_ Runtime, instance *effectcontract.EffectInstance, tick effectcontract.Tick, now time.Time) {
			if instance == nil {
				return
			}

			owner := lookupMeleeOwner(cfg.LookupOwner, instance.OwnerActorID)
			if owner == nil {
				return
			}

			effect, area := meleeEffectFromInstance(cfg, instance, owner, now)
			if effect == nil {
				return
			}

			if cfg.ResolveImpact != nil {
				cfg.ResolveImpact(effect, owner, instance.OwnerActorID, tick, now, area)
			}
		},
	}
}

func lookupMeleeOwner(lookup func(string) *MeleeOwner, actorID string) *MeleeOwner {
	if lookup == nil || actorID == "" {
		return nil
	}
	return lookup(actorID)
}

func meleeEffectFromInstance(cfg MeleeSpawnHookConfig, instance *effectcontract.EffectInstance, owner *MeleeOwner, now time.Time) (*State, MeleeImpactArea) {
	if instance == nil || owner == nil {
		return nil, MeleeImpactArea{}
	}

	geom := instance.DeliveryState.Geometry
	width := DequantizeWorldCoord(geom.Width, cfg.TileSize)
	height := DequantizeWorldCoord(geom.Height, cfg.TileSize)
	if width <= 0 {
		width = cfg.DefaultWidth
	}
	if height <= 0 {
		height = cfg.DefaultReach
	}

	offsetX := DequantizeWorldCoord(geom.OffsetX, cfg.TileSize)
	offsetY := DequantizeWorldCoord(geom.OffsetY, cfg.TileSize)
	centerX := owner.X + offsetX
	centerY := owner.Y + offsetY
	rectX := centerX - width/2
	rectY := centerY - height/2

	motion := instance.DeliveryState.Motion
	motion.PositionX = QuantizeWorldCoord(centerX, cfg.TileSize)
	motion.PositionY = QuantizeWorldCoord(centerY, cfg.TileSize)
	motion.VelocityX = 0
	motion.VelocityY = 0
	instance.DeliveryState.Motion = motion

	params := IntMapToFloat64(instance.BehaviorState.Extra)
	if params == nil {
		params = make(map[string]float64)
	}
	if _, ok := params["healthDelta"]; !ok {
		params["healthDelta"] = -cfg.DefaultDamage
	}
	if _, ok := params["reach"]; !ok {
		params["reach"] = cfg.DefaultReach
	}
	if _, ok := params["width"]; !ok {
		params["width"] = cfg.DefaultWidth
	}

	duration := cfg.DefaultDuration
	if duration < 0 {
		duration = 0
	}

	effect := &State{
		ID:                 instance.ID,
		Type:               instance.DefinitionID,
		Owner:              instance.OwnerActorID,
		Start:              now.UnixMilli(),
		Duration:           duration.Milliseconds(),
		X:                  rectX,
		Y:                  rectY,
		Width:              width,
		Height:             height,
		Params:             params,
		Instance:           *instance,
		TelemetrySpawnTick: instance.StartTick,
	}

	area := MeleeImpactArea{X: rectX, Y: rectY, Width: width, Height: height}
	return effect, area
}
