package main

import "time"

// NPCType enumerates the available neutral enemy archetypes.
type NPCType string

const (
	NPCTypeGoblin NPCType = "goblin"
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
	pathNodes        []vec2
	pathDesired      vec2
	pathActual       vec2
	pathLastDist     float64
	pathWaypointIdx  int
	pathIndex        int
	pathCooldownTick uint64
	pathHasFallback  bool
	cooldowns        map[string]time.Time
}

func (s *npcState) snapshot() NPC {
	return NPC{
		Actor:            s.snapshotActor(),
		Type:             s.Type,
		AIControlled:     true,
		ExperienceReward: s.ExperienceReward,
	}
}

func (s *npcState) resetPathNodes() {
	if s == nil {
		return
	}
	s.pathNodes = nil
	s.pathIndex = 0
	s.pathLastDist = 0
}

func (s *npcState) resetPathState() {
	if s == nil {
		return
	}
	s.resetPathNodes()
	s.pathDesired = vec2{}
	s.pathActual = vec2{}
	s.pathWaypointIdx = -1
	s.pathHasFallback = false
	s.pathCooldownTick = 0
}
