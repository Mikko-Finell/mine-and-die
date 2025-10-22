package server

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	combat "mine-and-die/server/internal/combat"
	internaleffects "mine-and-die/server/internal/effects"
	worldpkg "mine-and-die/server/internal/world"
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
	players map[string]*playerState
	npcs    map[string]*npcState
	// effects/effectTriggers/nextEffectID track contract-managed runtime state
	// and author the telemetry snapshots consumed by gameplay and analytics.
	effects               []*effectState
	effectsByID           map[string]*effectState
	effectsIndex          *effectSpatialIndex
	effectsRegistry       internaleffects.Registry
	effectTriggers        []EffectTrigger
	effectManager         *EffectManager
	obstacles             []Obstacle
	effectHitAdapter      combat.EffectHitCallback
	meleeAbilityGate      combat.MeleeAbilityGate
	projectileAbilityGate combat.ProjectileAbilityGate
	projectileTemplates   map[string]*ProjectileTemplate
	statusEffectDefs      map[StatusEffectType]*StatusEffectDefinition
	nextEffectID          uint64
	nextNPCID             uint64
	nextGroundItemID      uint64
	aiLibrary             *aiLibrary
	config                worldConfig
	rng                   *rand.Rand
	seed                  string
	publisher             logging.Publisher
	currentTick           uint64
	telemetry             *telemetryCounters
	recordAttackOverlap   func(ownerID string, tick uint64, ability string, playerHits []string, npcHits []string, metadata map[string]any)

	playerHitCallback worldpkg.EffectHitCallback
	npcHitCallback    worldpkg.EffectHitCallback

	groundItems       map[string]*groundItemState
	groundItemsByTile map[groundTileKey]map[string]*groundItemState
	journal           Journal
}

func (w *World) LegacyWorldMarker() {}

func (w *World) resolveStats(tick uint64) {
	if w == nil {
		return
	}

	if len(w.players) > 0 {
		actors := make([]worldpkg.StatsActor, 0, len(w.players))
		for _, player := range w.players {
			if player == nil {
				continue
			}
			player := player
			actors = append(actors, worldpkg.StatsActor{
				Component: &player.stats,
				SyncMaxHealth: func(max float64) {
					w.setActorHealth(&player.actorState, &player.version, player.ID, PatchPlayerHealth, max, player.Health)
				},
			})
		}
		worldpkg.ResolveStats(tick, actors)
	}

	if len(w.npcs) > 0 {
		actors := make([]worldpkg.StatsActor, 0, len(w.npcs))
		for _, npc := range w.npcs {
			if npc == nil {
				continue
			}
			npc := npc
			actors = append(actors, worldpkg.StatsActor{
				Component: &npc.stats,
				SyncMaxHealth: func(max float64) {
					w.setActorHealth(&npc.actorState, &npc.version, npc.ID, PatchNPCHealth, max, npc.Health)
				},
			})
		}
		worldpkg.ResolveStats(tick, actors)
	}
}

func (w *World) syncMaxHealth(actor *actorState, version *uint64, entityID string, kind PatchKind, comp *stats.Component) {
	if w == nil || actor == nil || version == nil || comp == nil || entityID == "" {
		return
	}

	worldpkg.SyncMaxHealth(comp, func(max float64) {
		w.setActorHealth(actor, version, entityID, kind, max, actor.Health)
	})
}

// legacyConstructWorld constructs an empty world with generated obstacles and seeded NPCs.
func legacyConstructWorld(cfg worldConfig, publisher logging.Publisher) *World {
	normalized := cfg.Normalized()

	if publisher == nil {
		publisher = logging.NopPublisher{}
	}

	capacity, maxAge := journalConfig()

	w := &World{
		players:             make(map[string]*playerState),
		npcs:                make(map[string]*npcState),
		effects:             make([]*effectState, 0),
		effectsByID:         make(map[string]*effectState),
		effectsIndex:        newEffectSpatialIndex(effectSpatialCellSize, effectSpatialMaxPerCell),
		effectTriggers:      make([]EffectTrigger, 0),
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
	w.configureEffectHitAdapter()
	w.configureMeleeAbilityGate()
	w.configureProjectileAbilityGate()
	w.playerHitCallback = combat.NewWorldPlayerEffectHitCallback(combat.WorldPlayerEffectHitCallbackConfig{
		Dispatcher: w.effectHitAdapter,
	})
	w.npcHitCallback = combat.NewWorldNPCEffectHitCallback(combat.WorldNPCEffectHitCallbackConfig{
		Dispatcher: w.effectHitAdapter,
		SpawnBlood: func(effect any, target any, now time.Time) {
			eff, _ := effect.(*effectState)
			npc, _ := target.(*npcState)
			if eff == nil || npc == nil {
				return
			}
			w.maybeSpawnBloodSplatter(eff, npc, now)
		},
		IsAlive: func(target any) bool {
			npc, _ := target.(*npcState)
			if npc == nil {
				return false
			}
			return npc.Health > 0
		},
		HandleDefeat: func(target any) {
			npc, _ := target.(*npcState)
			if npc == nil {
				return
			}
			w.handleNPCDefeat(npc)
		},
	})
	w.effectsRegistry = internaleffects.Registry{
		Effects: &w.effects,
		ByID:    &w.effectsByID,
		Index:   w.effectsIndex,
	}
	w.effectManager = newEffectManager(w)
	w.obstacles = w.generateObstacles(normalized.ObstaclesCount)
	w.spawnInitialNPCs()
	return w
}

func (w *World) effectRegistry() internaleffects.Registry {
	if w == nil {
		return internaleffects.Registry{}
	}
	if w.effectsRegistry.Effects == nil || w.effectsRegistry.Effects != &w.effects {
		w.effectsRegistry.Effects = &w.effects
	}
	if w.effectsRegistry.ByID == nil || w.effectsRegistry.ByID != &w.effectsByID {
		w.effectsRegistry.ByID = &w.effectsByID
	}
	if w.effectsRegistry.Index != w.effectsIndex {
		w.effectsRegistry.Index = w.effectsIndex
	}
	return w.effectsRegistry
}

func (w *World) attachTelemetry(t *telemetryCounters) {
	if w == nil {
		return
	}
	w.telemetry = t
	w.effectRegistry()
	if t != nil {
		w.effectsRegistry.RecordSpatialOverflow = t.RecordEffectSpatialOverflow
	} else {
		w.effectsRegistry.RecordSpatialOverflow = nil
	}
}

func requireLegacyWorld(instance worldpkg.LegacyWorld) *World {
	if instance == nil {
		return nil
	}
	world, ok := instance.(*World)
	if !ok {
		panic("world: legacy constructor returned unexpected type")
	}
	return world
}

func init() {
	worldpkg.RegisterLegacyConstructor(func(cfg worldpkg.Config, publisher logging.Publisher) worldpkg.LegacyWorld {
		return legacyConstructWorld(cfg, publisher)
	})
}

// Snapshot copies players and NPCs into broadcast-friendly structs.
func (w *World) Snapshot(now time.Time) ([]Player, []NPC) {
	players := make([]Player, 0, len(w.players))
	for _, player := range w.players {
		players = append(players, player.snapshot())
	}
	npcs := make([]NPC, 0, len(w.npcs))
	for _, npc := range w.npcs {
		npcs = append(npcs, npc.snapshot())
	}
	return players, npcs
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

func (w *World) recordEffectLifecycleEvent(event effectcontract.EffectLifecycleEvent) {
	if w == nil || event == nil || w.effectManager == nil {
		return
	}
	switch e := event.(type) {
	case effectcontract.EffectSpawnEvent:
		if producer := contractSpawnProducer(e.Instance.DefinitionID); producer != "" {
			w.recordEffectSpawn(e.Instance.DefinitionID, producer)
		}
		w.journal.RecordEffectSpawn(e)
	case effectcontract.EffectUpdateEvent:
		w.journal.RecordEffectUpdate(e)
	case effectcontract.EffectEndEvent:
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
	w.purgeEntityPatches(id)
	w.appendPatch(PatchPlayerRemoved, id, nil)
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
func (w *World) Step(tick uint64, now time.Time, dt float64, commands []Command, emitEffectEvent func(effectcontract.EffectLifecycleEvent)) []string {
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
			if w.effectManager == nil {
				continue
			}

			intent, ok := combat.StageMeleeIntent(combat.MeleeAbilityTriggerConfig{
				AbilityGate:  w.meleeAbilityGate,
				IntentConfig: meleeIntentConfig,
			}, action.actorID, now)
			if ok {
				w.effectManager.EnqueueIntent(intent)
			}
		case effectTypeFireball:
			if w.effectManager == nil {
				continue
			}

			tpl := w.projectileTemplates[effectTypeFireball]
			combatTpl, ok := projectileIntentTemplateFromConfig(tpl)
			if !ok {
				continue
			}

			intent, ok := combat.StageProjectileIntent(combat.ProjectileAbilityTriggerConfig{
				AbilityGate:  w.projectileAbilityGate,
				IntentConfig: projectileIntentConfig,
				Template:     combatTpl,
			}, action.actorID, now)
			if ok {
				w.effectManager.EnqueueIntent(intent)
			}
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
	if w.effectManager != nil {
		dispatcher := w.recordEffectLifecycleEvent
		if emitEffectEvent != nil {
			dispatcher = func(event effectcontract.EffectLifecycleEvent) {
				w.recordEffectLifecycleEvent(event)
				emitEffectEvent(event)
			}
		}
		w.effectManager.RunTick(effectcontract.Tick(int64(tick)), now, dispatcher)
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

	actors := make([]worldpkg.PositionActor, 0, len(w.players))
	for id, player := range w.players {
		if player == nil {
			continue
		}
		actors = append(actors, worldpkg.PositionActor{
			ID:      id,
			Current: worldpkg.Vec2{X: player.X, Y: player.Y},
		})
	}

	worldpkg.ApplyPlayerPositionMutations(initial, proposed, actors, func(id string, target worldpkg.Vec2) {
		w.SetPosition(id, target.X, target.Y)
	})
}

// applyNPCPositionMutations commits NPC movement resolved during the tick through
// the NPC write barrier so patches and versions stay consistent.
func (w *World) applyNPCPositionMutations(initial map[string]vec2, proposed map[string]vec2) {
	if w == nil {
		return
	}

	actors := make([]worldpkg.PositionActor, 0, len(w.npcs))
	for id, npc := range w.npcs {
		if npc == nil {
			continue
		}
		actors = append(actors, worldpkg.PositionActor{
			ID:      id,
			Current: worldpkg.Vec2{X: npc.X, Y: npc.Y},
		})
	}

	worldpkg.ApplyNPCPositionMutations(initial, proposed, actors, func(id string, target worldpkg.Vec2) {
		w.SetNPCPosition(id, target.X, target.Y)
	})
}

func (w *World) spawnInitialNPCs() {
	worldpkg.SeedInitialNPCs(worldNPCSpawner{world: w})
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
	worldpkg.SpawnExtraGoblins(worldNPCSpawner{world: w}, count)
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
	worldpkg.SpawnExtraRats(worldNPCSpawner{world: w}, count)
}

type worldNPCSpawner struct {
	world *World
}

func (s worldNPCSpawner) Config() worldpkg.Config {
	if s.world == nil {
		return worldpkg.DefaultConfig()
	}
	return s.world.config
}

func (s worldNPCSpawner) Dimensions() (float64, float64) {
	if s.world == nil {
		return worldpkg.DefaultWidth, worldpkg.DefaultHeight
	}
	return s.world.dimensions()
}

func (s worldNPCSpawner) SubsystemRNG(label string) *rand.Rand {
	if s.world == nil {
		return newDeterministicRNG(worldpkg.DefaultSeed, label)
	}
	return s.world.subsystemRNG(label)
}

func (s worldNPCSpawner) SpawnGoblinAt(x, y float64, waypoints []worldpkg.Vec2, goldQty, potionQty int) {
	if s.world == nil {
		return
	}
	converted := make([]vec2, len(waypoints))
	for i, wp := range waypoints {
		converted[i] = vec2{X: wp.X, Y: wp.Y}
	}
	s.world.spawnGoblinAt(x, y, converted, goldQty, potionQty)
}

func (s worldNPCSpawner) SpawnRatAt(x, y float64) {
	if s.world == nil {
		return
	}
	s.world.spawnRatAt(x, y)
}
