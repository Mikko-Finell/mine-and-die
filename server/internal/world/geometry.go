package world

import state "mine-and-die/server/internal/world/state"

// Vec2 aliases the shared state vector type for world helpers.
type Vec2 = state.Vec2

// Clamp limits value to the range [min, max].
func Clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// CircleRectOverlap reports whether a circle intersects an obstacle rectangle.
func CircleRectOverlap(cx, cy, radius float64, obs Obstacle) bool {
	closestX := Clamp(cx, obs.X, obs.X+obs.Width)
	closestY := Clamp(cy, obs.Y, obs.Y+obs.Height)
	dx := cx - closestX
	dy := cy - closestY
	return dx*dx+dy*dy < radius*radius
}

// ObstaclesOverlap checks for AABB overlap with optional padding.
func ObstaclesOverlap(a, b Obstacle, padding float64) bool {
	return a.X-padding < b.X+b.Width+padding &&
		a.X+a.Width+padding > b.X-padding &&
		a.Y-padding < b.Y+b.Height+padding &&
		a.Y+a.Height+padding > b.Y-padding
}
