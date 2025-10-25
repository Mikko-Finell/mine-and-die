package server

import (
	"math"
	"time"

	worldpkg "mine-and-die/server/internal/world"
	stats "mine-and-die/server/stats"
)

type actorState struct {
	Actor
	intentX       float64
	intentY       float64
	statusEffects map[StatusEffectType]*statusEffectInstance
}

type playerPathState = worldpkg.PlayerPathState

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
