package effects

import (
	"math"

	runtime "mine-and-die/server/internal/effects/runtime"
)

// EffectParamEpsilon defines the tolerance used when comparing effect parameter values.
const EffectParamEpsilon = 1e-6

const positionEpsilon = 1e-6

// SetPosition updates the effect coordinates stored on the provided runtime state
// while delegating spatial index updates to the supplied callback. The helper
// mirrors the legacy mutation path so callers can keep their state containers in
// sync with the centralized world bookkeeping.
func SetPosition(state *runtime.State, newX, newY float64, updateIndex func(oldX, oldY float64) bool) bool {
	if state == nil {
		return false
	}
	return updateCoordinates(&state.X, &state.Y, newX, newY, updateIndex)
}

// SetParam updates or inserts the provided parameter value when it changes
// beyond the configured epsilon. Callers receive a boolean indicating whether
// the mutation was applied so they can trigger version bumps and patch
// emission.
func SetParam(state *runtime.State, key string, value float64) bool {
	if state == nil {
		return false
	}
	return updateParams(&state.Params, key, value)
}

func updateCoordinates(x, y *float64, newX, newY float64, updateIndex func(oldX, oldY float64) bool) bool {
	if x == nil || y == nil {
		return false
	}

	oldX := *x
	oldY := *y
	if positionsEqual(oldX, oldY, newX, newY) {
		return false
	}

	*x = newX
	*y = newY

	if updateIndex != nil {
		if !updateIndex(oldX, oldY) {
			*x = oldX
			*y = oldY
			return false
		}
	}

	return true
}

func updateParams(params *map[string]float64, key string, value float64) bool {
	if params == nil || key == "" {
		return false
	}

	state := *params
	if state == nil {
		state = make(map[string]float64)
	}

	if current, exists := state[key]; exists {
		if math.Abs(current-value) < EffectParamEpsilon {
			if *params == nil {
				*params = state
			}
			return false
		}
	}

	state[key] = value
	*params = state
	return true
}

func positionsEqual(ax, ay, bx, by float64) bool {
	return math.Abs(ax-bx) <= positionEpsilon && math.Abs(ay-by) <= positionEpsilon
}
