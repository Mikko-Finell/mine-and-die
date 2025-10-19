package main

import (
	"mine-and-die/server/internal/sim"
	simpaches "mine-and-die/server/internal/sim/patches"
)

// PlayerPatchView mirrors the fields required to replay player patches onto a
// snapshot. It embeds the broadcast-facing Player representation and extends it
// with intent vectors that only exist in diff payloads today.
type PlayerPatchView struct {
	Player
	IntentDX float64
	IntentDY float64
}

// clonePlayerPatchView performs a deep copy of the view to avoid sharing slice
// memory between snapshots.
func clonePlayerPatchView(view PlayerPatchView) PlayerPatchView {
	cloned := view
	cloned.Inventory = view.Inventory.Clone()
	return cloned
}

// ApplyPatches applies a series of diff patches to the provided player
// snapshot, returning a new copy that reflects all mutations.
func ApplyPatches(base map[string]PlayerPatchView, patches []Patch) (map[string]PlayerPatchView, error) {
	simBase := simPlayerViewsFromLegacy(base)
	simPatches := simPatchesFromLegacy(patches)

	replayed, err := simpaches.ApplyPlayers(simBase, simPatches)
	if err != nil {
		return nil, err
	}

	return legacyPlayerViewsFromSim(replayed), nil
}

func simPlayerViewsFromLegacy(base map[string]PlayerPatchView) map[string]simpaches.PlayerView {
	if len(base) == 0 {
		return nil
	}
	converted := make(map[string]simpaches.PlayerView, len(base))
	for id, view := range base {
		converted[id] = simPlayerViewFromLegacy(view)
	}
	return converted
}

func simPlayerViewFromLegacy(view PlayerPatchView) simpaches.PlayerView {
	return simpaches.PlayerView{
		Player:   sim.Player{Actor: simActorFromLegacy(view.Player.Actor)},
		IntentDX: view.IntentDX,
		IntentDY: view.IntentDY,
	}
}

func legacyPlayerViewsFromSim(src map[string]simpaches.PlayerView) map[string]PlayerPatchView {
	converted := make(map[string]PlayerPatchView, len(src))
	for id, view := range src {
		converted[id] = legacyPlayerPatchViewFromSim(view)
	}
	return converted
}

func legacyPlayerPatchViewFromSim(view simpaches.PlayerView) PlayerPatchView {
	return PlayerPatchView{
		Player:   Player{Actor: legacyActorFromSim(view.Player.Actor)},
		IntentDX: view.IntentDX,
		IntentDY: view.IntentDY,
	}
}
