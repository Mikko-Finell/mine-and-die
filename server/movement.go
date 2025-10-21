package server

import worldpkg "mine-and-die/server/internal/world"

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

	states := make([]worldpkg.MovementActor, len(actors))
	pointers := make([]*worldpkg.MovementActor, len(actors))
	for i, actor := range actors {
		if actor == nil {
			continue
		}
		states[i] = worldpkg.MovementActor{X: actor.X, Y: actor.Y}
		pointers[i] = &states[i]
	}

	worldpkg.ResolveActorCollisions(pointers, obstacles, width, height)

	for i, actor := range actors {
		if actor == nil {
			continue
		}
		actor.X = states[i].X
		actor.Y = states[i].Y
	}
}
