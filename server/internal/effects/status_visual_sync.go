package effects

import (
	effectcontract "mine-and-die/server/effects/contract"
)

// ActorPosition captures the coordinates of an actor used for contract-managed
// status visuals. Callers provide the world-space position so the sync helper
// can mirror the historical offset and motion metadata without depending on the
// legacy world types.
type ActorPosition struct {
	X float64
	Y float64
}

// StatusVisualSyncConfig carries the state required to synchronize a
// contract-managed status visual instance with its legacy effect representation.
type StatusVisualSyncConfig struct {
	Instance         *effectcontract.EffectInstance
	Effect           *State
	Actor            *ActorPosition
	TileSize         float64
	DefaultFootprint float64
}

// SyncContractStatusVisualInstance mirrors the legacy helper that kept the
// contract burning visual in lockstep with its world effect. Geometry and motion
// metadata are updated so downstream consumers observe identical behaviour while
// the sync logic lives inside the effects package.
func SyncContractStatusVisualInstance(cfg StatusVisualSyncConfig) {
	instance := cfg.Instance
	effect := cfg.Effect
	if instance == nil || effect == nil {
		return
	}

	defaultFootprint := cfg.DefaultFootprint
	width := effect.Width
	if width <= 0 {
		width = defaultFootprint
	}
	height := effect.Height
	if height <= 0 {
		height = defaultFootprint
	}

	geometry := instance.DeliveryState.Geometry
	geometry.Width = QuantizeWorldCoord(width, cfg.TileSize)
	geometry.Height = QuantizeWorldCoord(height, cfg.TileSize)

	actor := cfg.Actor
	if actor != nil {
		effectCenterX := effect.X + effect.Width/2
		effectCenterY := effect.Y + effect.Height/2
		geometry.OffsetX = QuantizeWorldCoord(effectCenterX-actor.X, cfg.TileSize)
		geometry.OffsetY = QuantizeWorldCoord(effectCenterY-actor.Y, cfg.TileSize)
	}

	motion := instance.DeliveryState.Motion
	centerX := effect.X + effect.Width/2
	centerY := effect.Y + effect.Height/2
	if actor != nil {
		centerX = actor.X
		centerY = actor.Y
	}
	motion.PositionX = QuantizeWorldCoord(centerX, cfg.TileSize)
	motion.PositionY = QuantizeWorldCoord(centerY, cfg.TileSize)
	motion.VelocityX = 0
	motion.VelocityY = 0

	instance.DeliveryState.Geometry = geometry
	instance.DeliveryState.Motion = motion
}
