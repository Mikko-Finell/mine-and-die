package state

// NPCPathState tracks the path-following metadata for a single NPC.
type NPCPathState struct {
	Path             []Vec2
	PathIndex        int
	PathTarget       Vec2
	PathGoal         Vec2
	PathLastDistance float64
	PathStallTicks   uint16
	PathRecalcTick   uint64
	ArriveRadius     float64
}
