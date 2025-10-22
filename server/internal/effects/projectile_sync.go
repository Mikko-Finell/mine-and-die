package effects

import (
	"math"

	effectcontract "mine-and-die/server/effects/contract"
)

const defaultTickRate = 15

// ContractProjectileSyncConfig captures the inputs required to synchronize a
// contract-managed projectile instance with its runtime effect state.
type ContractProjectileSyncConfig struct {
	Instance *effectcontract.EffectInstance
	Owner    ProjectileOwner
	Effect   *State
	TileSize float64
	TickRate int
}

// SyncContractProjectileInstance mirrors the legacy sync routine used by the
// hub when reconciling contract-managed projectile instances. The helper keeps
// the contract instance payload aligned with the runtime effect without
// depending on server/world packages.
func SyncContractProjectileInstance(cfg ContractProjectileSyncConfig) {
	instance := cfg.Instance
	effect := cfg.Effect
	if instance == nil || effect == nil {
		return
	}

	if instance.BehaviorState.Extra == nil {
		instance.BehaviorState.Extra = make(map[string]int)
	}
	if instance.Params == nil {
		instance.Params = make(map[string]int)
	}

	if proj := effect.Projectile; proj != nil {
		remaining := int(math.Round(proj.RemainingRange))
		if remaining < 0 {
			remaining = 0
		}
		instance.BehaviorState.Extra["remainingRange"] = remaining
		instance.Params["remainingRange"] = remaining

		dx := quantizeCoord(proj.VelocityUnitX)
		dy := quantizeCoord(proj.VelocityUnitY)
		instance.BehaviorState.Extra["dx"] = dx
		instance.Params["dx"] = dx
		instance.BehaviorState.Extra["dy"] = dy
		instance.Params["dy"] = dy

		if tpl := proj.Template; tpl != nil && tpl.MaxDistance > 0 {
			distance := int(math.Round(tpl.MaxDistance))
			instance.BehaviorState.Extra["range"] = distance
			instance.Params["range"] = distance
		}
	}

	geometry := instance.DeliveryState.Geometry
	tileSize := cfg.TileSize
	if effect.Width > 0 {
		geometry.Width = QuantizeWorldCoord(effect.Width, tileSize)
	}
	if effect.Height > 0 {
		geometry.Height = QuantizeWorldCoord(effect.Height, tileSize)
	}
	radius := effect.Params["radius"]
	if radius <= 0 {
		radius = effect.Width / 2
	}
	geometry.Radius = QuantizeWorldCoord(radius, tileSize)

	if cfg.Owner != nil {
		ownerX, ownerY := cfg.Owner.Position()
		geometry.OffsetX = QuantizeWorldCoord(effectCenterX(effect)-ownerX, tileSize)
		geometry.OffsetY = QuantizeWorldCoord(effectCenterY(effect)-ownerY, tileSize)
	}

	motion := instance.DeliveryState.Motion
	motion.PositionX = QuantizeWorldCoord(effectCenterX(effect), tileSize)
	motion.PositionY = QuantizeWorldCoord(effectCenterY(effect), tileSize)
	if proj := effect.Projectile; proj != nil {
		speed := 0.0
		if tpl := proj.Template; tpl != nil {
			speed = tpl.Speed
		}
		motion.VelocityX = quantizeProjectileVelocity(proj.VelocityUnitX*speed, cfg.TickRate)
		motion.VelocityY = quantizeProjectileVelocity(proj.VelocityUnitY*speed, cfg.TickRate)
	}
	instance.DeliveryState.Motion = motion
	instance.DeliveryState.Geometry = geometry
}

func effectCenterX(effect *State) float64 {
	if effect == nil {
		return 0
	}
	return effect.X + effect.Width/2
}

func effectCenterY(effect *State) float64 {
	if effect == nil {
		return 0
	}
	return effect.Y + effect.Height/2
}

func quantizeProjectileVelocity(unitsPerSecond float64, tickRate int) int {
	if tickRate <= 0 {
		tickRate = defaultTickRate
	}
	perTick := unitsPerSecond / float64(tickRate)
	return quantizeCoord(perTick)
}
