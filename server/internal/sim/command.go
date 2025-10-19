package sim

import "time"

// CommandType enumerates the supported simulation commands.
type CommandType string

const (
	CommandMove      CommandType = "Move"
	CommandAction    CommandType = "Action"
	CommandHeartbeat CommandType = "Heartbeat"
	CommandSetPath   CommandType = "SetPath"
	CommandClearPath CommandType = "ClearPath"
)

// MoveCommand carries the desired movement vector and facing.
type MoveCommand struct {
	DX     float64         `json:"dx"`
	DY     float64         `json:"dy"`
	Facing FacingDirection `json:"facing"`
}

// ActionCommand identifies an ability or interaction trigger.
type ActionCommand struct {
	Name string `json:"name"`
}

// PathCommand identifies a navigation target for A* pathfinding.
type PathCommand struct {
	TargetX float64 `json:"targetX"`
	TargetY float64 `json:"targetY"`
}

// HeartbeatCommand updates connectivity metadata for an actor.
type HeartbeatCommand struct {
	ReceivedAt time.Time     `json:"receivedAt"`
	ClientSent int64         `json:"clientSent"`
	RTT        time.Duration `json:"rtt"`
}

// Command represents an intent captured for processing on the next tick.
type Command struct {
	OriginTick uint64            `json:"originTick"`
	ActorID    string            `json:"actorId"`
	Type       CommandType       `json:"type"`
	IssuedAt   time.Time         `json:"issuedAt"`
	Move       *MoveCommand      `json:"move,omitempty"`
	Action     *ActionCommand    `json:"action,omitempty"`
	Heartbeat  *HeartbeatCommand `json:"heartbeat,omitempty"`
	Path       *PathCommand      `json:"path,omitempty"`
}
