package effects

import (
	"math"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

// ProjectileOwner exposes the minimal state required to derive projectile
// spawn positions and fallback directions for contract-managed effects.
type ProjectileOwner interface {
	// Facing reports the owner's current facing direction. Callers should
	// return a sensible default when the underlying state is empty so the
	// helper can derive deterministic fallback motion.
	Facing() string
	// FacingVector returns the unit direction vector corresponding to the
	// owner's facing. When the facing cannot be resolved the method should
	// return (0, 0) so the helper can fall back to a canonical direction.
	FacingVector() (float64, float64)
	// Position reports the owner's current world-space coordinates.
	Position() (float64, float64)
}

// ProjectileSpawnConfig bundles the inputs required to construct a legacy
// projectile state for a contract-managed effect instance.
type ProjectileSpawnConfig struct {
	Instance *effectcontract.EffectInstance
	Owner    ProjectileOwner
	Template *ProjectileTemplate
	Now      time.Time
	TileSize float64
	TickRate int
}

// SpawnContractProjectileFromInstance materializes a legacy projectile effect
// from the provided contract instance. The helper mirrors the historical world
// behavior so callers outside the legacy world wrapper can instantiate
// projectiles while relying on the shared runtime types.
func SpawnContractProjectileFromInstance(cfg ProjectileSpawnConfig) *State {
	instance := cfg.Instance
	owner := cfg.Owner
	tpl := cfg.Template
	if instance == nil || owner == nil || tpl == nil {
		return nil
	}

	params := MergeParams(tpl.Params, IntMapToFloat64(instance.BehaviorState.Extra))
	if len(instance.Params) > 0 {
		params = MergeParams(params, IntMapToFloat64(instance.Params))
	}
	if params == nil {
		params = make(map[string]float64)
	}

	dirX := params["dx"]
	dirY := params["dy"]
	if raw, ok := instance.BehaviorState.Extra["dx"]; ok {
		dirX = float64(raw)
		if math.Abs(dirX) > 1 {
			dirX = dequantizeCoord(raw)
		}
		params["dx"] = dirX
	} else if raw, ok := instance.Params["dx"]; ok {
		dirX = float64(raw)
		if math.Abs(dirX) > 1 {
			dirX = dequantizeCoord(raw)
		}
		params["dx"] = dirX
	}
	if raw, ok := instance.BehaviorState.Extra["dy"]; ok {
		dirY = float64(raw)
		if math.Abs(dirY) > 1 {
			dirY = dequantizeCoord(raw)
		}
		params["dy"] = dirY
	} else if raw, ok := instance.Params["dy"]; ok {
		dirY = float64(raw)
		if math.Abs(dirY) > 1 {
			dirY = dequantizeCoord(raw)
		}
		params["dy"] = dirY
	}
	if dirX == 0 && dirY == 0 {
		dirX, dirY = owner.FacingVector()
		if dirX == 0 && dirY == 0 {
			dirX, dirY = 0, 1
		}
	}

	geometry := instance.DeliveryState.Geometry
	tileSize := cfg.TileSize
	if tileSize <= 0 {
		tileSize = 40
	}
	ownerX, ownerY := owner.Position()
	offsetX := DequantizeWorldCoord(geometry.OffsetX, tileSize)
	offsetY := DequantizeWorldCoord(geometry.OffsetY, tileSize)
	centerX := ownerX + offsetX
	centerY := ownerY + offsetY

	width, height := SpawnSizeFromShape(tpl)
	if geometry.Width != 0 {
		width = DequantizeWorldCoord(geometry.Width, tileSize)
	}
	if geometry.Height != 0 {
		height = DequantizeWorldCoord(geometry.Height, tileSize)
	}

	radius := SanitizedSpawnRadius(tpl.SpawnRadius)
	if geometry.Radius != 0 {
		radius = DequantizeWorldCoord(geometry.Radius, tileSize)
	} else if val, ok := params["radius"]; ok && val > 0 {
		radius = val
	}

	lifetime := EffectLifetime(tpl)
	if ticks := instance.BehaviorState.TicksRemaining; ticks > 0 {
		if persisted := TicksToDuration(ticks, cfg.TickRate); persisted > 0 {
			lifetime = persisted
		}
	}

	params = MergeParams(params, map[string]float64{
		"speed":  tpl.Speed,
		"radius": radius,
		"dx":     dirX,
		"dy":     dirY,
	})
	if _, ok := params["range"]; !ok && tpl.MaxDistance > 0 {
		params["range"] = tpl.MaxDistance
	}

	remainingRange := tpl.MaxDistance
	if val, ok := params["remainingRange"]; ok {
		remainingRange = val
	} else if raw, ok := instance.BehaviorState.Extra["remainingRange"]; ok {
		remainingRange = float64(raw)
		if remainingRange < 0 {
			remainingRange = 0
		}
		params["remainingRange"] = remainingRange
	}
	if remainingRange < 0 {
		remainingRange = 0
	}

	effect := &State{
		ID:        instance.ID,
		Type:      tpl.Type,
		Owner:     instance.OwnerActorID,
		Start:     cfg.Now.UnixMilli(),
		Duration:  lifetime.Milliseconds(),
		X:         centerX - width/2,
		Y:         centerY - height/2,
		Width:     width,
		Height:    height,
		Params:    params,
		Instance:  *instance,
		ExpiresAt: cfg.Now.Add(lifetime),
		Projectile: &ProjectileState{
			Template:       tpl,
			VelocityUnitX:  dirX,
			VelocityUnitY:  dirY,
			RemainingRange: remainingRange,
		},
		ContractManaged:    true,
		TelemetrySpawnTick: instance.StartTick,
	}
	return effect
}

// MergeParams returns a new map that contains the provided base parameters
// overridden by the supplied overrides.
func MergeParams(base, overrides map[string]float64) map[string]float64 {
	if len(base) == 0 && len(overrides) == 0 {
		return nil
	}
	merged := make(map[string]float64)
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range overrides {
		merged[k] = v
	}
	return merged
}

// IntMapToFloat64 converts an int-valued map to a float64-valued map.
func IntMapToFloat64(src map[string]int) map[string]float64 {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]float64, len(src))
	for k, v := range src {
		dst[k] = float64(v)
	}
	return dst
}

// SanitizedSpawnRadius ensures projectile spawn radii remain positive.
func SanitizedSpawnRadius(value float64) float64 {
	if value < 1 {
		return 1
	}
	return value
}

// SpawnSizeFromShape derives the initial projectile footprint from its template
// configuration.
func SpawnSizeFromShape(tpl *ProjectileTemplate) (float64, float64) {
	if tpl == nil {
		return 0, 0
	}
	if tpl.CollisionShape.UseRect {
		spawnDiameter := SanitizedSpawnRadius(tpl.SpawnRadius) * 2
		width := math.Max(tpl.CollisionShape.RectWidth, spawnDiameter)
		height := math.Max(tpl.CollisionShape.RectHeight, spawnDiameter)
		width = math.Max(width, 1)
		height = math.Max(height, 1)
		return width, height
	}
	radius := SanitizedSpawnRadius(tpl.SpawnRadius)
	diameter := radius * 2
	return diameter, diameter
}

// EffectLifetime returns the configured projectile lifetime or derives one from
// its speed and maximum distance.
func EffectLifetime(tpl *ProjectileTemplate) time.Duration {
	if tpl == nil {
		return 0
	}
	if tpl.Lifetime > 0 {
		return tpl.Lifetime
	}
	if tpl.Speed <= 0 || tpl.MaxDistance <= 0 {
		return 0
	}
	seconds := tpl.MaxDistance / tpl.Speed
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

// TicksToDuration converts simulation ticks into a wall-clock duration using
// the provided tick rate. The helper returns zero when ticks or tick rate are
// non-positive.
func TicksToDuration(ticks int, tickRate int) time.Duration {
	if ticks <= 0 || tickRate <= 0 {
		return 0
	}
	seconds := float64(ticks) / float64(tickRate)
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds * float64(time.Second))
}

// DequantizeWorldCoord converts quantized geometry coordinates back into world
// units using the provided tile size.
func DequantizeWorldCoord(value int, tileSize float64) float64 {
	if tileSize <= 0 {
		tileSize = 40
	}
	return dequantizeCoord(value) * tileSize
}

func dequantizeCoord(value int) float64 {
	return float64(value) / effectcontract.CoordScale
}
