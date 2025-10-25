package state

// maxAbilitySlots mirrors the legacy AI ability slot count for cooldown bookkeeping.
const maxAbilitySlots = 4

// Blackboard stores per-NPC AI memory required by the finite-state executor.
type Blackboard struct {
	WaypointIndex     int
	LastWaypointIndex int
	WaypointBestDist  float64
	WaypointLastDist  float64
	WaypointStall     uint16
	WaitUntil         uint64
	NextDecisionAt    uint64
	StateEnteredTick  uint64
	LastDecisionTick  uint64
	LastPos           Vec2
	LastMoveDelta     float64
	StuckCounter      uint8
	TargetActorID     string
	ChaseUntil        uint64
	PauseTicks        uint64
	PatrolSpeed       float64
	StuckEpsilon      float64
	NPCPathState

	NextAbilityReady [maxAbilitySlots]uint64
}
