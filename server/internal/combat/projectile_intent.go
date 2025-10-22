package combat

import (
	"math"

	effectcontract "mine-and-die/server/effects/contract"
)

// ProjectileIntentConfig carries the dependencies required to reproduce the
// legacy projectile intent construction outside of the server package.
type ProjectileIntentConfig struct {
	TileSize        float64
	DefaultFacing   string
	QuantizeCoord   func(float64) int
	FacingVector    func(string) (float64, float64)
	OwnerHalfExtent func(ProjectileIntentOwner) float64
}

// ProjectileIntentOwner captures the minimal owner metadata required to stage a
// projectile intent.
type ProjectileIntentOwner struct {
	ID     string
	X      float64
	Y      float64
	Facing string
}

// ProjectileIntentCollisionShape mirrors the legacy projectile collision shape
// configuration used when deriving spawn geometry.
type ProjectileIntentCollisionShape struct {
	UseRect bool
	Width   float64
	Height  float64
}

// ProjectileIntentTemplate captures the subset of projectile template metadata
// required to construct a contract intent.
type ProjectileIntentTemplate struct {
	Type           string
	Speed          float64
	MaxDistance    float64
	SpawnRadius    float64
	SpawnOffset    float64
	CollisionShape ProjectileIntentCollisionShape
	Params         map[string]float64
}

// NewProjectileIntent converts the provided template and owner into an
// EffectIntent that mirrors the legacy projectile spawn metadata.
func NewProjectileIntent(cfg ProjectileIntentConfig, owner ProjectileIntentOwner, tpl ProjectileIntentTemplate) (effectcontract.EffectIntent, bool) {
	if owner.ID == "" || tpl.Type == "" {
		return effectcontract.EffectIntent{}, false
	}
	if cfg.TileSize == 0 || cfg.QuantizeCoord == nil || cfg.FacingVector == nil {
		return effectcontract.EffectIntent{}, false
	}

	facing := owner.Facing
	if facing == "" {
		facing = cfg.DefaultFacing
	}

	dirX, dirY := cfg.FacingVector(facing)
	if dirX == 0 && dirY == 0 {
		dirX, dirY = 0, 1
	}

	spawnRadius := sanitizeSpawnRadius(tpl.SpawnRadius)
	spawnOffset := tpl.SpawnOffset
	if spawnOffset == 0 {
		half := 0.0
		if cfg.OwnerHalfExtent != nil {
			half = cfg.OwnerHalfExtent(owner)
		}
		spawnOffset = half + spawnRadius
	}

	centerX := owner.X + dirX*spawnOffset
	centerY := owner.Y + dirY*spawnOffset

	width, height := spawnSizeFromTemplate(tpl)

	quantizeWorld := func(value float64) int {
		return cfg.QuantizeCoord(value / cfg.TileSize)
	}

	geometry := effectcontract.EffectGeometry{
		Shape:   effectcontract.GeometryShapeCircle,
		OffsetX: quantizeWorld(centerX - owner.X),
		OffsetY: quantizeWorld(centerY - owner.Y),
	}

	if tpl.CollisionShape.UseRect {
		geometry.Shape = effectcontract.GeometryShapeRect
		geometry.Width = quantizeWorld(width)
		geometry.Height = quantizeWorld(height)
	} else {
		geometry.Radius = quantizeWorld(spawnRadius)
		if width > 0 {
			geometry.Width = quantizeWorld(width)
		}
		if height > 0 {
			geometry.Height = quantizeWorld(height)
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

	intent := effectcontract.EffectIntent{
		EntryID:       tpl.Type,
		TypeID:        tpl.Type,
		Delivery:      effectcontract.DeliveryKindArea,
		SourceActorID: owner.ID,
		Geometry:      geometry,
		Params:        params,
	}

	return intent, true
}

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

func sanitizeSpawnRadius(value float64) float64 {
	if value < 1 {
		return 1
	}
	return value
}

func spawnSizeFromTemplate(tpl ProjectileIntentTemplate) (float64, float64) {
	if tpl.CollisionShape.UseRect {
		spawnDiameter := sanitizeSpawnRadius(tpl.SpawnRadius) * 2
		width := math.Max(tpl.CollisionShape.Width, spawnDiameter)
		height := math.Max(tpl.CollisionShape.Height, spawnDiameter)
		width = math.Max(width, 1)
		height = math.Max(height, 1)
		return width, height
	}
	radius := sanitizeSpawnRadius(tpl.SpawnRadius)
	diameter := radius * 2
	return diameter, diameter
}
