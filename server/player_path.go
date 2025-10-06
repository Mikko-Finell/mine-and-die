package main

import "math"

func (w *World) advancePlayerPaths(tick uint64) {
	for _, player := range w.players {
		w.followPlayerPath(player, tick)
	}
}

func (w *World) followPlayerPath(player *playerState, tick uint64) {
	if player == nil {
		return
	}
	ps := &player.path
	if len(ps.Path) == 0 {
		if (ps.PathTarget.X != 0 || ps.PathTarget.Y != 0) && tick >= ps.PathRecalcTick {
			if w.recalculatePlayerPath(player, tick) {
				w.followPlayerPath(player, tick)
			}
		}
		return
	}
	if ps.PathIndex >= len(ps.Path) {
		w.finishPlayerPath(player)
		return
	}

	threshold := pathNodeReachedEpsilon
	if threshold <= 0 {
		threshold = playerHalf
	}

	for ps.PathIndex < len(ps.Path) {
		node := ps.Path[ps.PathIndex]
		dx := node.X - player.X
		dy := node.Y - player.Y
		dist := math.Hypot(dx, dy)

		limit := threshold
		if ps.PathIndex == len(ps.Path)-1 {
			limit = playerHalf
		}
		if dist <= limit {
			ps.PathIndex++
			ps.PathLastDistance = 0
			ps.PathStallTicks = 0
			continue
		}

		if ps.PathLastDistance == 0 || dist+0.1 < ps.PathLastDistance {
			ps.PathLastDistance = dist
			ps.PathStallTicks = 0
		} else {
			ps.PathStallTicks++
			if ps.PathStallTicks >= pathStallThresholdTicks {
				if tick >= ps.PathRecalcTick && w.recalculatePlayerPath(player, tick) {
					w.followPlayerPath(player, tick)
				}
				return
			}
		}

		if ps.PathLastDistance > 0 && dist > ps.PathLastDistance+pathPushRecalcThreshold {
			if tick >= ps.PathRecalcTick && w.recalculatePlayerPath(player, tick) {
				w.followPlayerPath(player, tick)
			}
			return
		}

		player.intentX = dx
		player.intentY = dy
		player.Facing = deriveFacing(dx, dy, player.Facing)
		return
	}

	w.finishPlayerPath(player)
}

func (w *World) ensurePlayerPath(player *playerState, target vec2, tick uint64) bool {
	if player == nil {
		return false
	}
	ps := &player.path
	ps.PathTarget = target
	path, goal, ok := w.computePlayerPath(player, target)
	if !ok {
		w.clearPlayerPath(player)
		ps.PathTarget = target
		ps.PathRecalcTick = tick + pathRecalcCooldownTicks
		player.intentX = 0
		player.intentY = 0
		return false
	}
	ps.Path = path
	ps.PathIndex = 0
	ps.PathGoal = goal
	ps.PathLastDistance = 0
	ps.PathStallTicks = 0
	ps.PathRecalcTick = tick + 1
	return true
}

func (w *World) recalculatePlayerPath(player *playerState, tick uint64) bool {
	if player == nil {
		return false
	}
	target := player.path.PathTarget
	if target.X == 0 && target.Y == 0 {
		return false
	}
	path, goal, ok := w.computePlayerPath(player, target)
	if !ok {
		w.clearPlayerPath(player)
		player.path.PathTarget = target
		player.path.PathRecalcTick = tick + pathRecalcCooldownTicks
		player.intentX = 0
		player.intentY = 0
		return false
	}
	player.path.Path = path
	player.path.PathIndex = 0
	player.path.PathGoal = goal
	player.path.PathLastDistance = 0
	player.path.PathStallTicks = 0
	player.path.PathRecalcTick = tick + 1
	return true
}

func (w *World) finishPlayerPath(player *playerState) {
	if player == nil {
		return
	}
	w.clearPlayerPath(player)
	player.intentX = 0
	player.intentY = 0
}

func (w *World) clearPlayerPath(player *playerState) {
	if player == nil {
		return
	}
	player.path.Path = nil
	player.path.PathIndex = 0
	player.path.PathGoal = vec2{}
	player.path.PathTarget = vec2{}
	player.path.PathLastDistance = 0
	player.path.PathStallTicks = 0
	player.path.PathRecalcTick = 0
}

func (w *World) computePlayerPath(player *playerState, target vec2) ([]vec2, vec2, bool) {
	if player == nil {
		return nil, vec2{}, false
	}
	return w.computePathFrom(player.X, player.Y, target, player.ID)
}
