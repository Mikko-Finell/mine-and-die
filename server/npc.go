package server

import (
	"time"

	ai "mine-and-die/server/internal/ai"
	stats "mine-and-die/server/stats"
)

type npcState struct {
	actorState
	stats            stats.Component
	Type             NPCType
	ExperienceReward int
	AIState          uint8
	AIConfigID       uint16
	Blackboard       ai.Blackboard
	Waypoints        []vec2
	Home             vec2
	cooldowns        map[string]time.Time
	version          uint64
}

func (s *npcState) snapshot() NPC {
	return NPC{
		Actor:            s.snapshotActor(),
		Type:             s.Type,
		AIControlled:     true,
		ExperienceReward: s.ExperienceReward,
	}
}
