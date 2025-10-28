package world

import state "mine-and-die/server/internal/world/state"

// AbilityActorSnapshot captures the sanitized metadata exposed to ability owner
// consumers. The snapshot mirrors the legacy ability actor struct without
// importing the combat package so callers can depend on internal state
// exclusively.
type AbilityActorSnapshot struct {
	ID     string
	X      float64
	Y      float64
	Facing string
}

func abilityActorSnapshot(actor *state.ActorState) *AbilityActorSnapshot {
	if actor == nil {
		return nil
	}

	facing := string(actor.Facing)
	if facing == "" {
		facing = string(state.DefaultFacing)
	}

	return &AbilityActorSnapshot{
		ID:     actor.ID,
		X:      actor.X,
		Y:      actor.Y,
		Facing: facing,
	}
}
