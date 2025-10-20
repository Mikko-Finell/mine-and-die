package world

import (
	"hash/fnv"
	"math"
	"math/rand"
)

const centralSpawnRegionRatio = 0.5

func DeterministicSeedValue(rootSeed, label string) int64 {
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

func NewDeterministicRNG(rootSeed, label string) *rand.Rand {
	seedValue := DeterministicSeedValue(rootSeed, label)
	return rand.New(rand.NewSource(seedValue))
}

func RandomFloat(rng *rand.Rand) float64 {
	if rng == nil {
		return rand.New(rand.NewSource(DeterministicSeedValue(DefaultSeed, "world"))).Float64()
	}
	return rng.Float64()
}

func RandomAngle(rng *rand.Rand) float64 {
	return RandomFloat(rng) * 2 * math.Pi
}

func RandomDistance(rng *rand.Rand, min, max float64) float64 {
	if max <= min {
		return min
	}
	return min + RandomFloat(rng)*(max-min)
}

func CentralTopLeftRange(total, center, margin, size float64) (float64, float64) {
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

func CentralCenterRange(total, center, margin, padding float64) (float64, float64) {
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
