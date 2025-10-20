package server

import worldpkg "mine-and-die/server/internal/world"

func (w *World) computePathFrom(startX, startY float64, ignoreID string, target vec2) ([]vec2, vec2, bool) {
	width, height := w.dimensions()
	actors := make([]worldpkg.PathActor, 0, len(w.npcs)+len(w.players))
	for _, npc := range w.npcs {
		if npc == nil {
			continue
		}
		actors = append(actors, worldpkg.PathActor{
			ID:       npc.ID,
			Position: worldpkg.Vec2{X: npc.X, Y: npc.Y},
		})
	}
	for _, player := range w.players {
		if player == nil {
			continue
		}
		actors = append(actors, worldpkg.PathActor{
			ID:       player.ID,
			Position: worldpkg.Vec2{X: player.X, Y: player.Y},
		})
	}
	req := worldpkg.ComputePathRequest{
		Start:     worldpkg.Vec2{X: startX, Y: startY},
		Target:    worldpkg.Vec2(target),
		Width:     width,
		Height:    height,
		Obstacles: w.obstacles,
	}
	path, goal, ok := worldpkg.ComputeNavigationPath(req, actors, ignoreID)
	if !ok {
		return nil, vec2{}, false
	}
	return path, vec2(goal), true
}
