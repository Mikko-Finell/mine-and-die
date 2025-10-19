package sim

import "time"

// Obstacle mirrors the legacy obstacle snapshot exposed via keyframes.
type Obstacle struct {
	ID     string  `json:"id"`
	Type   string  `json:"type,omitempty"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// WorldConfig captures the world generation toggles mirrored in keyframes.
type WorldConfig struct {
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

// Keyframe captures the immutable state snapshot stored in the journal.
type Keyframe struct {
	Tick        uint64       `json:"tick"`
	Sequence    uint64       `json:"sequence"`
	Players     []Player     `json:"players,omitempty"`
	NPCs        []NPC        `json:"npcs,omitempty"`
	Obstacles   []Obstacle   `json:"obstacles,omitempty"`
	GroundItems []GroundItem `json:"groundItems,omitempty"`
	Config      WorldConfig  `json:"config"`
	RecordedAt  time.Time    `json:"recordedAt"`
}
