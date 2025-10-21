package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	stdlog "log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	"mine-and-die/server/internal/net/proto"
	"mine-and-die/server/internal/sim"
	simpaches "mine-and-die/server/internal/sim/patches"
	"mine-and-die/server/internal/simutil"
	"mine-and-die/server/internal/telemetry"
	worldpkg "mine-and-die/server/internal/world"
	"mine-and-die/server/logging"
	loggingeconomy "mine-and-die/server/logging/economy"
	logginglifecycle "mine-and-die/server/logging/lifecycle"
	loggingnetwork "mine-and-die/server/logging/network"
	stats "mine-and-die/server/stats"
)

// Hub coordinates subscribers and orchestrates the deterministic world simulation.
type Hub struct {
	mu          sync.Mutex
	world       *World
	engine      sim.Engine
	adapter     *legacyEngineAdapter
	subscribers map[string]*subscriber
	config      worldConfig
	publisher   logging.Publisher
	telemetry   *telemetryCounters

	resubscribeBaselines map[string]simpaches.PlayerView

	nextID atomic.Uint64
	tick   atomic.Uint64
	seq    atomic.Uint64

	defaultKeyframeInterval int
	keyframeInterval        atomic.Int64
	lastKeyframeSeq         atomic.Uint64
	lastKeyframeTick        atomic.Uint64

	resyncNext               atomic.Bool
	forceKeyframeNext        atomic.Bool
	tickBudgetAlarmTriggered atomic.Bool
}

func (h *Hub) engineDeps() sim.Deps {
	if h == nil || h.engine == nil {
		return sim.Deps{}
	}
	return h.engine.Deps()
}

func (h *Hub) logger() telemetry.Logger {
	deps := h.engineDeps()
	if deps.Logger != nil {
		return deps.Logger
	}
	return telemetry.WrapLogger(stdlog.Default())
}

func (h *Hub) logf(format string, args ...any) {
	logger := h.logger()
	if logger == nil {
		return
	}
	logger.Printf(format, args...)
}

func (h *Hub) now() time.Time {
	if h == nil {
		return time.Now()
	}
	deps := h.engineDeps()
	if deps.Clock != nil {
		return deps.Clock.Now()
	}
	return time.Now()
}

// Engine exposes the simulation engine façade.
func (h *Hub) Engine() sim.Engine {
	if h == nil {
		return nil
	}
	return h.engine
}

// Tick reports the latest simulation tick processed by the hub.
func (h *Hub) Tick() uint64 {
	if h == nil {
		return 0
	}
	return h.tick.Load()
}

// Now returns the hub's current clock reading.
func (h *Hub) Now() time.Time {
	return h.now()
}

// HasPlayer reports whether the player is currently tracked by the hub.
func (h *Hub) HasPlayer(playerID string) bool {
	if h == nil {
		return false
	}
	return h.playerExists(playerID)
}

func (h *Hub) attachTelemetryMetrics() {
	if h == nil || h.telemetry == nil {
		return
	}
	h.telemetry.AttachMetrics(h.engineDeps().Metrics)
}

type subscriberConn interface {
	Write([]byte) error
	SetWriteDeadline(time.Time) error
	Close() error
}

type subscriber struct {
	conn           subscriberConn
	mu             sync.Mutex
	lastAck        atomic.Uint64
	lastCommandSeq atomic.Uint64
	limiter        keyframeRateLimiter
}

// Write sends a websocket message guarded by the subscriber's mutex and write deadline.
func (s *subscriber) Write(data []byte) error {
	return s.writeWithDeadline(time.Now(), data)
}

func (s *subscriber) writeWithDeadline(base time.Time, data []byte) error {
	if s == nil || s.conn == nil {
		return errors.New("subscriber closed")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.conn.SetWriteDeadline(base.Add(writeWait)); err != nil {
		return err
	}
	return s.conn.Write(data)
}

// LastCommandSeq returns the last command sequence acknowledged by the subscriber.
func (s *subscriber) LastCommandSeq() uint64 {
	if s == nil {
		return 0
	}
	return s.lastCommandSeq.Load()
}

// StoreLastCommandSeq records the latest command sequence acknowledged by the subscriber.
func (s *subscriber) StoreLastCommandSeq(seq uint64) {
	if s == nil {
		return
	}
	s.lastCommandSeq.Store(seq)
}

const (
	keyframeLimiterCapacity  = 3
	keyframeLimiterRefillPer = 2.0 // tokens per second

	commandQueueWarningStep   = 256
	commandQueuePerActorLimit = 32
	commandBufferCapacity     = 1024

	tickBudgetCatchupMaxTicks = 2
	tickBudgetAlarmMinStreak  = 3
	tickBudgetAlarmMinRatio   = 2.0

	commandRejectUnknownActor  = "unknown_actor"
	commandRejectInvalidAction = "invalid_action"
)

const (
	CommandRejectUnknownActor  = commandRejectUnknownActor
	CommandRejectInvalidAction = commandRejectInvalidAction
	CommandRejectQueueLimit    = sim.CommandRejectQueueLimit
)

type keyframeLookupStatus int

const (
	keyframeLookupMissing keyframeLookupStatus = iota
	keyframeLookupFound
	keyframeLookupExpired
)

type keyframeRateLimiter struct {
	capacity   float64
	tokens     float64
	refillRate float64
	lastRefill time.Time
}

func newKeyframeRateLimiter(capacity, refillRate float64) keyframeRateLimiter {
	if capacity <= 0 || refillRate <= 0 {
		return keyframeRateLimiter{}
	}
	now := time.Now()
	return keyframeRateLimiter{
		capacity:   capacity,
		tokens:     capacity,
		refillRate: refillRate,
		lastRefill: now,
	}
}

func (l *keyframeRateLimiter) allow(now time.Time) bool {
	if l == nil || l.capacity <= 0 || l.refillRate <= 0 {
		return true
	}
	if now.Before(l.lastRefill) {
		l.lastRefill = now
	}
	elapsed := now.Sub(l.lastRefill).Seconds()
	if elapsed > 0 {
		l.tokens += elapsed * l.refillRate
		if l.tokens > l.capacity {
			l.tokens = l.capacity
		}
		l.lastRefill = now
	}
	if l.tokens < 1 {
		return false
	}
	l.tokens -= 1
	return true
}

type HubConfig struct {
	KeyframeInterval int
	Logger           telemetry.Logger
	Metrics          telemetry.Metrics
}

func DefaultHubConfig() HubConfig {
	return HubConfig{KeyframeInterval: 30}
}

// newHub creates a hub with empty maps and a freshly generated world.
func newHub(pubs ...logging.Publisher) *Hub {
	return NewHubWithConfig(DefaultHubConfig(), pubs...)
}

func NewHubWithConfig(hubCfg HubConfig, pubs ...logging.Publisher) *Hub {
	cfg := worldpkg.DefaultConfig().Normalized()
	var pub logging.Publisher
	if len(pubs) > 0 && pubs[0] != nil {
		pub = pubs[0]
	}
	if pub == nil {
		pub = logging.NopPublisher{}
	}
	interval := hubCfg.KeyframeInterval
	if interval < 1 {
		interval = 1
	}

	world := requireLegacyWorld(worldpkg.New(cfg, pub))
	cfg = world.config

	metrics := hubCfg.Metrics
	if metrics == nil {
		if provider, ok := pub.(interface{ Metrics() *logging.Metrics }); ok {
			if candidate := provider.Metrics(); candidate != nil {
				metrics = telemetry.WrapMetrics(candidate)
			}
		}
	}

	clock := logging.Clock(logging.SystemClock{})
	if provider, ok := pub.(interface{ Clock() logging.Clock }); ok {
		if candidate := provider.Clock(); candidate != nil {
			clock = candidate
		}
	}

	engineDeps := sim.Deps{
		Logger:  hubCfg.Logger,
		Metrics: metrics,
		Clock:   clock,
	}
	if engineDeps.Logger == nil {
		engineDeps.Logger = telemetry.WrapLogger(stdlog.Default())
	}
	if world != nil {
		engineDeps.RNG = world.rng
	}

	engineAdapter := newLegacyEngineAdapter(world, engineDeps)

	hub := &Hub{
		world:                   world,
		adapter:                 engineAdapter,
		subscribers:             make(map[string]*subscriber),
		config:                  cfg,
		publisher:               pub,
		telemetry:               newTelemetryCounters(engineDeps.Metrics),
		defaultKeyframeInterval: interval,
		resubscribeBaselines:    nil,
	}
	loopCfg := sim.LoopConfig{
		TickRate:        tickRate,
		CatchupMaxTicks: tickBudgetCatchupMaxTicks,
		CommandCapacity: commandBufferCapacity,
		PerActorLimit:   commandQueuePerActorLimit,
		WarningStep:     commandQueueWarningStep,
	}
	loopHooks := sim.LoopHooks{
		NextTick: func() uint64 {
			return hub.tick.Add(1)
		},
		Prepare: func(ctx sim.LoopTickContext) {
			if hub.adapter != nil {
				hub.adapter.PrepareStep(ctx.Tick, ctx.Now, ctx.Delta, nil)
			}
		},
		AfterStep: func(result sim.LoopStepResult) {
			hub.handleLoopStep(result)
		},
		OnCommandDrop: func(reason string, cmd sim.Command) {
			if hub.telemetry != nil {
				mapped := reason
				if reason == sim.CommandRejectQueueLimit {
					mapped = "limit_exceeded"
				}
				hub.telemetry.RecordCommandDropped(mapped, string(cmd.Type))
			}
		},
		OnQueueWarning: func(length int) {
			hub.logf("[backpressure] pendingCommands=%d; investigate tick latency or raise throttle thresholds", length)
		},
	}
	hub.engine = sim.NewLoop(engineAdapter, loopCfg, loopHooks)

	hub.world.telemetry = hub.telemetry
	hub.world.journal.AttachTelemetry(hub.telemetry)
	hub.keyframeInterval.Store(int64(interval))
	hub.forceKeyframe()
	hub.attachTelemetryMetrics()
	return hub
}

func (h *Hub) effectCatalogSnapshotLocked() map[string]effectCatalogMetadata {
	if h == nil || h.world == nil || h.world.effectManager == nil {
		return nil
	}
	return snapshotEffectCatalog(h.world.effectManager.catalog)
}

func (h *Hub) resyncConfigSnapshot() worldConfig {
	if h == nil {
		return worldConfig{}
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	cfg := h.config
	return cfg
}

func (h *Hub) scheduleKeyframeResync() {
	if h == nil {
		return
	}
	h.forceKeyframe()
	h.resyncNext.Store(true)
}

// EffectCatalogSnapshot returns a cloned snapshot of the designer-authored effect catalog.
func (h *Hub) EffectCatalogSnapshot() map[string]effectCatalogMetadata {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.effectCatalogSnapshotLocked()
}

func (h *Hub) legacySnapshotLocked(includeGroundItems bool, includeEffectTriggers bool) ([]Player, []NPC, []EffectTrigger, []GroundItem) {
	if h == nil {
		return make([]Player, 0), make([]NPC, 0), nil, nil
	}

	if h.engine != nil {
		snapshot := h.engine.Snapshot()
		players := legacyPlayersFromSim(snapshot.Players)
		if players == nil {
			players = make([]Player, 0)
		}
		npcs := legacyNPCsFromSim(snapshot.NPCs)
		if npcs == nil {
			npcs = make([]NPC, 0)
		}
		var triggers []EffectTrigger
		if includeEffectTriggers {
			triggers = legacyEffectTriggersFromSim(snapshot.EffectEvents)
			if triggers == nil {
				triggers = make([]EffectTrigger, 0)
			}
		}
		var groundItems []GroundItem
		if includeGroundItems {
			groundItems = legacyGroundItemsFromSim(snapshot.GroundItems)
			if groundItems == nil {
				groundItems = make([]GroundItem, 0)
			}
		}
		return players, npcs, triggers, groundItems
	}

	now := h.now()
	players, npcs := h.world.Snapshot(now)
	if players == nil {
		players = make([]Player, 0)
	}
	if npcs == nil {
		npcs = make([]NPC, 0)
	}
	var triggers []EffectTrigger
	if includeEffectTriggers {
		triggers = h.world.flushEffectTriggersLocked()
		if triggers == nil {
			triggers = make([]EffectTrigger, 0)
		}
	}
	var groundItems []GroundItem
	if includeGroundItems {
		groundItems = h.world.GroundItemsSnapshot()
		if groundItems == nil {
			groundItems = make([]GroundItem, 0)
		}
	}
	return players, npcs, triggers, groundItems
}

func (h *Hub) simSnapshotLocked(includeGroundItems bool, includeEffectTriggers bool) sim.Snapshot {
	if h == nil {
		return sim.Snapshot{}
	}

	if h.engine != nil {
		snapshot := simutil.CloneSnapshot(h.engine.Snapshot())
		if !includeGroundItems {
			snapshot.GroundItems = nil
		}
		if !includeEffectTriggers {
			snapshot.EffectEvents = nil
		}
		return snapshot
	}

	now := h.now()
	players, npcs := h.world.Snapshot(now)
	snapshot := sim.Snapshot{
		Players:        simutil.ClonePlayers(simPlayersFromLegacy(players)),
		NPCs:           simutil.CloneNPCs(simNPCsFromLegacy(npcs)),
		Obstacles:      simutil.CloneObstacles(simObstaclesFromLegacy(h.world.obstacles)),
		AliveEffectIDs: simutil.CloneAliveEffectIDs(simAliveEffectIDsFromLegacy(h.world.effects)),
	}
	if includeGroundItems {
		snapshot.GroundItems = simutil.CloneGroundItems(simGroundItemsFromLegacy(h.world.GroundItemsSnapshot()))
	}
	if includeEffectTriggers {
		snapshot.EffectEvents = simutil.CloneEffectTriggers(simEffectTriggersFromLegacy(h.world.flushEffectTriggersLocked()))
	}
	return snapshot
}

// legacyGroundItemsSnapshotLocked returns a broadcast-friendly snapshot of ground items.
// Callers must hold h.mu.
func (h *Hub) legacyGroundItemsSnapshotLocked() []GroundItem {
	if h == nil {
		return make([]GroundItem, 0)
	}
	if h.engine != nil {
		items := legacyGroundItemsFromSim(h.engine.Snapshot().GroundItems)
		if items == nil {
			return make([]GroundItem, 0)
		}
		return items
	}
	items := h.world.GroundItemsSnapshot()
	if items == nil {
		return make([]GroundItem, 0)
	}
	return items
}

func (h *Hub) seedPlayerState(playerID string, now time.Time) *playerState {
	inventory := NewInventory()
	if _, err := inventory.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 50}); err != nil {
		loggingeconomy.ItemGrantFailed(
			context.Background(),
			h.publisher,
			h.tick.Load(),
			logging.EntityRef{ID: playerID, Kind: logging.EntityKind("player")},
			loggingeconomy.ItemGrantFailedPayload{ItemType: string(ItemTypeGold), Quantity: 50, Reason: "seed_player"},
			map[string]any{"error": err.Error()},
		)
	}
	if _, err := inventory.AddStack(ItemStack{Type: ItemTypeHealthPotion, Quantity: 2}); err != nil {
		loggingeconomy.ItemGrantFailed(
			context.Background(),
			h.publisher,
			h.tick.Load(),
			logging.EntityRef{ID: playerID, Kind: logging.EntityKind("player")},
			loggingeconomy.ItemGrantFailedPayload{ItemType: string(ItemTypeHealthPotion), Quantity: 2, Reason: "seed_player"},
			map[string]any{"error": err.Error()},
		)
	}

	statsComp := stats.DefaultComponent(stats.ArchetypePlayer)
	maxHealth := statsComp.GetDerived(stats.DerivedMaxHealth)

	return &playerState{
		actorState: actorState{
			Actor: Actor{
				ID:        playerID,
				X:         defaultSpawnX,
				Y:         defaultSpawnY,
				Facing:    defaultFacing,
				Health:    maxHealth,
				MaxHealth: maxHealth,
				Inventory: inventory,
				Equipment: NewEquipment(),
			},
		},
		stats:         statsComp,
		lastHeartbeat: now,
		cooldowns:     make(map[string]time.Time),
		path:          playerPathState{ArriveRadius: defaultPlayerArriveRadius},
	}
}

// Join registers a new player and returns the latest snapshot.
func (h *Hub) Join() joinResponse {
	id := h.nextID.Add(1)
	playerID := fmt.Sprintf("player-%d", id)
	now := h.now()

	player := h.seedPlayerState(playerID, now)

	h.mu.Lock()
	h.world.AddPlayer(player)
	snapshot := h.simSnapshotLocked(true, false)
	players := legacyPlayersFromSim(snapshot.Players)
	npcs := legacyNPCsFromSim(snapshot.NPCs)
	groundItems := legacyGroundItemsFromSim(snapshot.GroundItems)
	cfg := h.config
	h.mu.Unlock()

	logginglifecycle.PlayerJoined(
		context.Background(),
		h.publisher,
		h.tick.Load(),
		logging.EntityRef{ID: playerID, Kind: logging.EntityKind("player")},
		logginglifecycle.PlayerJoinedPayload{SpawnX: player.X, SpawnY: player.Y},
		nil,
	)

	h.forceKeyframe()
	go h.broadcastState(players, npcs, nil, groundItems)

	return joinResponse{
		Ver:               ProtocolVersion,
		ID:                playerID,
		Players:           snapshot.Players,
		NPCs:              snapshot.NPCs,
		Obstacles:         snapshot.Obstacles,
		GroundItems:       snapshot.GroundItems,
		Config:            simWorldConfigFromLegacy(cfg),
		Resync:            true,
		KeyframeInterval:  h.CurrentKeyframeInterval(),
		EffectCatalogHash: effectcontract.EffectCatalogHash,
	}
}

// ResetWorld replaces the current world with a freshly generated instance.
func (h *Hub) ResetWorld(cfg worldConfig) ([]Player, []NPC) {
	cfg = cfg.Normalized()
	now := h.now()

	if h.engine != nil {
		h.engine.DrainCommands()
	}

	h.mu.Lock()
	playerIDs := make([]string, 0, len(h.world.players))
	for id := range h.world.players {
		playerIDs = append(playerIDs, id)
	}

	newW := requireLegacyWorld(worldpkg.New(cfg, h.publisher))
	cfg = newW.config
	newW.telemetry = h.telemetry
	newW.journal.AttachTelemetry(h.telemetry)
	for _, id := range playerIDs {
		newW.AddPlayer(h.seedPlayerState(id, now))
	}
	h.world = newW
	if h.adapter != nil {
		h.adapter.SetWorld(newW)
	}
	h.config = cfg
	h.attachTelemetryMetrics()
	var players []Player
	var npcs []NPC
	if h.engine != nil {
		snapshot := h.engine.Snapshot()
		players, npcs = legacyActorsFromSimSnapshot(snapshot)
	} else {
		players, npcs = h.world.Snapshot(now)
	}
	h.resubscribeBaselines = nil
	h.mu.Unlock()

	h.tick.Store(0)
	h.resyncNext.Store(true)
	h.forceKeyframe()

	return players, npcs
}

// CurrentConfig returns a copy of the active world configuration.
func (h *Hub) CurrentConfig() worldConfig {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.config
}

// Subscribe associates a WebSocket connection with an existing player.
func (h *Hub) Subscribe(playerID string, conn subscriberConn) (*subscriber, []sim.Player, []sim.NPC, []sim.GroundItem, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	state, ok := h.world.players[playerID]
	if !ok {
		return nil, nil, nil, nil, false
	}

	state.lastHeartbeat = h.now()

	if existing, ok := h.subscribers[playerID]; ok {
		existing.conn.Close()
	}

	sub := &subscriber{conn: conn, limiter: newKeyframeRateLimiter(keyframeLimiterCapacity, keyframeLimiterRefillPer)}
	h.subscribers[playerID] = sub
	snapshot := h.simSnapshotLocked(true, false)
	return sub, snapshot.Players, snapshot.NPCs, snapshot.GroundItems, true
}

// RecordAck updates the latest acknowledged tick for the given subscriber.
func (h *Hub) RecordAck(playerID string, ack uint64) {
	h.mu.Lock()
	sub, ok := h.subscribers[playerID]
	h.mu.Unlock()
	if !ok {
		return
	}

	tick := h.tick.Load()
	actor := logging.EntityRef{ID: playerID, Kind: logging.EntityKind("player")}

	for {
		prev := sub.lastAck.Load()
		if ack <= prev {
			if ack < prev {
				loggingnetwork.AckRegression(
					context.Background(),
					h.publisher,
					tick,
					actor,
					loggingnetwork.AckPayload{Previous: prev, Ack: ack},
					nil,
				)
			}
			return
		}
		if sub.lastAck.CompareAndSwap(prev, ack) {
			return
		}
	}
}

// Disconnect removes a player and closes any active subscriber connection.
func (h *Hub) Disconnect(playerID string) ([]Player, []NPC) {
	h.mu.Lock()
	sub, subOK := h.subscribers[playerID]
	if subOK {
		delete(h.subscribers, playerID)
	}

	removed := h.world.RemovePlayer(playerID)
	var players []Player
	var npcs []NPC
	if removed {
		if h.engine != nil {
			snapshot := h.engine.Snapshot()
			players, npcs = legacyActorsFromSimSnapshot(snapshot)
		} else {
			now := h.now()
			players, npcs = h.world.Snapshot(now)
		}
	}
	h.mu.Unlock()

	if subOK {
		sub.conn.Close()
	}

	if !removed {
		return nil, nil
	}

	logginglifecycle.PlayerDisconnected(
		context.Background(),
		h.publisher,
		h.tick.Load(),
		logging.EntityRef{ID: playerID, Kind: logging.EntityKind("player")},
		logginglifecycle.PlayerDisconnectedPayload{Reason: "manual"},
		nil,
	)

	return players, npcs
}

func (h *Hub) enqueuePlayerCommand(playerID string, cmd sim.Command) (sim.Command, bool, string) {
	var zero sim.Command
	if !h.playerExists(playerID) {
		return zero, false, commandRejectUnknownActor
	}
	cmd.ActorID = playerID
	cmd.OriginTick = h.tick.Load()
	cmd.IssuedAt = h.now()
	if h.engine == nil {
		return zero, false, sim.CommandRejectQueueFull
	}
	if ok, reason := h.engine.Enqueue(cmd); !ok {
		return zero, false, reason
	}
	return cmd, true, ""
}

// UpdateIntent stores the latest movement vector and facing for a player.
func (h *Hub) UpdateIntent(playerID string, dx, dy float64, facing string) (sim.Command, bool, string) {
	parsedFacing := sim.FacingDirection("")
	if facing != "" {
		if face, ok := parseFacing(facing); ok {
			parsedFacing = toSimFacing(face)
		}
	}

	cmd := sim.Command{
		Type: sim.CommandMove,
		Move: &sim.MoveCommand{
			DX:     dx,
			DY:     dy,
			Facing: parsedFacing,
		},
	}

	return h.enqueuePlayerCommand(playerID, cmd)
}

// SetPlayerPath queues a command that asks the server to navigate the player toward a point.
func (h *Hub) SetPlayerPath(playerID string, x, y float64) (sim.Command, bool, string) {
	cmd := sim.Command{
		Type: sim.CommandSetPath,
		Path: &sim.PathCommand{
			TargetX: x,
			TargetY: y,
		},
	}

	return h.enqueuePlayerCommand(playerID, cmd)
}

// ClearPlayerPath stops any server-driven navigation for the player.
func (h *Hub) ClearPlayerPath(playerID string) (sim.Command, bool, string) {
	cmd := sim.Command{Type: sim.CommandClearPath}

	return h.enqueuePlayerCommand(playerID, cmd)
}

// HandleAction queues an action command for processing on the next tick.
func (h *Hub) HandleAction(playerID, action string) (sim.Command, bool, string) {
	switch action {
	case effectTypeAttack, effectTypeFireball:
	default:
		return sim.Command{}, false, commandRejectInvalidAction
	}

	cmd := sim.Command{
		Type: sim.CommandAction,
		Action: &sim.ActionCommand{
			Name: action,
		},
	}

	return h.enqueuePlayerCommand(playerID, cmd)
}

// HandleConsoleCommand executes a debug console command for the player.
func (h *Hub) HandleConsoleCommand(playerID, cmd string, qty int) (proto.ConsoleAck, bool) {
	ack := proto.NewConsoleAck(cmd)
	switch cmd {
	case "drop_gold":
		if qty <= 0 {
			ack.Status = "error"
			ack.Reason = "invalid_quantity"
			return ack, true
		}
		h.mu.Lock()
		player, ok := h.world.players[playerID]
		if !ok {
			h.mu.Unlock()
			ack.Status = "error"
			ack.Reason = "unknown_actor"
			return ack, true
		}
		result, failure := h.world.dropGold(&player.actorState, qty, "manual")
		if failure != nil {
			h.mu.Unlock()
			ack.Status = "error"
			ack.Reason = failure.Reason
			return ack, true
		}
		groundItems := h.legacyGroundItemsSnapshotLocked()
		h.mu.Unlock()

		ack.Status = "ok"
		if result != nil {
			ack.Qty = result.Quantity
			if result.StackID != "" {
				ack.StackID = result.StackID
			}
		}
		go h.broadcastState(nil, nil, nil, groundItems)
		return ack, true
	case "equip_slot":
		if qty < 0 {
			ack.Status = "error"
			ack.Reason = "invalid_inventory_slot"
			return ack, true
		}
		h.mu.Lock()
		if _, ok := h.world.players[playerID]; !ok {
			h.mu.Unlock()
			ack.Status = "error"
			ack.Reason = "unknown_actor"
			return ack, true
		}
		slot, err := h.world.EquipFromInventory(playerID, qty)
		h.mu.Unlock()
		if err != nil {
			ack.Status = "error"
			ack.Reason = equipErrorReason(err)
			return ack, true
		}
		ack.Status = "ok"
		ack.Slot = string(slot)
		go h.broadcastState(nil, nil, nil, nil)
		return ack, true
	case "unequip_slot":
		slot, ok := equipSlotFromOrdinal(qty)
		if !ok {
			ack.Status = "error"
			ack.Reason = "invalid_equip_slot"
			return ack, true
		}
		h.mu.Lock()
		if _, exists := h.world.players[playerID]; !exists {
			h.mu.Unlock()
			ack.Status = "error"
			ack.Reason = "unknown_actor"
			return ack, true
		}
		item, err := h.world.UnequipToInventory(playerID, slot)
		h.mu.Unlock()
		if err != nil {
			ack.Status = "error"
			ack.Reason = equipErrorReason(err)
			return ack, true
		}
		ack.Status = "ok"
		ack.Slot = string(slot)
		ack.Qty = item.Quantity
		go h.broadcastState(nil, nil, nil, nil)
		return ack, true
	case "pickup_gold":
		h.mu.Lock()
		player, ok := h.world.players[playerID]
		if !ok {
			h.mu.Unlock()
			ack.Status = "error"
			ack.Reason = "unknown_actor"
			return ack, true
		}
		actorRef := h.world.entityRef(playerID)
		result, failure := h.world.pickupNearestGold(&player.actorState)
		if failure != nil {
			failureReason := failure.Reason
			failureStackID := failure.StackID
			failureDistance := failure.Distance
			errMsg := failure.Err
			h.mu.Unlock()

			ack.Status = "error"
			ack.Reason = failureReason

			var metadata map[string]any
			switch failureReason {
			case worldpkg.PickupFailureReasonOutOfRange:
				meta := make(map[string]any)
				if failureStackID != "" {
					meta["stackId"] = failureStackID
				}
				if failureDistance > 0 {
					meta["distance"] = failureDistance
				}
				if len(meta) > 0 {
					metadata = meta
				}
			case worldpkg.PickupFailureReasonInventoryError:
				meta := make(map[string]any)
				if errMsg != "" {
					meta["error"] = errMsg
				}
				if failureStackID != "" {
					meta["stackId"] = failureStackID
				}
				if len(meta) > 0 {
					metadata = meta
				}
			default:
				if failureStackID != "" {
					metadata = map[string]any{"stackId": failureStackID}
				}
			}

			loggingeconomy.GoldPickupFailed(
				context.Background(),
				h.publisher,
				h.tick.Load(),
				actorRef,
				loggingeconomy.GoldPickupFailedPayload{Reason: failureReason},
				metadata,
			)
			return ack, true
		}
		qty := result.Quantity
		stackID := result.StackID
		groundItems := h.legacyGroundItemsSnapshotLocked()
		h.mu.Unlock()

		loggingeconomy.GoldPickedUp(
			context.Background(),
			h.publisher,
			h.tick.Load(),
			actorRef,
			loggingeconomy.GoldPickedUpPayload{Quantity: qty},
			map[string]any{"stackId": stackID},
		)

		ack.Status = "ok"
		ack.Qty = qty
		ack.StackID = stackID
		go h.broadcastState(nil, nil, nil, groundItems)
		return ack, true
	default:
		ack.Status = "error"
		ack.Reason = "unknown_command"
		return ack, true
	}
}

func equipErrorReason(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, errEquipUnknownActor):
		return "unknown_actor"
	case errors.Is(err, errEquipInvalidInventorySlot):
		return "invalid_inventory_slot"
	case errors.Is(err, errEquipEmptySlot):
		return "empty_slot"
	case errors.Is(err, errEquipNotEquippable):
		return "not_equippable"
	case errors.Is(err, errUnequipInvalidSlot):
		return "invalid_equip_slot"
	case errors.Is(err, errUnequipEmptySlot):
		return "slot_empty"
	default:
		return "internal_error"
	}
}

// UpdateHeartbeat records the most recent heartbeat time and RTT for a player.
func (h *Hub) UpdateHeartbeat(playerID string, receivedAt time.Time, clientSent int64) (time.Duration, bool) {
	if !h.playerExists(playerID) {
		return 0, false
	}

	var rtt time.Duration
	if clientSent > 0 {
		clientTime := time.UnixMilli(clientSent)
		if clientTime.Before(receivedAt.Add(5 * time.Second)) {
			rtt = receivedAt.Sub(clientTime)
			if rtt < 0 {
				rtt = 0
			}
		}
	}

	cmd := sim.Command{
		OriginTick: h.tick.Load(),
		ActorID:    playerID,
		Type:       sim.CommandHeartbeat,
		IssuedAt:   receivedAt,
		Heartbeat: &sim.HeartbeatCommand{
			ReceivedAt: receivedAt,
			ClientSent: clientSent,
			RTT:        rtt,
		},
	}
	if h.engine != nil {
		h.engine.Enqueue(cmd)
	}

	return rtt, true
}

// processLoopStep applies post-step bookkeeping and returns converted snapshots
// alongside subscribers that should be closed.
func (h *Hub) processLoopStep(result sim.LoopStepResult) ([]Player, []NPC, []EffectTrigger, []GroundItem, []*subscriber) {
	if h == nil {
		return nil, nil, nil, nil, nil
	}
	h.mu.Lock()
	if h.telemetry != nil {
		h.telemetry.RecordEffectsActive(len(h.world.effects))
	}
	toClose := make([]*subscriber, 0, len(result.RemovedPlayers))
	for _, id := range result.RemovedPlayers {
		if sub, ok := h.subscribers[id]; ok {
			toClose = append(toClose, sub)
			delete(h.subscribers, id)
		}
	}
	h.mu.Unlock()

	snapshot := result.Snapshot
	players := legacyPlayersFromSim(snapshot.Players)
	npcs := legacyNPCsFromSim(snapshot.NPCs)
	triggers := legacyEffectTriggersFromSim(snapshot.EffectEvents)
	groundItems := legacyGroundItemsFromSim(snapshot.GroundItems)

	return players, npcs, triggers, groundItems, toClose
}

func (h *Hub) advance(now time.Time, dt float64) ([]Player, []NPC, []EffectTrigger, []GroundItem, []*subscriber) {
	if h == nil || h.engine == nil {
		return nil, nil, nil, nil, nil
	}
	tick := h.tick.Add(1)
	result := h.engine.Advance(sim.LoopTickContext{Tick: tick, Now: now, Delta: dt})
	return h.processLoopStep(result)
}

func (h *Hub) handleLoopStep(result sim.LoopStepResult) {
	players, npcs, triggers, groundItems, toClose := h.processLoopStep(result)
	for _, sub := range toClose {
		sub.conn.Close()
	}
	h.broadcastState(players, npcs, triggers, groundItems)
	duration := result.Duration
	if h.telemetry != nil {
		h.telemetry.RecordTickDuration(duration)
	}
	budget := result.Budget
	if budget > 0 && duration > budget {
		ratio := float64(duration) / float64(budget)
		streak := uint64(0)
		if h.telemetry != nil {
			streak = h.telemetry.RecordTickBudgetOverrun(duration, budget)
		}
		h.logf(
			"[tick] budget overrun: duration=%s budget=%s ratio=%.2f streak=%d",
			duration,
			budget,
			ratio,
			streak,
		)
		if (ratio >= tickBudgetAlarmMinRatio || streak >= tickBudgetAlarmMinStreak) && h.tickBudgetAlarmTriggered.CompareAndSwap(false, true) {
			h.handleTickBudgetAlarm(duration, budget, ratio, streak, result.Delta, result.ClampedDelta, result.MaxDelta)
		}
	} else {
		h.resetTickBudgetAlarm()
	}
}

// RunSimulation drives the fixed-rate tick loop until the stop channel closes.
func (h *Hub) RunSimulation(stop <-chan struct{}) {
	if h == nil {
		<-stop
		return
	}
	if h.engine == nil {
		<-stop
		return
	}
	h.engine.Run(stop)
}

func (h *Hub) resetTickBudgetAlarm() {
	if h.telemetry != nil {
		h.telemetry.ResetTickBudgetOverrunStreak()
	}
	h.tickBudgetAlarmTriggered.Store(false)
}

func (h *Hub) handleTickBudgetAlarm(duration, budget time.Duration, ratio float64, streak uint64, dt float64, clamped bool, maxDtSeconds float64) {
	h.resyncNext.Store(true)
	h.forceKeyframe()

	tick := h.tick.Load()
	h.logf(
		"[tick] budget alarm triggered: scheduling resync ratio=%.2f streak=%d dt=%.4f clamped=%t",
		ratio,
		streak,
		dt,
		clamped,
	)

	if h.telemetry != nil {
		h.telemetry.RecordTickBudgetAlarm(tick, ratio)
	}
}

// DiagnosticsSnapshot exposes heartbeat data for the diagnostics endpoint.
func (h *Hub) DiagnosticsSnapshot() []diagnosticsPlayer {
	h.mu.Lock()
	defer h.mu.Unlock()

	players := make([]diagnosticsPlayer, 0, len(h.world.players))
	for _, state := range h.world.players {
		var ack uint64
		if sub, ok := h.subscribers[state.ID]; ok {
			ack = sub.lastAck.Load()
		}
		players = append(players, diagnosticsPlayer{
			Ver:           ProtocolVersion,
			ID:            state.ID,
			LastHeartbeat: state.lastHeartbeat.UnixMilli(),
			RTTMillis:     state.lastRTT.Milliseconds(),
			LastAck:       ack,
		})
	}
	return players
}

// marshalState serializes a world snapshot into the outbound state payload format.
func (h *Hub) forceKeyframe() {
	h.forceKeyframeNext.Store(true)
}

// ForceKeyframe marks the next broadcast as a keyframe.
func (h *Hub) ForceKeyframe() {
	h.forceKeyframe()
}

func (h *Hub) CurrentKeyframeInterval() int {
	if h == nil {
		return 1
	}
	value := int(h.keyframeInterval.Load())
	if value <= 0 {
		if h.defaultKeyframeInterval > 0 {
			return h.defaultKeyframeInterval
		}
		return 1
	}
	return value
}

func (h *Hub) SetKeyframeInterval(interval int) int {
	if h == nil {
		return 1
	}
	normalized := interval
	if normalized < 1 {
		normalized = h.defaultKeyframeInterval
	}
	if normalized < 1 {
		normalized = 1
	}
	current := h.CurrentKeyframeInterval()
	if current == normalized {
		return current
	}
	h.keyframeInterval.Store(int64(normalized))
	h.forceKeyframe()
	return normalized
}

func (h *Hub) shouldIncludeSnapshot() bool {
	interval := h.CurrentKeyframeInterval()
	if interval <= 1 {
		return true
	}
	if h.forceKeyframeNext.CompareAndSwap(true, false) {
		return true
	}
	interval64 := uint64(interval)
	if interval64 == 0 {
		return true
	}
	tick := h.tick.Load()
	last := h.lastKeyframeTick.Load()
	if last == 0 {
		return true
	}
	return tick >= last && tick-last >= interval64
}

func (h *Hub) marshalState(players []sim.Player, npcs []sim.NPC, triggers []sim.EffectTrigger, groundItems []sim.GroundItem, drainPatches bool, includeSnapshot bool) ([]byte, int, error) {
	h.mu.Lock()
	engine := h.engine
	var (
		simSnapshot    sim.Snapshot
		obstacles      []sim.Obstacle
		aliveEffectIDs []string
	)

	if engine != nil {
		simSnapshot = engine.Snapshot()
		if includeSnapshot {
			if players == nil {
				players = simutil.ClonePlayers(simSnapshot.Players)
			}
			if npcs == nil {
				npcs = simutil.CloneNPCs(simSnapshot.NPCs)
			}
			if groundItems == nil {
				groundItems = simutil.CloneGroundItems(simSnapshot.GroundItems)
			}
			if triggers == nil {
				triggers = simutil.CloneEffectTriggers(simSnapshot.EffectEvents)
			}
			obstacles = simutil.CloneObstacles(simSnapshot.Obstacles)
		} else if triggers == nil {
			triggers = make([]sim.EffectTrigger, 0)
		}
		aliveEffectIDs = simutil.CloneAliveEffectIDs(simSnapshot.AliveEffectIDs)
	} else {
		if includeSnapshot {
			needSnapshot := players == nil || npcs == nil
			needGround := groundItems == nil
			needTriggers := triggers == nil
			if needSnapshot || needGround || needTriggers {
				simSnapshot = h.simSnapshotLocked(needGround, needTriggers)
				if players == nil {
					players = simutil.ClonePlayers(simSnapshot.Players)
				}
				if npcs == nil {
					npcs = simutil.CloneNPCs(simSnapshot.NPCs)
				}
				if needGround && groundItems == nil {
					groundItems = simutil.CloneGroundItems(simSnapshot.GroundItems)
				}
				if needTriggers && triggers == nil {
					triggers = simutil.CloneEffectTriggers(simSnapshot.EffectEvents)
				}
			}
		} else if triggers == nil {
			triggers = make([]sim.EffectTrigger, 0)
		}
		if includeSnapshot {
			if groundItems == nil {
				groundItems = make([]sim.GroundItem, 0)
			}
			obstacles = simutil.CloneObstacles(simObstaclesFromLegacy(h.world.obstacles))
		} else {
			groundItems = nil
		}
		if includeSnapshot && players == nil {
			players = make([]sim.Player, 0)
		}
		if includeSnapshot && npcs == nil {
			npcs = make([]sim.NPC, 0)
		}
		if len(h.world.effects) > 0 {
			aliveEffectIDs = make([]string, 0, len(h.world.effects))
			for _, eff := range h.world.effects {
				if eff == nil || eff.ID == "" {
					continue
				}
				aliveEffectIDs = append(aliveEffectIDs, eff.ID)
			}
		}
	}

	if !includeSnapshot {
		players = nil
		npcs = nil
		groundItems = nil
	} else {
		if players == nil {
			players = make([]sim.Player, 0)
		}
		if npcs == nil {
			npcs = make([]sim.NPC, 0)
		}
		if groundItems == nil {
			groundItems = make([]sim.GroundItem, 0)
		}
	}
	if triggers == nil {
		triggers = make([]sim.EffectTrigger, 0)
	}
	if obstacles == nil {
		obstacles = make([]sim.Obstacle, 0)
	}

	var (
		patches                 []sim.Patch
		restorableLegacyPatches []Patch
		restorableSimPatches    []sim.Patch
	)
	if drainPatches {
		if engine != nil {
			drained := engine.DrainPatches()
			if len(drained) > 0 {
				restorableSimPatches = simutil.ClonePatches(drained)
				patches = simutil.ClonePatches(drained)
			}
		} else {
			legacy := h.world.drainPatchesLocked()
			if len(legacy) > 0 {
				restorableLegacyPatches = append([]Patch(nil), legacy...)
				patches = simPatchesFromLegacy(legacy)
			}
		}
	} else {
		if engine != nil {
			snapshot := engine.SnapshotPatches()
			patches = simutil.ClonePatches(snapshot)
		} else {
			patches = simPatchesFromLegacy(h.world.snapshotPatchesLocked())
		}
	}

	if len(patches) > 0 {
		alivePlayers := players
		aliveNPCs := npcs
		aliveItems := groundItems
		aliveEffects := aliveEffectIDs
		total := len(alivePlayers) + len(aliveNPCs) + len(aliveEffects)
		if aliveItems != nil {
			total += len(aliveItems)
		}
		if total > 0 {
			alive := make(map[string]struct{}, total)
			for _, player := range alivePlayers {
				id := player.Actor.ID
				if id == "" {
					continue
				}
				alive[id] = struct{}{}
			}
			for _, npc := range aliveNPCs {
				id := npc.Actor.ID
				if id == "" {
					continue
				}
				alive[id] = struct{}{}
			}
			for _, item := range aliveItems {
				if item.ID == "" {
					continue
				}
				alive[item.ID] = struct{}{}
			}
			for _, id := range aliveEffects {
				if id == "" {
					continue
				}
				alive[id] = struct{}{}
			}
			filtered := patches[:0]
			for _, patch := range patches {
				if patch.EntityID == "" {
					continue
				}
				if _, ok := alive[patch.EntityID]; !ok {
					if patch.Kind == sim.PatchPlayerRemoved {
						filtered = append(filtered, patch)
						continue
					}
					if patch.Kind == sim.PatchGroundItemQty {
						switch payload := patch.Payload.(type) {
						case sim.GroundItemQtyPayload:
							if payload.Qty <= 0 {
								filtered = append(filtered, patch)
							}
						case *sim.GroundItemQtyPayload:
							if payload != nil && payload.Qty <= 0 {
								filtered = append(filtered, patch)
							}
						}
					}
					continue
				}
				filtered = append(filtered, patch)
			}
			patches = filtered
		}
	}

	simPlayerPatches := filterPlayerPatches(patches)
	h.updateResubscribeBaselinesLocked(simSnapshot, includeSnapshot, simPlayerPatches)
	if includeSnapshot {
		players = mergePlayersFromBaselines(players, h.resubscribeBaselines)
	}

	cfg := h.config
	simCfg := simWorldConfigFromLegacy(cfg)
	tick := h.tick.Load()
	seq, resync := h.nextStateMeta(drainPatches)
	effectTransportEnabled := engine != nil
	h.mu.Unlock()

	effectBatch := EffectEventBatch{}
	simEffectBatch := sim.EffectEventBatch{}
	if engine != nil {
		if drainPatches {
			simEffectBatch = engine.DrainEffectEvents()
		} else {
			simEffectBatch = engine.SnapshotEffectEvents()
		}
		effectBatch = legacyEffectEventBatchFromSim(simEffectBatch)
	}

	if patches == nil {
		patches = make([]sim.Patch, 0)
	}

	keyframeSeq := h.lastKeyframeSeq.Load()
	if includeSnapshot {
		simFrame := sim.Keyframe{
			Tick:        tick,
			Sequence:    seq,
			Players:     simutil.ClonePlayers(players),
			NPCs:        simutil.CloneNPCs(npcs),
			Obstacles:   simutil.CloneObstacles(obstacles),
			GroundItems: simutil.CloneGroundItems(groundItems),
			Config:      simCfg,
		}
		var record sim.KeyframeRecordResult
		if engine != nil {
			record = engine.RecordKeyframe(simFrame)
		} else {
			legacyFrame := keyframe{
				Tick:        tick,
				Sequence:    seq,
				Players:     legacyPlayersFromSim(players),
				NPCs:        legacyNPCsFromSim(npcs),
				Obstacles:   legacyObstaclesFromSim(obstacles),
				GroundItems: legacyGroundItemsFromSim(groundItems),
				Config:      cfg,
			}
			legacyRecord := h.world.journal.RecordKeyframe(legacyFrame)
			record = simKeyframeRecordResultFromLegacy(legacyRecord)
		}
		h.lastKeyframeSeq.Store(seq)
		h.lastKeyframeTick.Store(tick)
		keyframeSeq = seq
		if h.telemetry != nil {
			h.telemetry.RecordKeyframeJournal(record.Size, record.OldestSequence, record.NewestSequence)
		}
		if h.telemetry != nil && h.telemetry.DebugEnabled() {
			h.logf("[journal] add sequence=%d tick=%d size=%d", seq, tick, record.Size)
			for _, eviction := range record.Evicted {
				h.logf("[journal] evict sequence=%d tick=%d size=%d reason=%s", eviction.Sequence, eviction.Tick, record.Size, eviction.Reason)
			}
			h.logf("[journal] window size=%d oldest=%d newest=%d", record.Size, record.OldestSequence, record.NewestSequence)
		}
	} else if keyframeSeq == 0 {
		keyframeSeq = seq
	}

	currentInterval := h.CurrentKeyframeInterval()
	msg := stateMessage{
		Ver:              ProtocolVersion,
		Type:             proto.TypeState,
		Players:          players,
		NPCs:             npcs,
		Obstacles:        obstacles,
		EffectTriggers:   triggers,
		GroundItems:      groundItems,
		Patches:          patches,
		Tick:             tick,
		Sequence:         seq,
		KeyframeSeq:      keyframeSeq,
		ServerTime:       h.now().UnixMilli(),
		Config:           simCfg,
		KeyframeInterval: currentInterval,
	}
	if resync {
		msg.Resync = true
	}
	if effectTransportEnabled {
		msg.EffectSpawns = effectBatch.Spawns
		msg.EffectUpdates = effectBatch.Updates
		msg.EffectEnds = effectBatch.Ends
		if len(effectBatch.LastSeqByID) > 0 {
			msg.EffectSeqCursors = effectBatch.LastSeqByID
		}
	}

	entities := len(msg.Players) + len(msg.NPCs) + len(msg.Obstacles) + len(msg.EffectTriggers) + len(msg.GroundItems)
	if effectTransportEnabled && (len(msg.EffectSpawns) > 0 || len(msg.EffectUpdates) > 0 || len(msg.EffectEnds) > 0) {
		entities += len(msg.EffectSpawns) + len(msg.EffectUpdates) + len(msg.EffectEnds)
	}
	data, err := proto.EncodeStateSnapshot(msg)
	if err != nil {
		if drainPatches {
			h.mu.Lock()
			if engine != nil {
				if len(restorableSimPatches) > 0 {
					engine.RestorePatches(restorableSimPatches)
				}
			} else if len(restorableLegacyPatches) > 0 {
				h.world.journal.RestorePatches(restorableLegacyPatches)
			}
			if effectTransportEnabled {
				engine.RestoreEffectEvents(simEffectBatch)
			}
			h.mu.Unlock()
		}
		return nil, 0, err
	}
	return data, entities, nil
}

// MarshalState serializes a world snapshot using the legacy hub marshaller.
func (h *Hub) MarshalState(players []sim.Player, npcs []sim.NPC, triggers []sim.EffectTrigger, groundItems []sim.GroundItem, drainPatches bool, includeSnapshot bool) ([]byte, int, error) {
	return h.marshalState(players, npcs, triggers, groundItems, drainPatches, includeSnapshot)
}

func (h *Hub) scheduleResyncIfNeeded() (bool, resyncSignal) {
	h.mu.Lock()
	engine := h.engine
	h.mu.Unlock()

	if engine == nil {
		return false, resyncSignal{}
	}
	simSignal, ok := engine.ConsumeEffectResyncHint()
	if !ok {
		return false, resyncSignal{}
	}
	signal := legacyEffectResyncSignalFromSim(simSignal)

	h.forceKeyframe()
	h.resyncNext.Store(true)

	summary := signal.summary()
	if summary == "" {
		h.logf("[effects] scheduling resync (journal hint)")
	} else {
		h.logf("[effects] scheduling resync (journal hint): %s", summary)
	}
	return true, signal
}

func (h *Hub) nextStateMeta(drainPatches bool) (seq uint64, resync bool) {
	resync = !drainPatches
	if !resync && h.resyncNext.CompareAndSwap(true, false) {
		resync = true
	}
	seq = h.seq.Add(1)
	return seq, resync
}

func (h *Hub) lookupKeyframe(sequence uint64) (keyframeMessage, keyframeLookupStatus) {
	if sequence == 0 {
		return keyframeMessage{}, keyframeLookupMissing
	}

	h.mu.Lock()
	engine := h.engine
	h.mu.Unlock()

	if engine == nil {
		return keyframeMessage{}, keyframeLookupMissing
	}

	frame, ok := engine.KeyframeBySequence(sequence)
	if ok {
		snapshot := keyframeMessage{
			Ver:         ProtocolVersion,
			Type:        proto.TypeKeyframe,
			Sequence:    frame.Sequence,
			Tick:        frame.Tick,
			Players:     simutil.ClonePlayers(frame.Players),
			NPCs:        simutil.CloneNPCs(frame.NPCs),
			Obstacles:   simutil.CloneObstacles(frame.Obstacles),
			GroundItems: simutil.CloneGroundItems(frame.GroundItems),
			Config:      frame.Config,
		}
		return snapshot, keyframeLookupFound
	}

	size, oldest, newest := engine.KeyframeWindow()
	if size == 0 {
		return keyframeMessage{}, keyframeLookupExpired
	}
	if sequence < oldest || sequence > newest {
		return keyframeMessage{}, keyframeLookupExpired
	}
	return keyframeMessage{}, keyframeLookupExpired
}

// Keyframe returns a serialized keyframe snapshot for the requested sequence.
func (h *Hub) Keyframe(sequence uint64) (keyframeMessage, bool) {
	snapshot, status := h.lookupKeyframe(sequence)
	return snapshot, status == keyframeLookupFound
}

func (h *Hub) HandleKeyframeRequest(playerID string, sub *subscriber, sequence uint64) (keyframeMessage, *keyframeNackMessage, bool) {
	if sequence == 0 {
		return keyframeMessage{}, nil, false
	}

	now := h.now()
	if sub != nil && !sub.limiter.allow(now) {
		if h.telemetry != nil {
			h.telemetry.RecordKeyframeRequest(0, false)
			h.telemetry.IncrementKeyframeRateLimited()
		}
		h.logf("[keyframe] rate_limited player=%s sequence=%d", playerID, sequence)
		nack := &keyframeNackMessage{
			Ver:      ProtocolVersion,
			Type:     proto.TypeKeyframeNack,
			Sequence: sequence,
			Reason:   "rate_limited",
			Resync:   true,
			Config:   simWorldConfigFromLegacy(h.resyncConfigSnapshot()),
		}
		h.scheduleKeyframeResync()
		return keyframeMessage{}, nack, true
	}

	snapshot, status := h.lookupKeyframe(sequence)
	latency := h.now().Sub(now)
	switch status {
	case keyframeLookupFound:
		if h.telemetry != nil {
			h.telemetry.RecordKeyframeRequest(latency, true)
		}
		h.logf("[keyframe] served player=%s sequence=%d tick=%d latency_ms=%d", playerID, snapshot.Sequence, snapshot.Tick, latency.Milliseconds())
		return snapshot, nil, true
	case keyframeLookupExpired:
		if h.telemetry != nil {
			h.telemetry.RecordKeyframeRequest(latency, false)
			h.telemetry.IncrementKeyframeExpired()
		}
		h.logf("[keyframe] expired player=%s sequence=%d", playerID, sequence)
		nack := &keyframeNackMessage{
			Ver:      ProtocolVersion,
			Type:     proto.TypeKeyframeNack,
			Sequence: sequence,
			Reason:   "expired",
			Resync:   true,
			Config:   simWorldConfigFromLegacy(h.resyncConfigSnapshot()),
		}
		h.scheduleKeyframeResync()
		return keyframeMessage{}, nack, true
	default:
		if h.telemetry != nil {
			h.telemetry.RecordKeyframeRequest(latency, false)
		}
		return keyframeMessage{}, nil, false
	}
}

// broadcastState sends the latest world snapshot to every subscriber.
func (h *Hub) broadcastState(players []Player, npcs []NPC, triggers []EffectTrigger, groundItems []GroundItem) {
	h.scheduleResyncIfNeeded()
	includeSnapshot := h.shouldIncludeSnapshot()
	simPlayers := simPlayersFromLegacy(players)
	simNPCs := simNPCsFromLegacy(npcs)
	var simTriggers []sim.EffectTrigger
	if len(triggers) > 0 {
		simTriggers = simEffectTriggersFromLegacy(triggers)
	}
	var simGroundItems []sim.GroundItem
	if len(groundItems) > 0 {
		simGroundItems = simGroundItemsFromLegacy(groundItems)
	}
	data, entities, err := h.marshalState(simPlayers, simNPCs, simTriggers, simGroundItems, true, includeSnapshot)
	if err != nil {
		h.logf("failed to marshal state message: %v", err)
		return
	}

	matched := make([]string, 0, 4)
	for _, marker := range []struct {
		label  string
		needle []byte
	}{
		{label: "blood-splatter", needle: []byte("blood-splatter")},
		{label: "attack", needle: []byte("attack")},
		{label: "fire", needle: []byte("fire")},
		{label: "fireball", needle: []byte("fireball")},
		{label: "melee-swing", needle: []byte("melee-swing")},
	} {
		if bytes.Contains(data, marker.needle) {
			matched = append(matched, marker.label)
		}
	}
	if len(matched) > 0 {
		h.logf(
			"[network] broadcasting payload markers=%s bytes=%d entities=%d snapshot=%t",
			strings.Join(matched, ","),
			len(data),
			entities,
			includeSnapshot,
		)
	}

	h.mu.Lock()
	subs := make(map[string]*subscriber, len(h.subscribers))
	for id, sub := range h.subscribers {
		subs[id] = sub
	}
	h.mu.Unlock()

	for id, sub := range subs {
		err := sub.writeWithDeadline(h.now(), data)
		if err != nil {
			h.logf("failed to send update to %s: %v", id, err)
			players, npcs := h.Disconnect(id)
			if players != nil {
				h.forceKeyframe()
				go h.broadcastState(players, npcs, nil, nil)
			}
		}
	}
	if h.telemetry != nil {
		h.telemetry.RecordBroadcast(len(data), entities)
	}
}

// BroadcastState sends a snapshot to every active subscriber.
func (h *Hub) BroadcastState(players []Player, npcs []NPC, triggers []EffectTrigger, groundItems []GroundItem) {
	h.broadcastState(players, npcs, triggers, groundItems)
}

// RecordTelemetryBroadcast records the size of a broadcast for telemetry consumers.
func (h *Hub) RecordTelemetryBroadcast(bytes, entities int) {
	if h == nil || h.telemetry == nil {
		return
	}
	h.telemetry.RecordBroadcast(bytes, entities)
}

func (h *Hub) TelemetrySnapshot() telemetrySnapshot {
	if h.telemetry == nil {
		return telemetrySnapshot{}
	}
	return h.telemetry.Snapshot()
}

func filterPlayerPatches(patches []sim.Patch) []sim.Patch {
	if len(patches) == 0 {
		return nil
	}
	filtered := make([]sim.Patch, 0, len(patches))
	for _, patch := range patches {
		switch patch.Kind {
		case sim.PatchPlayerPos,
			sim.PatchPlayerFacing,
			sim.PatchPlayerIntent,
			sim.PatchPlayerHealth,
			sim.PatchPlayerInventory,
			sim.PatchPlayerEquipment,
			sim.PatchPlayerRemoved:
			filtered = append(filtered, patch)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func (h *Hub) updateResubscribeBaselinesLocked(snapshot sim.Snapshot, includeSnapshot bool, patches []sim.Patch) {
	if h == nil {
		return
	}
	if includeSnapshot {
		h.resubscribeBaselines = playerViewsFromSimSnapshot(snapshot)
		return
	}
	if len(patches) == 0 {
		return
	}
	base := clonePlayerViewMap(h.resubscribeBaselines)
	updated, err := simpaches.ApplyPlayers(base, patches)
	if err != nil {
		h.resubscribeBaselines = playerViewsFromSimSnapshot(snapshot)
		return
	}
	h.resubscribeBaselines = updated
}

func playerViewsFromSimSnapshot(snapshot sim.Snapshot) map[string]simpaches.PlayerView {
	if len(snapshot.Players) == 0 {
		return nil
	}
	views := make(map[string]simpaches.PlayerView, len(snapshot.Players))
	for _, player := range snapshot.Players {
		id := player.Actor.ID
		if id == "" {
			continue
		}
		view := simpaches.PlayerView{
			Player:   player,
			IntentDX: player.IntentDX,
			IntentDY: player.IntentDY,
		}
		views[id] = view.Clone()
	}
	if len(views) == 0 {
		return nil
	}
	return views
}

func playerFromView(view simpaches.PlayerView) sim.Player {
	player := simutil.ClonePlayer(view.Player)
	player.IntentDX = view.IntentDX
	player.IntentDY = view.IntentDY
	return player
}

func clonePlayerViewMap(src map[string]simpaches.PlayerView) map[string]simpaches.PlayerView {
	if len(src) == 0 {
		return nil
	}
	clones := make(map[string]simpaches.PlayerView, len(src))
	for id, view := range src {
		clones[id] = view.Clone()
	}
	return clones
}

func mergePlayersFromBaselines(players []sim.Player, baselines map[string]simpaches.PlayerView) []sim.Player {
	if len(baselines) == 0 {
		return players
	}
	if players == nil {
		players = make([]sim.Player, 0, len(baselines))
	}

	seen := make(map[string]struct{}, len(players))
	for i := range players {
		id := players[i].Actor.ID
		if id == "" {
			continue
		}
		seen[id] = struct{}{}
		if view, ok := baselines[id]; ok {
			players[i] = playerFromView(view)
		}
	}

	if len(seen) == len(baselines) {
		return players
	}

	missing := make([]string, 0, len(baselines)-len(seen))
	for id := range baselines {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		missing = append(missing, id)
	}
	sort.Strings(missing)
	for _, id := range missing {
		players = append(players, playerFromView(baselines[id]))
	}
	return players
}

func (h *Hub) playerExists(playerID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.world.HasPlayer(playerID)
}
