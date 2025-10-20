package server

import worldpkg "mine-and-die/server/internal/world"

const maxAIAbilities = 4

// vec2 captures a 2D vector for blackboard bookkeeping.
type vec2 = worldpkg.Vec2

// npcBlackboard stores per-NPC AI memory required by the FSM executor.
type npcBlackboard struct {
	WaypointIndex     int
	LastWaypointIndex int
	WaypointBestDist  float64
	WaypointLastDist  float64
	WaypointStall     uint16
	WaitUntil         uint64
	NextDecisionAt    uint64
	StateEnteredTick  uint64
	LastDecisionTick  uint64
	LastPos           vec2
	LastMoveDelta     float64
	StuckCounter      uint8
	TargetActorID     string
	ChaseUntil        uint64
	PauseTicks        uint64
	PatrolSpeed       float64
	StuckEpsilon      float64
	worldpkg.NPCPathState
	nextAbilityReady [maxAIAbilities]uint64
}
