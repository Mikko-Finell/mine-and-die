package server

import (
	"math"
	"math/rand"

	worldpkg "mine-and-die/server/internal/world"
)

func deterministicSeedValue(rootSeed, label string) int64 {
	return worldpkg.DeterministicSeedValue(rootSeed, label)
}

func newDeterministicRNG(rootSeed, label string) *rand.Rand {
	return worldpkg.NewDeterministicRNG(rootSeed, label)
}

func (w *World) ensureRNG() {
	if w == nil {
		return
	}
	if w.seed == "" {
		w.seed = defaultWorldSeed
	}
	if w.rng == nil {
		w.rng = newDeterministicRNG(w.seed, "world")
	}
}

func (w *World) subsystemRNG(label string) *rand.Rand {
	root := defaultWorldSeed
	if w != nil && w.seed != "" {
		root = w.seed
	}
	return newDeterministicRNG(root, label)
}

func (w *World) randomFloat() float64 {
	if w != nil {
		w.ensureRNG()
		return w.rng.Float64()
	}
	return worldpkg.RandomFloat(nil)
}

func (w *World) randomAngle() float64 {
	return w.randomFloat() * 2 * math.Pi
}

func (w *World) randomDistance(min, max float64) float64 {
	if max <= min {
		return min
	}
	return min + w.randomFloat()*(max-min)
}

func centralTopLeftRange(total, center, margin, size float64) (float64, float64) {
	return worldpkg.CentralTopLeftRange(total, center, margin, size)
}

func centralCenterRange(total, center, margin, padding float64) (float64, float64) {
	return worldpkg.CentralCenterRange(total, center, margin, padding)
}
