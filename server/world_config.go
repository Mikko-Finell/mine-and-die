package server

import "strings"

const defaultWorldSeed = "prototype"

// worldConfig captures the toggles used when generating a world.
type worldConfig struct {
	Obstacles      bool    `json:"obstacles"`
	ObstaclesCount int     `json:"obstaclesCount"`
	GoldMines      bool    `json:"goldMines"`
	GoldMineCount  int     `json:"goldMineCount"`
	NPCs           bool    `json:"npcs"`
	GoblinCount    int     `json:"goblinCount"`
	RatCount       int     `json:"ratCount"`
	NPCCount       int     `json:"npcCount"`
	Lava           bool    `json:"lava"`
	LavaCount      int     `json:"lavaCount"`
	Seed           string  `json:"seed"`
	Width          float64 `json:"width"`
	Height         float64 `json:"height"`
}

// normalized returns a config with defaults applied.
func (cfg worldConfig) normalized() worldConfig {
	normalized := cfg
	normalized.Seed = strings.TrimSpace(normalized.Seed)
	if normalized.Seed == "" {
		normalized.Seed = defaultWorldSeed
	}
	if normalized.ObstaclesCount < 0 {
		normalized.ObstaclesCount = 0
	}
	if normalized.GoldMineCount < 0 {
		normalized.GoldMineCount = 0
	}
	if normalized.GoblinCount < 0 {
		normalized.GoblinCount = 0
	}
	if normalized.RatCount < 0 {
		normalized.RatCount = 0
	}
	if normalized.NPCCount < 0 {
		normalized.NPCCount = 0
	}
	if normalized.LavaCount < 0 {
		normalized.LavaCount = 0
	}
	totalSpecies := normalized.GoblinCount + normalized.RatCount
	if totalSpecies > 0 {
		normalized.NPCCount = totalSpecies
	}
	if normalized.Width <= 0 {
		normalized.Width = worldWidth
	}
	if normalized.Height <= 0 {
		normalized.Height = worldHeight
	}
	return normalized
}

// defaultWorldConfig enables every world feature and the default seed.
func defaultWorldConfig() worldConfig {
	return worldConfig{
		Obstacles:      false,
		ObstaclesCount: defaultObstacleCount,
		GoldMines:      false,
		GoldMineCount:  defaultGoldMineCount,
		NPCs:           false,
		GoblinCount:    defaultGoblinCount,
		RatCount:       defaultRatCount,
		NPCCount:       defaultNPCCount,
		Lava:           false,
		LavaCount:      defaultLavaCount,
		Seed:           defaultWorldSeed,
		Width:          worldWidth,
		Height:         worldHeight,
	}
}
