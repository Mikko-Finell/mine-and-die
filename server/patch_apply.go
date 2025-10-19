package main

import (
	simpaches "mine-and-die/server/internal/sim/patches"
)

// PlayerPatchView mirrors the state required to replay player patches onto a snapshot.
type PlayerPatchView = simpaches.PlayerView

// ApplyPatches applies a series of diff patches to the provided player snapshot, returning
// a new copy that reflects all mutations.
func ApplyPatches(base map[string]PlayerPatchView, patches []Patch) (map[string]PlayerPatchView, error) {
	simPatches := simPatchesFromLegacy(patches)

	replayed, err := simpaches.ApplyPlayers(base, simPatches)
	if err != nil {
		return nil, err
	}

	return replayed, nil
}
