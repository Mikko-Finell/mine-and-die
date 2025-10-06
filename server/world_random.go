package main

import (
	"hash/fnv"
	"math"
	"math/rand"
)

func deterministicSeedValue(rootSeed, label string) int64 {
	hasher := fnv.New64a()
	hasher.Write([]byte(rootSeed))
	hasher.Write([]byte{0})
	hasher.Write([]byte(label))
	sum := hasher.Sum64()
	if sum == 0 {
		sum = 1
	}
	return int64(sum)
}

func newDeterministicRNG(rootSeed, label string) *rand.Rand {
	seedValue := deterministicSeedValue(rootSeed, label)
	return rand.New(rand.NewSource(seedValue))
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
	return newDeterministicRNG(defaultWorldSeed, "world").Float64()
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
