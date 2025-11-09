package abilities

import state "mine-and-die/server/internal/world/state"

// ActorSnapshot captures the sanitized metadata exposed to ability owner
// consumers. The struct mirrors the legacy ability actor contract without
// importing combat so callers can operate purely on internal state.
type ActorSnapshot struct {
	ID     string
	X      float64
	Y      float64
	Facing string
}

// NewActorSnapshot converts the provided actor state into a sanitized ability
// owner snapshot. Missing facings fall back to the internal default to mirror
// the legacy behaviour.
func NewActorSnapshot(actor *state.ActorState) *ActorSnapshot {
	if actor == nil {
		return nil
	}

	facing := string(actor.Facing)
	if facing == "" {
		facing = string(state.DefaultFacing)
	}

	return &ActorSnapshot{
		ID:     actor.ID,
		X:      actor.X,
		Y:      actor.Y,
		Facing: facing,
	}
}
