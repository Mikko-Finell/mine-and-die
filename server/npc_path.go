package server

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
		w.SetNPCFacing(npc.ID, deriveFacing(dx, dy, npc.Facing))
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
	return w.computePathFrom(npc.X, npc.Y, npc.ID, target)
}
