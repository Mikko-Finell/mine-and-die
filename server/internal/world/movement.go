package world

import "math"

// MovementActor captures the minimal mutable state required to move an actor
// while resolving obstacle collisions.
type MovementActor struct {
	X       float64
	Y       float64
	IntentX float64
	IntentY float64
}

// MoveActorWithObstacles advances an actor while clamping speed, bounds, and
// blocking obstacles. Callers should pass the desired movement speed in units
// per second.
func MoveActorWithObstacles(state *MovementActor, dt float64, obstacles []Obstacle, width, height, speed float64) {
	if state == nil {
		return
	}

	dx := state.IntentX
	dy := state.IntentY
	length := math.Hypot(dx, dy)
	if length != 0 {
		dx /= length
		dy /= length
	}

	deltaX := dx * speed * dt
	deltaY := dy * speed * dt

	newX := Clamp(state.X+deltaX, PlayerHalf, width-PlayerHalf)
	if deltaX != 0 {
		newX = resolveAxisMoveX(state.X, state.Y, newX, deltaX, obstacles, width)
	}

	newY := Clamp(state.Y+deltaY, PlayerHalf, height-PlayerHalf)
	if deltaY != 0 {
		newY = resolveAxisMoveY(newX, state.Y, newY, deltaY, obstacles, height)
	}

	state.X = newX
	state.Y = newY

	ResolveObstaclePenetration(state, obstacles, width, height)
}

// resolveAxisMoveX applies horizontal movement while stopping at obstacle edges.
func resolveAxisMoveX(oldX, oldY, proposedX, deltaX float64, obstacles []Obstacle, width float64) float64 {
	newX := proposedX
	for _, obs := range obstacles {
		if obs.Type == ObstacleTypeLava {
			continue
		}
		minY := obs.Y - PlayerHalf
		maxY := obs.Y + obs.Height + PlayerHalf
		if oldY < minY || oldY > maxY {
			continue
		}

		if deltaX > 0 {
			boundary := obs.X - PlayerHalf
			if oldX <= boundary && newX > boundary {
				newX = boundary
			}
		} else if deltaX < 0 {
			boundary := obs.X + obs.Width + PlayerHalf
			if oldX >= boundary && newX < boundary {
				newX = boundary
			}
		}
	}
	return Clamp(newX, PlayerHalf, width-PlayerHalf)
}

// resolveAxisMoveY applies vertical movement while stopping at obstacle edges.
func resolveAxisMoveY(oldX, oldY, proposedY, deltaY float64, obstacles []Obstacle, height float64) float64 {
	newY := proposedY
	for _, obs := range obstacles {
		if obs.Type == ObstacleTypeLava {
			continue
		}
		minX := obs.X - PlayerHalf
		maxX := obs.X + obs.Width + PlayerHalf
		if oldX < minX || oldX > maxX {
			continue
		}

		if deltaY > 0 {
			boundary := obs.Y - PlayerHalf
			if oldY <= boundary && newY > boundary {
				newY = boundary
			}
		} else if deltaY < 0 {
			boundary := obs.Y + obs.Height + PlayerHalf
			if oldY >= boundary && newY < boundary {
				newY = boundary
			}
		}
	}
	return Clamp(newY, PlayerHalf, height-PlayerHalf)
}

// ResolveObstaclePenetration nudges an actor out of overlapping obstacles.
func ResolveObstaclePenetration(state *MovementActor, obstacles []Obstacle, width, height float64) {
	if state == nil {
		return
	}

	for _, obs := range obstacles {
		if obs.Type == ObstacleTypeLava {
			continue
		}
		if !CircleRectOverlap(state.X, state.Y, PlayerHalf, obs) {
			continue
		}

		closestX := Clamp(state.X, obs.X, obs.X+obs.Width)
		closestY := Clamp(state.Y, obs.Y, obs.Y+obs.Height)
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
				state.X = obs.X - PlayerHalf
			case 1:
				state.X = obs.X + obs.Width + PlayerHalf
			case 2:
				state.Y = obs.Y - PlayerHalf
			case 3:
				state.Y = obs.Y + obs.Height + PlayerHalf
			}
		} else {
			dist := math.Sqrt(distSq)
			if dist < PlayerHalf {
				overlap := PlayerHalf - dist
				nx := dx / dist
				ny := dy / dist
				state.X += nx * overlap
				state.Y += ny * overlap
			}
		}

		state.X = Clamp(state.X, PlayerHalf, width-PlayerHalf)
		state.Y = Clamp(state.Y, PlayerHalf, height-PlayerHalf)
	}
}
