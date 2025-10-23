package sim

import itemspkg "mine-and-die/server/internal/items"

// Actor captures the shared state for any living entity in the world.
type Actor struct {
	ID        string          `json:"id"`
	X         float64         `json:"x"`
	Y         float64         `json:"y"`
	Facing    FacingDirection `json:"facing"`
	Health    float64         `json:"health"`
	MaxHealth float64         `json:"maxHealth"`
	Inventory Inventory       `json:"inventory"`
	Equipment Equipment       `json:"equipment"`
}

// Player mirrors the actor state for human-controlled characters.
type Player struct {
	Actor
	IntentDX float64 `json:"intentDX,omitempty"`
	IntentDY float64 `json:"intentDY,omitempty"`
}

// NPCType enumerates the available neutral enemy archetypes.
type NPCType string

const (
	NPCTypeGoblin NPCType = "goblin"
	NPCTypeRat    NPCType = "rat"
)

// NPC describes an AI-controlled entity mirrored to the client.
type NPC struct {
	Actor
	Type             NPCType `json:"type"`
	AIControlled     bool    `json:"aiControlled"`
	ExperienceReward int     `json:"experienceReward"`
}

// EffectTrigger represents a one-shot visual instruction that the client may execute without additional server updates.
type EffectTrigger struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Start    int64              `json:"start,omitempty"`
	Duration int64              `json:"duration,omitempty"`
	X        float64            `json:"x,omitempty"`
	Y        float64            `json:"y,omitempty"`
	Width    float64            `json:"width,omitempty"`
	Height   float64            `json:"height,omitempty"`
	Params   map[string]float64 `json:"params,omitempty"`
	Colors   []string           `json:"colors,omitempty"`
}

// GroundItem mirrors the shared ground item stack exposed to callers.
type GroundItem = itemspkg.GroundItem

// Snapshot captures the state exposed to non-simulation callers.
type Snapshot struct {
	Players        []Player        `json:"players,omitempty"`
	NPCs           []NPC           `json:"npcs,omitempty"`
	GroundItems    []GroundItem    `json:"groundItems,omitempty"`
	EffectEvents   []EffectTrigger `json:"effectTriggers,omitempty"`
	Obstacles      []Obstacle      `json:"obstacles,omitempty"`
	AliveEffectIDs []string        `json:"aliveEffectIDs,omitempty"`
}
