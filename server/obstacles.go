package server

import (
	"math/rand"

	worldpkg "mine-and-die/server/internal/world"
)

type Obstacle = worldpkg.Obstacle

const (
	obstacleTypeGoldOre = worldpkg.ObstacleTypeGoldOre
	obstacleTypeLava    = worldpkg.ObstacleTypeLava
)

// generateObstacles scatters blocking rectangles and ore deposits around the map.
func (w *World) generateObstacles(count int) []Obstacle {
	return worldpkg.GenerateObstacles(worldObstacleGenerator{world: w}, count)
}

// circleRectOverlap reports whether a circle intersects an obstacle rectangle.
func circleRectOverlap(cx, cy, radius float64, obs Obstacle) bool {
	return worldpkg.CircleRectOverlap(cx, cy, radius, obs)
}

// obstaclesOverlap checks for AABB overlap with optional padding.
func obstaclesOverlap(a, b Obstacle, padding float64) bool {
	return worldpkg.ObstaclesOverlap(a, b, padding)
}

// clamp limits value to the range [min, max].
func clamp(value, min, max float64) float64 {
	return worldpkg.Clamp(value, min, max)
}

type worldObstacleGenerator struct {
	world *World
}

func (g worldObstacleGenerator) Config() worldpkg.Config {
	if g.world == nil {
		return worldpkg.DefaultConfig()
	}
	return g.world.config
}

func (g worldObstacleGenerator) Dimensions() (float64, float64) {
	if g.world == nil {
		return worldpkg.DefaultWidth, worldpkg.DefaultHeight
	}
	return g.world.dimensions()
}

func (g worldObstacleGenerator) SubsystemRNG(label string) *rand.Rand {
	if g.world == nil {
		return newDeterministicRNG(worldpkg.DefaultSeed, label)
	}
	return g.world.subsystemRNG(label)
}
