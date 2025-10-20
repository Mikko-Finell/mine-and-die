package world

import "strings"

const (
	DefaultSeed   = "prototype"
	DefaultWidth  = 100.0
	DefaultHeight = 100.0
)

type Config struct {
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

func (cfg Config) normalized() Config {
	normalized := cfg
	normalized.Seed = strings.TrimSpace(normalized.Seed)
	if normalized.Seed == "" {
		normalized.Seed = DefaultSeed
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
		normalized.Width = DefaultWidth
	}
	if normalized.Height <= 0 {
		normalized.Height = DefaultHeight
	}
	return normalized
}

func (cfg Config) Normalized() Config {
	return cfg.normalized()
}

func DefaultConfig() Config {
	return Config{
		Obstacles:      false,
		ObstaclesCount: 0,
		GoldMines:      false,
		GoldMineCount:  0,
		NPCs:           false,
		GoblinCount:    0,
		RatCount:       0,
		NPCCount:       0,
		Lava:           false,
		LavaCount:      0,
		Seed:           DefaultSeed,
		Width:          DefaultWidth,
		Height:         DefaultHeight,
	}
}
