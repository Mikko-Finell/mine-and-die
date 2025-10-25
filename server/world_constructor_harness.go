package server

import (
	worldpkg "mine-and-die/server/internal/world"
)

// ConstructorHarness exposes the shared construction state for parity verification.
func (w *World) ConstructorHarness() worldpkg.ConstructorHarness {
	if w == nil {
		return worldpkg.ConstructorHarness{}
	}
	return worldpkg.ConstructorHarness{
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
		EffectsRegistry:   w.effectRegistry(),
	}
}
