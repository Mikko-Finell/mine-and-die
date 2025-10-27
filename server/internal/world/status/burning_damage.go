package status

import (
	"math"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

// BurningDamageEffect captures the parameters required to apply burning damage
// during a status effect tick without exposing the legacy effect state struct.
type BurningDamageEffect struct {
	EffectType   string
	OwnerID      string
	StatusEffect string
	HealthDelta  float64
	StartMillis  int64
	SpawnTick    effectcontract.Tick
}

// ApplyBurningDamageConfig bundles the inputs required to apply lava damage
// through the centralized world helpers while keeping the legacy world wrapper
// thin.
type ApplyBurningDamageConfig struct {
	EffectType   string
	OwnerID      string
	ActorID      string
	StatusEffect string
	Delta        float64
	Now          time.Time
	CurrentTick  uint64
	Apply        func(BurningDamageEffect)
}

// ApplyBurningDamage normalizes the provided lava damage request and invokes the
// supplied callback with the effect configuration when the delta represents a
// damage tick. The helper mirrors the legacy behavior by falling back to the
// target actor as the owner when the request omits an explicit source.
func ApplyBurningDamage(cfg ApplyBurningDamageConfig) {
	if cfg.Apply == nil {
		return
	}
	if cfg.Delta >= 0 || math.IsNaN(cfg.Delta) || math.IsInf(cfg.Delta, 0) {
		return
	}

	owner := cfg.OwnerID
	if owner == "" {
		owner = cfg.ActorID
	}

	cfg.Apply(BurningDamageEffect{
		EffectType:   cfg.EffectType,
		OwnerID:      owner,
		StatusEffect: cfg.StatusEffect,
		HealthDelta:  cfg.Delta,
		StartMillis:  cfg.Now.UnixMilli(),
		SpawnTick:    effectcontract.Tick(int64(cfg.CurrentTick)),
	})
}
