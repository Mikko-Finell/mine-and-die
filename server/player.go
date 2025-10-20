package server

import (
	"math"
	"time"

	stats "mine-and-die/server/stats"
)

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
}

type FacingDirection string

const (
	FacingUp    FacingDirection = "up"
	FacingDown  FacingDirection = "down"
	FacingLeft  FacingDirection = "left"
	FacingRight FacingDirection = "right"

	defaultFacing FacingDirection = FacingDown
)

// parseFacing validates a facing string received from the client.
func parseFacing(value string) (FacingDirection, bool) {
	switch FacingDirection(value) {
	case FacingUp, FacingDown, FacingLeft, FacingRight:
		return FacingDirection(value), true
	default:
		return "", false
	}
}

// deriveFacing picks the facing direction that best matches the movement
// vector, falling back to the last known facing when idle.
func deriveFacing(dx, dy float64, fallback FacingDirection) FacingDirection {
	if fallback == "" {
		fallback = defaultFacing
	}

	const epsilon = 1e-6

	if math.Abs(dx) < epsilon {
		dx = 0
	}
	if math.Abs(dy) < epsilon {
		dy = 0
	}

	if dx == 0 && dy == 0 {
		return fallback
	}

	absX := math.Abs(dx)
	absY := math.Abs(dy)

	if absY >= absX && dy != 0 {
		if dy > 0 {
			return FacingDown
		}
		return FacingUp
	}

	if dx > 0 {
		return FacingRight
	}
	return FacingLeft
}

// facingToVector returns a unit vector for the given facing.
func facingToVector(facing FacingDirection) (float64, float64) {
	switch facing {
	case FacingUp:
		return 0, -1
	case FacingDown:
		return 0, 1
	case FacingLeft:
		return -1, 0
	case FacingRight:
		return 1, 0
	default:
		return 0, 1
	}
}

type actorState struct {
	Actor
	intentX       float64
	intentY       float64
	statusEffects map[StatusEffectType]*statusEffectInstance
}

type playerPathState struct {
	Path             []vec2
	PathIndex        int
	PathTarget       vec2
	PathGoal         vec2
	PathLastDistance float64
	PathStallTicks   int
	PathRecalcTick   uint64
	ArriveRadius     float64
}

func (s *actorState) snapshotActor() Actor {
	actor := s.Actor
	if actor.Facing == "" {
		actor.Facing = defaultFacing
	}
	actor.Inventory = s.Inventory.Clone()
	actor.Equipment = s.Equipment.Clone()
	return actor
}

// applyHealthDelta adjusts the actor's health while clamping to [0, MaxHealth].
// It returns true when the value actually changes.
func (s *actorState) applyHealthDelta(delta float64) bool {
	if delta == 0 {
		return false
	}
	max := s.MaxHealth
	next := s.Health + delta
	if next < 0 {
		next = 0
	}
	if max > 0 && next > max {
		next = max
	}
	if math.Abs(next-s.Health) < 1e-6 {
		return false
	}
	s.Health = next
	return true
}

// playerState wraps actorState with simulation metadata. Mutate the embedded
// Actor's position via World.SetPosition so versioning and patch emission stay
// consistent.
type playerState struct {
	actorState
	stats         stats.Component
	lastInput     time.Time
	lastHeartbeat time.Time
	lastRTT       time.Duration
	cooldowns     map[string]time.Time
	path          playerPathState
	version       uint64
}

func (s *playerState) snapshot() Player {
	return Player{Actor: s.snapshotActor()}
}
