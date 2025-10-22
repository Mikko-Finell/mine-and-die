package world

import (
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

// ProjectileStopAdapterConfig captures the callbacks required to build projectile
// stop bindings without importing the legacy server types. Callers provide
// allocation, registration, telemetry, and bookkeeping hooks so the adapter can
// recreate the legacy wiring from inside the world package.
type ProjectileStopAdapterConfig struct {
	AllocateID        func() string
	RegisterEffect    func(any) bool
	RecordEffectSpawn func(effectType, category string)
	CurrentTick       func() effectcontract.Tick

	SetRemainingRange func(any, float64)
	RecordEffectEnd   func(any, string)
}

// ProjectileStopAdapter binds projectile stop callbacks for reuse across
// stopping and advancing flows.
type ProjectileStopAdapter struct {
	allocateID        func() string
	registerEffect    func(any) bool
	recordEffectSpawn func(effectType, category string)
	currentTick       func() effectcontract.Tick

	setRemainingRange func(any, float64)
	recordEffectEnd   func(any, string)
}

// AreaEffectSpawnConfig carries the callbacks and metadata required to spawn an
// area-of-effect explosion without referencing internal/effects types.
type AreaEffectSpawnConfig struct {
	Source any
	Now    time.Time

	CurrentTick effectcontract.Tick

	AllocateID  func() string
	Register    func(any) bool
	RecordSpawn func(effectType, category string)
}

// ProjectileStopConfig exposes the stop-time bindings produced by the adapter.
// Callers use the returned functions to keep projectile state and telemetry in
// sync with the legacy world behaviour while delegating explosion spawning
// through the shared effects helper.
type ProjectileStopConfig struct {
	Effect any
	Now    time.Time

	AreaEffectSpawn AreaEffectSpawnConfig

	SetRemainingRange func(float64)
	RecordEffectEnd   func(string)
}

// ProjectileStopOptions capture the stop triggers requested by the caller.
type ProjectileStopOptions struct {
	TriggerImpact bool
	TriggerExpiry bool
}

// ProjectileStopper applies the provided stop configuration with the supplied
// options.
type ProjectileStopper func(ProjectileStopConfig, ProjectileStopOptions)

// NewProjectileStopAdapter constructs a projectile stop adapter using the
// provided configuration.
func NewProjectileStopAdapter(cfg ProjectileStopAdapterConfig) ProjectileStopAdapter {
	return ProjectileStopAdapter{
		allocateID:        cfg.AllocateID,
		registerEffect:    cfg.RegisterEffect,
		recordEffectSpawn: cfg.RecordEffectSpawn,
		currentTick:       cfg.CurrentTick,
		setRemainingRange: cfg.SetRemainingRange,
		recordEffectEnd:   cfg.RecordEffectEnd,
	}
}

// StopConfig materialises the projectile stop bindings for the provided effect
// and timestamp.
func (a ProjectileStopAdapter) StopConfig(effect any, now time.Time) ProjectileStopConfig {
	spawnCfg := AreaEffectSpawnConfig{
		Source:      effect,
		Now:         now,
		AllocateID:  a.allocateID,
		Register:    a.registerEffect,
		RecordSpawn: a.recordEffectSpawn,
	}
	if a.currentTick != nil {
		spawnCfg.CurrentTick = a.currentTick()
	}

	cfg := ProjectileStopConfig{
		Effect:          effect,
		Now:             now,
		AreaEffectSpawn: spawnCfg,
	}

	if a.setRemainingRange != nil {
		cfg.SetRemainingRange = func(remaining float64) {
			a.setRemainingRange(effect, remaining)
		}
	}
	if a.recordEffectEnd != nil {
		cfg.RecordEffectEnd = func(reason string) {
			a.recordEffectEnd(effect, reason)
		}
	}

	return cfg
}
