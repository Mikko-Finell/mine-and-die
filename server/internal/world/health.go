package world

import "math"

// HealthEpsilon defines the tolerance used when comparing health values.
const HealthEpsilon = 1e-6

// HealthState captures the current health values for an actor.
type HealthState struct {
	Health    float64
	MaxHealth float64
}

// SetActorHealth updates the actor's health and max-health values based on the
// provided computed maximum and requested health. It returns true when either
// value changes after normalization and clamping, signalling the caller to
// persist the mutation.
func SetActorHealth(state *HealthState, computedMax float64, health float64) bool {
	if state == nil {
		return false
	}

	if math.IsNaN(health) || math.IsInf(health, 0) {
		return false
	}

	max := computedMax
	if max <= 0 {
		max = state.MaxHealth
	}
	if max <= 0 {
		max = health
	}

	if health < 0 {
		health = 0
	}
	if max > 0 && health > max {
		health = max
	}

	maxDiff := math.Abs(state.MaxHealth - max)
	healthDiff := math.Abs(state.Health - health)
	if maxDiff < HealthEpsilon && healthDiff < HealthEpsilon {
		return false
	}

	state.Health = health
	state.MaxHealth = max
	return true
}
