package server

import (
	"math"
	"sort"
	"time"
)

const (
	maxAIDecisionsPerTick   = 64
	waypointProgressEpsilon = 0.5
	waypointStallThreshold  = 30
	maxWaypointStall        = ^uint16(0)
)

var abilityIDToCommand = map[aiAbilityID]string{
	aiAbilityAttack:   effectTypeAttack,
	aiAbilityFireball: effectTypeFireball,
}

func (w *World) runAI(tick uint64, now time.Time) []Command {
	if w == nil || w.aiLibrary == nil || len(w.npcs) == 0 {
		return nil
	}
	npcIDs := make([]string, 0, len(w.npcs))
	for id := range w.npcs {
		npcIDs = append(npcIDs, id)
	}
	sort.Strings(npcIDs)

	commands := make([]Command, 0)
	decisions := 0
	for _, id := range npcIDs {
		npc := w.npcs[id]
		if npc == nil {
			continue
		}
		if npc.Blackboard.NextDecisionAt > tick {
			w.updateBlackboard(npc)
			continue
		}
		if npc.AIConfigID == 0 {
			continue
		}
		cfg := w.aiLibrary.ConfigByID(npc.AIConfigID)
		if cfg == nil || len(cfg.states) == 0 {
			continue
		}
		if decisions >= maxAIDecisionsPerTick {
			npc.Blackboard.NextDecisionAt = tick + 1
			continue
		}
		decisions++

		stateIndex := npc.AIState
		if int(stateIndex) >= len(cfg.states) {
			stateIndex = cfg.initialState
			npc.AIState = stateIndex
		}
		state := &cfg.states[stateIndex]

		for _, transition := range state.transitions {
			if w.evaluateCondition(cfg, npc, &transition, tick, now) {
				if transition.toState >= uint8(len(cfg.states)) {
					break
				}
				if npc.AIState != transition.toState {
					npc.AIState = transition.toState
					npc.Blackboard.StateEnteredTick = tick
					if cfg.states[npc.AIState].enterTimer > 0 {
						npc.Blackboard.WaitUntil = tick + uint64(cfg.states[npc.AIState].enterTimer)
					}
				}
				stateIndex = npc.AIState
				state = &cfg.states[stateIndex]
				break
			}
		}

		if npc.Blackboard.StateEnteredTick == 0 && npc.Blackboard.LastDecisionTick == 0 {
			npc.Blackboard.StateEnteredTick = tick
		}
		w.executeActions(cfg, npc, state, tick, now, &commands)
		cadence := state.cadence
		if cadence == 0 {
			npc.Blackboard.NextDecisionAt = tick + 1
		} else {
			npc.Blackboard.NextDecisionAt = tick + uint64(cadence)
		}
		npc.Blackboard.LastDecisionTick = tick
		w.updateBlackboard(npc)
	}
	return commands
}

func (w *World) evaluateCondition(cfg *aiCompiledConfig, npc *npcState, transition *aiCompiledTransition, tick uint64, now time.Time) bool {
	if cfg == nil || npc == nil || transition == nil {
		return false
	}
	switch transition.conditionID {
	case aiConditionReachedWaypoint:
		var params aiReachedWaypointParams
		if int(transition.paramIndex) < len(cfg.reachedWaypointParams) {
			params = cfg.reachedWaypointParams[transition.paramIndex]
		}
		radius := params.ArriveRadius
		if radius <= 0 {
			radius = npc.Blackboard.ArriveRadius
		}
		if radius <= 0 {
			radius = 12
		}
		if len(npc.Waypoints) == 0 {
			return false
		}
		idx := npc.Blackboard.WaypointIndex
		if idx < 0 {
			idx = 0
		}
		if idx >= len(npc.Waypoints) {
			idx = idx % len(npc.Waypoints)
		}
		waypoint := npc.Waypoints[idx]
		dx := npc.X - waypoint.X
		dy := npc.Y - waypoint.Y
		dist := math.Hypot(dx, dy)
		if dist <= radius {
			return true
		}

		// If we've been circling the waypoint without making progress,
		// gradually relax the arrival radius so patrols don't stall on
		// geometry seams.
		stall := npc.Blackboard.WaypointStall
		if stall == 0 {
			return false
		}
		steps := int(stall / waypointStallThreshold)
		if steps <= 0 {
			return false
		}
		baseRelax := math.Max(radius*0.5, 12)
		relaxed := radius + float64(steps)*baseRelax
		best := npc.Blackboard.WaypointBestDist
		if best <= 0 {
			best = dist
		}
		if dist <= relaxed || best <= relaxed {
			return true
		}
		// As a final fallback, treat the waypoint as reached if we
		// haven't improved for multiple windows and are effectively
		// stuck near an obstacle.
		if steps > 3 {
			return true
		}
		return false
	case aiConditionTimerExpired:
		wait := npc.Blackboard.WaitUntil
		return wait > 0 && tick >= wait
	case aiConditionPlayerWithin:
		var params aiPlayerWithinParams
		if int(transition.paramIndex) < len(cfg.playerWithinParams) {
			params = cfg.playerWithinParams[transition.paramIndex]
		}
		radius := params.Radius
		if radius <= 0 {
			radius = 4
		}
		id, distSq, ok := w.closestPlayer(npc.X, npc.Y)
		if !ok {
			return false
		}
		if distSq <= radius*radius {
			npc.Blackboard.TargetActorID = id
			return true
		}
		return false
	case aiConditionNonRatWithin:
		var params aiActorWithinParams
		if int(transition.paramIndex) < len(cfg.nonRatWithinParams) {
			params = cfg.nonRatWithinParams[transition.paramIndex]
		}
		radius := params.Radius
		if radius <= 0 {
			radius = 6
		}
		id, distSq, ok := w.closestNonRatActor(npc)
		if !ok {
			return false
		}
		if distSq <= radius*radius {
			npc.Blackboard.TargetActorID = id
			return true
		}
		return false
	case aiConditionLostSight:
		if npc.Blackboard.TargetActorID == "" {
			return true
		}
		var params aiLostSightParams
		if int(transition.paramIndex) < len(cfg.lostSightParams) {
			params = cfg.lostSightParams[transition.paramIndex]
		}
		threshold := params.Distance
		if threshold <= 0 {
			threshold = 8
		}
		targetX, targetY, ok := w.actorPosition(npc.Blackboard.TargetActorID)
		if !ok {
			return true
		}
		dx := targetX - npc.X
		dy := targetY - npc.Y
		return math.Hypot(dx, dy) > threshold
	case aiConditionCooldownReady:
		var params aiCooldownReadyParams
		if int(transition.paramIndex) < len(cfg.cooldownReadyParams) {
			params = cfg.cooldownReadyParams[transition.paramIndex]
		}
		ability := params.Ability
		if ability == aiAbilityNone {
			return true
		}
		next := npc.Blackboard.nextAbilityReady[ability]
		return tick >= next
	case aiConditionStuck:
		var params aiStuckParams
		if int(transition.paramIndex) < len(cfg.stuckParams) {
			params = cfg.stuckParams[transition.paramIndex]
		}
		threshold := params.Decisions
		if threshold == 0 {
			threshold = 3
		}
		epsilon := params.Epsilon
		if epsilon <= 0 {
			epsilon = npc.Blackboard.StuckEpsilon
			if epsilon <= 0 {
				epsilon = 0.5
			}
		}
		return npc.Blackboard.StuckCounter >= threshold && npc.Blackboard.LastMoveDelta < epsilon
	default:
		return false
	}
}

func (w *World) executeActions(cfg *aiCompiledConfig, npc *npcState, state *aiCompiledState, tick uint64, now time.Time, commands *[]Command) {
	if cfg == nil || npc == nil || state == nil || commands == nil {
		return
	}
	for _, action := range state.actions {
		switch action.id {
		case aiActionMoveToward:
			w.actionMoveToward(cfg, npc, action, tick)
		case aiActionStop:
			w.clearNPCPath(npc)
			cmd := Command{
				OriginTick: tick,
				ActorID:    npc.ID,
				Type:       CommandMove,
				IssuedAt:   now,
				Move: &MoveCommand{
					DX:     0,
					DY:     0,
					Facing: npc.Facing,
				},
			}
			*commands = append(*commands, cmd)
		case aiActionUseAbility:
			w.actionUseAbility(cfg, npc, action, tick, now, commands)
		case aiActionFace:
			if cmd := w.actionFace(cfg, npc, action, tick, now); cmd != nil {
				*commands = append(*commands, *cmd)
			}
		case aiActionSetTimer:
			w.actionSetTimer(cfg, npc, action, tick)
		case aiActionSetWaypoint:
			w.actionSetWaypoint(cfg, npc, action, tick)
		case aiActionSetRandomDestination:
			w.actionSetRandomDestination(cfg, npc, action, tick)
		case aiActionMoveAway:
			w.actionMoveAwayFromTarget(cfg, npc, action, tick)
		}
	}
}

func (w *World) actionMoveToward(cfg *aiCompiledConfig, npc *npcState, action aiCompiledAction, tick uint64) {
	if cfg == nil || npc == nil {
		return
	}
	width, height := w.dimensions()
	var params aiMoveTowardParams
	if int(action.paramIndex) < len(cfg.moveTowardParams) {
		params = cfg.moveTowardParams[action.paramIndex]
	}

	var target vec2
	var ok bool

	switch params.Target {
	case aiMoveTargetPlayer:
		if npc.Blackboard.TargetActorID == "" {
			w.clearNPCPath(npc)
			return
		}
		player, exists := w.players[npc.Blackboard.TargetActorID]
		if !exists {
			w.clearNPCPath(npc)
			return
		}
		target = vec2{X: player.X, Y: player.Y}
		ok = true
	case aiMoveTargetVector:
		target = vec2{
			X: clamp(npc.X+params.Vector.X, playerHalf, width-playerHalf),
			Y: clamp(npc.Y+params.Vector.Y, playerHalf, height-playerHalf),
		}
		ok = true
	default:
		if len(npc.Waypoints) == 0 {
			w.clearNPCPath(npc)
			return
		}
		idx := npc.Blackboard.WaypointIndex
		if idx < 0 {
			idx = 0
		}
		if idx >= len(npc.Waypoints) {
			idx = idx % len(npc.Waypoints)
		}
		target = npc.Waypoints[idx]
		ok = true
	}

	if !ok {
		w.clearNPCPath(npc)
		return
	}

	w.ensureNPCPath(npc, target, tick)
}

func (w *World) actionUseAbility(cfg *aiCompiledConfig, npc *npcState, action aiCompiledAction, tick uint64, now time.Time, commands *[]Command) {
	if cfg == nil || npc == nil || commands == nil {
		return
	}
	var params aiUseAbilityParams
	if int(action.paramIndex) < len(cfg.useAbilityParams) {
		params = cfg.useAbilityParams[action.paramIndex]
	}
	ability := params.Ability
	if ability == aiAbilityNone {
		return
	}
	name, ok := abilityIDToCommand[ability]
	if !ok || name == "" {
		return
	}
	cmd := Command{
		OriginTick: tick,
		ActorID:    npc.ID,
		Type:       CommandAction,
		IssuedAt:   now,
		Action: &ActionCommand{
			Name: name,
		},
	}
	*commands = append(*commands, cmd)
	cooldown := abilityCooldownTicks(ability)
	if cooldown > 0 {
		npc.Blackboard.nextAbilityReady[ability] = tick + cooldown
	}
}

func (w *World) actionFace(cfg *aiCompiledConfig, npc *npcState, action aiCompiledAction, tick uint64, now time.Time) *Command {
	if cfg == nil || npc == nil {
		return nil
	}
	var params aiFaceParams
	if int(action.paramIndex) < len(cfg.faceParams) {
		params = cfg.faceParams[action.paramIndex]
	}
	dx, dy := 0.0, 0.0
	switch params.Target {
	case aiMoveTargetPlayer:
		if npc.Blackboard.TargetActorID == "" {
			return nil
		}
		target, ok := w.players[npc.Blackboard.TargetActorID]
		if !ok {
			return nil
		}
		dx = target.X - npc.X
		dy = target.Y - npc.Y
	case aiMoveTargetVector:
		// Unsupported for facing; default to previous facing.
	default:
		if len(npc.Waypoints) == 0 {
			return nil
		}
		idx := npc.Blackboard.WaypointIndex
		if idx < 0 {
			idx = 0
		}
		if idx >= len(npc.Waypoints) {
			idx = idx % len(npc.Waypoints)
		}
		waypoint := npc.Waypoints[idx]
		dx = waypoint.X - npc.X
		dy = waypoint.Y - npc.Y
	}
	if dx == 0 && dy == 0 {
		return nil
	}
	facing := deriveFacing(dx, dy, npc.Facing)
	w.SetNPCFacing(npc.ID, facing)
	return &Command{
		OriginTick: tick,
		ActorID:    npc.ID,
		Type:       CommandMove,
		IssuedAt:   now,
		Move: &MoveCommand{
			DX:     0,
			DY:     0,
			Facing: facing,
		},
	}
}

func (w *World) actionSetTimer(cfg *aiCompiledConfig, npc *npcState, action aiCompiledAction, tick uint64) {
	if cfg == nil || npc == nil {
		return
	}
	if tick != npc.Blackboard.StateEnteredTick {
		return
	}
	var params aiSetTimerParams
	if int(action.paramIndex) < len(cfg.setTimerParams) {
		params = cfg.setTimerParams[action.paramIndex]
	}
	duration := params.Duration
	if duration == 0 {
		duration = uint16(npc.Blackboard.PauseTicks)
	}
	if duration == 0 {
		return
	}
	npc.Blackboard.WaitUntil = tick + uint64(duration)
}

func (w *World) actionSetWaypoint(cfg *aiCompiledConfig, npc *npcState, action aiCompiledAction, tick uint64) {
	if cfg == nil || npc == nil {
		return
	}
	if len(npc.Waypoints) == 0 {
		return
	}
	if tick != npc.Blackboard.StateEnteredTick {
		return
	}
	w.clearNPCPath(npc)
	var params aiSetWaypointParams
	if int(action.paramIndex) < len(cfg.setWaypointParams) {
		params = cfg.setWaypointParams[action.paramIndex]
	}
	if params.Advance {
		npc.Blackboard.WaypointIndex = (npc.Blackboard.WaypointIndex + 1) % len(npc.Waypoints)
	} else {
		if params.Index < 0 {
			return
		}
		npc.Blackboard.WaypointIndex = params.Index % len(npc.Waypoints)
	}

	idx := npc.Blackboard.WaypointIndex
	if idx < 0 {
		idx = 0
	}
	if idx >= len(npc.Waypoints) {
		idx = idx % len(npc.Waypoints)
	}
	waypoint := npc.Waypoints[idx]
	dx := npc.X - waypoint.X
	dy := npc.Y - waypoint.Y
	npc.Blackboard.LastWaypointIndex = idx
	npc.Blackboard.WaypointBestDist = math.Hypot(dx, dy)
	npc.Blackboard.WaypointLastDist = npc.Blackboard.WaypointBestDist
	npc.Blackboard.WaypointStall = 0
}

func (w *World) actionSetRandomDestination(cfg *aiCompiledConfig, npc *npcState, action aiCompiledAction, tick uint64) {
	if cfg == nil || npc == nil {
		return
	}
	if tick != npc.Blackboard.StateEnteredTick {
		return
	}
	var params aiRandomDestinationParams
	if int(action.paramIndex) < len(cfg.randomDestinationParams) {
		params = cfg.randomDestinationParams[action.paramIndex]
	}
	radius := params.Radius
	if radius <= 0 {
		radius = 180
	}
	minRadius := params.MinRadius
	if minRadius < 0 {
		minRadius = 0
	}
	if minRadius > radius {
		minRadius = radius * 0.5
	}
	center := npc.Home
	if center.X == 0 && center.Y == 0 {
		center = vec2{X: npc.X, Y: npc.Y}
		npc.Home = center
	}
	attempts := 6
	width, height := w.dimensions()
	for i := 0; i < attempts; i++ {
		angle := w.randomAngle()
		distance := w.randomDistance(minRadius, radius)
		if distance <= 0 {
			distance = radius
		}
		target := vec2{
			X: clamp(center.X+math.Cos(angle)*distance, playerHalf, width-playerHalf),
			Y: clamp(center.Y+math.Sin(angle)*distance, playerHalf, height-playerHalf),
		}
		if w.ensureNPCPath(npc, target, tick) {
			return
		}
	}
	w.clearNPCPath(npc)
}

func (w *World) actionMoveAwayFromTarget(cfg *aiCompiledConfig, npc *npcState, action aiCompiledAction, tick uint64) {
	if cfg == nil || npc == nil {
		return
	}
	var params aiMoveAwayParams
	if int(action.paramIndex) < len(cfg.moveAwayParams) {
		params = cfg.moveAwayParams[action.paramIndex]
	}
	distance := params.Distance
	if distance <= 0 {
		distance = 220
	}
	minDistance := params.MinDistance
	if minDistance < 0 {
		minDistance = 0
	}
	if minDistance > distance {
		minDistance = distance * 0.5
	}
	targetID := npc.Blackboard.TargetActorID
	if targetID == "" {
		w.clearNPCPath(npc)
		return
	}
	tx, ty, ok := w.actorPosition(targetID)
	if !ok {
		w.clearNPCPath(npc)
		return
	}
	dx := npc.X - tx
	dy := npc.Y - ty
	if dx == 0 && dy == 0 {
		angle := w.randomAngle()
		dx = math.Cos(angle)
		dy = math.Sin(angle)
	}
	norm := math.Hypot(dx, dy)
	if norm == 0 {
		w.clearNPCPath(npc)
		return
	}
	dx /= norm
	dy /= norm
	goalDist := w.randomDistance(minDistance, distance)
	if goalDist <= 0 {
		goalDist = distance
	}
	width, height := w.dimensions()
	target := vec2{
		X: clamp(npc.X+dx*goalDist, playerHalf, width-playerHalf),
		Y: clamp(npc.Y+dy*goalDist, playerHalf, height-playerHalf),
	}
	w.ensureNPCPath(npc, target, tick)
}

func (w *World) updateBlackboard(npc *npcState) {
	if npc == nil {
		return
	}
	epsilon := npc.Blackboard.StuckEpsilon
	if epsilon <= 0 {
		epsilon = 0.5
	}
	dx := npc.X - npc.Blackboard.LastPos.X
	dy := npc.Y - npc.Blackboard.LastPos.Y
	delta := math.Hypot(dx, dy)
	npc.Blackboard.LastMoveDelta = delta
	if delta < epsilon {
		if npc.Blackboard.StuckCounter < math.MaxUint8 {
			npc.Blackboard.StuckCounter++
		}
	} else {
		npc.Blackboard.StuckCounter = 0
	}
	npc.Blackboard.LastPos = vec2{X: npc.X, Y: npc.Y}

	if len(npc.Waypoints) == 0 {
		npc.Blackboard.LastWaypointIndex = -1
		npc.Blackboard.WaypointBestDist = 0
		npc.Blackboard.WaypointLastDist = 0
		npc.Blackboard.WaypointStall = 0
		return
	}

	idx := npc.Blackboard.WaypointIndex
	if idx < 0 {
		idx = 0
	}
	if idx >= len(npc.Waypoints) {
		idx = idx % len(npc.Waypoints)
	}
	waypoint := npc.Waypoints[idx]
	dist := math.Hypot(npc.X-waypoint.X, npc.Y-waypoint.Y)

	if npc.Blackboard.LastWaypointIndex != idx {
		npc.Blackboard.LastWaypointIndex = idx
		npc.Blackboard.WaypointBestDist = dist
		npc.Blackboard.WaypointLastDist = dist
		npc.Blackboard.WaypointStall = 0
		return
	}

	if npc.Blackboard.WaypointBestDist == 0 || dist+waypointProgressEpsilon < npc.Blackboard.WaypointBestDist {
		npc.Blackboard.WaypointBestDist = dist
		npc.Blackboard.WaypointStall = 0
	} else if npc.Blackboard.WaypointStall < maxWaypointStall {
		npc.Blackboard.WaypointStall++
	}
	npc.Blackboard.WaypointLastDist = dist
}

func (w *World) closestPlayer(x, y float64) (string, float64, bool) {
	if len(w.players) == 0 {
		return "", 0, false
	}
	bestID := ""
	bestDistSq := math.MaxFloat64
	for id, player := range w.players {
		dx := player.X - x
		dy := player.Y - y
		distSq := dx*dx + dy*dy
		if distSq < bestDistSq-1e-6 || (math.Abs(distSq-bestDistSq) <= 1e-6 && id < bestID) {
			bestDistSq = distSq
			bestID = id
		}
	}
	if bestID == "" {
		return "", 0, false
	}
	return bestID, bestDistSq, true
}

func (w *World) closestNonRatActor(npc *npcState) (string, float64, bool) {
	if w == nil || npc == nil {
		return "", 0, false
	}
	type candidate struct {
		id string
		x  float64
		y  float64
	}
	candidates := make([]candidate, 0, len(w.players)+len(w.npcs))
	for id, player := range w.players {
		if player == nil {
			continue
		}
		candidates = append(candidates, candidate{id: id, x: player.X, y: player.Y})
	}
	for id, other := range w.npcs {
		if other == nil || other.ID == npc.ID || other.Type == NPCTypeRat {
			continue
		}
		candidates = append(candidates, candidate{id: id, x: other.X, y: other.Y})
	}
	if len(candidates) == 0 {
		return "", 0, false
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].id < candidates[j].id
	})
	bestID := ""
	bestDist := math.MaxFloat64
	for _, cand := range candidates {
		dx := cand.x - npc.X
		dy := cand.y - npc.Y
		distSq := dx*dx + dy*dy
		if distSq < bestDist-1e-6 || (math.Abs(distSq-bestDist) <= 1e-6 && cand.id < bestID) {
			bestDist = distSq
			bestID = cand.id
		}
	}
	if bestID == "" {
		return "", 0, false
	}
	return bestID, bestDist, true
}

func (w *World) actorPosition(id string) (float64, float64, bool) {
	if w == nil || id == "" {
		return 0, 0, false
	}
	if player, ok := w.players[id]; ok && player != nil {
		return player.X, player.Y, true
	}
	if npc, ok := w.npcs[id]; ok && npc != nil {
		return npc.X, npc.Y, true
	}
	return 0, 0, false
}

func abilityCooldownTicks(ability aiAbilityID) uint64 {
	switch ability {
	case aiAbilityAttack:
		return uint64(math.Ceil(meleeAttackCooldown.Seconds() * float64(tickRate)))
	case aiAbilityFireball:
		return uint64(math.Ceil(fireballCooldown.Seconds() * float64(tickRate)))
	default:
		return 0
	}
}
