package state

import (
	"time"

	stats "mine-and-die/server/stats"
)

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

type NPCState struct {
	ActorState
	Stats            stats.Component
	Type             NPCType
	ExperienceReward int
	AIState          uint8
	AIConfigID       uint16
	Blackboard       Blackboard
	Waypoints        []Vec2
	Home             Vec2
	Cooldowns        map[string]time.Time
	Version          uint64
}

// Snapshot returns a sanitized NPC snapshot for serialization.
func (s *NPCState) Snapshot() NPC {
	return NPC{
		Actor:            s.SnapshotActor(),
		Type:             s.Type,
		AIControlled:     true,
		ExperienceReward: s.ExperienceReward,
	}
}
