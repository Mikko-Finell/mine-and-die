package server

import worldpkg "mine-and-die/server/internal/world"

func (w *World) applyPlayerPositionMutations(initial map[string]worldpkg.Vec2, proposed map[string]worldpkg.Vec2) {
	if w == nil {
		return
	}
	actors := make([]worldpkg.PositionCommit, 0, len(w.players))
	for id, player := range w.players {
		actors = append(actors, worldpkg.PositionCommit{
			ID:      id,
			Current: worldpkg.Vec2{X: player.X, Y: player.Y},
		})
	}
	worldpkg.ApplyPlayerPositionMutations(actors, initial, proposed, w.SetPosition)
}

func (w *World) applyNPCPositionMutations(initial map[string]worldpkg.Vec2, proposed map[string]worldpkg.Vec2) {
	if w == nil {
		return
	}
	actors := make([]worldpkg.PositionCommit, 0, len(w.npcs))
	for id, npc := range w.npcs {
		actors = append(actors, worldpkg.PositionCommit{
			ID:      id,
			Current: worldpkg.Vec2{X: npc.X, Y: npc.Y},
		})
	}
	worldpkg.ApplyNPCPositionMutations(actors, initial, proposed, w.SetNPCPosition)
}
