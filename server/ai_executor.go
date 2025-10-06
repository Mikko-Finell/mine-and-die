package main

import (
	"math"
	"sort"
	"time"
)

const (
	maxAIDecisionsPerTick         = 64
	waypointProgressEpsilon       = 0.5
	waypointStallThreshold        = 30
	maxWaypointStall              = ^uint16(0)
	waypointArrivalSpeedThreshold = 0.3
	waypointArrivalHysteresis     = 1.5
	waypointNoProgressWindow      = 5
	waypointIntentEpsilon         = 1e-3
	waypointOrbitTolerance        = 0.25
)

var abilityIDToCommand = map[aiAbilityID]string{
	aiAbilityAttack:   effectTypeAttack,
	aiAbilityFireball: effectTypeFireball,
}

func (w *World) runAI(tick uint64, now time.Time) ([]Command, []Event) {
	if w == nil || w.aiLibrary == nil || len(w.npcs) == 0 {
		return nil, nil
	}
	npcIDs := make([]string, 0, len(w.npcs))
	for id := range w.npcs {
		npcIDs = append(npcIDs, id)
	}
	sort.Strings(npcIDs)

	commands := make([]Command, 0)
	events := make([]Event, 0)
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
		previousState := npc.AIState

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
					events = append(events, Event{
						Tick:     tick,
						EntityID: npc.ID,
						Type:     EventAIStateChanged,
						Payload: map[string]any{
							"from": cfg.stateName(previousState),
							"to":   cfg.stateName(npc.AIState),
						},
					})
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
	return commands, events
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
		baseRadius := params.ArriveRadius
		if baseRadius <= 0 {
			baseRadius = npc.Blackboard.BaseArriveRadius
			if baseRadius <= 0 {
				baseRadius = npc.Blackboard.ArriveRadius
			}
		}
		if baseRadius <= 0 {
			baseRadius = 12
		}
		if npc.Blackboard.BaseArriveRadius <= 0 {
			npc.Blackboard.BaseArriveRadius = baseRadius
		}
		radius := npc.Blackboard.ArriveRadius
		if radius <= 0 {
			radius = baseRadius
		}
		relaxedCap := math.Min(baseRadius*3, 32)
		if radius > relaxedCap {
			radius = relaxedCap
			npc.Blackboard.ArriveRadius = radius
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
		toTargetX := waypoint.X - npc.X
		toTargetY := waypoint.Y - npc.Y
		dist := math.Hypot(toTargetX, toTargetY)

		if npc.Blackboard.WaypointArrived && npc.Blackboard.WaypointArrivedIndex == idx {
			hysteresisRadius := npc.Blackboard.WaypointArriveRadius
			if hysteresisRadius <= 0 {
				hysteresisRadius = radius
			}
			if dist <= hysteresisRadius*waypointArrivalHysteresis {
				return true
			}
			w.clearWaypointArrival(npc, dist)
		}

		if dist > radius {
			return false
		}

		maxStep := moveSpeed / float64(tickRate)
		if maxStep > 0 {
			speedFraction := npc.Blackboard.LastMoveDelta / maxStep
			if speedFraction > waypointArrivalSpeedThreshold {
				if npc.Blackboard.HoldPositionUntil < tick+1 {
					npc.Blackboard.HoldPositionUntil = tick + 1
				}
				return false
			}
		}

		if !w.hasLineOfSight(npc.X, npc.Y, waypoint.X, waypoint.Y) {
			return false
		}

		intentLength := math.Hypot(npc.intentX, npc.intentY)
		if intentLength > waypointIntentEpsilon {
			intentDot := npc.intentX*toTargetX + npc.intentY*toTargetY
			if intentDot < 0 {
				return false
			}
		}

		facingOK := true
		if dist > waypointIntentEpsilon {
			fx, fy := facingToVector(npc.Facing)
			dotFacing := (fx*toTargetX + fy*toTargetY) / dist
			facingOK = dotFacing >= 0
		}
		if !facingOK {
			return false
		}

		w.markWaypointArrived(npc, idx, radius, waypoint, tick, dist)
		return true
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
		target, ok := w.players[npc.Blackboard.TargetActorID]
		if !ok {
			return true
		}
		dx := target.X - npc.X
		dy := target.Y - npc.Y
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
			if npc.Blackboard.HoldPositionUntil > 0 && tick < npc.Blackboard.HoldPositionUntil {
				npc.intentX = 0
				npc.intentY = 0
			} else {
				w.actionMoveToward(cfg, npc, action)
			}
			if cmd := buildMoveCommand(npc, tick, now); cmd != nil {
				*commands = append(*commands, *cmd)
			}
		case aiActionStop:
			npc.intentX = 0
			npc.intentY = 0
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
		}
	}
}

func (w *World) actionMoveToward(cfg *aiCompiledConfig, npc *npcState, action aiCompiledAction) {
	if cfg == nil || npc == nil {
		return
	}
	var params aiMoveTowardParams
	if int(action.paramIndex) < len(cfg.moveTowardParams) {
		params = cfg.moveTowardParams[action.paramIndex]
	}
	switch params.Target {
	case aiMoveTargetPlayer:
		if npc.Blackboard.TargetActorID == "" {
			npc.intentX = 0
			npc.intentY = 0
			return
		}
		target, ok := w.players[npc.Blackboard.TargetActorID]
		if !ok {
			npc.intentX = 0
			npc.intentY = 0
			return
		}
		npc.intentX = target.X - npc.X
		npc.intentY = target.Y - npc.Y
	case aiMoveTargetVector:
		npc.intentX = params.Vector.X
		npc.intentY = params.Vector.Y
	default:
		if len(npc.Waypoints) == 0 {
			npc.intentX = 0
			npc.intentY = 0
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
		npc.intentX = waypoint.X - npc.X
		npc.intentY = waypoint.Y - npc.Y
	}
	npc.Facing = deriveFacing(npc.intentX, npc.intentY, npc.Facing)
}

func buildMoveCommand(npc *npcState, tick uint64, now time.Time) *Command {
	if npc == nil {
		return nil
	}
	cmd := &Command{
		OriginTick: tick,
		ActorID:    npc.ID,
		Type:       CommandMove,
		IssuedAt:   now,
		Move: &MoveCommand{
			DX:     npc.intentX,
			DY:     npc.intentY,
			Facing: npc.Facing,
		},
	}
	return cmd
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
	npc.Facing = facing
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
	dist := math.Hypot(npc.X-waypoint.X, npc.Y-waypoint.Y)
	w.resetWaypointProgress(npc, idx, dist)
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

	baseRadius := npc.Blackboard.BaseArriveRadius
	if baseRadius <= 0 {
		baseRadius = npc.Blackboard.ArriveRadius
		if baseRadius <= 0 {
			baseRadius = 12
		}
		npc.Blackboard.BaseArriveRadius = baseRadius
	}
	if npc.Blackboard.ArriveRadius < baseRadius {
		npc.Blackboard.ArriveRadius = baseRadius
	}
	relaxedCap := math.Min(baseRadius*3, 32)
	if npc.Blackboard.ArriveRadius > relaxedCap {
		npc.Blackboard.ArriveRadius = relaxedCap
	}

	if len(npc.Waypoints) == 0 {
		npc.Blackboard.LastWaypointIndex = -1
		npc.Blackboard.WaypointBestDist = 0
		npc.Blackboard.WaypointLastDist = 0
		npc.Blackboard.WaypointStall = 0
		npc.Blackboard.WaypointNoProgress = 0
		npc.Blackboard.WaypointArrived = false
		npc.Blackboard.WaypointArrivedIndex = -1
		npc.Blackboard.WaypointArriveRadius = 0
		npc.Blackboard.HoldPositionUntil = 0
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
		w.resetWaypointProgress(npc, idx, dist)
		return
	}

	if npc.Blackboard.WaypointArrived && npc.Blackboard.WaypointArrivedIndex == idx {
		npc.Blackboard.WaypointLastDist = dist
		return
	}

	if npc.Blackboard.WaypointBestDist == 0 || dist+waypointProgressEpsilon < npc.Blackboard.WaypointBestDist {
		npc.Blackboard.WaypointBestDist = dist
		npc.Blackboard.WaypointStall = 0
		npc.Blackboard.WaypointNoProgress = 0
		npc.Blackboard.ArriveRadius = baseRadius
	} else if delta > epsilon {
		npc.Blackboard.WaypointNoProgress++
		if math.Abs(dist-npc.Blackboard.WaypointLastDist) <= waypointOrbitTolerance {
			bumpWaypointStall(&npc.Blackboard, uint16(waypointStallThreshold))
		} else if npc.Blackboard.WaypointNoProgress >= waypointNoProgressWindow {
			bumpWaypointStall(&npc.Blackboard, 1)
		}
	} else {
		npc.Blackboard.WaypointNoProgress = 0
	}

	npc.Blackboard.WaypointLastDist = dist

	if npc.Blackboard.StuckCounter >= uint8(waypointStallThreshold) && npc.Blackboard.WaypointBestDist > 0 && dist+waypointProgressEpsilon >= npc.Blackboard.WaypointBestDist {
		bumpWaypointStall(&npc.Blackboard, uint16(waypointStallThreshold))
	}

	if npc.Blackboard.WaypointStall > 0 {
		steps := int(npc.Blackboard.WaypointStall) / waypointStallThreshold
		if steps > 0 {
			increment := math.Max(baseRadius*0.5, 12)
			widened := baseRadius + float64(steps)*increment
			if widened > relaxedCap {
				widened = relaxedCap
			}
			npc.Blackboard.ArriveRadius = widened
		}
	}

	if int(npc.Blackboard.WaypointStall) > waypointStallThreshold*4 {
		w.advanceWaypoint(npc)
	}
}

func bumpWaypointStall(bb *npcBlackboard, amount uint16) {
	if bb == nil || amount == 0 {
		return
	}
	if bb.WaypointStall > maxWaypointStall-amount {
		bb.WaypointStall = maxWaypointStall
		return
	}
	bb.WaypointStall += amount
}

func (w *World) resetWaypointProgress(npc *npcState, idx int, dist float64) {
	if npc == nil {
		return
	}
	if npc.Blackboard.BaseArriveRadius <= 0 {
		if npc.Blackboard.ArriveRadius > 0 {
			npc.Blackboard.BaseArriveRadius = npc.Blackboard.ArriveRadius
		} else {
			npc.Blackboard.BaseArriveRadius = 12
		}
	}
	base := npc.Blackboard.BaseArriveRadius
	if base <= 0 {
		base = 12
	}
	npc.Blackboard.LastWaypointIndex = idx
	npc.Blackboard.WaypointBestDist = dist
	npc.Blackboard.WaypointLastDist = dist
	npc.Blackboard.WaypointStall = 0
	npc.Blackboard.WaypointNoProgress = 0
	npc.Blackboard.WaypointArrived = false
	npc.Blackboard.WaypointArrivedIndex = -1
	npc.Blackboard.WaypointArriveRadius = 0
	npc.Blackboard.HoldPositionUntil = 0
	npc.Blackboard.ArriveRadius = base
}

func (w *World) advanceWaypoint(npc *npcState) {
	if npc == nil || len(npc.Waypoints) == 0 {
		return
	}
	npc.Blackboard.WaypointIndex = (npc.Blackboard.WaypointIndex + 1) % len(npc.Waypoints)
	idx := npc.Blackboard.WaypointIndex
	if idx < 0 {
		idx = 0
	}
	waypoint := npc.Waypoints[idx]
	dist := math.Hypot(npc.X-waypoint.X, npc.Y-waypoint.Y)
	w.resetWaypointProgress(npc, idx, dist)
}

func (w *World) markWaypointArrived(npc *npcState, idx int, radius float64, waypoint vec2, tick uint64, dist float64) {
	if npc == nil {
		return
	}
	base := npc.Blackboard.BaseArriveRadius
	if base <= 0 {
		if radius > 0 {
			base = radius
		} else {
			base = 12
		}
		npc.Blackboard.BaseArriveRadius = base
	}
	npc.Blackboard.WaypointArrived = true
	npc.Blackboard.WaypointArrivedIndex = idx
	npc.Blackboard.WaypointArriveRadius = radius
	npc.Blackboard.WaypointStall = 0
	npc.Blackboard.WaypointBestDist = 0
	npc.Blackboard.WaypointLastDist = 0
	npc.Blackboard.WaypointNoProgress = 0
	npc.Blackboard.ArriveRadius = base
	npc.Blackboard.LastWaypointIndex = idx
	npc.Blackboard.HoldPositionUntil = 0
	if npc.Blackboard.PauseTicks > 0 {
		npc.Blackboard.HoldPositionUntil = tick + npc.Blackboard.PauseTicks
	}
	npc.intentX = 0
	npc.intentY = 0
	if dist <= 1 {
		npc.X = waypoint.X
		npc.Y = waypoint.Y
	}
	npc.Blackboard.LastPos = vec2{X: npc.X, Y: npc.Y}
}

func (w *World) clearWaypointArrival(npc *npcState, dist float64) {
	if npc == nil {
		return
	}
	npc.Blackboard.WaypointArrived = false
	npc.Blackboard.WaypointArrivedIndex = -1
	npc.Blackboard.WaypointArriveRadius = 0
	npc.Blackboard.HoldPositionUntil = 0
	npc.Blackboard.WaypointStall = 0
	npc.Blackboard.WaypointNoProgress = 0
	npc.Blackboard.WaypointBestDist = dist
	npc.Blackboard.WaypointLastDist = dist
	base := npc.Blackboard.BaseArriveRadius
	if base > 0 {
		npc.Blackboard.ArriveRadius = base
	}
}

func (w *World) hasLineOfSight(ax, ay, bx, by float64) bool {
	if w == nil {
		return true
	}
	for _, obs := range w.obstacles {
		if segmentIntersectsRect(ax, ay, bx, by, obs) {
			return false
		}
	}
	return true
}

func segmentIntersectsRect(ax, ay, bx, by float64, obs Obstacle) bool {
	minX := obs.X
	maxX := obs.X + obs.Width
	minY := obs.Y
	maxY := obs.Y + obs.Height

	if ax >= minX && ax <= maxX && ay >= minY && ay <= maxY {
		return true
	}
	if bx >= minX && bx <= maxX && by >= minY && by <= maxY {
		return true
	}

	segMinX := math.Min(ax, bx)
	segMaxX := math.Max(ax, bx)
	segMinY := math.Min(ay, by)
	segMaxY := math.Max(ay, by)

	if segMaxX < minX || segMinX > maxX || segMaxY < minY || segMinY > maxY {
		return false
	}

	edges := [4][4]float64{
		{minX, minY, maxX, minY},
		{maxX, minY, maxX, maxY},
		{maxX, maxY, minX, maxY},
		{minX, maxY, minX, minY},
	}
	for _, edge := range edges {
		if segmentsIntersect(ax, ay, bx, by, edge[0], edge[1], edge[2], edge[3]) {
			return true
		}
	}
	return false
}

func segmentsIntersect(ax, ay, bx, by, cx, cy, dx, dy float64) bool {
	o1 := orientation(ax, ay, bx, by, cx, cy)
	o2 := orientation(ax, ay, bx, by, dx, dy)
	o3 := orientation(cx, cy, dx, dy, ax, ay)
	o4 := orientation(cx, cy, dx, dy, bx, by)

	if o1*o2 < 0 && o3*o4 < 0 {
		return true
	}

	if math.Abs(o1) < 1e-6 && onSegment(ax, ay, bx, by, cx, cy) {
		return true
	}
	if math.Abs(o2) < 1e-6 && onSegment(ax, ay, bx, by, dx, dy) {
		return true
	}
	if math.Abs(o3) < 1e-6 && onSegment(cx, cy, dx, dy, ax, ay) {
		return true
	}
	if math.Abs(o4) < 1e-6 && onSegment(cx, cy, dx, dy, bx, by) {
		return true
	}
	return false
}

func onSegment(ax, ay, bx, by, px, py float64) bool {
	minX := math.Min(ax, bx) - 1e-6
	maxX := math.Max(ax, bx) + 1e-6
	minY := math.Min(ay, by) - 1e-6
	maxY := math.Max(ay, by) + 1e-6
	return px >= minX && px <= maxX && py >= minY && py <= maxY
}

func orientation(ax, ay, bx, by, cx, cy float64) float64 {
	return (bx-ax)*(cy-ay) - (by-ay)*(cx-ax)
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
