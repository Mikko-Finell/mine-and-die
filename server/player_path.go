package main

import "math"

const defaultPlayerArriveRadius = 12.0

func (w *World) advancePlayerPaths(tick uint64) {
	for _, player := range w.players {
		w.followPlayerPath(player, tick)
	}
}

func (w *World) followPlayerPath(player *playerState, tick uint64) {
	if player == nil {
		return
	}
	path := &player.path
	if len(path.Path) == 0 {
		if path.PathTarget.X == 0 && path.PathTarget.Y == 0 {
			return
		}
		player.intentX = 0
		player.intentY = 0
		if tick >= path.PathRecalcTick {
			if w.recalculatePlayerPath(player, tick) {
				w.followPlayerPath(player, tick)
			}
		}
		return
	}
	if path.PathIndex >= len(path.Path) {
		w.finishPlayerPath(player)
		return
	}

	threshold := pathNodeReachedEpsilon
	if threshold <= 0 {
		threshold = playerHalf
	}

	for path.PathIndex < len(path.Path) {
		node := path.Path[path.PathIndex]
		dx := node.X - player.X
		dy := node.Y - player.Y
		dist := math.Hypot(dx, dy)

		limit := threshold
		if path.PathIndex == len(path.Path)-1 {
			radius := path.ArriveRadius
			if radius <= 0 {
				radius = defaultPlayerArriveRadius
			}
			limit = radius
		}
		if dist <= limit {
			path.PathIndex++
			path.PathLastDistance = 0
			path.PathStallTicks = 0
			continue
		}

		if path.PathLastDistance == 0 || dist+0.1 < path.PathLastDistance {
			path.PathLastDistance = dist
			path.PathStallTicks = 0
		} else {
			path.PathStallTicks++
			if path.PathStallTicks >= pathStallThresholdTicks {
				if tick >= path.PathRecalcTick && w.recalculatePlayerPath(player, tick) {
					w.followPlayerPath(player, tick)
				}
				return
			}
		}

		if path.PathLastDistance > 0 && dist > path.PathLastDistance+pathPushRecalcThreshold {
			if tick >= path.PathRecalcTick && w.recalculatePlayerPath(player, tick) {
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

func (w *World) finishPlayerPath(player *playerState) {
	if player == nil {
		return
	}
	radius := player.path.ArriveRadius
	player.intentX = 0
	player.intentY = 0
	player.path = playerPathState{ArriveRadius: radius}
}

func (w *World) clearPlayerPath(player *playerState) {
	if player == nil {
		return
	}
	radius := player.path.ArriveRadius
	player.path = playerPathState{ArriveRadius: radius}
}

func (w *World) ensurePlayerPath(player *playerState, target vec2, tick uint64) bool {
	if player == nil {
		return false
	}
	player.path.PathTarget = vec2{
		X: clamp(target.X, playerHalf, worldWidth-playerHalf),
		Y: clamp(target.Y, playerHalf, worldHeight-playerHalf),
	}
	path, goal, ok := w.computePlayerPath(player, player.path.PathTarget)
	if !ok {
		radius := player.path.ArriveRadius
		player.path = playerPathState{ArriveRadius: radius, PathTarget: player.path.PathTarget, PathRecalcTick: tick + pathRecalcCooldownTicks}
		player.intentX = 0
		player.intentY = 0
		return false
	}
	radius := player.path.ArriveRadius
	if radius <= 0 {
		radius = defaultPlayerArriveRadius
	}
	player.path.Path = path
	player.path.PathIndex = 0
	player.path.PathGoal = goal
	player.path.PathLastDistance = 0
	player.path.PathStallTicks = 0
	player.path.PathRecalcTick = tick + 1
	player.path.ArriveRadius = radius
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
		radius := player.path.ArriveRadius
		player.path.Path = nil
		player.path.PathIndex = 0
		player.path.PathGoal = vec2{}
		player.path.PathLastDistance = 0
		player.path.PathStallTicks = 0
		player.path.PathRecalcTick = tick + pathRecalcCooldownTicks
		player.path.ArriveRadius = radius
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

func (w *World) computePlayerPath(player *playerState, target vec2) ([]vec2, vec2, bool) {
	if player == nil {
		return nil, vec2{}, false
	}
	return w.computePathFrom(player.X, player.Y, player.ID, target)
}
