package world

import (
	"time"

	combat "mine-and-die/server/internal/combat"
	internaleffects "mine-and-die/server/internal/effects"
	worldeffects "mine-and-die/server/internal/world/effects"
	state "mine-and-die/server/internal/world/state"
)

const (
	bloodSplatterDuration = 1200 * time.Millisecond
	bloodTileSize         = 40.0
)

// HandleNPCDefeat removes the defeated NPC from the world after draining its inventory.
func (w *World) HandleNPCDefeat(npc *state.NPCState) {
	if w == nil || npc == nil {
		return
	}

	current, ok := w.npcs[npc.ID]
	if !ok || current == nil {
		return
	}

	w.DropAllInventory(&current.ActorState, "death")
	delete(w.npcs, npc.ID)
	w.PurgeEntity(npc.ID)
}

// MaybeSpawnBloodSplatter mirrors the legacy NPC hit visuals for eligible actors.
func (w *World) MaybeSpawnBloodSplatter(effect *worldeffects.State, target *state.NPCState, now time.Time) {
	if w == nil || effect == nil || target == nil {
		return
	}
	if effect.Type != combat.EffectTypeAttack {
		return
	}
	if target.Type != state.NPCTypeGoblin && target.Type != state.NPCTypeRat {
		return
	}
	if w.effectManager == nil {
		return
	}

	intent, ok := internaleffects.NewBloodSplatterIntent(internaleffects.BloodSplatterIntentConfig{
		SourceActorID: effect.Owner,
		TargetActorID: target.ID,
		Target:        &internaleffects.ActorPosition{X: target.X, Y: target.Y},
		TileSize:      bloodTileSize,
		Footprint:     PlayerHalf * 2,
		Duration:      bloodSplatterDuration,
		TickRate:      TickRate,
	})
	if !ok {
		return
	}

	w.effectManager.EnqueueIntent(intent)
}
