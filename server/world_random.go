package main

import (
	"math"
	"math/rand"
)

func (w *World) randomFloat() float64 {
	if w != nil && w.rng != nil {
		return w.rng.Float64()
	}
	return rand.Float64()
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
