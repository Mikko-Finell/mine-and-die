package main

import "math"

const positionEpsilon = 1e-6

// positionsEqual reports whether two coordinate pairs are effectively the same.
func positionsEqual(ax, ay, bx, by float64) bool {
	return math.Abs(ax-bx) <= positionEpsilon && math.Abs(ay-by) <= positionEpsilon
}

// SetPosition updates a player's position, bumps the version, and records a patch.
func (w *World) SetPosition(playerID string, x, y float64) {
	if w == nil {
		return
	}

	player, ok := w.players[playerID]
	if !ok {
		return
	}

	if positionsEqual(player.X, player.Y, x, y) {
		return
	}

	player.X = x
	player.Y = y
	player.version++

	w.journal.AppendPatch(Patch{
		Kind:     PatchPlayerPos,
		EntityID: playerID,
		Payload: PlayerPosPayload{
			X: x,
			Y: y,
		},
	})
}
