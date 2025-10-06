package main

import "math"

func (w *World) advanceNPCPaths(tick uint64) {
	for _, npc := range w.npcs {
		w.followNPCPath(npc, tick)
	}
}

func (w *World) followNPCPath(npc *npcState, tick uint64) {
	if npc == nil {
		return
	}
	bb := &npc.Blackboard
	if len(bb.Path) == 0 {
		npc.intentX = 0
		npc.intentY = 0
		if (bb.PathTarget.X != 0 || bb.PathTarget.Y != 0) && tick >= bb.PathRecalcTick {
			if w.recalculateNPCPath(npc, tick) {
				w.followNPCPath(npc, tick)
			}
		}
		return
	}
	if bb.PathIndex >= len(bb.Path) {
		w.finishNPCPath(npc)
		return
	}

	threshold := pathNodeReachedEpsilon
	if threshold <= 0 {
		threshold = playerHalf
	}

	for bb.PathIndex < len(bb.Path) {
		node := bb.Path[bb.PathIndex]
		dx := node.X - npc.X
		dy := node.Y - npc.Y
		dist := math.Hypot(dx, dy)

		limit := threshold
		if bb.PathIndex == len(bb.Path)-1 {
			radius := bb.ArriveRadius
			if radius <= 0 {
				radius = 12
			}
			limit = radius
		}
		if dist <= limit {
			bb.PathIndex++
			bb.PathLastDistance = 0
			bb.PathStallTicks = 0
			continue
		}

		if bb.PathLastDistance == 0 || dist+0.1 < bb.PathLastDistance {
			bb.PathLastDistance = dist
			bb.PathStallTicks = 0
		} else {
			bb.PathStallTicks++
			if bb.PathStallTicks >= pathStallThresholdTicks || bb.StuckCounter >= 8 {
				if tick >= bb.PathRecalcTick && w.recalculateNPCPath(npc, tick) {
					w.followNPCPath(npc, tick)
				}
				return
			}
		}

		if bb.PathLastDistance > 0 && dist > bb.PathLastDistance+pathPushRecalcThreshold {
			if tick >= bb.PathRecalcTick && w.recalculateNPCPath(npc, tick) {
				w.followNPCPath(npc, tick)
			}
			return
		}

		npc.intentX = dx
		npc.intentY = dy
		npc.Facing = deriveFacing(dx, dy, npc.Facing)
		return
	}

	w.finishNPCPath(npc)
}

func (w *World) finishNPCPath(npc *npcState) {
	if npc == nil {
		return
	}
	bb := &npc.Blackboard
	bb.Path = nil
	bb.PathIndex = 0
	bb.PathGoal = vec2{}
	bb.PathTarget = vec2{}
	bb.PathLastDistance = 0
	bb.PathStallTicks = 0
	bb.PathRecalcTick = 0
	npc.intentX = 0
	npc.intentY = 0
}

func (w *World) clearNPCPath(npc *npcState) {
	if npc == nil {
		return
	}
	bb := &npc.Blackboard
	bb.Path = nil
	bb.PathIndex = 0
	bb.PathGoal = vec2{}
	bb.PathTarget = vec2{}
	bb.PathLastDistance = 0
	bb.PathStallTicks = 0
	bb.PathRecalcTick = 0
	npc.intentX = 0
	npc.intentY = 0
}

func (w *World) ensureNPCPath(npc *npcState, target vec2, tick uint64) bool {
	if npc == nil {
		return false
	}
	bb := &npc.Blackboard
	bb.PathTarget = target
	if len(bb.Path) > 0 && bb.PathIndex < len(bb.Path) {
		if math.Hypot(bb.PathGoal.X-target.X, bb.PathGoal.Y-target.Y) <= navCellSize*0.5 {
			return true
		}
	}
	path, goal, ok := w.computeNPCPath(npc, target)
	if !ok {
		bb.Path = nil
		bb.PathIndex = 0
		bb.PathGoal = vec2{}
		bb.PathLastDistance = 0
		bb.PathStallTicks = 0
		bb.PathRecalcTick = tick + pathRecalcCooldownTicks
		npc.intentX = 0
		npc.intentY = 0
		return false
	}
	bb.Path = path
	bb.PathIndex = 0
	bb.PathGoal = goal
	bb.PathLastDistance = 0
	bb.PathStallTicks = 0
	bb.PathRecalcTick = tick + 1
	return true
}

func (w *World) recalculateNPCPath(npc *npcState, tick uint64) bool {
	if npc == nil {
		return false
	}
	target := npc.Blackboard.PathTarget
	if target.X == 0 && target.Y == 0 {
		return false
	}
	path, goal, ok := w.computeNPCPath(npc, target)
	if !ok {
		npc.Blackboard.Path = nil
		npc.Blackboard.PathIndex = 0
		npc.Blackboard.PathGoal = vec2{}
		npc.Blackboard.PathLastDistance = 0
		npc.Blackboard.PathStallTicks = 0
		npc.Blackboard.PathRecalcTick = tick + pathRecalcCooldownTicks
		npc.intentX = 0
		npc.intentY = 0
		return false
	}
	npc.Blackboard.Path = path
	npc.Blackboard.PathIndex = 0
	npc.Blackboard.PathGoal = goal
	npc.Blackboard.PathLastDistance = 0
	npc.Blackboard.PathStallTicks = 0
	npc.Blackboard.PathRecalcTick = tick + 1
	return true
}

func (w *World) computeNPCPath(npc *npcState, target vec2) ([]vec2, vec2, bool) {
	if npc == nil {
		return nil, vec2{}, false
	}
	grid := newNavGrid(w.obstacles)
	if grid == nil {
		return nil, vec2{}, false
	}
	blocked := w.buildDynamicBlockers(grid, npc)
	path, ok := grid.findPath(npc.X, npc.Y, target, blocked)
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
			X: clamp(target.X+offset.X, playerHalf, worldWidth-playerHalf),
			Y: clamp(target.Y+offset.Y, playerHalf, worldHeight-playerHalf),
		}
		if math.Hypot(alt.X-target.X, alt.Y-target.Y) < 1 {
			continue
		}
		candidate, ok := grid.findPath(npc.X, npc.Y, alt, blocked)
		if !ok {
			continue
		}
		score := math.Hypot(alt.X-target.X, alt.Y-target.Y) + float64(len(candidate))
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

func (w *World) buildDynamicBlockers(grid *navGrid, npc *npcState) map[int]struct{} {
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
	for _, other := range w.npcs {
		if other == nil || (npc != nil && other.ID == npc.ID) {
			continue
		}
		mark(other.X, other.Y)
	}
	for _, player := range w.players {
		if player == nil {
			continue
		}
		mark(player.X, player.Y)
	}
	if len(blocked) == 0 {
		return nil
	}
	return blocked
}
