package world

import (
	"math"

       state "mine-and-die/server/internal/world/state"
)

// DefaultNPCArriveRadius mirrors the legacy arrival radius fallback for NPC
// navigation.
const DefaultNPCArriveRadius = 12.0

// NPCPathState aliases the shared NPC path state for legacy helpers.
type NPCPathState = state.NPCPathState

// NPCPathActor exposes the minimal legacy state required to follow an NPC path.
type NPCPathActor struct {
	ID           string
	X            float64
	Y            float64
	Facing       string
	Path         *NPCPathState
	StuckCounter uint8
}

// NPCPathController surfaces the hooks required to mutate intent and facing and
// to compute navigation paths for NPCs.
type NPCPathController interface {
	SetIntent(actorID string, dx, dy float64)
	SetFacing(actorID string, facing string)
	DeriveFacing(dx, dy float64, fallback string) string
	ComputeNPCPath(actorID string, target Vec2) ([]Vec2, Vec2, bool)
}

// AdvanceNPCPaths walks each actor and advances their navigation state.
func AdvanceNPCPaths(actors []*NPCPathActor, tick uint64, controller NPCPathController) {
	if len(actors) == 0 {
		return
	}
	for _, actor := range actors {
		FollowNPCPath(actor, tick, controller)
	}
}

// FollowNPCPath mirrors the legacy NPC path-following logic, performing stall
// detection and path recalculation when required.
func FollowNPCPath(actor *NPCPathActor, tick uint64, controller NPCPathController) {
	if actor == nil || actor.Path == nil {
		return
	}

	path := actor.Path
	if len(path.Path) == 0 {
		if controller != nil {
			controller.SetIntent(actor.ID, 0, 0)
		}
		if (path.PathTarget.X != 0 || path.PathTarget.Y != 0) && tick >= path.PathRecalcTick {
			if RecalculateNPCPath(actor, tick, controller) {
				FollowNPCPath(actor, tick, controller)
			}
		}
		return
	}

	if path.PathIndex >= len(path.Path) {
		FinishNPCPath(actor, controller)
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
				radius = DefaultNPCArriveRadius
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
			if int(path.PathStallTicks) >= PathStallThresholdTicks || actor.StuckCounter >= 8 {
				if tick >= path.PathRecalcTick && RecalculateNPCPath(actor, tick, controller) {
					FollowNPCPath(actor, tick, controller)
				}
				return
			}
		}

		if path.PathLastDistance > 0 && dist > path.PathLastDistance+PathPushRecalcThreshold {
			if tick >= path.PathRecalcTick && RecalculateNPCPath(actor, tick, controller) {
				FollowNPCPath(actor, tick, controller)
			}
			return
		}

		if controller != nil {
			controller.SetIntent(actor.ID, dx, dy)
			facing := controller.DeriveFacing(dx, dy, actor.Facing)
			controller.SetFacing(actor.ID, facing)
			actor.Facing = facing
		}
		return
	}

	FinishNPCPath(actor, controller)
}

// FinishNPCPath stops an NPC at their destination and clears the active
// navigation path.
func FinishNPCPath(actor *NPCPathActor, controller NPCPathController) {
	if actor == nil || actor.Path == nil {
		return
	}

	actor.Path.Path = nil
	actor.Path.PathIndex = 0
	actor.Path.PathGoal = Vec2{}
	actor.Path.PathTarget = Vec2{}
	actor.Path.PathLastDistance = 0
	actor.Path.PathStallTicks = 0
	actor.Path.PathRecalcTick = 0
	if controller != nil {
		controller.SetIntent(actor.ID, 0, 0)
	}
}

// ClearNPCPath clears any outstanding navigation instructions without touching
// NPC-specific metadata beyond intent and recalc bookkeeping.
func ClearNPCPath(actor *NPCPathActor, controller NPCPathController) {
	if actor == nil || actor.Path == nil {
		return
	}

	actor.Path.Path = nil
	actor.Path.PathIndex = 0
	actor.Path.PathGoal = Vec2{}
	actor.Path.PathTarget = Vec2{}
	actor.Path.PathLastDistance = 0
	actor.Path.PathStallTicks = 0
	actor.Path.PathRecalcTick = 0
	if controller != nil {
		controller.SetIntent(actor.ID, 0, 0)
	}
}

// EnsureNPCPath computes and installs a navigation path to the requested
// target, recording recalc metadata when the request fails.
func EnsureNPCPath(actor *NPCPathActor, target Vec2, tick uint64, controller NPCPathController) bool {
	if actor == nil || actor.Path == nil {
		return false
	}

	actor.Path.PathTarget = target
	if len(actor.Path.Path) > 0 && actor.Path.PathIndex < len(actor.Path.Path) {
		goal := actor.Path.PathGoal
		if math.Hypot(goal.X-target.X, goal.Y-target.Y) <= NavCellSize*0.5 {
			return true
		}
	}

	if controller == nil {
		actor.Path.Path = nil
		actor.Path.PathIndex = 0
		actor.Path.PathGoal = Vec2{}
		actor.Path.PathLastDistance = 0
		actor.Path.PathStallTicks = 0
		actor.Path.PathRecalcTick = tick + PathRecalcCooldownTicks
		return false
	}

	path, goal, ok := controller.ComputeNPCPath(actor.ID, target)
	if !ok {
		actor.Path.Path = nil
		actor.Path.PathIndex = 0
		actor.Path.PathGoal = Vec2{}
		actor.Path.PathLastDistance = 0
		actor.Path.PathStallTicks = 0
		actor.Path.PathRecalcTick = tick + PathRecalcCooldownTicks
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

// RecalculateNPCPath recomputes the current navigation path while handling
// failure cooldowns.
func RecalculateNPCPath(actor *NPCPathActor, tick uint64, controller NPCPathController) bool {
	if actor == nil || actor.Path == nil || controller == nil {
		return false
	}

	target := actor.Path.PathTarget
	if target.X == 0 && target.Y == 0 {
		return false
	}

	path, goal, ok := controller.ComputeNPCPath(actor.ID, target)
	if !ok {
		actor.Path.Path = nil
		actor.Path.PathIndex = 0
		actor.Path.PathGoal = Vec2{}
		actor.Path.PathLastDistance = 0
		actor.Path.PathStallTicks = 0
		actor.Path.PathRecalcTick = tick + PathRecalcCooldownTicks
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
