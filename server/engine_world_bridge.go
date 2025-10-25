package server

import (
	"math/rand"

	"mine-and-die/server/internal/sim"
)

// EngineAdapter constructs the legacy engine adapter for the provided dependencies.
func (w *World) EngineAdapter(deps sim.Deps) sim.EngineCore {
	if w == nil {
		return nil
	}
	return newLegacyEngineAdapter(w, deps)
}

// EngineRNG exposes the RNG seeded for the legacy world so the engine can share it.
func (w *World) EngineRNG() *rand.Rand {
	if w == nil {
		return nil
	}
	return w.rng
}
