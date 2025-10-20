package server

import worldpkg "mine-and-die/server/internal/world"

func (w *World) computePathFrom(startX, startY float64, ignoreID string, target vec2) ([]vec2, vec2, bool) {
	width, height := w.dimensions()
	blockers := w.dynamicBlockerPositions(ignoreID)
	req := worldpkg.ComputePathRequest{
		Start:     worldpkg.Vec2{X: startX, Y: startY},
		Target:    worldpkg.Vec2{X: target.X, Y: target.Y},
		Width:     width,
		Height:    height,
		Obstacles: w.obstacles,
		Blockers:  blockers,
	}
	path, goal, ok := worldpkg.ComputePathFrom(req)
	if !ok {
		return nil, vec2{}, false
	}
	return convertWorldPath(path), vec2{X: goal.X, Y: goal.Y}, true
}

func (w *World) dynamicBlockerPositions(ignoreID string) []worldpkg.Vec2 {
	if w == nil {
		return nil
	}
	estimated := len(w.npcs) + len(w.players)
	blockers := make([]worldpkg.Vec2, 0, estimated)
	for _, npc := range w.npcs {
		if npc == nil || npc.ID == ignoreID {
			continue
		}
		blockers = append(blockers, worldpkg.Vec2{X: npc.X, Y: npc.Y})
	}
	for _, player := range w.players {
		if player == nil || player.ID == ignoreID {
			continue
		}
		blockers = append(blockers, worldpkg.Vec2{X: player.X, Y: player.Y})
	}
	if len(blockers) == 0 {
		return nil
	}
	return blockers
}

func convertWorldPath(path []worldpkg.Vec2) []vec2 {
	if len(path) == 0 {
		return nil
	}
	converted := make([]vec2, len(path))
	for i, node := range path {
		converted[i] = vec2{X: node.X, Y: node.Y}
	}
	return converted
}
