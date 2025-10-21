package world

import "math"

// PositionActor captures the identifier and current coordinates for an actor
// whose movement mutations may be committed after collision resolution.
type PositionActor struct {
	ID      string
	Current Vec2
}

// PositionEpsilon defines the tolerance used when comparing actor positions.
const PositionEpsilon = 1e-6

// PositionsEqual reports whether two coordinate pairs are effectively the same
// within the configured epsilon tolerance.
func PositionsEqual(ax, ay, bx, by float64) bool {
	return math.Abs(ax-bx) <= PositionEpsilon && math.Abs(ay-by) <= PositionEpsilon
}

// ApplyPlayerPositionMutations commits resolved player positions through the
// provided callback so callers can route writes through their legacy adapters.
func ApplyPlayerPositionMutations(initial map[string]Vec2, proposed map[string]Vec2, actors []PositionActor, commit func(string, Vec2)) {
	applyPositionMutations(initial, proposed, actors, commit)
}

// ApplyNPCPositionMutations commits resolved NPC positions through the
// provided callback so callers can route writes through their legacy adapters.
func ApplyNPCPositionMutations(initial map[string]Vec2, proposed map[string]Vec2, actors []PositionActor, commit func(string, Vec2)) {
	applyPositionMutations(initial, proposed, actors, commit)
}

func applyPositionMutations(initial map[string]Vec2, proposed map[string]Vec2, actors []PositionActor, commit func(string, Vec2)) {
	if len(actors) == 0 || commit == nil {
		return
	}

	for _, actor := range actors {
		if actor.ID == "" {
			continue
		}

		start := actor.Current
		if initial != nil {
			if candidate, ok := initial[actor.ID]; ok {
				start = candidate
			}
		}

		target := actor.Current
		if proposed != nil {
			if candidate, ok := proposed[actor.ID]; ok {
				target = candidate
			}
		}

		if PositionsEqual(start.X, start.Y, target.X, target.Y) {
			continue
		}

		commit(actor.ID, target)
	}
}
