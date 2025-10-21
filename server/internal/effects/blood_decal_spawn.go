package effects

import (
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

// BloodDecalSpawnConfig carries the inputs required to construct a legacy
// blood decal state for a contract-managed effect instance.
type BloodDecalSpawnConfig struct {
	Instance        *effectcontract.EffectInstance
	Now             time.Time
	TileSize        float64
	TickRate        int
	DefaultSize     float64
	DefaultDuration time.Duration
	Params          map[string]float64
	Colors          []string
}

// BloodDecalSyncConfig carries the inputs required to synchronize a
// contract-managed blood decal instance with its legacy effect state.
type BloodDecalSyncConfig struct {
	Instance    *effectcontract.EffectInstance
	Effect      *State
	TileSize    float64
	DefaultSize float64
	Colors      []string
}

// SpawnContractBloodDecalFromInstance materializes a legacy blood decal effect
// from the provided contract instance. The helper mirrors the historical world
// behaviour so callers outside the legacy world wrapper can instantiate blood
// decals while relying on the shared runtime types.
func SpawnContractBloodDecalFromInstance(cfg BloodDecalSpawnConfig) *State {
	instance := cfg.Instance
	if instance == nil {
		return nil
	}
	params := instance.BehaviorState.Extra
	if len(params) == 0 {
		return nil
	}
	centerXVal, okX := params["centerX"]
	centerYVal, okY := params["centerY"]
	if !okX || !okY {
		return nil
	}
	tileSize := cfg.TileSize
	if tileSize <= 0 {
		tileSize = 40
	}
	centerX := DequantizeWorldCoord(centerXVal, tileSize)
	centerY := DequantizeWorldCoord(centerYVal, tileSize)
	width := DequantizeWorldCoord(instance.DeliveryState.Geometry.Width, tileSize)
	if width <= 0 {
		width = cfg.DefaultSize
	}
	height := DequantizeWorldCoord(instance.DeliveryState.Geometry.Height, tileSize)
	if height <= 0 {
		height = cfg.DefaultSize
	}
	lifetime := TicksToDuration(instance.BehaviorState.TicksRemaining, cfg.TickRate)
	if lifetime <= 0 {
		lifetime = cfg.DefaultDuration
	}
	if lifetime <= 0 {
		lifetime = time.Millisecond
	}
	effectType := instance.DefinitionID
	if effectType == "" {
		effectType = effectcontract.EffectIDBloodSplatter
	}
	paramsCopy := cloneFloatMap(cfg.Params)
	colorsCopy := cloneStringSlice(cfg.Colors)
	effect := &State{
		ID:                 instance.ID,
		Type:               effectType,
		Owner:              instance.OwnerActorID,
		Start:              cfg.Now.UnixMilli(),
		Duration:           lifetime.Milliseconds(),
		X:                  centerX - width/2,
		Y:                  centerY - height/2,
		Width:              width,
		Height:             height,
		Params:             paramsCopy,
		Colors:             colorsCopy,
		Instance:           *instance,
		ExpiresAt:          cfg.Now.Add(lifetime),
		ContractManaged:    true,
		TelemetrySpawnTick: instance.StartTick,
	}
	return effect
}

// SyncContractBloodDecalInstance mirrors the historical legacy sync helper so
// contract-managed instances can update delivery geometry and metadata without
// relying on the world wrapper.
func SyncContractBloodDecalInstance(cfg BloodDecalSyncConfig) {
	instance := cfg.Instance
	effect := cfg.Effect
	if instance == nil || effect == nil {
		return
	}

	width := effect.Width
	if width <= 0 {
		width = cfg.DefaultSize
	}
	height := effect.Height
	if height <= 0 {
		height = cfg.DefaultSize
	}

	geometry := instance.DeliveryState.Geometry
	geometry.Width = QuantizeWorldCoord(width, cfg.TileSize)
	geometry.Height = QuantizeWorldCoord(height, cfg.TileSize)
	instance.DeliveryState.Geometry = geometry

	if instance.BehaviorState.Extra == nil {
		instance.BehaviorState.Extra = make(map[string]int)
	}
	if instance.Params == nil {
		instance.Params = make(map[string]int)
	}

	centerX := QuantizeWorldCoord(effect.X+effect.Width/2, cfg.TileSize)
	centerY := QuantizeWorldCoord(effect.Y+effect.Height/2, cfg.TileSize)
	instance.BehaviorState.Extra["centerX"] = centerX
	instance.BehaviorState.Extra["centerY"] = centerY
	instance.Params["centerX"] = centerX
	instance.Params["centerY"] = centerY

	instance.Colors = cloneStringSlice(cfg.Colors)
}

func cloneFloatMap(src map[string]float64) map[string]float64 {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]float64, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneStringSlice(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}
