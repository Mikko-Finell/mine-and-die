package ai

import (
	"time"

	worldpkg "mine-and-die/server/internal/world"
)

const maxAbilitySlots = 4

// Vec2 captures a 2D vector for blackboard bookkeeping.
type Vec2 = worldpkg.Vec2

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
	worldpkg.NPCPathState

	nextAbilityReady [maxAbilitySlots]uint64
}

// PositionRef exposes direct references to an NPC's positional coordinates.
type PositionRef struct {
	X *float64
	Y *float64
}

// X returns the referenced X coordinate or zero when no pointer was supplied.
func (r PositionRef) XValue() float64 {
	if r.X == nil {
		return 0
	}
	return *r.X
}

// Y returns the referenced Y coordinate or zero when no pointer was supplied.
func (r PositionRef) YValue() float64 {
	if r.Y == nil {
		return 0
	}
	return *r.Y
}

// FacingAdapter wraps callbacks for reading and mutating an NPC's facing.
type FacingAdapter struct {
	Get func() string
	Set func(string)
}

// Value retrieves the current facing using the configured getter.
func (f FacingAdapter) Value() string {
	if f.Get == nil {
		return ""
	}
	return f.Get()
}

// Apply updates the facing using the configured setter when present.
func (f FacingAdapter) Apply(value string) {
	if f.Set == nil {
		return
	}
	f.Set(value)
}

// NPCHooks bundles callbacks required to mutate NPC navigation state.
type NPCHooks struct {
	ClearPath  func()
	EnsurePath func(target Vec2, tick uint64) bool
}

// NPC exposes the runtime state required by the AI executor while hiding the
// legacy world type behind adapters.
type NPC struct {
	ID         string
	Type       string
	AIConfigID uint16
	AIState    *uint8
	Position   PositionRef
	Facing     FacingAdapter
	Waypoints  *[]Vec2
	Home       *Vec2
	Blackboard *Blackboard
	Hooks      NPCHooks
}

// Player mirrors the subset of player state required by the AI executor.
type Player struct {
	ID string
	X  float64
	Y  float64
}

// CommandType enumerates the supported command kinds produced by the executor.
type CommandType string

const (
	CommandMove      CommandType = "Move"
	CommandAction    CommandType = "Action"
	CommandHeartbeat CommandType = "Heartbeat"
	CommandSetPath   CommandType = "SetPath"
	CommandClearPath CommandType = "ClearPath"
)

// Command mirrors the legacy simulation command structure with only the fields
// used by the AI executor populated.
type Command struct {
	OriginTick uint64
	ActorID    string
	Type       CommandType
	IssuedAt   time.Time
	Move       *MoveCommand
	Action     *ActionCommand
	Heartbeat  *HeartbeatCommand
	Path       *PathCommand
}

// MoveCommand carries desired movement vector and facing.
type MoveCommand struct {
	DX     float64
	DY     float64
	Facing string
}

// ActionCommand identifies an ability or interaction trigger.
type ActionCommand struct {
	Name string
}

// HeartbeatCommand mirrors connectivity metadata for parity with legacy
// commands. The AI executor never emits heartbeat commands but the structure is
// preserved for completeness.
type HeartbeatCommand struct {
	ReceivedAt time.Time
	ClientSent int64
	RTT        time.Duration
}

// PathCommand mirrors navigation commands. The executor does not emit path
// commands but the field is retained for structural parity.
type PathCommand struct {
	TargetX float64
	TargetY float64
}
