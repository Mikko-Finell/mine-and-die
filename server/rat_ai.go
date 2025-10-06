package main

import (
	"math"
	"math/rand"
	"time"
)

const (
	ratWanderSpeed       = 0.45
	ratFleeSpeed         = 1.0
	ratFleeRadius        = 140.0
	ratFleeDurationTicks = 60
	ratArriveRadius      = 12.0
	ratWanderRadius      = 200.0
	ratFleeDecisionDelay = 5
	ratIdleDecisionDelay = 15
	ratWanderDecisionMin = 20
	ratWanderDecisionMax = 60
)

func (w *World) runRatBehavior(npc *npcState, tick uint64, now time.Time) []Command {
	if w == nil || npc == nil {
		return nil
	}

	w.ensureRatDefaults(npc)

	commands := make([]Command, 0, 1)
	decisionDelay := uint64(ratIdleDecisionDelay)

	fleeVector, fleeing := w.closestRatThreat(npc)
	if fleeing {
		npc.fleeVector = fleeVector
		npc.fleeUntilTick = tick + ratFleeDurationTicks
	} else if npc.fleeUntilTick > tick && (npc.fleeVector.X != 0 || npc.fleeVector.Y != 0) {
		fleeVector = npc.fleeVector
		fleeing = true
	}

	if fleeing {
		dirX, dirY := normalizeVector(fleeVector)
		if dirX == 0 && dirY == 0 {
			fallback := w.randomUnitVector()
			dirX, dirY = fallback.X, fallback.Y
		}
		commands = append(commands, Command{
			OriginTick: tick,
			ActorID:    npc.ID,
			Type:       CommandMove,
			IssuedAt:   now,
			Move: &MoveCommand{
				DX:     dirX * ratFleeSpeed,
				DY:     dirY * ratFleeSpeed,
				Facing: npc.Facing,
			},
		})
		decisionDelay = uint64(ratFleeDecisionDelay)
		npc.Blackboard.NextDecisionAt = tick + decisionDelay
		return commands
	}

	if npc.nextWanderTick <= tick || distance(npc.X, npc.Y, npc.wanderTarget.X, npc.wanderTarget.Y) < ratArriveRadius {
		npc.wanderTarget = w.randomRatTarget(npc)
		npc.nextWanderTick = tick + w.randomRatInterval(ratWanderDecisionMin, ratWanderDecisionMax)
	}

	dx := npc.wanderTarget.X - npc.X
	dy := npc.wanderTarget.Y - npc.Y
	dist := math.Hypot(dx, dy)
	if dist > 1 {
		dirX := dx / dist
		dirY := dy / dist
		commands = append(commands, Command{
			OriginTick: tick,
			ActorID:    npc.ID,
			Type:       CommandMove,
			IssuedAt:   now,
			Move: &MoveCommand{
				DX:     dirX * ratWanderSpeed,
				DY:     dirY * ratWanderSpeed,
				Facing: npc.Facing,
			},
		})
		decisionDelay = w.randomRatInterval(ratWanderDecisionMin, ratWanderDecisionMax)
	} else {
		npc.nextWanderTick = tick + 1
		commands = append(commands, Command{
			OriginTick: tick,
			ActorID:    npc.ID,
			Type:       CommandMove,
			IssuedAt:   now,
			Move: &MoveCommand{
				DX:     0,
				DY:     0,
				Facing: npc.Facing,
			},
		})
	}

	npc.Blackboard.NextDecisionAt = tick + decisionDelay
	return commands
}

func (w *World) ensureRatDefaults(npc *npcState) {
	if npc == nil {
		return
	}
	if npc.wanderOrigin.X == 0 && npc.wanderOrigin.Y == 0 {
		npc.wanderOrigin = vec2{X: npc.X, Y: npc.Y}
	}
	if npc.wanderTarget.X == 0 && npc.wanderTarget.Y == 0 {
		npc.wanderTarget = npc.wanderOrigin
	}
	if npc.Blackboard.LastWaypointIndex == 0 && len(npc.Waypoints) == 0 {
		npc.Blackboard.LastWaypointIndex = -1
	}
	if npc.Blackboard.LastPos == (vec2{}) {
		npc.Blackboard.LastPos = vec2{X: npc.X, Y: npc.Y}
	}
}

func (w *World) closestRatThreat(npc *npcState) (vec2, bool) {
	if w == nil || npc == nil {
		return vec2{}, false
	}
	bestDistSq := ratFleeRadius * ratFleeRadius
	found := false
	best := vec2{}

	for _, player := range w.players {
		if player == nil || player.Health <= 0 {
			continue
		}
		dx := npc.X - player.X
		dy := npc.Y - player.Y
		distSq := dx*dx + dy*dy
		if distSq < bestDistSq {
			bestDistSq = distSq
			best = vec2{X: dx, Y: dy}
			found = true
		}
	}

	for _, other := range w.npcs {
		if other == nil || other.ID == npc.ID || other.Type == NPCTypeRat || other.Health <= 0 {
			continue
		}
		dx := npc.X - other.X
		dy := npc.Y - other.Y
		distSq := dx*dx + dy*dy
		if distSq < bestDistSq {
			bestDistSq = distSq
			best = vec2{X: dx, Y: dy}
			found = true
		}
	}

	return best, found
}

func (w *World) randomRatTarget(npc *npcState) vec2 {
	rng := w.ensureRNG()
	if rng == nil {
		return vec2{X: npc.X, Y: npc.Y}
	}
	base := npc.wanderOrigin
	if base.X == 0 && base.Y == 0 {
		base = vec2{X: npc.X, Y: npc.Y}
	}
	angle := rng.Float64() * 2 * math.Pi
	dist := ratWanderRadius * math.Sqrt(rng.Float64())
	targetX := clamp(base.X+math.Cos(angle)*dist, playerHalf, worldWidth-playerHalf)
	targetY := clamp(base.Y+math.Sin(angle)*dist, playerHalf, worldHeight-playerHalf)
	return vec2{X: targetX, Y: targetY}
}

func (w *World) randomRatInterval(min, max uint64) uint64 {
	if max <= min {
		return min
	}
	rng := w.ensureRNG()
	if rng == nil {
		return min
	}
	span := max - min
	if span > 0x7fffffff {
		span = 0x7fffffff
	}
	return min + uint64(rng.Intn(int(span)+1))
}

func (w *World) randomUnitVector() vec2 {
	rng := w.ensureRNG()
	if rng == nil {
		return vec2{X: 1, Y: 0}
	}
	angle := rng.Float64() * 2 * math.Pi
	return vec2{X: math.Cos(angle), Y: math.Sin(angle)}
}

func (w *World) ensureRNG() *rand.Rand {
	if w == nil {
		return nil
	}
	if w.rng == nil {
		w.rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	return w.rng
}

func normalizeVector(v vec2) (float64, float64) {
	length := math.Hypot(v.X, v.Y)
	if length == 0 {
		return 0, 0
	}
	return v.X / length, v.Y / length
}

func distance(x1, y1, x2, y2 float64) float64 {
	return math.Hypot(x1-x2, y1-y2)
}
