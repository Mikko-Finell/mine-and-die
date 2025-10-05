package main

import (
	"fmt"
	"math/rand"
	"time"
)

const (
	obstacleTypeGoldOre = "gold-ore"
	obstacleTypeLava    = "lava"
)

type Obstacle struct {
	ID     string  `json:"id"`
	Type   string  `json:"type,omitempty"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// generateObstacles scatters blocking rectangles and ore deposits around the map.
func (h *Hub) generateObstacles(count int) []Obstacle {
	if count <= 0 {
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		obstacles := h.generateGoldOreNodes(goldOreCount, nil, rng)
		lavaPools := h.generateLavaPools(obstacles)
		return append(obstacles, lavaPools...)
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	obstacles := make([]Obstacle, 0, count)
	attempts := 0
	maxAttempts := count * 20

	for len(obstacles) < count && attempts < maxAttempts {
		attempts++

		width := obstacleMinWidth + rng.Float64()*(obstacleMaxWidth-obstacleMinWidth)
		height := obstacleMinHeight + rng.Float64()*(obstacleMaxHeight-obstacleMinHeight)

		maxX := worldWidth - obstacleSpawnMargin - width
		maxY := worldHeight - obstacleSpawnMargin - height
		if maxX <= obstacleSpawnMargin || maxY <= obstacleSpawnMargin {
			break
		}

		x := obstacleSpawnMargin + rng.Float64()*(maxX-obstacleSpawnMargin)
		y := obstacleSpawnMargin + rng.Float64()*(maxY-obstacleSpawnMargin)

		candidate := Obstacle{
			ID:     fmt.Sprintf("obstacle-%d", len(obstacles)+1),
			X:      x,
			Y:      y,
			Width:  width,
			Height: height,
		}

		if circleRectOverlap(80, 80, playerSpawnSafeRadius, candidate) {
			continue
		}

		overlapsExisting := false
		for _, obs := range obstacles {
			if obstaclesOverlap(candidate, obs, playerHalf) {
				overlapsExisting = true
				break
			}
		}

		if overlapsExisting {
			continue
		}

		obstacles = append(obstacles, candidate)
	}

	goldOre := h.generateGoldOreNodes(goldOreCount, obstacles, rng)
	obstacles = append(obstacles, goldOre...)

	lavaPools := h.generateLavaPools(obstacles)
	return append(obstacles, lavaPools...)
}

// generateGoldOreNodes places ore obstacles while avoiding overlaps.
func (h *Hub) generateGoldOreNodes(count int, existing []Obstacle, rng *rand.Rand) []Obstacle {
	if count <= 0 || rng == nil {
		return nil
	}

	ores := make([]Obstacle, 0, count)
	attempts := 0
	maxAttempts := count * 30

	for len(ores) < count && attempts < maxAttempts {
		attempts++

		width := goldOreMinSize + rng.Float64()*(goldOreMaxSize-goldOreMinSize)
		height := goldOreMinSize + rng.Float64()*(goldOreMaxSize-goldOreMinSize)

		maxX := worldWidth - obstacleSpawnMargin - width
		maxY := worldHeight - obstacleSpawnMargin - height
		if maxX <= obstacleSpawnMargin || maxY <= obstacleSpawnMargin {
			break
		}

		x := obstacleSpawnMargin + rng.Float64()*(maxX-obstacleSpawnMargin)
		y := obstacleSpawnMargin + rng.Float64()*(maxY-obstacleSpawnMargin)

		candidate := Obstacle{
			ID:     fmt.Sprintf("gold-ore-%d", len(ores)+1),
			Type:   obstacleTypeGoldOre,
			X:      x,
			Y:      y,
			Width:  width,
			Height: height,
		}

		if circleRectOverlap(80, 80, playerSpawnSafeRadius, candidate) {
			continue
		}

		overlaps := false

		for _, obs := range existing {
			if obstaclesOverlap(candidate, obs, playerHalf) {
				overlaps = true
				break
			}
		}

		if overlaps {
			continue
		}

		for _, ore := range ores {
			if obstaclesOverlap(candidate, ore, playerHalf) {
				overlaps = true
				break
			}
		}

		if overlaps {
			continue
		}

		ores = append(ores, candidate)
	}

	return ores
}

// generateLavaPools inserts deterministic lava hazards that remain walkable but harmful.
func (h *Hub) generateLavaPools(existing []Obstacle) []Obstacle {
	templates := []Obstacle{
		{ID: "lava-1", Type: obstacleTypeLava, X: 320, Y: 120, Width: 80, Height: 80},
		{ID: "lava-2", Type: obstacleTypeLava, X: 520, Y: 260, Width: 80, Height: 80},
		{ID: "lava-3", Type: obstacleTypeLava, X: 200, Y: 360, Width: 80, Height: 80},
	}

	pools := make([]Obstacle, 0, len(templates))
	for _, tpl := range templates {
		overlaps := false
		for _, obs := range existing {
			if obstaclesOverlap(tpl, obs, 0) {
				overlaps = true
				break
			}
		}
		if overlaps {
			continue
		}
		pools = append(pools, tpl)
	}
	return pools
}

// circleRectOverlap reports whether a circle intersects an obstacle rectangle.
func circleRectOverlap(cx, cy, radius float64, obs Obstacle) bool {
	closestX := clamp(cx, obs.X, obs.X+obs.Width)
	closestY := clamp(cy, obs.Y, obs.Y+obs.Height)
	dx := cx - closestX
	dy := cy - closestY
	return dx*dx+dy*dy < radius*radius
}

// obstaclesOverlap checks for AABB overlap with optional padding.
func obstaclesOverlap(a, b Obstacle, padding float64) bool {
	return a.X-padding < b.X+b.Width+padding &&
		a.X+a.Width+padding > b.X-padding &&
		a.Y-padding < b.Y+b.Height+padding &&
		a.Y+a.Height+padding > b.Y-padding
}

// clamp limits value to the range [min, max].
func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
