package main

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"mine-and-die/server/logging"
	loggingeconomy "mine-and-die/server/logging/economy"
	logginglifecycle "mine-and-die/server/logging/lifecycle"
)

// CommandType enumerates the supported simulation commands.
type CommandType string

const (
	CommandMove       CommandType = "Move"
	CommandAction     CommandType = "Action"
	CommandHeartbeat  CommandType = "Heartbeat"
	CommandSetPath    CommandType = "SetPath"
	CommandClearPath  CommandType = "ClearPath"
	CommandDropGold   CommandType = "DropGold"
	CommandPickupGold CommandType = "PickupGold"
)

// Command represents an intent captured for processing on the next tick.
type Command struct {
	OriginTick    uint64
	ActorID       string
	Type          CommandType
	IssuedAt      time.Time
	Move          *MoveCommand
	Action        *ActionCommand
	Heartbeat     *HeartbeatCommand
	Path          *PathCommand
	DropGold      *DropGoldCommand
	PickupGold    *PickupGoldCommand
	ConsoleResult chan<- consoleCommandResult
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

// DropGoldCommand requests dropping a quantity of gold at the actor's position.
type DropGoldCommand struct {
	Quantity int
}

// PickupGoldCommand requests picking up nearby ground gold stacks.
type PickupGoldCommand struct{}

// HeartbeatCommand updates connectivity metadata for an actor.
type HeartbeatCommand struct {
	ReceivedAt time.Time
	ClientSent int64
	RTT        time.Duration
}

type consoleCommandResult struct {
	Command  string
	Status   string
	Quantity int
	Reason   string
}

const (
	consoleStatusOK    = "ok"
	consoleStatusError = "error"
)

const (
	consoleReasonInvalidQuantity  = "invalid_quantity"
	consoleReasonInsufficientGold = "insufficient_quantity"
	consoleReasonActorMissing     = "actor_missing"
	consoleReasonNotFound         = "not_found"
	consoleReasonOutOfRange       = "out_of_range"
	consoleReasonInventoryError   = "inventory_error"
	consoleReasonInvalidRequest   = "invalid_request"
	consoleReasonTimeout          = "timeout"
)

func sendConsoleResult(ch chan<- consoleCommandResult, result consoleCommandResult) {
	if ch == nil {
		return
	}
	ch <- result
	close(ch)
}

// World owns the authoritative simulation state.
type World struct {
	players             map[string]*playerState
	npcs                map[string]*npcState
	effects             []*effectState
	effectTriggers      []EffectTrigger
	obstacles           []Obstacle
	effectBehaviors     map[string]effectBehavior
	projectileTemplates map[string]*ProjectileTemplate
	conditionDefs       map[ConditionType]*ConditionDefinition
	nextEffectID        uint64
	nextNPCID           uint64
	aiLibrary           *aiLibrary
	config              worldConfig
	rng                 *rand.Rand
	seed                string
	publisher           logging.Publisher
	currentTick         uint64
	groundItems         map[string]*GroundItem
	groundIndex         map[string]string
	nextGroundItemID    uint64
}

// newWorld constructs an empty world with generated obstacles and seeded NPCs.
func newWorld(cfg worldConfig, publisher logging.Publisher) *World {
	normalized := cfg.normalized()

	if publisher == nil {
		publisher = logging.NopPublisher{}
	}

	w := &World{
		players:             make(map[string]*playerState),
		npcs:                make(map[string]*npcState),
		effects:             make([]*effectState, 0),
		effectTriggers:      make([]EffectTrigger, 0),
		effectBehaviors:     newEffectBehaviors(),
		projectileTemplates: newProjectileTemplates(),
		conditionDefs:       newConditionDefinitions(),
		aiLibrary:           globalAILibrary,
		config:              normalized,
		rng:                 newDeterministicRNG(normalized.Seed, "world"),
		seed:                normalized.Seed,
		publisher:           publisher,
		groundItems:         make(map[string]*GroundItem),
		groundIndex:         make(map[string]string),
	}
	w.obstacles = w.generateObstacles(normalized.ObstaclesCount)
	w.spawnInitialNPCs()
	return w
}

// Snapshot copies players, NPCs, effects, and ground items into broadcast-friendly structs.
func (w *World) Snapshot(now time.Time) ([]Player, []NPC, []Effect, []GroundItem) {
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
	groundItems := make([]GroundItem, 0, len(w.groundItems))
	if len(w.groundItems) > 0 {
		ids := make([]string, 0, len(w.groundItems))
		for id := range w.groundItems {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			item := w.groundItems[id]
			if item == nil || item.Qty <= 0 {
				continue
			}
			groundItems = append(groundItems, GroundItem{ID: item.ID, X: item.X, Y: item.Y, Qty: item.Qty})
		}
	}
	return players, npcs, effects, groundItems
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
	w.dropAllGold(&npc.actorState, "death")
	delete(w.npcs, npc.ID)
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
func (w *World) Step(tick uint64, now time.Time, dt float64, commands []Command) []string {
	if dt <= 0 {
		dt = 1.0 / float64(tickRate)
	}

	w.currentTick = tick

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
				player.intentX = dx
				player.intentY = dy
				player.Facing = deriveFacing(dx, dy, player.Facing)
				if dx == 0 && dy == 0 {
					if cmd.Move.Facing != "" {
						player.Facing = cmd.Move.Facing
					}
				}
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
				npc.Facing = deriveFacing(dx, dy, npc.Facing)
				if dx == 0 && dy == 0 {
					if cmd.Move.Facing != "" {
						npc.Facing = cmd.Move.Facing
					}
				}
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
				player.intentX = 0
				player.intentY = 0
				if !cmd.IssuedAt.IsZero() {
					player.lastInput = cmd.IssuedAt
				} else {
					player.lastInput = now
				}
			}
		case CommandDropGold:
			result := consoleCommandResult{Command: "drop_gold", Status: consoleStatusError}
			if cmd.DropGold == nil {
				result.Reason = consoleReasonInvalidRequest
				sendConsoleResult(cmd.ConsoleResult, result)
				continue
			}
			result = w.handleDropGoldCommand(cmd.ActorID, cmd.DropGold)
			sendConsoleResult(cmd.ConsoleResult, result)
		case CommandPickupGold:
			result := consoleCommandResult{Command: "pickup_gold", Status: consoleStatusError}
			if cmd.PickupGold == nil {
				result.Reason = consoleReasonInvalidRequest
				sendConsoleResult(cmd.ConsoleResult, result)
				continue
			}
			result = w.handlePickupGoldCommand(cmd.ActorID, cmd.PickupGold)
			sendConsoleResult(cmd.ConsoleResult, result)
		}
	}

	w.advancePlayerPaths(tick)
	w.advanceNPCPaths(tick)

	actors := make([]*actorState, 0, len(w.players)+len(w.npcs))
	// Movement system.
	for _, player := range w.players {
		if player.intentX != 0 || player.intentY != 0 {
			moveActorWithObstacles(&player.actorState, dt, w.obstacles)
		}
		actors = append(actors, &player.actorState)
	}
	for _, npc := range w.npcs {
		if npc.intentX != 0 || npc.intentY != 0 {
			moveActorWithObstacles(&npc.actorState, dt, w.obstacles)
		}
		actors = append(actors, &npc.actorState)
	}

	resolveActorCollisions(actors, w.obstacles)

	// Ability and effect staging.
	for _, action := range stagedActions {
		switch action.command.Name {
		case effectTypeAttack:
			w.triggerMeleeAttack(action.actorID, tick, now)
		case effectTypeFireball:
			w.triggerFireball(action.actorID, now)
		}
	}

	// Environmental systems.
	w.applyEnvironmentalConditions(actors, now)

	w.advanceConditions(now)
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

func (w *World) handleDropGoldCommand(actorID string, cmd *DropGoldCommand) consoleCommandResult {
	result := consoleCommandResult{Command: "drop_gold", Status: consoleStatusError}
	if cmd == nil {
		result.Reason = consoleReasonInvalidRequest
		return result
	}
	quantity := cmd.Quantity
	if quantity <= 0 {
		result.Reason = consoleReasonInvalidQuantity
		return result
	}
	player, ok := w.players[actorID]
	if !ok {
		result.Reason = consoleReasonActorMissing
		return result
	}
	slotIndex := -1
	for i := range player.Inventory.Slots {
		if player.Inventory.Slots[i].Item.Type == ItemTypeGold {
			slotIndex = i
			break
		}
	}
	if slotIndex == -1 || player.Inventory.Slots[slotIndex].Item.Quantity < quantity {
		result.Reason = consoleReasonInsufficientGold
		return result
	}
	if _, err := player.Inventory.RemoveQuantity(slotIndex, quantity); err != nil {
		result.Reason = consoleReasonInsufficientGold
		return result
	}
	w.spawnGroundGold(player.X, player.Y, quantity)
	loggingeconomy.GoldDropped(
		context.Background(),
		w.publisher,
		w.currentTick,
		w.entityRef(actorID),
		loggingeconomy.GoldDroppedPayload{Quantity: quantity, Reason: "manual"},
		nil,
	)
	result.Status = consoleStatusOK
	result.Quantity = quantity
	return result
}

func (w *World) handlePickupGoldCommand(actorID string, cmd *PickupGoldCommand) consoleCommandResult {
	result := consoleCommandResult{Command: "pickup_gold", Status: consoleStatusError}
	if cmd == nil {
		result.Reason = consoleReasonInvalidRequest
		return result
	}
	player, ok := w.players[actorID]
	if !ok {
		result.Reason = consoleReasonActorMissing
		return result
	}
	item, distance := w.closestGroundItem(player.X, player.Y)
	if item == nil {
		result.Reason = consoleReasonNotFound
		loggingeconomy.GoldPickupFailed(
			context.Background(),
			w.publisher,
			w.currentTick,
			w.entityRef(actorID),
			loggingeconomy.GoldPickupFailedPayload{Reason: consoleReasonNotFound},
			nil,
		)
		return result
	}
	if distance > tileSize+1e-6 {
		result.Reason = consoleReasonOutOfRange
		loggingeconomy.GoldPickupFailed(
			context.Background(),
			w.publisher,
			w.currentTick,
			w.entityRef(actorID),
			loggingeconomy.GoldPickupFailedPayload{Reason: consoleReasonOutOfRange},
			nil,
		)
		return result
	}
	qty := item.Qty
	if qty <= 0 {
		result.Reason = consoleReasonNotFound
		w.removeGroundItem(item.ID)
		loggingeconomy.GoldPickupFailed(
			context.Background(),
			w.publisher,
			w.currentTick,
			w.entityRef(actorID),
			loggingeconomy.GoldPickupFailedPayload{Reason: consoleReasonNotFound},
			map[string]any{"itemId": item.ID},
		)
		return result
	}
	if _, err := player.Inventory.AddStack(ItemStack{Type: ItemTypeGold, Quantity: qty}); err != nil {
		result.Reason = consoleReasonInventoryError
		loggingeconomy.GoldPickupFailed(
			context.Background(),
			w.publisher,
			w.currentTick,
			w.entityRef(actorID),
			loggingeconomy.GoldPickupFailedPayload{Reason: consoleReasonInventoryError},
			map[string]any{"error": err.Error()},
		)
		return result
	}
	w.removeGroundItem(item.ID)
	loggingeconomy.GoldPickedUp(
		context.Background(),
		w.publisher,
		w.currentTick,
		w.entityRef(actorID),
		loggingeconomy.GoldPickedUpPayload{Quantity: qty},
		nil,
	)
	result.Status = consoleStatusOK
	result.Quantity = qty
	return result
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

	goblin := &npcState{
		actorState: actorState{
			Actor: Actor{
				ID:        id,
				X:         x,
				Y:         y,
				Facing:    defaultFacing,
				Health:    60,
				MaxHealth: 60,
				Inventory: inventory,
			},
		},
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

	resolveObstaclePenetration(&goblin.actorState, w.obstacles)
	goblin.Blackboard.LastPos = vec2{X: goblin.X, Y: goblin.Y}
	w.npcs[goblin.ID] = goblin
}

func (w *World) spawnExtraGoblins(count int) {
	if count <= 0 {
		return
	}
	rng := w.subsystemRNG("npcs.extraGoblin")
	const patrolRadius = 60.0
	minX := obstacleSpawnMargin + patrolRadius
	maxX := worldWidth - obstacleSpawnMargin - patrolRadius
	if maxX <= minX {
		minX = playerHalf + patrolRadius
		maxX = worldWidth - playerHalf - patrolRadius
	}
	minY := obstacleSpawnMargin + patrolRadius
	maxY := worldHeight - obstacleSpawnMargin - patrolRadius
	if maxY <= minY {
		minY = playerHalf + patrolRadius
		maxY = worldHeight - playerHalf - patrolRadius
	}

	centralMinX, centralMaxX := centralCenterRange(worldWidth, defaultSpawnX, obstacleSpawnMargin, patrolRadius)
	if centralMaxX >= centralMinX {
		minX = centralMinX
		maxX = centralMaxX
	}
	centralMinY, centralMaxY := centralCenterRange(worldHeight, defaultSpawnY, obstacleSpawnMargin, patrolRadius)
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

		topLeftX := clamp(x-patrolRadius, playerHalf, worldWidth-playerHalf)
		topLeftY := clamp(y-patrolRadius, playerHalf, worldHeight-playerHalf)
		topRightX := clamp(x+patrolRadius, playerHalf, worldWidth-playerHalf)
		bottomY := clamp(y+patrolRadius, playerHalf, worldHeight-playerHalf)

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
	rat := &npcState{
		actorState: actorState{
			Actor: Actor{
				ID:        id,
				X:         x,
				Y:         y,
				Facing:    defaultFacing,
				Health:    18,
				MaxHealth: 18,
				Inventory: NewInventory(),
			},
		},
		Type:             NPCTypeRat,
		ExperienceReward: 8,
		Home:             vec2{X: x, Y: y},
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

	resolveObstaclePenetration(&rat.actorState, w.obstacles)
	rat.Blackboard.LastPos = vec2{X: rat.X, Y: rat.Y}
	w.npcs[rat.ID] = rat
}

func (w *World) spawnExtraRats(count int) {
	if count <= 0 {
		return
	}
	rng := w.subsystemRNG("npcs.extra")
	minX, maxX := centralCenterRange(worldWidth, defaultSpawnX, obstacleSpawnMargin, playerHalf)
	minY, maxY := centralCenterRange(worldHeight, defaultSpawnY, obstacleSpawnMargin, playerHalf)

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
