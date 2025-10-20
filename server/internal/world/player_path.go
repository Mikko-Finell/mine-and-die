package world

import "math"

// DefaultPlayerArriveRadius mirrors the legacy arrival radius used when players
// reach the end of their navigation path.
const DefaultPlayerArriveRadius = 12.0

// PlayerPathState tracks the legacy path-following metadata for a single
// player. Callers should treat the fields as authoritative while migrating the
// simulation package.
type PlayerPathState struct {
	Path             []Vec2
	PathIndex        int
	PathTarget       Vec2
	PathGoal         Vec2
	PathLastDistance float64
	PathStallTicks   int
	PathRecalcTick   uint64
	ArriveRadius     float64
}

// PlayerPathActor exposes the minimal legacy state required to follow a path.
type PlayerPathActor struct {
	ID     string
	X      float64
	Y      float64
	Facing string
	Path   *PlayerPathState
}

// PlayerPathController surfaces the legacy world hooks required to mutate
// intent, facing, and path state during the transition.
type PlayerPathController interface {
	SetIntent(actorID string, dx, dy float64)
	SetFacing(actorID string, facing string)
	DeriveFacing(dx, dy float64, fallback string) string
	Dimensions() (float64, float64)
	ComputePlayerPath(actorID string, target Vec2) ([]Vec2, Vec2, bool)
}

// AdvancePlayerPaths walks each actor and advances their navigation state.
func AdvancePlayerPaths(players []*PlayerPathActor, tick uint64, controller PlayerPathController) {
	if len(players) == 0 {
		return
	}
	for _, actor := range players {
		FollowPlayerPath(actor, tick, controller)
	}
}

// FollowPlayerPath mirrors the legacy world logic for marching a player along a
// precomputed navigation path, performing stall detection and path
// recalculation when required.
func FollowPlayerPath(actor *PlayerPathActor, tick uint64, controller PlayerPathController) {
	if actor == nil || actor.Path == nil {
		return
	}

	path := actor.Path
	if len(path.Path) == 0 {
		if path.PathTarget.X == 0 && path.PathTarget.Y == 0 {
			return
		}
		if controller != nil {
			controller.SetIntent(actor.ID, 0, 0)
		}
		if tick >= path.PathRecalcTick {
			if RecalculatePlayerPath(actor, tick, controller) {
				FollowPlayerPath(actor, tick, controller)
			}
		}
		return
	}

	if path.PathIndex >= len(path.Path) {
		FinishPlayerPath(actor, controller)
		return
	}

	threshold := PathNodeReachedEpsilon
	if threshold <= 0 {
		threshold = PlayerHalf
	}

	for path.PathIndex < len(path.Path) {
		node := path.Path[path.PathIndex]
		dx := node.X - actor.X
		dy := node.Y - actor.Y
		dist := math.Hypot(dx, dy)

		limit := threshold
		if path.PathIndex == len(path.Path)-1 {
			radius := path.ArriveRadius
			if radius <= 0 {
				radius = DefaultPlayerArriveRadius
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
			if path.PathStallTicks >= PathStallThresholdTicks {
				if tick >= path.PathRecalcTick && RecalculatePlayerPath(actor, tick, controller) {
					FollowPlayerPath(actor, tick, controller)
				}
				return
			}
		}

		if path.PathLastDistance > 0 && dist > path.PathLastDistance+PathPushRecalcThreshold {
			if tick >= path.PathRecalcTick && RecalculatePlayerPath(actor, tick, controller) {
				FollowPlayerPath(actor, tick, controller)
			}
			return
		}

		moveDX := dx
		moveDY := dy
		if dist > 1 {
			moveDX = dx / dist
			moveDY = dy / dist
		}

		if controller != nil {
			controller.SetIntent(actor.ID, moveDX, moveDY)
			facing := controller.DeriveFacing(moveDX, moveDY, actor.Facing)
			controller.SetFacing(actor.ID, facing)
			actor.Facing = facing
		}
		return
	}

	FinishPlayerPath(actor, controller)
}

// FinishPlayerPath stops a player at their destination and clears the active
// navigation path while preserving the configured arrival radius.
func FinishPlayerPath(actor *PlayerPathActor, controller PlayerPathController) {
	if actor == nil || actor.Path == nil {
		return
	}

	radius := actor.Path.ArriveRadius
	if controller != nil {
		controller.SetIntent(actor.ID, 0, 0)
	}
	*actor.Path = PlayerPathState{ArriveRadius: radius}
}

// ClearPlayerPath clears any outstanding navigation instructions without
// touching player intent.
func ClearPlayerPath(actor *PlayerPathActor) {
	if actor == nil || actor.Path == nil {
		return
	}
	radius := actor.Path.ArriveRadius
	*actor.Path = PlayerPathState{ArriveRadius: radius}
}

// EnsurePlayerPath computes and installs a navigation path to the requested
// target, clamping to the playable area and recording recalc metadata when the
// request fails.
func EnsurePlayerPath(actor *PlayerPathActor, target Vec2, tick uint64, controller PlayerPathController) bool {
	if actor == nil || actor.Path == nil || controller == nil {
		return false
	}

	width, height := controller.Dimensions()
	actor.Path.PathTarget = Vec2{
		X: Clamp(target.X, PlayerHalf, width-PlayerHalf),
		Y: Clamp(target.Y, PlayerHalf, height-PlayerHalf),
	}

	path, goal, ok := controller.ComputePlayerPath(actor.ID, actor.Path.PathTarget)
	if !ok {
		radius := actor.Path.ArriveRadius
		*actor.Path = PlayerPathState{
			ArriveRadius:   radius,
			PathTarget:     actor.Path.PathTarget,
			PathRecalcTick: tick + PathRecalcCooldownTicks,
		}
		controller.SetIntent(actor.ID, 0, 0)
		return false
	}

	radius := actor.Path.ArriveRadius
	if radius <= 0 {
		radius = DefaultPlayerArriveRadius
	}
	actor.Path.Path = path
	actor.Path.PathIndex = 0
	actor.Path.PathGoal = goal
	actor.Path.PathLastDistance = 0
	actor.Path.PathStallTicks = 0
	actor.Path.PathRecalcTick = tick + 1
	actor.Path.ArriveRadius = radius
	return true
}

// RecalculatePlayerPath recomputes the current navigation path, preserving the
// target while handling failure cooldowns.
func RecalculatePlayerPath(actor *PlayerPathActor, tick uint64, controller PlayerPathController) bool {
	if actor == nil || actor.Path == nil || controller == nil {
		return false
	}

	target := actor.Path.PathTarget
	if target.X == 0 && target.Y == 0 {
		return false
	}

	path, goal, ok := controller.ComputePlayerPath(actor.ID, target)
	if !ok {
		radius := actor.Path.ArriveRadius
		actor.Path.Path = nil
		actor.Path.PathIndex = 0
		actor.Path.PathGoal = Vec2{}
		actor.Path.PathLastDistance = 0
		actor.Path.PathStallTicks = 0
		actor.Path.PathRecalcTick = tick + PathRecalcCooldownTicks
		actor.Path.ArriveRadius = radius
		controller.SetIntent(actor.ID, 0, 0)
		return false
	}

	actor.Path.Path = path
	actor.Path.PathIndex = 0
	actor.Path.PathGoal = goal
	actor.Path.PathLastDistance = 0
	actor.Path.PathStallTicks = 0
	actor.Path.PathRecalcTick = tick + 1
	return true
}
