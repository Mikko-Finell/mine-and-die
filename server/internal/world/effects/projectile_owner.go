package effects

import state "mine-and-die/server/internal/world/state"

// ProjectileOwnerSnapshot exposes the sanitized projectile owner metadata used
// when spawning or synchronising contract-managed projectiles. The struct
// implements the internal projectile owner interface so callers can construct it
// without importing the legacy server package.
type ProjectileOwnerSnapshot struct {
	X           float64
	Y           float64
	FacingValue string
}

// Facing reports the owner's facing direction, falling back to the default when
// unspecified to preserve legacy behaviour.
func (o ProjectileOwnerSnapshot) Facing() string {
	if o.FacingValue == "" {
		return string(state.DefaultFacing)
	}
	return o.FacingValue
}

// FacingVector returns the unit vector corresponding to the owner's facing.
func (o ProjectileOwnerSnapshot) FacingVector() (float64, float64) {
	return state.FacingToVector(state.FacingDirection(o.Facing()))
}

// Position reports the owner's current coordinates.
func (o ProjectileOwnerSnapshot) Position() (float64, float64) {
	return o.X, o.Y
}
