package world

import (
	"fmt"
	"math/rand"
)

// Obstacle mirrors the legacy obstacle snapshot exposed to callers.
type Obstacle struct {
	ID     string  `json:"id"`
	Type   string  `json:"type,omitempty"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// ObstacleGenerator describes the minimal world surface required to reproduce
// legacy obstacle layouts.
type ObstacleGenerator interface {
	Config() Config
	Dimensions() (float64, float64)
	SubsystemRNG(label string) *rand.Rand
}

// GenerateObstacles scatters blocking rectangles and ore deposits around the map.
func GenerateObstacles(gen ObstacleGenerator, count int) []Obstacle {
	if gen == nil {
		return nil
	}

	baseCount := count
	if baseCount < 0 {
		baseCount = 0
	}

	cfg := gen.Config()
	worldW, worldH := gen.Dimensions()

	obstacles := make([]Obstacle, 0, baseCount)

	if cfg.Obstacles && baseCount > 0 {
		rng := gen.SubsystemRNG("obstacles.base")
		attempts := 0
		maxAttempts := baseCount * 20

		for len(obstacles) < baseCount && attempts < maxAttempts {
			attempts++

			width := ObstacleMinWidth + rng.Float64()*(ObstacleMaxWidth-ObstacleMinWidth)
			height := ObstacleMinHeight + rng.Float64()*(ObstacleMaxHeight-ObstacleMinHeight)

			globalMinX := ObstacleSpawnMargin
			globalMaxX := worldW - ObstacleSpawnMargin - width
			globalMinY := ObstacleSpawnMargin
			globalMaxY := worldH - ObstacleSpawnMargin - height
			if globalMaxX <= globalMinX || globalMaxY <= globalMinY {
				break
			}

			minX, maxX := CentralTopLeftRange(worldW, DefaultSpawnX, ObstacleSpawnMargin, width)
			if maxX <= minX {
				minX = globalMinX
				maxX = globalMaxX
			}
			minY, maxY := CentralTopLeftRange(worldH, DefaultSpawnY, ObstacleSpawnMargin, height)
			if maxY <= minY {
				minY = globalMinY
				maxY = globalMaxY
			}

			x := minX
			if maxX > minX {
				x += rng.Float64() * (maxX - minX)
			}
			y := minY
			if maxY > minY {
				y += rng.Float64() * (maxY - minY)
			}

			candidate := Obstacle{
				ID:     fmt.Sprintf("obstacle-%d", len(obstacles)+1),
				X:      x,
				Y:      y,
				Width:  width,
				Height: height,
			}

			if CircleRectOverlap(DefaultSpawnX, DefaultSpawnY, PlayerSpawnSafeRadius, candidate) {
				continue
			}

			overlapsExisting := false
			for _, obs := range obstacles {
				if ObstaclesOverlap(candidate, obs, PlayerHalf) {
					overlapsExisting = true
					break
				}
			}

			if overlapsExisting {
				continue
			}

			obstacles = append(obstacles, candidate)
		}
	}

	if cfg.GoldMines && cfg.GoldMineCount > 0 {
		oreRNG := gen.SubsystemRNG("obstacles.gold")
		goldOre := generateGoldOreNodes(gen, cfg.GoldMineCount, obstacles, oreRNG)
		obstacles = append(obstacles, goldOre...)
	}

	if cfg.Lava {
		lavaPools := generateLavaPools(cfg.LavaCount, obstacles)
		obstacles = append(obstacles, lavaPools...)
	}

	return obstacles
}

// generateGoldOreNodes places ore obstacles while avoiding overlaps.
func generateGoldOreNodes(gen ObstacleGenerator, count int, existing []Obstacle, rng *rand.Rand) []Obstacle {
	if count <= 0 || rng == nil || gen == nil {
		return nil
	}

	ores := make([]Obstacle, 0, count)
	attempts := 0
	maxAttempts := count * 30
	worldW, worldH := gen.Dimensions()

	for len(ores) < count && attempts < maxAttempts {
		attempts++

		width := GoldOreMinSize + rng.Float64()*(GoldOreMaxSize-GoldOreMinSize)
		height := GoldOreMinSize + rng.Float64()*(GoldOreMaxSize-GoldOreMinSize)

		globalMinX := ObstacleSpawnMargin
		globalMaxX := worldW - ObstacleSpawnMargin - width
		globalMinY := ObstacleSpawnMargin
		globalMaxY := worldH - ObstacleSpawnMargin - height
		if globalMaxX <= globalMinX || globalMaxY <= globalMinY {
			break
		}

		minX, maxX := CentralTopLeftRange(worldW, DefaultSpawnX, ObstacleSpawnMargin, width)
		if maxX <= minX {
			minX = globalMinX
			maxX = globalMaxX
		}
		minY, maxY := CentralTopLeftRange(worldH, DefaultSpawnY, ObstacleSpawnMargin, height)
		if maxY <= minY {
			minY = globalMinY
			maxY = globalMaxY
		}

		x := minX
		if maxX > minX {
			x += rng.Float64() * (maxX - minX)
		}
		y := minY
		if maxY > minY {
			y += rng.Float64() * (maxY - minY)
		}

		candidate := Obstacle{
			ID:     fmt.Sprintf("gold-ore-%d", len(ores)+1),
			Type:   ObstacleTypeGoldOre,
			X:      x,
			Y:      y,
			Width:  width,
			Height: height,
		}

		if CircleRectOverlap(DefaultSpawnX, DefaultSpawnY, PlayerSpawnSafeRadius, candidate) {
			continue
		}

		overlaps := false

		for _, obs := range existing {
			if ObstaclesOverlap(candidate, obs, PlayerHalf) {
				overlaps = true
				break
			}
		}

		if overlaps {
			continue
		}

		for _, ore := range ores {
			if ObstaclesOverlap(candidate, ore, PlayerHalf) {
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
func generateLavaPools(count int, existing []Obstacle) []Obstacle {
	if count <= 0 {
		return nil
	}

	templates := []Obstacle{
		{ID: "lava-1", Type: ObstacleTypeLava, X: 320, Y: 120, Width: 80, Height: 80},
		{ID: "lava-2", Type: ObstacleTypeLava, X: 520, Y: 260, Width: 80, Height: 80},
		{ID: "lava-3", Type: ObstacleTypeLava, X: 200, Y: 360, Width: 80, Height: 80},
	}

	pools := make([]Obstacle, 0, len(templates))
	for _, tpl := range templates {
		if len(pools) >= count {
			break
		}
		overlaps := false
		for _, obs := range existing {
			if ObstaclesOverlap(tpl, obs, 0) {
				overlaps = true
				break
			}
		}
		if overlaps {
			continue
		}
		for _, pool := range pools {
			if ObstaclesOverlap(tpl, pool, 0) {
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
