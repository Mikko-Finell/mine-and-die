package effects

import (
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

// StatusVisualTarget captures the minimal actor metadata required to
// instantiate a contract-managed status visual effect.
type StatusVisualTarget struct {
	ID string
	X  float64
	Y  float64
}

// StatusVisualSpawnConfig bundles the inputs required to construct a legacy
// status visual state for a contract-managed effect instance.
type StatusVisualSpawnConfig struct {
	Instance         *effectcontract.EffectInstance
	Target           *StatusVisualTarget
	Lifetime         time.Duration
	Now              time.Time
	DefaultFootprint float64
	FallbackLifetime time.Duration
	StatusEffect     StatusEffectType
}

// SpawnContractStatusVisualFromInstance materializes a legacy status visual
// effect from the provided contract instance. The helper mirrors the
// historical world behaviour so callers outside the legacy world wrapper can
// instantiate visuals while relying on the shared runtime types.
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
