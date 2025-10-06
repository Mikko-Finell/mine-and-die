package main

import "time"

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

type npcState struct {
	actorState
	Type             NPCType
	ExperienceReward int
	AIState          uint8
	AIConfigID       uint16
	Blackboard       npcBlackboard
	Waypoints        []vec2
	cooldowns        map[string]time.Time

	wanderOrigin   vec2
	wanderTarget   vec2
	nextWanderTick uint64
	fleeUntilTick  uint64
	fleeVector     vec2
}

func (s *npcState) snapshot() NPC {
	return NPC{
		Actor:            s.snapshotActor(),
		Type:             s.Type,
		AIControlled:     true,
		ExperienceReward: s.ExperienceReward,
	}
}
