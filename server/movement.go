package server

import (
	"math"

	worldpkg "mine-and-die/server/internal/world"
)

// moveActorWithObstacles advances an actor while clamping speed, bounds, and walls.
func moveActorWithObstacles(state *actorState, dt float64, obstacles []Obstacle, width, height float64) {
	if state == nil {
		return
	}

	movement := worldpkg.MovementActor{
		X:       state.X,
		Y:       state.Y,
		IntentX: state.intentX,
		IntentY: state.intentY,
	}
	worldpkg.MoveActorWithObstacles(&movement, dt, obstacles, width, height, moveSpeed)
	state.X = movement.X
	state.Y = movement.Y
}

// resolveObstaclePenetration nudges an actor out of overlapping obstacles.
func resolveObstaclePenetration(state *actorState, obstacles []Obstacle, width, height float64) {
	if state == nil {
		return
	}

	movement := worldpkg.MovementActor{X: state.X, Y: state.Y}
	worldpkg.ResolveObstaclePenetration(&movement, obstacles, width, height)
	state.X = movement.X
	state.Y = movement.Y
}

// resolveActorCollisions separates overlapping actors while respecting walls.
func resolveActorCollisions(actors []*actorState, obstacles []Obstacle, width, height float64) {
	if len(actors) < 2 {
		return
	}

	const iterations = 4
	for iter := 0; iter < iterations; iter++ {
		adjusted := false
		for i := 0; i < len(actors); i++ {
			for j := i + 1; j < len(actors); j++ {
				p1 := actors[i]
				p2 := actors[j]
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

				p1.X = clamp(p1.X, playerHalf, width-playerHalf)
				p1.Y = clamp(p1.Y, playerHalf, height-playerHalf)
				p2.X = clamp(p2.X, playerHalf, width-playerHalf)
				p2.Y = clamp(p2.Y, playerHalf, height-playerHalf)

				resolveObstaclePenetration(p1, obstacles, width, height)
				resolveObstaclePenetration(p2, obstacles, width, height)

				adjusted = true
			}
		}

		if !adjusted {
			break
		}
	}
}
