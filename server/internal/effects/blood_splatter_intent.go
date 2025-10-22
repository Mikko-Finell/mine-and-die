package effects

import (
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

// BloodSplatterIntentConfig bundles the inputs required to mirror the legacy
// blood decal intent when queuing contract-managed visuals.
type BloodSplatterIntentConfig struct {
	SourceActorID string
	TargetActorID string
	Target        *ActorPosition
	TileSize      float64
	Footprint     float64
	Duration      time.Duration
	TickRate      int
}

// NewBloodSplatterIntent normalizes the provided request using the shared
// quantization helpers and returns a contract intent that matches the
// historical blood decal spawn metadata.
func NewBloodSplatterIntent(cfg BloodSplatterIntentConfig) (effectcontract.EffectIntent, bool) {
	if cfg.TargetActorID == "" || cfg.Target == nil {
		return effectcontract.EffectIntent{}, false
	}

	footprint := cfg.Footprint
	if footprint <= 0 {
		footprint = 1
	}

	geometry := effectcontract.EffectGeometry{
		Shape:  effectcontract.GeometryShapeRect,
		Width:  QuantizeWorldCoord(footprint, cfg.TileSize),
		Height: QuantizeWorldCoord(footprint, cfg.TileSize),
	}

	params := map[string]int{
		"centerX": QuantizeWorldCoord(cfg.Target.X, cfg.TileSize),
		"centerY": QuantizeWorldCoord(cfg.Target.Y, cfg.TileSize),
	}

	intent := effectcontract.EffectIntent{
		EntryID:       effectcontract.EffectIDBloodSplatter,
		TypeID:        effectcontract.EffectIDBloodSplatter,
		Delivery:      effectcontract.DeliveryKindVisual,
		SourceActorID: cfg.SourceActorID,
		Geometry:      geometry,
		DurationTicks: durationToTicks(cfg.Duration, cfg.TickRate),
		Params:        params,
	}

	if intent.DurationTicks < 1 {
		intent.DurationTicks = 1
	}

	return intent, true
}
