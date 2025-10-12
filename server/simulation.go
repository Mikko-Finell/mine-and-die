package main

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	"mine-and-die/server/logging"
	loggingeconomy "mine-and-die/server/logging/economy"
	logginglifecycle "mine-and-die/server/logging/lifecycle"
	stats "mine-and-die/server/stats"
)

// CommandType enumerates the supported simulation commands.
type CommandType string

const (
	CommandMove      CommandType = "Move"
	CommandAction    CommandType = "Action"
	CommandHeartbeat CommandType = "Heartbeat"
	CommandSetPath   CommandType = "SetPath"
	CommandClearPath CommandType = "ClearPath"
)

// Command represents an intent captured for processing on the next tick.
type Command struct {
	OriginTick uint64
	ActorID    string
	Type       CommandType
	IssuedAt   time.Time
	Move       *MoveCommand
	Action     *ActionCommand
	Heartbeat  *HeartbeatCommand
	Path       *PathCommand
}

// MoveCommand carries the desired movement vector and facing.
type MoveCommand struct {
	DX     float64
	DY     float64
	Facing FacingDirection
}

// ActionCommand identifies an ability or interaction trigger.
type ActionCommand struct {
	Name string
}

// PathCommand identifies a navigation target for A* pathfinding.
type PathCommand struct {
	TargetX float64
	TargetY float64
}

// HeartbeatCommand updates connectivity metadata for an actor.
type HeartbeatCommand struct {
	ReceivedAt time.Time
	ClientSent int64
	RTT        time.Duration
}

// World owns the authoritative simulation state.
type World struct {
	players             map[string]*playerState
	npcs                map[string]*npcState
	effects             []*effectState
	effectTriggers      []EffectTrigger
	effectManager       *EffectManager
	obstacles           []Obstacle
	effectBehaviors     map[string]effectBehavior
	projectileTemplates map[string]*ProjectileTemplate
	statusEffectDefs    map[StatusEffectType]*StatusEffectDefinition
	nextEffectID        uint64
	nextNPCID           uint64
	nextGroundItemID    uint64
	aiLibrary           *aiLibrary
	config              worldConfig
	rng                 *rand.Rand
	seed                string
	publisher           logging.Publisher
	currentTick         uint64
	telemetry           *telemetryCounters

	groundItems       map[string]*groundItemState
	groundItemsByTile map[groundTileKey]map[string]*groundItemState
	journal           Journal
}

func (w *World) resolveStats(tick uint64) {
	for _, player := range w.players {
		player.stats.Resolve(tick)
		w.syncMaxHealth(&player.actorState, &player.version, player.ID, PatchPlayerHealth, &player.stats)
	}
	for _, npc := range w.npcs {
		npc.stats.Resolve(tick)
		w.syncMaxHealth(&npc.actorState, &npc.version, npc.ID, PatchNPCHealth, &npc.stats)
	}
}

func (w *World) syncMaxHealth(actor *actorState, version *uint64, entityID string, kind PatchKind, comp *stats.Component) {
	if w == nil || actor == nil || version == nil || comp == nil || entityID == "" {
		return
	}

	maxHealth := comp.GetDerived(stats.DerivedMaxHealth)
	if maxHealth <= 0 {
		return
	}

	w.setActorHealth(actor, version, entityID, kind, maxHealth, actor.Health)
}

// newWorld constructs an empty world with generated obstacles and seeded NPCs.
func newWorld(cfg worldConfig, publisher logging.Publisher) *World {
	normalized := cfg.normalized()

	if publisher == nil {
		publisher = logging.NopPublisher{}
	}

	capacity, maxAge := journalConfig()

	w := &World{
		players:             make(map[string]*playerState),
		npcs:                make(map[string]*npcState),
		effects:             make([]*effectState, 0),
		effectTriggers:      make([]EffectTrigger, 0),
		effectBehaviors:     newEffectBehaviors(),
		projectileTemplates: newProjectileTemplates(),
		statusEffectDefs:    newStatusEffectDefinitions(),
		aiLibrary:           globalAILibrary,
		config:              normalized,
		rng:                 newDeterministicRNG(normalized.Seed, "world"),
		seed:                normalized.Seed,
		publisher:           publisher,
		telemetry:           nil,
		groundItems:         make(map[string]*groundItemState),
		groundItemsByTile:   make(map[groundTileKey]map[string]*groundItemState),
		journal:             newJournal(capacity, maxAge),
	}
	if enableContractEffectManager {
		w.effectManager = newEffectManager(w)
	}
	w.obstacles = w.generateObstacles(normalized.ObstaclesCount)
	w.spawnInitialNPCs()
	return w
}

// Snapshot copies players, NPCs, and effects into broadcast-friendly structs.
func (w *World) Snapshot(now time.Time) ([]Player, []NPC, []Effect) {
	players := make([]Player, 0, len(w.players))
	for _, player := range w.players {
		players = append(players, player.snapshot())
	}
	npcs := make([]NPC, 0, len(w.npcs))
	for _, npc := range w.npcs {
		npcs = append(npcs, npc.snapshot())
	}
	effects := make([]Effect, 0, len(w.effects))
	for _, eff := range w.effects {
		if now.Before(eff.expiresAt) {
			effects = append(effects, eff.Effect)
		}
	}
	return players, npcs, effects
}

// flushEffectTriggersLocked drains the queued fire-and-forget triggers. Callers
// must hold the world mutex.
func (w *World) flushEffectTriggersLocked() []EffectTrigger {
	if len(w.effectTriggers) == 0 {
		return nil
	}
	drained := make([]EffectTrigger, len(w.effectTriggers))
	copy(drained, w.effectTriggers)
	w.effectTriggers = w.effectTriggers[:0]
	return drained
}

// drainPatchesLocked returns any accumulated patches for the current tick and
// clears the journal. Callers must hold the world mutex.
func (w *World) drainPatchesLocked() []Patch {
	return w.journal.DrainPatches()
}

// snapshotPatchesLocked returns a copy of any staged patches without clearing
// the journal. Callers must hold the world mutex.
func (w *World) snapshotPatchesLocked() []Patch {
	return w.journal.SnapshotPatches()
}

func (w *World) recordEffectLifecycleEvent(event EffectLifecycleEvent) {
	if w == nil || event == nil || w.effectManager == nil {
		return
	}
	switch e := event.(type) {
	case EffectSpawnEvent:
		w.journal.RecordEffectSpawn(e)
	case EffectUpdateEvent:
		w.journal.RecordEffectUpdate(e)
	case EffectEndEvent:
		w.journal.RecordEffectEnd(e)
	}
}

// HasPlayer reports whether the world currently tracks the given player.
func (w *World) HasPlayer(id string) bool {
	_, ok := w.players[id]
	return ok
}

// AddPlayer registers a new player state with the world.
func (w *World) AddPlayer(state *playerState) {
	if state == nil {
		return
	}
	state.stats.Resolve(w.currentTick)
	w.syncMaxHealth(&state.actorState, &state.version, state.ID, PatchPlayerHealth, &state.stats)
	w.players[state.ID] = state
}

// RemovePlayer drops a player from the world and returns whether it was present.
func (w *World) RemovePlayer(id string) bool {
	if _, ok := w.players[id]; !ok {
		return false
	}
	delete(w.players, id)
	return true
}

func (w *World) handleNPCDefeat(npc *npcState) {
	if npc == nil {
		return
	}
	if _, ok := w.npcs[npc.ID]; !ok {
		return
	}
	w.dropAllInventory(&npc.actorState, "death")
	delete(w.npcs, npc.ID)
	w.purgeEntityPatches(npc.ID)
}

func (w *World) pruneDefeatedNPCs() {
	if len(w.npcs) == 0 {
		return
	}
	defeated := make([]*npcState, 0)
	for _, npc := range w.npcs {
		if npc.Health <= 0 {
			defeated = append(defeated, npc)
		}
	}
	for _, npc := range defeated {
		w.handleNPCDefeat(npc)
	}
}

// Step advances the simulation by a single tick applying all staged commands.
func (w *World) Step(tick uint64, now time.Time, dt float64, commands []Command, emitEffectEvent func(EffectLifecycleEvent)) []string {
	if dt <= 0 {
		dt = 1.0 / float64(tickRate)
	}

	w.currentTick = tick

	w.resolveStats(tick)

	aiCommands := w.runAI(tick, now)
	if len(aiCommands) > 0 {
		combined := make([]Command, 0, len(aiCommands)+len(commands))
		combined = append(combined, aiCommands...)
		combined = append(combined, commands...)
		commands = combined
	}

	type stagedAction struct {
		actorID string
		command *ActionCommand
	}

	stagedActions := make([]stagedAction, 0)
	// Process commands.
	for _, cmd := range commands {
		switch cmd.Type {
		case CommandMove:
			if cmd.Move == nil {
				continue
			}
			if player, ok := w.players[cmd.ActorID]; ok {
				dx := cmd.Move.DX
				dy := cmd.Move.DY
				length := math.Hypot(dx, dy)
				if length > 1 {
					dx /= length
					dy /= length
				}
				if dx != 0 || dy != 0 {
					w.clearPlayerPath(player)
				}
				w.SetIntent(player.ID, dx, dy)
				nextFacing := deriveFacing(dx, dy, player.Facing)
				if dx == 0 && dy == 0 {
					if cmd.Move.Facing != "" {
						nextFacing = cmd.Move.Facing
					}
				}
				w.SetFacing(player.ID, nextFacing)
				if !cmd.IssuedAt.IsZero() {
					player.lastInput = cmd.IssuedAt
				} else {
					player.lastInput = now
				}
			} else if npc, ok := w.npcs[cmd.ActorID]; ok {
				dx := cmd.Move.DX
				dy := cmd.Move.DY
				length := math.Hypot(dx, dy)
				if length > 1 {
					dx /= length
					dy /= length
				}
				npc.intentX = dx
				npc.intentY = dy
				nextFacing := deriveFacing(dx, dy, npc.Facing)
				if dx == 0 && dy == 0 && cmd.Move.Facing != "" {
					nextFacing = cmd.Move.Facing
				}
				w.SetNPCFacing(npc.ID, nextFacing)
			}
		case CommandAction:
			if cmd.Action == nil {
				continue
			}
			stagedActions = append(stagedActions, stagedAction{actorID: cmd.ActorID, command: cmd.Action})
		case CommandHeartbeat:
			if cmd.Heartbeat == nil {
				continue
			}
			if player, ok := w.players[cmd.ActorID]; ok {
				player.lastHeartbeat = cmd.Heartbeat.ReceivedAt
				player.lastRTT = cmd.Heartbeat.RTT
			}
		case CommandSetPath:
			if cmd.Path == nil {
				continue
			}
			if player, ok := w.players[cmd.ActorID]; ok {
				target := vec2{X: cmd.Path.TargetX, Y: cmd.Path.TargetY}
				w.ensurePlayerPath(player, target, tick)
				if !cmd.IssuedAt.IsZero() {
					player.lastInput = cmd.IssuedAt
				} else {
					player.lastInput = now
				}
			}
		case CommandClearPath:
			if player, ok := w.players[cmd.ActorID]; ok {
				w.clearPlayerPath(player)
				w.SetIntent(player.ID, 0, 0)
				if !cmd.IssuedAt.IsZero() {
					player.lastInput = cmd.IssuedAt
				} else {
					player.lastInput = now
				}
			}
		}
	}

	w.advancePlayerPaths(tick)
	w.advanceNPCPaths(tick)

	initialPlayerPositions := make(map[string]vec2, len(w.players))
	for id, player := range w.players {
		initialPlayerPositions[id] = vec2{X: player.X, Y: player.Y}
	}

	actorsForCollisions := make([]*actorState, 0, len(w.players)+len(w.npcs))
	proposedPlayerStates := make(map[string]*actorState, len(w.players))
	width, height := w.dimensions()
	// Movement system.
	for id, player := range w.players {
		// Operate on a copy so player coordinates can be committed via
		// SetPosition after all collision resolution completes.
		scratch := player.actorState
		if player.intentX != 0 || player.intentY != 0 {
			moveActorWithObstacles(&scratch, dt, w.obstacles, width, height)
		}
		proposedPlayerStates[id] = &scratch
		actorsForCollisions = append(actorsForCollisions, &scratch)
	}
	initialNPCPositions := make(map[string]vec2, len(w.npcs))
	proposedNPCStates := make(map[string]*actorState, len(w.npcs))
	for id, npc := range w.npcs {
		initialNPCPositions[id] = vec2{X: npc.X, Y: npc.Y}
		scratch := npc.actorState
		if npc.intentX != 0 || npc.intentY != 0 {
			moveActorWithObstacles(&scratch, dt, w.obstacles, width, height)
		}
		proposedNPCStates[id] = &scratch
		actorsForCollisions = append(actorsForCollisions, &scratch)
	}

	resolveActorCollisions(actorsForCollisions, w.obstacles, width, height)

	proposedPositions := make(map[string]vec2, len(proposedPlayerStates))
	for id, state := range proposedPlayerStates {
		if state == nil {
			continue
		}
		proposedPositions[id] = vec2{X: state.X, Y: state.Y}
	}

	w.applyPlayerPositionMutations(initialPlayerPositions, proposedPositions)

	proposedNPCPositions := make(map[string]vec2, len(proposedNPCStates))
	for id, state := range proposedNPCStates {
		if state == nil {
			continue
		}
		proposedNPCPositions[id] = vec2{X: state.X, Y: state.Y}
	}

	w.applyNPCPositionMutations(initialNPCPositions, proposedNPCPositions)

	// Ability and effect staging.
	for _, action := range stagedActions {
		switch action.command.Name {
		case effectTypeAttack:
			if enableContractEffectManager && w.effectManager != nil {
				w.effectManager.EnqueueIntent(EffectIntent{
					TypeID:        effectTypeAttack,
					Delivery:      DeliveryKindArea,
					SourceActorID: action.actorID,
					Geometry:      EffectGeometry{Shape: GeometryShapeRect},
				})
			}
			w.triggerMeleeAttack(action.actorID, tick, now)
		case effectTypeFireball:
			if enableContractEffectManager && w.effectManager != nil {
				w.effectManager.EnqueueIntent(EffectIntent{
					TypeID:        effectTypeFireball,
					Delivery:      DeliveryKindArea,
					SourceActorID: action.actorID,
					Geometry:      EffectGeometry{Shape: GeometryShapeCircle},
				})
			}
			w.triggerFireball(action.actorID, now)
		}
	}

	// Environmental systems.
	actorsForHazards := make([]*actorState, 0, len(w.players)+len(w.npcs))
	for _, player := range w.players {
		actorsForHazards = append(actorsForHazards, &player.actorState)
	}
	for _, npc := range w.npcs {
		actorsForHazards = append(actorsForHazards, &npc.actorState)
	}
	w.applyEnvironmentalStatusEffects(actorsForHazards, now)

	w.advanceStatusEffects(now)
	if enableContractEffectManager && w.effectManager != nil {
		dispatcher := w.recordEffectLifecycleEvent
		if emitEffectEvent != nil {
			dispatcher = func(event EffectLifecycleEvent) {
				w.recordEffectLifecycleEvent(event)
				emitEffectEvent(event)
			}
		}
		w.effectManager.RunTick(Tick(int64(tick)), dispatcher)
	}
	w.advanceEffects(now, dt)
	w.pruneEffects(now)
	w.pruneDefeatedNPCs()

	// Lifecycle system: remove stale players.
	cutoff := now.Add(-disconnectAfter)
	removedPlayers := make([]string, 0)
	for id, player := range w.players {
		if player.lastHeartbeat.IsZero() {
			continue
		}
		if player.lastHeartbeat.Before(cutoff) {
			if w.publisher != nil {
				logginglifecycle.PlayerDisconnected(
					context.Background(),
					w.publisher,
					w.currentTick,
					logging.EntityRef{ID: id, Kind: logging.EntityKind("player")},
					logginglifecycle.PlayerDisconnectedPayload{Reason: "timeout"},
					map[string]any{"lastHeartbeat": player.lastHeartbeat},
				)
			}
			delete(w.players, id)
			removedPlayers = append(removedPlayers, id)
		}
	}

	return removedPlayers
}

// applyPlayerPositionMutations commits any movement resolved during the tick
// via the SetPosition write barrier so patches and versions stay consistent.
func (w *World) applyPlayerPositionMutations(initial map[string]vec2, proposed map[string]vec2) {
	if w == nil {
		return
	}

	for id, player := range w.players {
		start, ok := initial[id]
		if !ok {
			start = vec2{X: player.X, Y: player.Y}
		}

		target, ok := proposed[id]
		if !ok {
			target = vec2{X: player.X, Y: player.Y}
		}

		if positionsEqual(start.X, start.Y, target.X, target.Y) {
			continue
		}

		w.SetPosition(id, target.X, target.Y)
	}
}

// applyNPCPositionMutations commits NPC movement resolved during the tick through
// the NPC write barrier so patches and versions stay consistent.
func (w *World) applyNPCPositionMutations(initial map[string]vec2, proposed map[string]vec2) {
	if w == nil {
		return
	}

	for id, npc := range w.npcs {
		start, ok := initial[id]
		if !ok {
			start = vec2{X: npc.X, Y: npc.Y}
		}

		target, ok := proposed[id]
		if !ok {
			target = vec2{X: npc.X, Y: npc.Y}
		}

		if positionsEqual(start.X, start.Y, target.X, target.Y) {
			continue
		}

		w.SetNPCPosition(id, target.X, target.Y)
	}
}

func (w *World) spawnInitialNPCs() {
	if !w.config.NPCs {
		return
	}

	goblinTarget := w.config.GoblinCount
	ratTarget := w.config.RatCount
	if goblinTarget <= 0 && ratTarget <= 0 {
		return
	}

	centerX := defaultSpawnX
	centerY := defaultSpawnY

	goblinsSpawned := 0
	if goblinTarget >= 1 {
		patrolOffset := 160.0
		w.spawnGoblinAt(centerX-patrolOffset, centerY-patrolOffset, []vec2{
			{X: centerX - patrolOffset, Y: centerY - patrolOffset},
			{X: centerX + patrolOffset, Y: centerY - patrolOffset},
			{X: centerX + patrolOffset, Y: centerY + patrolOffset},
			{X: centerX - patrolOffset, Y: centerY + patrolOffset},
		}, 12, 1)
		goblinsSpawned++
	}
	if goblinTarget >= 2 {
		topLeftX := centerX + 120.0
		height := 220.0
		width := 220.0
		topLeftY := centerY - height/2
		w.spawnGoblinAt(topLeftX, topLeftY, []vec2{
			{X: topLeftX, Y: topLeftY},
			{X: topLeftX + width, Y: topLeftY},
			{X: topLeftX + width, Y: topLeftY + height},
			{X: topLeftX, Y: topLeftY + height},
		}, 8, 1)
		goblinsSpawned++
	}
	extraGoblins := goblinTarget - goblinsSpawned
	if extraGoblins > 0 {
		w.spawnExtraGoblins(extraGoblins)
	}

	ratsSpawned := 0
	if ratTarget >= 1 {
		w.spawnRatAt(centerX-200, centerY+240)
		ratsSpawned++
	}
	extraRats := ratTarget - ratsSpawned
	if extraRats > 0 {
		w.spawnExtraRats(extraRats)
	}
}

func (w *World) spawnGoblinAt(x, y float64, waypoints []vec2, goldQty, potionQty int) {
	w.nextNPCID++
	id := fmt.Sprintf("npc-goblin-%d", w.nextNPCID)
	inventory := NewInventory()
	if goldQty > 0 {
		if _, err := inventory.AddStack(ItemStack{Type: ItemTypeGold, Quantity: goldQty}); err != nil {
			loggingeconomy.ItemGrantFailed(
				context.Background(),
				w.publisher,
				w.currentTick,
				logging.EntityRef{ID: id, Kind: logging.EntityKind("npc")},
				loggingeconomy.ItemGrantFailedPayload{ItemType: string(ItemTypeGold), Quantity: goldQty, Reason: "seed_goblin"},
				map[string]any{"error": err.Error()},
			)
		}
	}
	if potionQty > 0 {
		if _, err := inventory.AddStack(ItemStack{Type: ItemTypeHealthPotion, Quantity: potionQty}); err != nil {
			loggingeconomy.ItemGrantFailed(
				context.Background(),
				w.publisher,
				w.currentTick,
				logging.EntityRef{ID: id, Kind: logging.EntityKind("npc")},
				loggingeconomy.ItemGrantFailedPayload{ItemType: string(ItemTypeHealthPotion), Quantity: potionQty, Reason: "seed_goblin"},
				map[string]any{"error": err.Error()},
			)
		}
	}

	statsComp := stats.DefaultComponent(stats.ArchetypeGoblin)
	maxHealth := statsComp.GetDerived(stats.DerivedMaxHealth)

	goblin := &npcState{
		actorState: actorState{
			Actor: Actor{
				ID:        id,
				X:         x,
				Y:         y,
				Facing:    defaultFacing,
				Health:    maxHealth,
				MaxHealth: maxHealth,
				Inventory: inventory,
				Equipment: NewEquipment(),
			},
		},
		stats:            statsComp,
		Type:             NPCTypeGoblin,
		ExperienceReward: 25,
		Waypoints:        append([]vec2(nil), waypoints...),
	}
	w.initializeGoblinState(goblin)
}

func (w *World) initializeGoblinState(goblin *npcState) {
	if goblin == nil {
		return
	}
	if w.aiLibrary != nil {
		if cfg := w.aiLibrary.ConfigForType(NPCTypeGoblin); cfg != nil {
			goblin.AIConfigID = cfg.id
			goblin.AIState = cfg.initialState
			cfg.applyDefaults(&goblin.Blackboard)
		}
	}
	if goblin.Blackboard.ArriveRadius <= 0 {
		goblin.Blackboard.ArriveRadius = 16
	}
	if goblin.Blackboard.PauseTicks == 0 {
		goblin.Blackboard.PauseTicks = 30
	}
	if goblin.Blackboard.StuckEpsilon <= 0 {
		goblin.Blackboard.StuckEpsilon = 0.5
	}
	if goblin.Blackboard.WaypointIndex < 0 || goblin.Blackboard.WaypointIndex >= len(goblin.Waypoints) {
		goblin.Blackboard.WaypointIndex = 0
	}
	goblin.Blackboard.NextDecisionAt = 0
	goblin.Blackboard.LastWaypointIndex = -1

	resolveObstaclePenetration(&goblin.actorState, w.obstacles, w.width(), w.height())
	goblin.Blackboard.LastPos = vec2{X: goblin.X, Y: goblin.Y}
	w.npcs[goblin.ID] = goblin
}

func (w *World) spawnExtraGoblins(count int) {
	if count <= 0 {
		return
	}
	rng := w.subsystemRNG("npcs.extraGoblin")
	const patrolRadius = 60.0
	width, height := w.dimensions()
	minX := obstacleSpawnMargin + patrolRadius
	maxX := width - obstacleSpawnMargin - patrolRadius
	if maxX <= minX {
		minX = playerHalf + patrolRadius
		maxX = width - playerHalf - patrolRadius
	}
	minY := obstacleSpawnMargin + patrolRadius
	maxY := height - obstacleSpawnMargin - patrolRadius
	if maxY <= minY {
		minY = playerHalf + patrolRadius
		maxY = height - playerHalf - patrolRadius
	}

	centralMinX, centralMaxX := centralCenterRange(width, defaultSpawnX, obstacleSpawnMargin, patrolRadius)
	if centralMaxX >= centralMinX {
		minX = centralMinX
		maxX = centralMaxX
	}
	centralMinY, centralMaxY := centralCenterRange(height, defaultSpawnY, obstacleSpawnMargin, patrolRadius)
	if centralMaxY >= centralMinY {
		minY = centralMinY
		maxY = centralMaxY
	}

	for i := 0; i < count; i++ {
		x := minX
		if maxX > minX {
			x = minX + rng.Float64()*(maxX-minX)
		}
		y := minY
		if maxY > minY {
			y = minY + rng.Float64()*(maxY-minY)
		}

		topLeftX := clamp(x-patrolRadius, playerHalf, width-playerHalf)
		topLeftY := clamp(y-patrolRadius, playerHalf, height-playerHalf)
		topRightX := clamp(x+patrolRadius, playerHalf, width-playerHalf)
		bottomY := clamp(y+patrolRadius, playerHalf, height-playerHalf)

		waypoints := []vec2{
			{X: topLeftX, Y: topLeftY},
			{X: topRightX, Y: topLeftY},
			{X: topRightX, Y: bottomY},
			{X: topLeftX, Y: bottomY},
		}

		w.spawnGoblinAt(topLeftX, topLeftY, waypoints, 10, 1)
	}
}

func (w *World) spawnRatAt(x, y float64) {
	w.nextNPCID++
	id := fmt.Sprintf("npc-rat-%d", w.nextNPCID)
	statsComp := stats.DefaultComponent(stats.ArchetypeRat)
	maxHealth := statsComp.GetDerived(stats.DerivedMaxHealth)

	rat := &npcState{
		actorState: actorState{
			Actor: Actor{
				ID:        id,
				X:         x,
				Y:         y,
				Facing:    defaultFacing,
				Health:    maxHealth,
				MaxHealth: maxHealth,
				Inventory: NewInventory(),
				Equipment: NewEquipment(),
			},
		},
		stats:            statsComp,
		Type:             NPCTypeRat,
		ExperienceReward: 8,
		Home:             vec2{X: x, Y: y},
	}
	if _, err := rat.Inventory.AddStack(ItemStack{Type: ItemTypeRatTail, Quantity: 1}); err != nil {
		_ = rat.Inventory.RemoveAllOf(ItemTypeRatTail)
	}
	w.initializeRatState(rat)
}

func (w *World) initializeRatState(rat *npcState) {
	if rat == nil {
		return
	}
	if w.aiLibrary != nil {
		if cfg := w.aiLibrary.ConfigForType(NPCTypeRat); cfg != nil {
			rat.AIConfigID = cfg.id
			rat.AIState = cfg.initialState
			cfg.applyDefaults(&rat.Blackboard)
		}
	}
	if rat.Blackboard.ArriveRadius <= 0 {
		rat.Blackboard.ArriveRadius = 10
	}
	if rat.Blackboard.PauseTicks == 0 {
		rat.Blackboard.PauseTicks = 20
	}
	if rat.Blackboard.StuckEpsilon <= 0 {
		rat.Blackboard.StuckEpsilon = 0.5
	}
	rat.Blackboard.WaypointIndex = 0
	rat.Blackboard.NextDecisionAt = 0
	rat.Blackboard.LastWaypointIndex = -1

	resolveObstaclePenetration(&rat.actorState, w.obstacles, w.width(), w.height())
	rat.Blackboard.LastPos = vec2{X: rat.X, Y: rat.Y}
	w.npcs[rat.ID] = rat
}

func (w *World) spawnExtraRats(count int) {
	if count <= 0 {
		return
	}
	rng := w.subsystemRNG("npcs.extra")
	width, height := w.dimensions()
	minX, maxX := centralCenterRange(width, defaultSpawnX, obstacleSpawnMargin, playerHalf)
	minY, maxY := centralCenterRange(height, defaultSpawnY, obstacleSpawnMargin, playerHalf)

	for i := 0; i < count; i++ {
		x := minX
		if maxX > minX {
			x = minX + rng.Float64()*(maxX-minX)
		}
		y := minY
		if maxY > minY {
			y = minY + rng.Float64()*(maxY-minY)
		}
		w.spawnRatAt(x, y)
	}
}
