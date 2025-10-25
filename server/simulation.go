package server

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	ai "mine-and-die/server/internal/ai"
	combat "mine-and-die/server/internal/combat"
	internaleffects "mine-and-die/server/internal/effects"
	itemspkg "mine-and-die/server/internal/items"
	internalstats "mine-and-die/server/internal/stats"
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
	effects                 []*effectState
	effectsByID             map[string]*effectState
	effectsIndex            *effectSpatialIndex
	effectsRegistry         internaleffects.Registry
	effectTriggers          []EffectTrigger
	effectManager           *EffectManager
	obstacles               []Obstacle
	effectHitAdapter        combat.EffectHitCallback
	meleeAbilityGate        combat.MeleeAbilityGate
	projectileAbilityGate   combat.ProjectileAbilityGate
	abilityOwnerLookup      worldpkg.AbilityOwnerLookup[*actorState, combat.AbilityActor]
	abilityOwnerStateLookup worldpkg.AbilityOwnerStateLookup[*actorState]
	projectileStopAdapter   worldpkg.ProjectileStopAdapter
	projectileTemplates     map[string]*ProjectileTemplate
	statusEffectDefs        map[StatusEffectType]worldpkg.ApplyStatusEffectDefinition
	nextEffectID            uint64
	nextNPCID               uint64
	nextGroundItemID        uint64
	aiLibrary               *ai.Library
	config                  worldConfig
	rng                     *rand.Rand
	seed                    string
	publisher               logging.Publisher
	currentTick             uint64
	telemetry               *telemetryCounters
	recordAttackOverlap     func(ownerID string, tick uint64, ability string, playerHits []string, npcHits []string, metadata map[string]any)

	playerHitCallback worldpkg.EffectHitCallback
	npcHitCallback    worldpkg.EffectHitCallback

	groundItems       map[string]*itemspkg.GroundItemState
	groundItemsByTile map[itemspkg.GroundTileKey]map[string]*itemspkg.GroundItemState
	journal           Journal
}

func (w *World) LegacyWorldMarker() {}

func (w *World) resolveStats(tick uint64) {
	if w == nil {
		return
	}

	if len(w.players) > 0 {
		actors := make([]internalstats.Actor, 0, len(w.players))
		for _, player := range w.players {
			if player == nil {
				continue
			}
			player := player
			actors = append(actors, internalstats.Actor{
				Component: &player.Stats,
				SyncMaxHealth: func(max float64) {
					w.setActorHealth(&player.ActorState, &player.Version, player.ID, PatchPlayerHealth, max, player.Health)
				},
			})
		}
		internalstats.Resolve(tick, actors)
	}

	if len(w.npcs) > 0 {
		actors := make([]internalstats.Actor, 0, len(w.npcs))
		for _, npc := range w.npcs {
			if npc == nil {
				continue
			}
			npc := npc
			actors = append(actors, internalstats.Actor{
				Component: &npc.Stats,
				SyncMaxHealth: func(max float64) {
					w.setActorHealth(&npc.ActorState, &npc.Version, npc.ID, PatchNPCHealth, max, npc.Health)
				},
			})
		}
		internalstats.Resolve(tick, actors)
	}
}

func (w *World) syncMaxHealth(actor *actorState, version *uint64, entityID string, kind PatchKind, comp *stats.Component) {
	if w == nil || actor == nil || version == nil || comp == nil || entityID == "" {
		return
	}

	internalstats.SyncMaxHealth(comp, func(max float64) {
		w.setActorHealth(actor, version, entityID, kind, max, actor.Health)
	})
}

// legacyConstructWorld constructs an empty world with generated obstacles and seeded NPCs.
func legacyConstructWorld(cfg worldConfig, publisher logging.Publisher, deps worldpkg.Deps) *World {
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
		aiLibrary:           ai.GlobalLibrary,
		config:              normalized,
		rng:                 newDeterministicRNG(normalized.Seed, "world"),
		seed:                normalized.Seed,
		publisher:           publisher,
		telemetry:           nil,
		groundItems:         make(map[string]*itemspkg.GroundItemState),
		groundItemsByTile:   make(map[itemspkg.GroundTileKey]map[string]*itemspkg.GroundItemState),
		journal:             newJournal(capacity, maxAge),
	}
	w.statusEffectDefs = newStatusEffectDefinitions(w)
	w.configureAbilityOwnerAdapters()
	w.configureEffectHitAdapter()
	w.configureMeleeAbilityGate()
	w.configureProjectileAbilityGate()
	w.projectileStopAdapter = worldpkg.NewProjectileStopAdapter(worldpkg.ProjectileStopAdapterConfig{
		AllocateID: func() string {
			w.nextEffectID++
			return fmt.Sprintf("effect-%d", w.nextEffectID)
		},
		RegisterEffect: func(effect any) bool {
			state, _ := effect.(*effectState)
			if state == nil {
				if cast, ok := effect.(*internaleffects.State); ok {
					state = (*effectState)(cast)
				}
			}
			return w.registerEffect(state)
		},
		RecordEffectSpawn: w.recordEffectSpawn,
		CurrentTick: func() effectcontract.Tick {
			return effectcontract.Tick(int64(w.currentTick))
		},
		SetRemainingRange: func(effect any, remaining float64) {
			state, _ := effect.(*effectState)
			if state == nil {
				if cast, ok := effect.(*internaleffects.State); ok {
					state = (*effectState)(cast)
				}
			}
			if state != nil {
				w.SetEffectParam(state, "remainingRange", remaining)
			}
		},
		RecordEffectEnd: func(effect any, reason string) {
			state, _ := effect.(*effectState)
			if state == nil {
				if cast, ok := effect.(*internaleffects.State); ok {
					state = (*effectState)(cast)
				}
			}
			if state != nil {
				w.recordEffectEnd(state, reason)
			}
		},
	})
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

	if deps.JournalTelemetry != nil {
		w.journal.AttachTelemetry(deps.JournalTelemetry)
	}
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
	worldpkg.RegisterLegacyConstructor(func(cfg worldpkg.Config, publisher logging.Publisher, deps worldpkg.Deps) worldpkg.LegacyWorld {
		return legacyConstructWorld(cfg, publisher, deps)
	})
}

// Snapshot copies players and NPCs into broadcast-friendly structs.
func (w *World) Snapshot(now time.Time) ([]Player, []NPC) {
	players := make([]Player, 0, len(w.players))
	for _, player := range w.players {
		players = append(players, player.Snapshot())
	}
	npcs := make([]NPC, 0, len(w.npcs))
	for _, npc := range w.npcs {
		npcs = append(npcs, npc.Snapshot())
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
	return w.DrainPatches()
}

// snapshotPatchesLocked returns a copy of any staged patches without clearing
// the journal. Callers must hold the world mutex.
func (w *World) snapshotPatchesLocked() []Patch {
	return w.SnapshotPatches()
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
		w.RecordEffectSpawn(e)
	case effectcontract.EffectUpdateEvent:
		w.RecordEffectUpdate(e)
	case effectcontract.EffectEndEvent:
		w.RecordEffectEnd(e)
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
	state.Stats.Resolve(w.currentTick)
	w.syncMaxHealth(&state.ActorState, &state.Version, state.ID, PatchPlayerHealth, &state.Stats)
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
	w.dropAllInventory(&npc.ActorState, "death")
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
					player.LastInput = cmd.IssuedAt
				} else {
					player.LastInput = now
				}
			} else if npc, ok := w.npcs[cmd.ActorID]; ok {
				dx := cmd.Move.DX
				dy := cmd.Move.DY
				length := math.Hypot(dx, dy)
				if length > 1 {
					dx /= length
					dy /= length
				}
				npc.IntentX = dx
				npc.IntentY = dy
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
				player.LastHeartbeat = cmd.Heartbeat.ReceivedAt
				player.LastRTT = cmd.Heartbeat.RTT
			}
		case CommandSetPath:
			if cmd.Path == nil {
				continue
			}
			if player, ok := w.players[cmd.ActorID]; ok {
				target := vec2{X: cmd.Path.TargetX, Y: cmd.Path.TargetY}
				w.ensurePlayerPath(player, target, tick)
				if !cmd.IssuedAt.IsZero() {
					player.LastInput = cmd.IssuedAt
				} else {
					player.LastInput = now
				}
			}
		case CommandClearPath:
			if player, ok := w.players[cmd.ActorID]; ok {
				w.clearPlayerPath(player)
				w.SetIntent(player.ID, 0, 0)
				if !cmd.IssuedAt.IsZero() {
					player.LastInput = cmd.IssuedAt
				} else {
					player.LastInput = now
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
		scratch := player.ActorState
		if player.IntentX != 0 || player.IntentY != 0 {
			moveActorWithObstacles(&scratch, dt, w.obstacles, width, height)
		}
		proposedPlayerStates[id] = &scratch
		actorsForCollisions = append(actorsForCollisions, &scratch)
	}
	initialNPCPositions := make(map[string]vec2, len(w.npcs))
	proposedNPCStates := make(map[string]*actorState, len(w.npcs))
	for id, npc := range w.npcs {
		initialNPCPositions[id] = vec2{X: npc.X, Y: npc.Y}
		scratch := npc.ActorState
		if npc.IntentX != 0 || npc.IntentY != 0 {
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
		actorsForHazards = append(actorsForHazards, &player.ActorState)
	}
	for _, npc := range w.npcs {
		actorsForHazards = append(actorsForHazards, &npc.ActorState)
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
		if player.LastHeartbeat.IsZero() {
			continue
		}
		if player.LastHeartbeat.Before(cutoff) {
			if w.publisher != nil {
				logginglifecycle.PlayerDisconnected(
					context.Background(),
					w.publisher,
					w.currentTick,
					logging.EntityRef{ID: id, Kind: logging.EntityKind("player")},
					logginglifecycle.PlayerDisconnectedPayload{Reason: "timeout"},
					map[string]any{"lastHeartbeat": player.LastHeartbeat},
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
	ai.SeedInitialNPCs(w.npcSpawner())
}

func (w *World) npcSpawner() ai.WorldNPCSpawner {
	return ai.WorldNPCSpawner{
		ConfigFunc: func() worldpkg.Config {
			if w == nil {
				return worldpkg.DefaultConfig()
			}
			return w.config
		},
		DimensionsFunc: func() (float64, float64) {
			if w == nil {
				return worldpkg.DefaultWidth, worldpkg.DefaultHeight
			}
			return w.dimensions()
		},
		SubsystemRNGFunc: func(label string) *rand.Rand {
			if w == nil {
				return nil
			}
			return w.subsystemRNG(label)
		},
		SpawnGoblinFunc: func(x, y float64, waypoints []worldpkg.Vec2, goldQty, potionQty int) {
			if w == nil {
				return
			}
			w.spawnGoblinAt(x, y, waypoints, goldQty, potionQty)
		},
		SpawnRatFunc: func(x, y float64) {
			if w == nil {
				return
			}
			w.spawnRatAt(x, y)
		},
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
		ActorState: actorState{
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
		Stats:            statsComp,
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
	ai.BootstrapNPC(ai.SpawnBootstrapConfig{
		Library:       w.aiLibrary,
		Type:          string(NPCTypeGoblin),
		ConfigID:      &goblin.AIConfigID,
		State:         &goblin.AIState,
		Blackboard:    &goblin.Blackboard,
		WaypointCount: len(goblin.Waypoints),
	})

	resolveObstaclePenetration(&goblin.ActorState, w.obstacles, w.width(), w.height())
	goblin.Blackboard.LastPos = vec2{X: goblin.X, Y: goblin.Y}
	w.npcs[goblin.ID] = goblin
}

func (w *World) spawnExtraGoblins(count int) {
	ai.SpawnExtraGoblins(w.npcSpawner(), count)
}

func (w *World) spawnRatAt(x, y float64) {
	w.nextNPCID++
	id := fmt.Sprintf("npc-rat-%d", w.nextNPCID)
	statsComp := stats.DefaultComponent(stats.ArchetypeRat)
	maxHealth := statsComp.GetDerived(stats.DerivedMaxHealth)

	rat := &npcState{
		ActorState: actorState{
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
		Stats:            statsComp,
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
	ai.BootstrapNPC(ai.SpawnBootstrapConfig{
		Library:       w.aiLibrary,
		Type:          string(NPCTypeRat),
		ConfigID:      &rat.AIConfigID,
		State:         &rat.AIState,
		Blackboard:    &rat.Blackboard,
		WaypointCount: len(rat.Waypoints),
	})

	resolveObstaclePenetration(&rat.ActorState, w.obstacles, w.width(), w.height())
	rat.Blackboard.LastPos = vec2{X: rat.X, Y: rat.Y}
	w.npcs[rat.ID] = rat
}

func (w *World) spawnExtraRats(count int) {
	ai.SpawnExtraRats(w.npcSpawner(), count)
}
