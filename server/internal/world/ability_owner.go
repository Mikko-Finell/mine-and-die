package world

import (
	abilitiespkg "mine-and-die/server/internal/world/abilities"
	state "mine-and-die/server/internal/world/state"
)

// AbilityActorSnapshot re-exports the shared ability owner snapshot used by the
// internal ability helpers so callers can depend on the world package without
// importing the subpackage directly.
type AbilityActorSnapshot = abilitiespkg.ActorSnapshot

func abilityActorSnapshot(actor *state.ActorState) *AbilityActorSnapshot {
	return abilitiespkg.NewActorSnapshot(actor)
}
