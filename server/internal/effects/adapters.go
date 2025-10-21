package effects

import worldpkg "mine-and-die/server/internal/world"

// SetPosition updates the effect coordinates while delegating spatial index
// updates to the provided callback. The helper mirrors the legacy mutation path
// so world code can continue to operate on shared effect state.
func SetPosition(eff *State, x, y float64, updateIndex func(oldX, oldY float64) bool) bool {
	if eff == nil {
		return false
	}
	return worldpkg.SetEffectPosition(&eff.X, &eff.Y, x, y, updateIndex)
}

// SetParam updates or inserts the provided parameter value when it changes.
// The helper forwards to the shared world mutation logic so parameter updates
// remain consistent with legacy callers.
func SetParam(eff *State, key string, value float64) bool {
	if eff == nil {
		return false
	}
	return worldpkg.SetEffectParam(&eff.Params, key, value)
}
