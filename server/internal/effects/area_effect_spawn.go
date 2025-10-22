package effects

import (
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

// AreaEffectSpawnConfig captures the dependencies required to instantiate an
// explosion-style area effect that mirrors the legacy projectile helper.
type AreaEffectSpawnConfig struct {
	Source      *State
	Spec        *ExplosionSpec
	Now         time.Time
	CurrentTick effectcontract.Tick

	AllocateID  func() string
	Register    func(*State) bool
	RecordSpawn func(effectType, category string)
}

// SpawnAreaEffect materializes an area-of-effect explosion using the provided
// configuration, registering it via the supplied adapters. The helper returns
// the registered effect when successful so callers can observe the created
// state, mirroring the legacy world behaviour.
func SpawnAreaEffect(cfg AreaEffectSpawnConfig) *State {
	source := cfg.Source
	spec := cfg.Spec
	if source == nil || spec == nil {
		return nil
	}

	id := ""
	if cfg.AllocateID != nil {
		id = cfg.AllocateID()
	}
	if id == "" {
		return nil
	}

	radius := spec.Radius
	size := radius * 2
	if size <= 0 {
		size = source.Width
		if size <= 0 {
			size = 1
		}
	}

	params := MergeParams(spec.Params, map[string]float64{
		"radius": radius,
	})
	if spec.Duration > 0 {
		if params == nil {
			params = make(map[string]float64)
		}
		params["duration_ms"] = float64(spec.Duration.Milliseconds())
	}

	effect := &State{
		ID:       id,
		Type:     spec.EffectType,
		Owner:    source.Owner,
		Start:    cfg.Now.UnixMilli(),
		Duration: spec.Duration.Milliseconds(),
		X:        effectCenterX(source) - size/2,
		Y:        effectCenterY(source) - size/2,
		Width:    size,
		Height:   size,
		Params:   params,
		Instance: effectcontract.EffectInstance{
			ID:           id,
			DefinitionID: spec.EffectType,
			OwnerActorID: source.Owner,
			StartTick:    cfg.CurrentTick,
		},
		ExpiresAt:          cfg.Now.Add(spec.Duration),
		TelemetrySpawnTick: cfg.CurrentTick,
	}

	if cfg.Register != nil {
		if !cfg.Register(effect) {
			return nil
		}
	}

	if cfg.RecordSpawn != nil {
		cfg.RecordSpawn(spec.EffectType, "explosion")
	}

	return effect
}
