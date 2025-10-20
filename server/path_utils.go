package server

import "math"

func pathTravelCost(startX, startY float64, path []vec2) float64 {
	if len(path) == 0 {
		return 0
	}
	cost := 0.0
	prevX := startX
	prevY := startY
	for _, node := range path {
		cost += math.Hypot(node.X-prevX, node.Y-prevY)
		prevX = node.X
		prevY = node.Y
	}
	return cost
}

func (w *World) computePathFrom(startX, startY float64, ignoreID string, target vec2) ([]vec2, vec2, bool) {
	width, height := w.dimensions()
	grid := newNavGrid(w.obstacles, width, height)
	if grid == nil {
		return nil, vec2{}, false
	}
	target = vec2{
		X: clamp(target.X, playerHalf, width-playerHalf),
		Y: clamp(target.Y, playerHalf, height-playerHalf),
	}
	blocked := w.buildDynamicBlockers(grid, ignoreID)
	path, ok := grid.findPath(startX, startY, target, blocked)
	if ok {
		return append([]vec2(nil), path...), target, true
	}
	step := grid.cellSize
	offsets := []vec2{
		{X: step, Y: 0},
		{X: -step, Y: 0},
		{X: 0, Y: step},
		{X: 0, Y: -step},
		{X: step, Y: step},
		{X: step, Y: -step},
		{X: -step, Y: step},
		{X: -step, Y: -step},
		{X: 2 * step, Y: 0},
		{X: -2 * step, Y: 0},
		{X: 0, Y: 2 * step},
		{X: 0, Y: -2 * step},
	}
	bestScore := math.MaxFloat64
	var bestPath []vec2
	var bestGoal vec2
	for _, offset := range offsets {
		alt := vec2{
			X: clamp(target.X+offset.X, playerHalf, width-playerHalf),
			Y: clamp(target.Y+offset.Y, playerHalf, height-playerHalf),
		}
		if math.Hypot(alt.X-target.X, alt.Y-target.Y) < 1 {
			continue
		}
		candidate, ok := grid.findPath(startX, startY, alt, blocked)
		if !ok {
			continue
		}
		score := math.Hypot(alt.X-target.X, alt.Y-target.Y) + pathTravelCost(startX, startY, candidate)
		if score < bestScore {
			bestScore = score
			bestGoal = alt
			bestPath = append([]vec2(nil), candidate...)
		}
	}
	if len(bestPath) == 0 {
		return nil, vec2{}, false
	}
	return bestPath, bestGoal, true
}

func (w *World) buildDynamicBlockers(grid *navGrid, ignoreID string) map[int]struct{} {
	if grid == nil {
		return nil
	}
	blocked := make(map[int]struct{})
	mark := func(x, y float64) {
		minCol := int(math.Floor((x - playerHalf) / grid.cellSize))
		maxCol := int(math.Ceil((x + playerHalf) / grid.cellSize))
		minRow := int(math.Floor((y - playerHalf) / grid.cellSize))
		maxRow := int(math.Ceil((y + playerHalf) / grid.cellSize))
		for row := minRow; row <= maxRow; row++ {
			for col := minCol; col <= maxCol; col++ {
				if !grid.inBounds(col, row) {
					continue
				}
				idx := grid.index(col, row)
				if !grid.walkable[idx] {
					continue
				}
				cx := (float64(col) + 0.5) * grid.cellSize
				cy := (float64(row) + 0.5) * grid.cellSize
				if math.Hypot(cx-x, cy-y) <= playerHalf {
					blocked[idx] = struct{}{}
				}
			}
		}
	}
	for _, npc := range w.npcs {
		if npc == nil || npc.ID == ignoreID {
			continue
		}
		mark(npc.X, npc.Y)
	}
	for _, player := range w.players {
		if player == nil || player.ID == ignoreID {
			continue
		}
		mark(player.X, player.Y)
	}
	if len(blocked) == 0 {
		return nil
	}
	return blocked
}
