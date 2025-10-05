package main

import (
	"math"
	"time"
)

type Player struct {
	ID        string          `json:"id"`
	X         float64         `json:"x"`
	Y         float64         `json:"y"`
	Facing    FacingDirection `json:"facing"`
	Inventory Inventory       `json:"inventory"`
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

type playerState struct {
	Player
	intentX       float64
	intentY       float64
	lastInput     time.Time
	lastHeartbeat time.Time
	lastRTT       time.Duration
	cooldowns     map[string]time.Time
}

func (s *playerState) snapshot() Player {
	player := s.Player
	player.Inventory = s.Inventory.Clone()
	return player
}
