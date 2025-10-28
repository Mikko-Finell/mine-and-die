package effects

import (
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

type StatusVisualTarget struct {
	ID string
	X  float64
	Y  float64
}

type StatusVisualSpawnConfig struct {
	Instance         *effectcontract.EffectInstance
	Target           *StatusVisualTarget
	Lifetime         time.Duration
	Now              time.Time
	DefaultFootprint float64
	FallbackLifetime time.Duration
	StatusEffect     StatusEffectType
}

func SpawnContractStatusVisualFromInstance(cfg StatusVisualSpawnConfig) *State {
	instance := cfg.Instance
	target := cfg.Target
	if instance == nil || target == nil {
		return nil
	}

	lifetime := cfg.Lifetime
	if lifetime <= 0 {
		lifetime = cfg.FallbackLifetime
	}
	if lifetime <= 0 {
		lifetime = time.Millisecond
	}

	footprint := cfg.DefaultFootprint
	if footprint <= 0 {
		footprint = 1
	}

	statusEffect := cfg.StatusEffect
	if statusEffect == "" {
		statusEffect = StatusEffectType("burning")
	}

	effect := &State{
		ID:                 instance.ID,
		Type:               instance.DefinitionID,
		Owner:              target.ID,
		Start:              cfg.Now.UnixMilli(),
		Duration:           lifetime.Milliseconds(),
		X:                  target.X - footprint/2,
		Y:                  target.Y - footprint/2,
		Width:              footprint,
		Height:             footprint,
		Instance:           *instance,
		ExpiresAt:          cfg.Now.Add(lifetime),
		FollowActorID:      target.ID,
		StatusEffect:       statusEffect,
		ContractManaged:    true,
		TelemetrySpawnTick: instance.StartTick,
	}
	return effect
}
