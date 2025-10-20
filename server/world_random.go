package server

import (
	"hash/fnv"
	"math"
	"math/rand"
)

const centralSpawnRegionRatio = 0.5

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

func centralTopLeftRange(total, center, margin, size float64) (float64, float64) {
	if total <= 0 {
		return margin, margin
	}

	regionHalf := total * centralSpawnRegionRatio / 2
	min := center - regionHalf
	max := center + regionHalf - size

	if min < margin {
		min = margin
	}
	maxLimit := total - margin - size
	if max > maxLimit {
		max = maxLimit
	}
	if max < min {
		max = min
	}

	return min, max
}

func centralCenterRange(total, center, margin, padding float64) (float64, float64) {
	if total <= 0 {
		return margin, margin
	}

	regionHalf := total * centralSpawnRegionRatio / 2
	min := center - regionHalf
	max := center + regionHalf

	minLimit := margin + padding
	if min < minLimit {
		min = minLimit
	}
	maxLimit := total - margin - padding
	if max > maxLimit {
		max = maxLimit
	}
	if max < min {
		max = min
	}

	return min, max
}
