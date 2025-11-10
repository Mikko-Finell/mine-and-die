package server

import (
	worldpkg "mine-and-die/server/internal/world"
)

func worldAbilityActorSnapshot(state *actorState) *worldpkg.AbilityActorSnapshot {
	if state == nil {
		return nil
	}

	facing := string(state.Facing)
	if facing == "" {
		facing = string(defaultFacing)
	}

	return &worldpkg.AbilityActorSnapshot{
		ID:     state.ID,
		X:      state.X,
		Y:      state.Y,
		Facing: facing,
	}
}
