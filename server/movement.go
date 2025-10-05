package main

import "math"

// movePlayerWithObstacles advances a player while clamping speed, bounds, and walls.
func movePlayerWithObstacles(state *playerState, dt float64, obstacles []Obstacle) {
	dx := state.intentX
	dy := state.intentY
	length := math.Hypot(dx, dy)
	if length != 0 {
		dx /= length
		dy /= length
	}

	deltaX := dx * moveSpeed * dt
	deltaY := dy * moveSpeed * dt

	newX := clamp(state.X+deltaX, playerHalf, worldWidth-playerHalf)
	if deltaX != 0 {
		newX = resolveAxisMoveX(state.X, state.Y, newX, deltaX, obstacles)
	}

	newY := clamp(state.Y+deltaY, playerHalf, worldHeight-playerHalf)
	if deltaY != 0 {
		newY = resolveAxisMoveY(newX, state.Y, newY, deltaY, obstacles)
	}

	state.X = newX
	state.Y = newY

	resolveObstaclePenetration(state, obstacles)
}

// resolveAxisMoveX applies horizontal movement while stopping at obstacle edges.
func resolveAxisMoveX(oldX, oldY, proposedX, deltaX float64, obstacles []Obstacle) float64 {
	newX := proposedX
	for _, obs := range obstacles {
		if obs.Type == obstacleTypeLava {
			continue
		}
		minY := obs.Y - playerHalf
		maxY := obs.Y + obs.Height + playerHalf
		if oldY < minY || oldY > maxY {
			continue
		}

		if deltaX > 0 {
			boundary := obs.X - playerHalf
			if oldX <= boundary && newX > boundary {
				newX = boundary
			}
		} else if deltaX < 0 {
			boundary := obs.X + obs.Width + playerHalf
			if oldX >= boundary && newX < boundary {
				newX = boundary
			}
		}
	}
	return clamp(newX, playerHalf, worldWidth-playerHalf)
}

// resolveAxisMoveY applies vertical movement while stopping at obstacle edges.
func resolveAxisMoveY(oldX, oldY, proposedY, deltaY float64, obstacles []Obstacle) float64 {
	newY := proposedY
	for _, obs := range obstacles {
		if obs.Type == obstacleTypeLava {
			continue
		}
		minX := obs.X - playerHalf
		maxX := obs.X + obs.Width + playerHalf
		if oldX < minX || oldX > maxX {
			continue
		}

		if deltaY > 0 {
			boundary := obs.Y - playerHalf
			if oldY <= boundary && newY > boundary {
				newY = boundary
			}
		} else if deltaY < 0 {
			boundary := obs.Y + obs.Height + playerHalf
			if oldY >= boundary && newY < boundary {
				newY = boundary
			}
		}
	}
	return clamp(newY, playerHalf, worldHeight-playerHalf)
}

// resolveObstaclePenetration nudges a player out of overlapping obstacles.
func resolveObstaclePenetration(state *playerState, obstacles []Obstacle) {
	for _, obs := range obstacles {
		if obs.Type == obstacleTypeLava {
			continue
		}
		if !circleRectOverlap(state.X, state.Y, playerHalf, obs) {
			continue
		}

		closestX := clamp(state.X, obs.X, obs.X+obs.Width)
		closestY := clamp(state.Y, obs.Y, obs.Y+obs.Height)
		dx := state.X - closestX
		dy := state.Y - closestY
		distSq := dx*dx + dy*dy

		if distSq == 0 {
			left := math.Abs(state.X - obs.X)
			right := math.Abs((obs.X + obs.Width) - state.X)
			top := math.Abs(state.Y - obs.Y)
			bottom := math.Abs((obs.Y + obs.Height) - state.Y)

			minDist := left
			direction := 0
			if right < minDist {
				minDist = right
				direction = 1
			}
			if top < minDist {
				minDist = top
				direction = 2
			}
			if bottom < minDist {
				direction = 3
			}

			switch direction {
			case 0:
				state.X = obs.X - playerHalf
			case 1:
				state.X = obs.X + obs.Width + playerHalf
			case 2:
				state.Y = obs.Y - playerHalf
			case 3:
				state.Y = obs.Y + obs.Height + playerHalf
			}
		} else {
			dist := math.Sqrt(distSq)
			if dist < playerHalf {
				overlap := playerHalf - dist
				nx := dx / dist
				ny := dy / dist
				state.X += nx * overlap
				state.Y += ny * overlap
			}
		}

		state.X = clamp(state.X, playerHalf, worldWidth-playerHalf)
		state.Y = clamp(state.Y, playerHalf, worldHeight-playerHalf)
	}
}

// resolvePlayerCollisions separates overlapping players while respecting walls.
func resolvePlayerCollisions(players []*playerState, obstacles []Obstacle) {
	if len(players) < 2 {
		return
	}

	const iterations = 4
	for iter := 0; iter < iterations; iter++ {
		adjusted := false
		for i := 0; i < len(players); i++ {
			for j := i + 1; j < len(players); j++ {
				p1 := players[i]
				p2 := players[j]
				dx := p2.X - p1.X
				dy := p2.Y - p1.Y
				distSq := dx*dx + dy*dy
				minDist := playerHalf * 2

				var dist float64
				if distSq == 0 {
					dx = 1
					dy = 0
					dist = 1
				} else {
					dist = math.Sqrt(distSq)
				}

				if dist >= minDist {
					continue
				}

				overlap := (minDist - dist) / 2
				nx := dx / dist
				ny := dy / dist

				p1.X -= nx * overlap
				p1.Y -= ny * overlap
				p2.X += nx * overlap
				p2.Y += ny * overlap

				p1.X = clamp(p1.X, playerHalf, worldWidth-playerHalf)
				p1.Y = clamp(p1.Y, playerHalf, worldHeight-playerHalf)
				p2.X = clamp(p2.X, playerHalf, worldWidth-playerHalf)
				p2.Y = clamp(p2.Y, playerHalf, worldHeight-playerHalf)

				resolveObstaclePenetration(p1, obstacles)
				resolveObstaclePenetration(p2, obstacles)

				adjusted = true
			}
		}

		if !adjusted {
			break
		}
	}
}
