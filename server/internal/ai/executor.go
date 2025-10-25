package ai

import (
	"math"
	"sort"
	"strings"
	"time"

	worldpkg "mine-and-die/server/internal/world"
)

const (
	maxDecisionsPerTick    = 64
	waypointProgressEps    = 0.5
	waypointStallThreshold = 30
	maxWaypointStall       = ^uint16(0)
)

// RunConfig captures the runtime dependencies required to execute NPC AI.
type RunConfig struct {
	Tick   uint64
	Now    time.Time
	Width  float64
	Height float64

	Library *Library

	NPCs    []*NPC
	Players []Player

	RandomAngle    func() float64
	RandomDistance func(min, max float64) float64
	DeriveFacing   func(dx, dy float64, fallback string) string

	AbilityCommand  func(AbilityID) (string, bool)
	AbilityCooldown func(AbilityID) uint64
}

// Run executes the AI state machines for the provided NPCs and returns the
// resulting simulation commands.
func Run(cfg RunConfig) []Command {
	if cfg.Library == nil || len(cfg.NPCs) == 0 {
		return nil
	}

	npcs := make([]*NPC, 0, len(cfg.NPCs))
	for _, npc := range cfg.NPCs {
		if npc == nil || npc.Blackboard == nil || npc.AIState == nil {
			continue
		}
		npcs = append(npcs, npc)
	}
	if len(npcs) == 0 {
		return nil
	}

	sort.Slice(npcs, func(i, j int) bool {
		return npcs[i].ID < npcs[j].ID
	})

	env := runEnv{cfg: cfg}
	commands := make([]Command, 0)
	decisions := 0

	for _, npc := range npcs {
		if npc.Blackboard.NextDecisionAt > cfg.Tick {
			updateBlackboard(&env, npc)
			continue
		}
		if npc.AIConfigID == 0 {
			continue
		}
		compiled := cfg.Library.ConfigByID(npc.AIConfigID)
		if compiled == nil || len(compiled.states) == 0 {
			continue
		}
		if decisions >= maxDecisionsPerTick {
			npc.Blackboard.NextDecisionAt = cfg.Tick + 1
			continue
		}
		decisions++

		stateIndex := *npc.AIState
		if int(stateIndex) >= len(compiled.states) {
			stateIndex = compiled.initialState
			*npc.AIState = stateIndex
		}
		state := &compiled.states[stateIndex]

		for _, transition := range state.transitions {
			if evaluateCondition(&env, compiled, npc, &transition, cfg.Tick, cfg.Now) {
				if transition.toState >= uint8(len(compiled.states)) {
					break
				}
				if *npc.AIState != transition.toState {
					*npc.AIState = transition.toState
					npc.Blackboard.StateEnteredTick = cfg.Tick
					next := compiled.states[*npc.AIState].enterTimer
					if next > 0 {
						npc.Blackboard.WaitUntil = cfg.Tick + uint64(next)
					}
				}
				stateIndex = *npc.AIState
				state = &compiled.states[stateIndex]
				break
			}
		}

		if npc.Blackboard.StateEnteredTick == 0 && npc.Blackboard.LastDecisionTick == 0 {
			npc.Blackboard.StateEnteredTick = cfg.Tick
		}
		executeActions(&env, compiled, npc, state, cfg.Tick, cfg.Now, &commands)
		cadence := state.cadence
		if cadence == 0 {
			npc.Blackboard.NextDecisionAt = cfg.Tick + 1
		} else {
			npc.Blackboard.NextDecisionAt = cfg.Tick + uint64(cadence)
		}
		npc.Blackboard.LastDecisionTick = cfg.Tick
		updateBlackboard(&env, npc)
	}

	return commands
}

type runEnv struct {
	cfg RunConfig
}

func (env *runEnv) width() float64 {
	if env == nil || env.cfg.Width <= 0 {
		return worldpkg.DefaultWidth
	}
	return env.cfg.Width
}

func (env *runEnv) height() float64 {
	if env == nil || env.cfg.Height <= 0 {
		return worldpkg.DefaultHeight
	}
	return env.cfg.Height
}

func evaluateCondition(env *runEnv, cfg *CompiledConfig, npc *NPC, transition *compiledTransition, tick uint64, now time.Time) bool {
	if env == nil || cfg == nil || npc == nil || transition == nil {
		return false
	}
	switch transition.conditionID {
	case conditionReachedWaypoint:
		var params reachedWaypointParams
		if int(transition.paramIndex) < len(cfg.reachedWaypointParams) {
			params = cfg.reachedWaypointParams[transition.paramIndex]
		}
		radius := params.ArriveRadius
		if radius <= 0 {
			radius = npc.Blackboard.ArriveRadius
		}
		if radius <= 0 {
			radius = worldpkg.DefaultNPCArriveRadius
		}
		waypoints := npc.waypoints()
		if len(waypoints) == 0 {
			return false
		}
		idx := npc.Blackboard.WaypointIndex
		if idx < 0 {
			idx = 0
		}
		if idx >= len(waypoints) {
			idx %= len(waypoints)
		}
		waypoint := waypoints[idx]
		dx := npc.Position.XValue() - waypoint.X
		dy := npc.Position.YValue() - waypoint.Y
		dist := math.Hypot(dx, dy)
		if dist <= radius {
			return true
		}
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
		if steps > 3 {
			return true
		}
		return false
	case conditionTimerExpired:
		wait := npc.Blackboard.WaitUntil
		return wait > 0 && tick >= wait
	case conditionPlayerWithin:
		var params playerWithinParams
		if int(transition.paramIndex) < len(cfg.playerWithinParams) {
			params = cfg.playerWithinParams[transition.paramIndex]
		}
		radius := params.Radius
		if radius <= 0 {
			radius = 4
		}
		id, distSq, ok := env.closestPlayer(npc.Position.XValue(), npc.Position.YValue())
		if !ok {
			return false
		}
		if distSq <= radius*radius {
			npc.Blackboard.TargetActorID = id
			return true
		}
		return false
	case conditionLostSight:
		if npc.Blackboard.TargetActorID == "" {
			return true
		}
		var params lostSightParams
		if int(transition.paramIndex) < len(cfg.lostSightParams) {
			params = cfg.lostSightParams[transition.paramIndex]
		}
		threshold := params.Distance
		if threshold <= 0 {
			threshold = 8
		}
		x, y, ok := env.actorPosition(npc.Blackboard.TargetActorID)
		if !ok {
			return true
		}
		dx := x - npc.Position.XValue()
		dy := y - npc.Position.YValue()
		return math.Hypot(dx, dy) > threshold
	case conditionCooldownReady:
		var params cooldownReadyParams
		if int(transition.paramIndex) < len(cfg.cooldownReadyParams) {
			params = cfg.cooldownReadyParams[transition.paramIndex]
		}
		ability := params.Ability
		if ability == AbilityNone {
			return true
		}
		next := npc.Blackboard.NextAbilityReady[ability]
		return tick >= next
	case conditionStuck:
		var params stuckParams
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
	case conditionNonRatWithin:
		var params actorWithinParams
		if int(transition.paramIndex) < len(cfg.actorWithinParams) {
			params = cfg.actorWithinParams[transition.paramIndex]
		}
		radius := params.Radius
		if radius <= 0 {
			radius = 6
		}
		id, distSq, ok := env.closestNonRatActor(npc)
		if !ok {
			return false
		}
		if distSq <= radius*radius {
			npc.Blackboard.TargetActorID = id
			return true
		}
		return false
	default:
		return false
	}
}

func executeActions(env *runEnv, cfg *CompiledConfig, npc *NPC, state *compiledState, tick uint64, now time.Time, commands *[]Command) {
	if env == nil || cfg == nil || npc == nil || state == nil || commands == nil {
		return
	}
	for _, action := range state.actions {
		switch action.id {
		case actionIDMoveToward:
			actionMoveToward(env, cfg, npc, action, tick)
		case actionIDStop:
			npc.clearPath()
			*commands = append(*commands, Command{
				OriginTick: tick,
				ActorID:    npc.ID,
				Type:       CommandMove,
				IssuedAt:   now,
				Move: &MoveCommand{
					DX:     0,
					DY:     0,
					Facing: npc.Facing.Value(),
				},
			})
		case actionIDUseAbility:
			actionUseAbility(env, cfg, npc, action, tick, now, commands)
		case actionIDFace:
			if cmd := actionFaceTarget(env, cfg, npc, action, tick, now); cmd != nil {
				*commands = append(*commands, *cmd)
			}
		case actionIDSetTimer:
			actionSetTimer(cfg, npc, action, tick)
		case actionIDSetWaypoint:
			actionSetWaypoint(cfg, npc, action, tick)
		case actionIDSetRandomDestination:
			actionSetRandomDestination(env, cfg, npc, action, tick)
		case actionIDMoveAway:
			actionMoveAway(env, cfg, npc, action, tick)
		}
	}
}

func actionMoveToward(env *runEnv, cfg *CompiledConfig, npc *NPC, action compiledAction, tick uint64) {
	if env == nil || cfg == nil || npc == nil {
		return
	}
	width, height := env.width(), env.height()
	var params moveTowardParams
	if int(action.paramIndex) < len(cfg.moveTowardParams) {
		params = cfg.moveTowardParams[action.paramIndex]
	}

	var target Vec2
	var ok bool

	switch params.Target {
	case moveTargetPlayer:
		if npc.Blackboard.TargetActorID == "" {
			npc.clearPath()
			return
		}
		x, y, exists := env.actorPosition(npc.Blackboard.TargetActorID)
		if !exists {
			npc.clearPath()
			return
		}
		target = Vec2{X: x, Y: y}
		ok = true
	case moveTargetVector:
		x := clamp(npc.Position.XValue()+params.Vector.X, worldpkg.PlayerHalf, width-worldpkg.PlayerHalf)
		y := clamp(npc.Position.YValue()+params.Vector.Y, worldpkg.PlayerHalf, height-worldpkg.PlayerHalf)
		target = Vec2{X: x, Y: y}
		ok = true
	default:
		waypoints := npc.waypoints()
		if len(waypoints) == 0 {
			npc.clearPath()
			return
		}
		idx := npc.Blackboard.WaypointIndex
		if idx < 0 {
			idx = 0
		}
		if idx >= len(waypoints) {
			idx %= len(waypoints)
		}
		target = waypoints[idx]
		ok = true
	}

	if !ok {
		npc.clearPath()
		return
	}

	npc.ensurePath(target, tick)
}

func actionUseAbility(env *runEnv, cfg *CompiledConfig, npc *NPC, action compiledAction, tick uint64, now time.Time, commands *[]Command) {
	if env == nil || cfg == nil || npc == nil || commands == nil {
		return
	}
	var params useAbilityParams
	if int(action.paramIndex) < len(cfg.useAbilityParams) {
		params = cfg.useAbilityParams[action.paramIndex]
	}
	ability := params.Ability
	if ability == AbilityNone {
		return
	}
	name, ok := env.commandForAbility(ability)
	if !ok || name == "" {
		return
	}
	*commands = append(*commands, Command{
		OriginTick: tick,
		ActorID:    npc.ID,
		Type:       CommandAction,
		IssuedAt:   now,
		Action: &ActionCommand{
			Name: name,
		},
	})
	if cooldown := env.cooldownForAbility(ability); cooldown > 0 {
		npc.Blackboard.NextAbilityReady[ability] = tick + cooldown
	}
}

func actionFaceTarget(env *runEnv, cfg *CompiledConfig, npc *NPC, action compiledAction, tick uint64, now time.Time) *Command {
	if env == nil || cfg == nil || npc == nil {
		return nil
	}
	var params faceParams
	if int(action.paramIndex) < len(cfg.faceParams) {
		params = cfg.faceParams[action.paramIndex]
	}
	dx, dy := 0.0, 0.0
	switch params.Target {
	case moveTargetPlayer:
		if npc.Blackboard.TargetActorID == "" {
			return nil
		}
		x, y, ok := env.actorPosition(npc.Blackboard.TargetActorID)
		if !ok {
			return nil
		}
		dx = x - npc.Position.XValue()
		dy = y - npc.Position.YValue()
	case moveTargetVector:
		// Unsupported; default to previous facing.
	default:
		waypoints := npc.waypoints()
		if len(waypoints) == 0 {
			return nil
		}
		idx := npc.Blackboard.WaypointIndex
		if idx < 0 {
			idx = 0
		}
		if idx >= len(waypoints) {
			idx %= len(waypoints)
		}
		waypoint := waypoints[idx]
		dx = waypoint.X - npc.Position.XValue()
		dy = waypoint.Y - npc.Position.YValue()
	}
	if dx == 0 && dy == 0 {
		return nil
	}
	facing := env.deriveFacing(dx, dy, npc.Facing.Value())
	npc.Facing.Apply(facing)
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

func actionSetTimer(cfg *CompiledConfig, npc *NPC, action compiledAction, tick uint64) {
	if cfg == nil || npc == nil {
		return
	}
	if tick != npc.Blackboard.StateEnteredTick {
		return
	}
	var params setTimerParams
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

func actionSetWaypoint(cfg *CompiledConfig, npc *NPC, action compiledAction, tick uint64) {
	if cfg == nil || npc == nil {
		return
	}
	waypoints := npc.waypoints()
	if len(waypoints) == 0 {
		return
	}
	if tick != npc.Blackboard.StateEnteredTick {
		return
	}
	npc.clearPath()
	var params setWaypointParams
	if int(action.paramIndex) < len(cfg.setWaypointParams) {
		params = cfg.setWaypointParams[action.paramIndex]
	}
	if params.Advance {
		npc.Blackboard.WaypointIndex = (npc.Blackboard.WaypointIndex + 1) % len(waypoints)
	} else {
		if params.Index < 0 {
			return
		}
		npc.Blackboard.WaypointIndex = params.Index % len(waypoints)
	}

	idx := npc.Blackboard.WaypointIndex
	if idx < 0 {
		idx = 0
	}
	if idx >= len(waypoints) {
		idx %= len(waypoints)
	}
	waypoint := waypoints[idx]
	dx := npc.Position.XValue() - waypoint.X
	dy := npc.Position.YValue() - waypoint.Y
	npc.Blackboard.LastWaypointIndex = idx
	npc.Blackboard.WaypointBestDist = math.Hypot(dx, dy)
	npc.Blackboard.WaypointLastDist = npc.Blackboard.WaypointBestDist
	npc.Blackboard.WaypointStall = 0
}

func actionSetRandomDestination(env *runEnv, cfg *CompiledConfig, npc *NPC, action compiledAction, tick uint64) {
	if env == nil || cfg == nil || npc == nil {
		return
	}
	if tick != npc.Blackboard.StateEnteredTick {
		return
	}
	var params randomDestinationParams
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
	center := npc.home()
	if center.X == 0 && center.Y == 0 {
		center = Vec2{X: npc.Position.XValue(), Y: npc.Position.YValue()}
		npc.setHome(center)
	}
	attempts := 6
	width, height := env.width(), env.height()
	for i := 0; i < attempts; i++ {
		angle := env.randomAngle()
		distance := env.randomDistance(minRadius, radius)
		if distance <= 0 {
			distance = radius
		}
		target := Vec2{
			X: clamp(center.X+math.Cos(angle)*distance, worldpkg.PlayerHalf, width-worldpkg.PlayerHalf),
			Y: clamp(center.Y+math.Sin(angle)*distance, worldpkg.PlayerHalf, height-worldpkg.PlayerHalf),
		}
		if npc.ensurePath(target, tick) {
			return
		}
	}
	npc.clearPath()
}

func actionMoveAway(env *runEnv, cfg *CompiledConfig, npc *NPC, action compiledAction, tick uint64) {
	if env == nil || cfg == nil || npc == nil {
		return
	}
	var params moveAwayParams
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
		npc.clearPath()
		return
	}
	tx, ty, ok := env.actorPosition(targetID)
	if !ok {
		npc.clearPath()
		return
	}
	dx := npc.Position.XValue() - tx
	dy := npc.Position.YValue() - ty
	if dx == 0 && dy == 0 {
		angle := env.randomAngle()
		dx = math.Cos(angle)
		dy = math.Sin(angle)
	}
	norm := math.Hypot(dx, dy)
	if norm == 0 {
		npc.clearPath()
		return
	}
	dx /= norm
	dy /= norm
	goalDist := env.randomDistance(minDistance, distance)
	if goalDist <= 0 {
		goalDist = distance
	}
	width, height := env.width(), env.height()
	target := Vec2{
		X: clamp(npc.Position.XValue()+dx*goalDist, worldpkg.PlayerHalf, width-worldpkg.PlayerHalf),
		Y: clamp(npc.Position.YValue()+dy*goalDist, worldpkg.PlayerHalf, height-worldpkg.PlayerHalf),
	}
	npc.ensurePath(target, tick)
}

func updateBlackboard(env *runEnv, npc *NPC) {
	if env == nil || npc == nil || npc.Blackboard == nil {
		return
	}
	epsilon := npc.Blackboard.StuckEpsilon
	if epsilon <= 0 {
		epsilon = 0.5
	}
	dx := npc.Position.XValue() - npc.Blackboard.LastPos.X
	dy := npc.Position.YValue() - npc.Blackboard.LastPos.Y
	delta := math.Hypot(dx, dy)
	npc.Blackboard.LastMoveDelta = delta
	if delta < epsilon {
		if npc.Blackboard.StuckCounter < math.MaxUint8 {
			npc.Blackboard.StuckCounter++
		}
	} else {
		npc.Blackboard.StuckCounter = 0
	}
	npc.Blackboard.LastPos = Vec2{X: npc.Position.XValue(), Y: npc.Position.YValue()}

	waypoints := npc.waypoints()
	if len(waypoints) == 0 {
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
	if idx >= len(waypoints) {
		idx %= len(waypoints)
	}
	waypoint := waypoints[idx]
	dist := math.Hypot(npc.Position.XValue()-waypoint.X, npc.Position.YValue()-waypoint.Y)

	if npc.Blackboard.LastWaypointIndex != idx {
		npc.Blackboard.LastWaypointIndex = idx
		npc.Blackboard.WaypointBestDist = dist
		npc.Blackboard.WaypointLastDist = dist
		npc.Blackboard.WaypointStall = 0
		return
	}

	if npc.Blackboard.WaypointBestDist == 0 || dist+waypointProgressEps < npc.Blackboard.WaypointBestDist {
		npc.Blackboard.WaypointBestDist = dist
		npc.Blackboard.WaypointStall = 0
	} else if npc.Blackboard.WaypointStall < maxWaypointStall {
		npc.Blackboard.WaypointStall++
	}
	npc.Blackboard.WaypointLastDist = dist
}

func (env *runEnv) closestPlayer(x, y float64) (string, float64, bool) {
	if env == nil || len(env.cfg.Players) == 0 {
		return "", 0, false
	}
	bestID := ""
	bestDist := math.MaxFloat64
	for _, player := range env.cfg.Players {
		dx := player.X - x
		dy := player.Y - y
		distSq := dx*dx + dy*dy
		if distSq < bestDist-1e-6 || (math.Abs(distSq-bestDist) <= 1e-6 && player.ID < bestID) {
			bestDist = distSq
			bestID = player.ID
		}
	}
	if bestID == "" {
		return "", 0, false
	}
	return bestID, bestDist, true
}

func (env *runEnv) closestNonRatActor(npc *NPC) (string, float64, bool) {
	if env == nil || npc == nil {
		return "", 0, false
	}
	type candidate struct {
		id string
		x  float64
		y  float64
	}
	candidates := make([]candidate, 0, len(env.cfg.Players)+len(env.cfg.NPCs))
	for _, player := range env.cfg.Players {
		candidates = append(candidates, candidate{id: player.ID, x: player.X, y: player.Y})
	}
	for _, other := range env.cfg.NPCs {
		if other == nil || other.ID == npc.ID || stringsEqualFold(other.Type, "rat") {
			continue
		}
		candidates = append(candidates, candidate{id: other.ID, x: other.Position.XValue(), y: other.Position.YValue()})
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
		dx := cand.x - npc.Position.XValue()
		dy := cand.y - npc.Position.YValue()
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

func (env *runEnv) actorPosition(id string) (float64, float64, bool) {
	if env == nil || id == "" {
		return 0, 0, false
	}
	for _, player := range env.cfg.Players {
		if player.ID == id {
			return player.X, player.Y, true
		}
	}
	for _, npc := range env.cfg.NPCs {
		if npc != nil && npc.ID == id {
			return npc.Position.XValue(), npc.Position.YValue(), true
		}
	}
	return 0, 0, false
}

func (env *runEnv) randomAngle() float64 {
	if env == nil || env.cfg.RandomAngle == nil {
		return 0
	}
	return env.cfg.RandomAngle()
}

func (env *runEnv) randomDistance(min, max float64) float64 {
	if env == nil || env.cfg.RandomDistance == nil {
		if max <= min {
			return min
		}
		return min + (max-min)/2
	}
	return env.cfg.RandomDistance(min, max)
}

func (env *runEnv) deriveFacing(dx, dy float64, fallback string) string {
	if env == nil || env.cfg.DeriveFacing == nil {
		return fallback
	}
	return env.cfg.DeriveFacing(dx, dy, fallback)
}

func (env *runEnv) commandForAbility(id AbilityID) (string, bool) {
	if env == nil || env.cfg.AbilityCommand == nil {
		return "", false
	}
	return env.cfg.AbilityCommand(id)
}

func (env *runEnv) cooldownForAbility(id AbilityID) uint64 {
	if env == nil || env.cfg.AbilityCooldown == nil {
		return 0
	}
	return env.cfg.AbilityCooldown(id)
}

func (npc *NPC) waypoints() []Vec2 {
	if npc == nil || npc.Waypoints == nil {
		return nil
	}
	return *npc.Waypoints
}

func (npc *NPC) home() Vec2 {
	if npc == nil || npc.Home == nil {
		return Vec2{}
	}
	return *npc.Home
}

func (npc *NPC) setHome(value Vec2) {
	if npc == nil || npc.Home == nil {
		return
	}
	*npc.Home = value
}

func (npc *NPC) clearPath() {
	if npc == nil {
		return
	}
	if npc.Hooks.ClearPath != nil {
		npc.Hooks.ClearPath()
	}
}

func (npc *NPC) ensurePath(target Vec2, tick uint64) bool {
	if npc == nil || npc.Hooks.EnsurePath == nil {
		return false
	}
	return npc.Hooks.EnsurePath(target, tick)
}

func stringsEqualFold(a, b string) bool {
	if len(a) != len(b) {
		return strings.ToLower(a) == strings.ToLower(b)
	}
	return strings.EqualFold(a, b)
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
