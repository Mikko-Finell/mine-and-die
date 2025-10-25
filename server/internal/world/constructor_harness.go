package world

import (
	"math/rand"

	internalruntime "mine-and-die/server/internal/effects/runtime"
	itemspkg "mine-and-die/server/internal/items"
	journalpkg "mine-and-die/server/internal/journal"
       state "mine-and-die/server/internal/world/state"
)

// ConstructorHarness captures the shared state instances produced during world construction.
type ConstructorHarness struct {
	Players           map[string]*state.PlayerState
	NPCs              map[string]*state.NPCState
	Effects           []*internalruntime.State
	EffectsByID       map[string]*internalruntime.State
	EffectsIndex      *internalruntime.SpatialIndex
	GroundItems       map[string]*itemspkg.GroundItemState
	GroundItemsByTile map[itemspkg.GroundTileKey]map[string]*itemspkg.GroundItemState
	Journal           *journalpkg.Journal
	RNG               *rand.Rand
	Config            Config
	Seed              string
	NextEffectID      uint64
	EffectsRegistry   internalruntime.Registry
}

// ConstructorHarness returns the shared construction state for parity tests.
func (w *World) ConstructorHarness() ConstructorHarness {
	if w == nil {
		return ConstructorHarness{}
	}
	registry := w.EffectRegistry()
	return ConstructorHarness{
		Players:           w.players,
		NPCs:              w.npcs,
		Effects:           w.effects,
		EffectsByID:       w.effectsByID,
		EffectsIndex:      w.effectsIndex,
		GroundItems:       w.groundItems,
		GroundItemsByTile: w.groundItemsByTile,
		Journal:           &w.journal,
		RNG:               w.rng,
		Config:            w.config,
		Seed:              w.seed,
		NextEffectID:      w.nextEffectID,
		EffectsRegistry:   registry,
	}
}
