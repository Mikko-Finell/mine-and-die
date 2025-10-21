package world

import "math"

// EffectParamEpsilon defines the tolerance used when comparing effect parameter values.
const EffectParamEpsilon = 1e-6

// SetEffectPosition updates the effect coordinates when they change and invokes the
// provided callback to keep any spatial index in sync. The callback receives the
// previous coordinates and should return true when the index accepts the updated
// position. When the callback reports failure the coordinates are restored and
// the mutation is discarded.
func SetEffectPosition(x, y *float64, newX, newY float64, updateIndex func(oldX, oldY float64) bool) bool {
	if x == nil || y == nil {
		return false
	}

	oldX := *x
	oldY := *y
	if PositionsEqual(oldX, oldY, newX, newY) {
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

// SetEffectParam updates or inserts the provided parameter value when it changes
// beyond the configured epsilon. The map pointer is updated when a new map needs
// to be allocated so callers observe the mutation through their struct fields.
func SetEffectParam(params *map[string]float64, key string, value float64) bool {
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
