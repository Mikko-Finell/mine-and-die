package main

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"
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

// EventType captures the discrete kinds of simulation output.
type EventType string

const (
	EventActorMoved     EventType = "ActorMoved"
	EventHealthChanged  EventType = "HealthChanged"
	EventEffectSpawned  EventType = "EffectSpawned"
	EventItemAdded      EventType = "ItemAdded"
	EventActorDespawned EventType = "ActorDespawned"
	EventAIStateChanged EventType = "AIStateChanged"
)

// Event describes a state change emitted during a tick.
type Event struct {
	Tick     uint64
	EntityID string
	Type     EventType
	Payload  map[string]any
}

// StepOutput aggregates the results of a simulation step.
type StepOutput struct {
	Events           []Event
	RemovedPlayerIDs []string
}

// World owns the authoritative simulation state.
type World struct {
	players         map[string]*playerState
	npcs            map[string]*npcState
	effects         []*effectState
	obstacles       []Obstacle
	effectBehaviors map[string]effectBehavior
	nextEffectID    uint64
	nextNPCID       uint64
	aiLibrary       *aiLibrary
	config          worldConfig
	rng             *rand.Rand
	seed            string
}

// newWorld constructs an empty world with generated obstacles and seeded NPCs.
func newWorld(cfg worldConfig) *World {
	normalized := cfg.normalized()

	w := &World{
		players:         make(map[string]*playerState),
		npcs:            make(map[string]*npcState),
		effects:         make([]*effectState, 0),
		effectBehaviors: newEffectBehaviors(),
		aiLibrary:       globalAILibrary,
		config:          normalized,
		rng:             newDeterministicRNG(normalized.Seed, "world"),
		seed:            normalized.Seed,
	}
	w.obstacles = w.generateObstacles(obstacleCount)
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

func (w *World) handleNPCDefeat(npc *npcState, tick uint64, output *StepOutput) {
	if npc == nil {
		return
	}
	if _, ok := w.npcs[npc.ID]; !ok {
		return
	}
	delete(w.npcs, npc.ID)
	if output == nil {
		return
	}
	payload := map[string]any{"reason": "defeated"}
	output.Events = append(output.Events, Event{
		Tick:     tick,
		EntityID: npc.ID,
		Type:     EventActorDespawned,
		Payload:  payload,
	})
}

func (w *World) pruneDefeatedNPCs(tick uint64, output *StepOutput) {
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
		w.handleNPCDefeat(npc, tick, output)
	}
}

// Step advances the simulation by a single tick applying all staged commands.
func (w *World) Step(tick uint64, now time.Time, dt float64, commands []Command) StepOutput {
	if dt <= 0 {
		dt = 1.0 / float64(tickRate)
	}
	output := StepOutput{Events: make([]Event, 0)}

	aiCommands, aiEvents := w.runAI(tick, now)
	if len(aiEvents) > 0 {
		output.Events = append(output.Events, aiEvents...)
	}
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
		}
	}

	w.advancePlayerPaths(tick)
	w.advanceNPCPaths(tick)

	actors := make([]*actorState, 0, len(w.players)+len(w.npcs))
	// Movement system.
	for _, player := range w.players {
		prevX, prevY := player.X, player.Y
		if player.intentX != 0 || player.intentY != 0 {
			moveActorWithObstacles(&player.actorState, dt, w.obstacles)
		}
		actors = append(actors, &player.actorState)
		if prevX != player.X || prevY != player.Y {
			output.Events = append(output.Events, Event{
				Tick:     tick,
				EntityID: player.ID,
				Type:     EventActorMoved,
				Payload: map[string]any{
					"x": player.X,
					"y": player.Y,
				},
			})
		}
	}
	for _, npc := range w.npcs {
		prevX, prevY := npc.X, npc.Y
		if npc.intentX != 0 || npc.intentY != 0 {
			moveActorWithObstacles(&npc.actorState, dt, w.obstacles)
		}
		actors = append(actors, &npc.actorState)
		if prevX != npc.X || prevY != npc.Y {
			output.Events = append(output.Events, Event{
				Tick:     tick,
				EntityID: npc.ID,
				Type:     EventActorMoved,
				Payload: map[string]any{
					"x": npc.X,
					"y": npc.Y,
				},
			})
		}
	}

	resolveActorCollisions(actors, w.obstacles)

	// Ability and effect staging.
	for _, action := range stagedActions {
		switch action.command.Name {
		case effectTypeAttack:
			w.triggerMeleeAttack(action.actorID, now, tick, &output)
		case effectTypeFireball:
			w.triggerFireball(action.actorID, now, tick, &output)
		}
	}

	// Environmental systems.
	w.applyEnvironmentalDamage(actors, dt, tick, &output)

	w.advanceEffects(now, dt, tick, &output)
	w.pruneEffects(now)
	w.pruneDefeatedNPCs(tick, &output)

	// Lifecycle system: remove stale players.
	cutoff := now.Add(-disconnectAfter)
	for id, player := range w.players {
		if player.lastHeartbeat.IsZero() {
			continue
		}
		if player.lastHeartbeat.Before(cutoff) {
			delete(w.players, id)
			output.RemovedPlayerIDs = append(output.RemovedPlayerIDs, id)
			output.Events = append(output.Events, Event{
				Tick:     tick,
				EntityID: id,
				Type:     EventActorDespawned,
				Payload:  map[string]any{"reason": "heartbeatTimeout"},
			})
		}
	}

	return output
}

func (w *World) spawnInitialNPCs() {
	if !w.config.NPCs {
		return
	}
	inventory := NewInventory()
	if _, err := inventory.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 12}); err != nil {
		log.Printf("failed to seed goblin gold: %v", err)
	}
	if _, err := inventory.AddStack(ItemStack{Type: ItemTypeHealthPotion, Quantity: 1}); err != nil {
		log.Printf("failed to seed goblin potion: %v", err)
	}

	w.nextNPCID++
	id := fmt.Sprintf("npc-goblin-%d", w.nextNPCID)
	goblin := &npcState{
		actorState: actorState{
			Actor: Actor{
				ID:        id,
				X:         360,
				Y:         260,
				Facing:    defaultFacing,
				Health:    60,
				MaxHealth: 60,
				Inventory: inventory,
			},
		},
		Type:             NPCTypeGoblin,
		ExperienceReward: 25,
		Waypoints: []vec2{
			{X: 360, Y: 260},
			{X: 480, Y: 260},
			{X: 480, Y: 380},
			{X: 360, Y: 380},
		},
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
	w.npcs[id] = goblin

	w.nextNPCID++
	id = fmt.Sprintf("npc-goblin-%d", w.nextNPCID)
	southGoblinInventory := NewInventory()
	if _, err := southGoblinInventory.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 8}); err != nil {
		log.Printf("failed to seed goblin gold: %v", err)
	}
	if _, err := southGoblinInventory.AddStack(ItemStack{Type: ItemTypeHealthPotion, Quantity: 1}); err != nil {
		log.Printf("failed to seed goblin potion: %v", err)
	}
	southernGoblin := &npcState{
		actorState: actorState{
			Actor: Actor{
				ID:        id,
				X:         640,
				Y:         480,
				Facing:    defaultFacing,
				Health:    60,
				MaxHealth: 60,
				Inventory: southGoblinInventory,
			},
		},
		Type:             NPCTypeGoblin,
		ExperienceReward: 25,
		Waypoints: []vec2{
			{X: 640, Y: 480},
			{X: 760, Y: 480},
			{X: 760, Y: 600},
			{X: 520, Y: 600},
			{X: 520, Y: 480},
		},
	}

	if w.aiLibrary != nil {
		if cfg := w.aiLibrary.ConfigForType(NPCTypeGoblin); cfg != nil {
			southernGoblin.AIConfigID = cfg.id
			southernGoblin.AIState = cfg.initialState
			cfg.applyDefaults(&southernGoblin.Blackboard)
		}
	}
	if southernGoblin.Blackboard.ArriveRadius <= 0 {
		southernGoblin.Blackboard.ArriveRadius = 16
	}
	if southernGoblin.Blackboard.PauseTicks == 0 {
		southernGoblin.Blackboard.PauseTicks = 30
	}
	if southernGoblin.Blackboard.StuckEpsilon <= 0 {
		southernGoblin.Blackboard.StuckEpsilon = 0.5
	}
	if southernGoblin.Blackboard.WaypointIndex < 0 || southernGoblin.Blackboard.WaypointIndex >= len(southernGoblin.Waypoints) {
		southernGoblin.Blackboard.WaypointIndex = 0
	}
	southernGoblin.Blackboard.NextDecisionAt = 0
	southernGoblin.Blackboard.LastWaypointIndex = -1

	resolveObstaclePenetration(&southernGoblin.actorState, w.obstacles)
	southernGoblin.Blackboard.LastPos = vec2{X: southernGoblin.X, Y: southernGoblin.Y}
	w.npcs[id] = southernGoblin

	w.nextNPCID++
	id = fmt.Sprintf("npc-rat-%d", w.nextNPCID)
	rat := &npcState{
		actorState: actorState{
			Actor: Actor{
				ID:        id,
				X:         280,
				Y:         520,
				Facing:    defaultFacing,
				Health:    18,
				MaxHealth: 18,
				Inventory: NewInventory(),
			},
		},
		Type:             NPCTypeRat,
		ExperienceReward: 8,
		Home:             vec2{X: 280, Y: 520},
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
	w.npcs[id] = rat
}
