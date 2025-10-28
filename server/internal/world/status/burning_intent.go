package status

import (
	"math"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	worldeffects "mine-and-die/server/internal/world/effects"
)

// BurningTickIntentConfig bundles the inputs required to enqueue a contract
// intent that applies burning damage via the centralized world helpers.
type BurningTickIntentConfig struct {
	EffectType    string
	TargetActorID string
	SourceActorID string
	StatusEffect  StatusEffectType
	Delta         float64
	TileSize      float64
	Footprint     float64
	Now           time.Time
	CurrentTick   uint64
}

// NewBurningTickIntent normalizes the provided burning damage request using the
// shared world helper and returns a contract intent that matches the legacy
// lava damage queue behaviour.
func NewBurningTickIntent(cfg BurningTickIntentConfig) (effectcontract.EffectIntent, bool) {
	if cfg.TargetActorID == "" || cfg.EffectType == "" {
		return effectcontract.EffectIntent{}, false
	}

	var (
		intent effectcontract.EffectIntent
		ok     bool
	)

	ApplyBurningDamage(ApplyBurningDamageConfig{
		EffectType:   cfg.EffectType,
		OwnerID:      cfg.SourceActorID,
		ActorID:      cfg.TargetActorID,
		StatusEffect: string(cfg.StatusEffect),
		Delta:        cfg.Delta,
		Now:          cfg.Now,
		CurrentTick:  cfg.CurrentTick,
		Apply: func(effect BurningDamageEffect) {
			rounded := int(math.Round(effect.HealthDelta))
			if rounded == 0 {
				return
			}

			footprint := cfg.Footprint
			if footprint <= 0 {
				footprint = 1
			}

			geometry := effectcontract.EffectGeometry{
				Shape:   effectcontract.GeometryShapeRect,
				Width:   worldeffects.QuantizeWorldCoord(footprint, cfg.TileSize),
				Height:  worldeffects.QuantizeWorldCoord(footprint, cfg.TileSize),
				OffsetX: 0,
				OffsetY: 0,
			}

			intent = effectcontract.EffectIntent{
				EntryID:       effect.EffectType,
				TypeID:        effect.EffectType,
				Delivery:      effectcontract.DeliveryKindTarget,
				SourceActorID: effect.OwnerID,
				TargetActorID: cfg.TargetActorID,
				Geometry:      geometry,
				DurationTicks: 1,
				Params: map[string]int{
					"healthDelta": rounded,
				},
			}
			ok = true
		},
	})

	if !ok {
		return effectcontract.EffectIntent{}, false
	}

	return intent, true
}
