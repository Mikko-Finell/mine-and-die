package world

import "math"

// PositionEpsilon matches the legacy tolerance used when comparing actor
// coordinates. Values within this epsilon are considered unchanged.
const PositionEpsilon = 1e-6

// PositionCommit captures the minimal metadata required to commit an actor's
// resolved position for the current tick.
type PositionCommit struct {
	ID      string
	Current Vec2
}

// ApplyPlayerPositionMutations mirrors the legacy logic for committing player
// movement through the world write barrier. Callers should pass the per-player
// starting and resolved positions for the current tick alongside the commit
// callback that records authoritative position updates.
func ApplyPlayerPositionMutations(players []PositionCommit, initial, proposed map[string]Vec2, commit func(string, float64, float64)) {
	applyPositionMutations(players, initial, proposed, commit)
}

// ApplyNPCPositionMutations mirrors the legacy logic for committing NPC
// movement through the world write barrier.
func ApplyNPCPositionMutations(npcs []PositionCommit, initial, proposed map[string]Vec2, commit func(string, float64, float64)) {
	applyPositionMutations(npcs, initial, proposed, commit)
}

func applyPositionMutations(actors []PositionCommit, initial, proposed map[string]Vec2, commit func(string, float64, float64)) {
	if len(actors) == 0 || commit == nil {
		return
	}
	for _, actor := range actors {
		if actor.ID == "" {
			continue
		}
		start := actor.Current
		if pos, ok := initial[actor.ID]; ok {
			start = pos
		}
		target := actor.Current
		if pos, ok := proposed[actor.ID]; ok {
			target = pos
		}
		if PositionsEqual(start.X, start.Y, target.X, target.Y) {
			continue
		}
		commit(actor.ID, target.X, target.Y)
	}
}

// PositionsEqual reports whether two coordinate pairs are effectively the same
// using the legacy epsilon tolerance.
func PositionsEqual(ax, ay, bx, by float64) bool {
	return math.Abs(ax-bx) <= PositionEpsilon && math.Abs(ay-by) <= PositionEpsilon
}
