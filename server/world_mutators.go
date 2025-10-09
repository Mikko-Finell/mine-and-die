package main

// SetPosition updates a player's coordinates while recording a journal patch.
func (w *World) SetPosition(playerID string, x, y float64) {
	if w == nil {
		return
	}
	player, ok := w.players[playerID]
	if !ok || player == nil {
		return
	}
	if player.X == x && player.Y == y {
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
