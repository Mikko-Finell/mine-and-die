package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdlog "log"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	effectcontract "mine-and-die/server/effects/contract"
	"mine-and-die/server/internal/sim"
	simpaches "mine-and-die/server/internal/sim/patches"
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

	commandsMu             sync.Mutex    // protects pendingCommands between network handlers and the tick loop
	pendingCommands        []sim.Command // staged commands applied in order at the next simulation step
	pendingCommandsByActor map[string]int
	droppedCommandsByActor map[string]uint64
}

func (h *Hub) engineDeps() sim.Deps {
	if h == nil || h.engine == nil {
		return sim.Deps{}
	}
	return h.engine.Deps()
}

func (h *Hub) logger() *stdlog.Logger {
	deps := h.engineDeps()
	if deps.Logger != nil {
		return deps.Logger
	}
	return stdlog.Default()
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

func (h *Hub) attachTelemetryMetrics() {
	if h == nil || h.telemetry == nil {
		return
	}
	h.telemetry.AttachMetrics(h.engineDeps().Metrics)
}

type subscriber struct {
	conn           *websocket.Conn
	mu             sync.Mutex
	lastAck        atomic.Uint64
	lastCommandSeq atomic.Uint64
	limiter        keyframeRateLimiter
}

// WriteMessage sends a websocket message guarded by the subscriber's mutex and write deadline.
func (s *subscriber) WriteMessage(messageType int, data []byte) error {
	if s == nil || s.conn == nil {
		return errors.New("subscriber closed")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
		return err
	}
	return s.conn.WriteMessage(messageType, data)
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

	tickBudgetCatchupMaxTicks = 2
	tickBudgetAlarmMinStreak  = 3
	tickBudgetAlarmMinRatio   = 2.0

	commandRejectUnknownActor  = "unknown_actor"
	commandRejectInvalidAction = "invalid_action"
	commandRejectQueueLimit    = "queue_limit"
)

const (
	CommandRejectUnknownActor  = commandRejectUnknownActor
	CommandRejectInvalidAction = commandRejectInvalidAction
	CommandRejectQueueLimit    = commandRejectQueueLimit
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
}

func DefaultHubConfig() HubConfig {
	return HubConfig{KeyframeInterval: 30}
}

// newHub creates a hub with empty maps and a freshly generated world.
func newHub(pubs ...logging.Publisher) *Hub {
	return NewHubWithConfig(DefaultHubConfig(), pubs...)
}

func NewHubWithConfig(hubCfg HubConfig, pubs ...logging.Publisher) *Hub {
	cfg := defaultWorldConfig().normalized()
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

	world := newWorld(cfg, pub)
	cfg = world.config

	var metrics *logging.Metrics
	if provider, ok := pub.(interface{ Metrics() *logging.Metrics }); ok {
		metrics = provider.Metrics()
	}

	clock := logging.Clock(logging.SystemClock{})
	if provider, ok := pub.(interface{ Clock() logging.Clock }); ok {
		if candidate := provider.Clock(); candidate != nil {
			clock = candidate
		}
	}

	engineDeps := sim.Deps{
		Logger:  stdlog.Default(),
		Metrics: metrics,
		Clock:   clock,
	}
	if world != nil {
		engineDeps.RNG = world.rng
	}

	engineAdapter := newLegacyEngineAdapter(world, engineDeps)

	hub := &Hub{
		world:                   world,
		engine:                  engineAdapter,
		adapter:                 engineAdapter,
		subscribers:             make(map[string]*subscriber),
		pendingCommands:         make([]sim.Command, 0),
		pendingCommandsByActor:  make(map[string]int),
		droppedCommandsByActor:  make(map[string]uint64),
		config:                  cfg,
		publisher:               pub,
		telemetry:               newTelemetryCounters(engineDeps.Metrics),
		defaultKeyframeInterval: interval,
		resubscribeBaselines:    nil,
	}
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
	players, npcs, _, groundItems := h.legacySnapshotLocked(true, false)
	obstacles := append([]Obstacle(nil), h.world.obstacles...)
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
		Players:           players,
		NPCs:              npcs,
		Obstacles:         obstacles,
		GroundItems:       groundItems,
		Config:            cfg,
		Resync:            true,
		KeyframeInterval:  h.CurrentKeyframeInterval(),
		EffectCatalogHash: effectcontract.EffectCatalogHash,
	}
}

// ResetWorld replaces the current world with a freshly generated instance.
func (h *Hub) ResetWorld(cfg worldConfig) ([]Player, []NPC) {
	cfg = cfg.normalized()
	now := h.now()

	h.commandsMu.Lock()
	h.pendingCommands = nil
	h.commandsMu.Unlock()

	h.mu.Lock()
	playerIDs := make([]string, 0, len(h.world.players))
	for id := range h.world.players {
		playerIDs = append(playerIDs, id)
	}

	newW := newWorld(cfg, h.publisher)
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
	if h.adapter != nil {
		h.engine = h.adapter
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
func (h *Hub) Subscribe(playerID string, conn *websocket.Conn) (*subscriber, []Player, []NPC, []GroundItem, bool) {
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
	players, npcs, _, groundItems := h.legacySnapshotLocked(true, false)
	return sub, players, npcs, groundItems, true
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

// UpdateIntent stores the latest movement vector and facing for a player.
func (h *Hub) UpdateIntent(playerID string, dx, dy float64, facing string) (sim.Command, bool, string) {
	var zero sim.Command
	parsedFacing := sim.FacingDirection("")
	if facing != "" {
		if face, ok := parseFacing(facing); ok {
			parsedFacing = toSimFacing(face)
		}
	}

	if !h.playerExists(playerID) {
		return zero, false, commandRejectUnknownActor
	}

	cmd := sim.Command{
		OriginTick: h.tick.Load(),
		ActorID:    playerID,
		Type:       sim.CommandMove,
		IssuedAt:   h.now(),
		Move: &sim.MoveCommand{
			DX:     dx,
			DY:     dy,
			Facing: parsedFacing,
		},
	}
	if ok, reason := h.enqueueCommand(cmd); !ok {
		return zero, false, reason
	}
	return cmd, true, ""
}

// SetPlayerPath queues a command that asks the server to navigate the player toward a point.
func (h *Hub) SetPlayerPath(playerID string, x, y float64) (sim.Command, bool, string) {
	var zero sim.Command
	if !h.playerExists(playerID) {
		return zero, false, commandRejectUnknownActor
	}
	cmd := sim.Command{
		OriginTick: h.tick.Load(),
		ActorID:    playerID,
		Type:       sim.CommandSetPath,
		IssuedAt:   h.now(),
		Path: &sim.PathCommand{
			TargetX: x,
			TargetY: y,
		},
	}
	if ok, reason := h.enqueueCommand(cmd); !ok {
		return zero, false, reason
	}
	return cmd, true, ""
}

// ClearPlayerPath stops any server-driven navigation for the player.
func (h *Hub) ClearPlayerPath(playerID string) (sim.Command, bool, string) {
	var zero sim.Command
	if !h.playerExists(playerID) {
		return zero, false, commandRejectUnknownActor
	}
	cmd := sim.Command{
		OriginTick: h.tick.Load(),
		ActorID:    playerID,
		Type:       sim.CommandClearPath,
		IssuedAt:   h.now(),
	}
	if ok, reason := h.enqueueCommand(cmd); !ok {
		return zero, false, reason
	}
	return cmd, true, ""
}

// HandleAction queues an action command for processing on the next tick.
func (h *Hub) HandleAction(playerID, action string) (sim.Command, bool, string) {
	var zero sim.Command
	switch action {
	case effectTypeAttack, effectTypeFireball:
	default:
		return zero, false, commandRejectInvalidAction
	}
	if !h.playerExists(playerID) {
		return zero, false, commandRejectUnknownActor
	}
	cmd := sim.Command{
		OriginTick: h.tick.Load(),
		ActorID:    playerID,
		Type:       sim.CommandAction,
		IssuedAt:   h.now(),
		Action: &sim.ActionCommand{
			Name: action,
		},
	}
	if ok, reason := h.enqueueCommand(cmd); !ok {
		return zero, false, reason
	}
	return cmd, true, ""
}

// HandleConsoleCommand executes a debug console command for the player.
func (h *Hub) HandleConsoleCommand(playerID, cmd string, qty int) (consoleAckMessage, bool) {
	ack := consoleAckMessage{Ver: ProtocolVersion, Type: "console_ack", Cmd: cmd}
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
		available := player.Inventory.QuantityOf(ItemTypeGold)
		if available < qty {
			h.mu.Unlock()
			ack.Status = "error"
			ack.Reason = "insufficient_gold"
			return ack, true
		}
		var removed int
		err := h.world.MutateInventory(playerID, func(inv *Inventory) error {
			var innerErr error
			removed, innerErr = inv.RemoveItemTypeQuantity(ItemTypeGold, qty)
			return innerErr
		})
		if err != nil || removed != qty {
			h.mu.Unlock()
			ack.Status = "error"
			ack.Reason = "inventory_error"
			return ack, true
		}
		stack := h.world.upsertGroundItem(&player.actorState, ItemStack{Type: ItemTypeGold, Quantity: removed}, "manual")
		groundItems := h.legacyGroundItemsSnapshotLocked()
		h.mu.Unlock()

		ack.Status = "ok"
		ack.Qty = removed
		if stack != nil {
			ack.StackID = stack.ID
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
		item, distance := h.world.nearestGroundItem(&player.actorState, ItemTypeGold)
		if item == nil {
			h.mu.Unlock()
			ack.Status = "error"
			ack.Reason = "not_found"
			loggingeconomy.GoldPickupFailed(
				context.Background(),
				h.publisher,
				h.tick.Load(),
				actorRef,
				loggingeconomy.GoldPickupFailedPayload{Reason: "not_found"},
				nil,
			)
			return ack, true
		}
		if distance > groundPickupRadius {
			h.mu.Unlock()
			ack.Status = "error"
			ack.Reason = "out_of_range"
			loggingeconomy.GoldPickupFailed(
				context.Background(),
				h.publisher,
				h.tick.Load(),
				actorRef,
				loggingeconomy.GoldPickupFailedPayload{Reason: "out_of_range"},
				map[string]any{"stackId": item.ID, "distance": distance},
			)
			return ack, true
		}
		qty := item.Qty
		stackID := item.ID
		if qty <= 0 {
			h.world.removeGroundItem(item)
			h.mu.Unlock()
			ack.Status = "error"
			ack.Reason = "not_found"
			loggingeconomy.GoldPickupFailed(
				context.Background(),
				h.publisher,
				h.tick.Load(),
				actorRef,
				loggingeconomy.GoldPickupFailedPayload{Reason: "not_found"},
				map[string]any{"stackId": item.ID},
			)
			return ack, true
		}
		err := h.world.MutateInventory(playerID, func(inv *Inventory) error {
			_, addErr := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: qty})
			return addErr
		})
		if err != nil {
			h.mu.Unlock()
			ack.Status = "error"
			ack.Reason = "inventory_error"
			loggingeconomy.GoldPickupFailed(
				context.Background(),
				h.publisher,
				h.tick.Load(),
				actorRef,
				loggingeconomy.GoldPickupFailedPayload{Reason: "inventory_error"},
				map[string]any{"error": err.Error(), "stackId": item.ID},
			)
			return ack, true
		}
		h.world.removeGroundItem(item)
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
	h.enqueueCommand(cmd)

	return rtt, true
}

// advance runs a single simulation step and returns updated snapshots plus stale subscribers.
func (h *Hub) advance(now time.Time, dt float64) ([]Player, []NPC, []EffectTrigger, []GroundItem, []*subscriber) {
	tick := h.tick.Add(1)
	commands := h.drainCommands()

	h.mu.Lock()
	if h.adapter != nil {
		h.adapter.PrepareStep(tick, now, dt, nil)
	}
	var snapshot sim.Snapshot
	var removed []string
	if h.engine != nil {
		_ = h.engine.Apply(commands)
		h.engine.Step()
		snapshot = h.engine.Snapshot()
	}
	if h.adapter != nil {
		removed = h.adapter.RemovedPlayers()
	}
	if h.telemetry != nil {
		h.telemetry.RecordEffectsActive(len(h.world.effects))
	}
	toClose := make([]*subscriber, 0, len(removed))
	for _, id := range removed {
		if sub, ok := h.subscribers[id]; ok {
			toClose = append(toClose, sub)
			delete(h.subscribers, id)
		}
	}
	h.mu.Unlock()

	players := legacyPlayersFromSim(snapshot.Players)
	npcs := legacyNPCsFromSim(snapshot.NPCs)
	triggers := legacyEffectTriggersFromSim(snapshot.EffectEvents)
	groundItems := legacyGroundItemsFromSim(snapshot.GroundItems)

	return players, npcs, triggers, groundItems, toClose
}

// RunSimulation drives the fixed-rate tick loop until the stop channel closes.
func (h *Hub) RunSimulation(stop <-chan struct{}) {
	ticker := time.NewTicker(time.Second / tickRate)
	defer ticker.Stop()

	last := h.now()
	tickBudget := time.Second / time.Duration(tickRate)
	budgetSeconds := 1.0 / float64(tickRate)
	maxDtSeconds := budgetSeconds
	if tickBudgetCatchupMaxTicks > 1 {
		maxDtSeconds = budgetSeconds * float64(tickBudgetCatchupMaxTicks)
	}
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			tickStart := h.now()
			current := h.now()
			dt := current.Sub(last).Seconds()
			clamped := false
			if dt <= 0 {
				dt = budgetSeconds
			} else if dt > maxDtSeconds {
				dt = maxDtSeconds
				clamped = true
			}
			last = current

			players, npcs, triggers, groundItems, toClose := h.advance(current, dt)
			for _, sub := range toClose {
				sub.conn.Close()
			}
			h.broadcastState(players, npcs, triggers, groundItems)
			duration := h.now().Sub(tickStart)
			if h.telemetry != nil {
				h.telemetry.RecordTickDuration(duration)
			}
			if tickBudget > 0 && duration > tickBudget {
				ratio := float64(duration) / float64(tickBudget)
				streak := uint64(0)
				if h.telemetry != nil {
					streak = h.telemetry.RecordTickBudgetOverrun(duration, tickBudget)
				}
				h.logf(
					"[tick] budget overrun: duration=%s budget=%s ratio=%.2f streak=%d",
					duration,
					tickBudget,
					ratio,
					streak,
				)
				if (ratio >= tickBudgetAlarmMinRatio || streak >= tickBudgetAlarmMinStreak) && h.tickBudgetAlarmTriggered.CompareAndSwap(false, true) {
					h.handleTickBudgetAlarm(duration, tickBudget, ratio, streak, dt, clamped, maxDtSeconds)
				}
			} else {
				h.resetTickBudgetAlarm()
			}
		}
	}
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

func (h *Hub) marshalState(players []Player, npcs []NPC, triggers []EffectTrigger, groundItems []GroundItem, drainPatches bool, includeSnapshot bool) ([]byte, int, error) {
	h.mu.Lock()
	engine := h.engine
	var (
		aliveEffectIDs []string
		obstacles      []Obstacle
		simSnapshot    sim.Snapshot
	)
	if engine != nil {
		simSnapshot = engine.Snapshot()
		if includeSnapshot {
			if players == nil {
				players = legacyPlayersFromSim(simSnapshot.Players)
				if players == nil {
					players = make([]Player, 0)
				}
			}
			if npcs == nil {
				npcs = legacyNPCsFromSim(simSnapshot.NPCs)
				if npcs == nil {
					npcs = make([]NPC, 0)
				}
			}
			if groundItems == nil {
				groundItems = legacyGroundItemsFromSim(simSnapshot.GroundItems)
				if groundItems == nil {
					groundItems = make([]GroundItem, 0)
				}
			}
			if triggers == nil {
				triggers = legacyEffectTriggersFromSim(simSnapshot.EffectEvents)
				if triggers == nil {
					triggers = make([]EffectTrigger, 0)
				}
			}
			obstacles = legacyObstaclesFromSim(simSnapshot.Obstacles)
		} else if triggers == nil {
			triggers = make([]EffectTrigger, 0)
		}
		if len(simSnapshot.AliveEffectIDs) > 0 {
			aliveEffectIDs = make([]string, 0, len(simSnapshot.AliveEffectIDs))
			for _, id := range simSnapshot.AliveEffectIDs {
				if id == "" {
					continue
				}
				aliveEffectIDs = append(aliveEffectIDs, id)
			}
			if len(aliveEffectIDs) == 0 {
				aliveEffectIDs = nil
			}
		}
	} else {
		if includeSnapshot {
			needSnapshot := players == nil || npcs == nil
			needGround := groundItems == nil
			needTriggers := triggers == nil
			if needSnapshot || needGround || needTriggers {
				snapPlayers, snapNPCs, snapTriggers, snapGround := h.legacySnapshotLocked(needGround, needTriggers)
				if players == nil {
					players = snapPlayers
				}
				if npcs == nil {
					npcs = snapNPCs
				}
				if groundItems == nil && needGround {
					groundItems = snapGround
				}
				if triggers == nil && needTriggers {
					triggers = snapTriggers
				}
			}
		} else if triggers == nil {
			triggers = make([]EffectTrigger, 0)
		}
		if groundItems == nil {
			groundItems = make([]GroundItem, 0)
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
		if includeSnapshot {
			obstacles = append([]Obstacle(nil), h.world.obstacles...)
		}
	}
	if groundItems == nil {
		groundItems = make([]GroundItem, 0)
	}
	var (
		patches                 []Patch
		restorableLegacyPatches []Patch
		restorableSimPatches    []sim.Patch
	)
	if drainPatches {
		if engine != nil {
			drained := engine.DrainPatches()
			if len(drained) > 0 {
				restorableSimPatches = append([]sim.Patch(nil), drained...)
			}
			patches = legacyPatchesFromSim(drained)
		} else {
			patches = h.world.drainPatchesLocked()
		}
		if len(patches) > 0 && engine == nil {
			restorableLegacyPatches = append([]Patch(nil), patches...)
		}
	} else {
		if engine != nil {
			snapshot := engine.SnapshotPatches()
			patches = legacyPatchesFromSim(snapshot)
		} else {
			patches = h.world.snapshotPatchesLocked()
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
				if player.ID == "" {
					continue
				}
				alive[player.ID] = struct{}{}
			}
			for _, npc := range aliveNPCs {
				if npc.ID == "" {
					continue
				}
				alive[npc.ID] = struct{}{}
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
					if patch.Kind == PatchPlayerRemoved {
						filtered = append(filtered, patch)
						continue
					}
					if patch.Kind == PatchGroundItemQty {
						if payload, ok := patch.Payload.(GroundItemQtyPayload); ok && payload.Qty <= 0 {
							filtered = append(filtered, patch)
						}
					}
					continue
				}
				filtered = append(filtered, patch)
			}
			patches = filtered
		}
	}
	simPlayerPatches := filterPlayerPatches(simPatchesFromLegacy(patches))
	h.updateResubscribeBaselinesLocked(simSnapshot, includeSnapshot, simPlayerPatches)
	if includeSnapshot {
		players = mergePlayersFromBaselines(players, h.resubscribeBaselines)
	}
	cfg := h.config
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
		patches = make([]Patch, 0)
	}

	if includeSnapshot {
		if players == nil {
			players = make([]Player, 0)
		}
		if npcs == nil {
			npcs = make([]NPC, 0)
		}
	} else {
		players = nil
		npcs = nil
	}
	if triggers == nil {
		triggers = make([]EffectTrigger, 0)
	}
	if includeSnapshot {
		if groundItems == nil {
			groundItems = make([]GroundItem, 0)
		}
	} else {
		groundItems = nil
	}
	if obstacles == nil {
		obstacles = make([]Obstacle, 0)
	}

	keyframeSeq := h.lastKeyframeSeq.Load()
	if includeSnapshot {
		simFrame := sim.Keyframe{
			Tick:        tick,
			Sequence:    seq,
			Players:     simPlayersFromLegacy(players),
			NPCs:        simNPCsFromLegacy(npcs),
			Obstacles:   simObstaclesFromLegacy(obstacles),
			GroundItems: simGroundItemsFromLegacy(groundItems),
			Config:      simWorldConfigFromLegacy(cfg),
		}
		var record sim.KeyframeRecordResult
		if engine != nil {
			record = engine.RecordKeyframe(simFrame)
		} else {
			legacyFrame := keyframe{
				Tick:        tick,
				Sequence:    seq,
				Players:     players,
				NPCs:        npcs,
				Obstacles:   obstacles,
				GroundItems: groundItems,
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
		Type:             "state",
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
		Config:           cfg,
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
	data, err := json.Marshal(msg)
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
func (h *Hub) MarshalState(players []Player, npcs []NPC, triggers []EffectTrigger, groundItems []GroundItem, drainPatches bool, includeSnapshot bool) ([]byte, int, error) {
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
		legacy := legacyKeyframeFromSim(frame)
		snapshot := keyframeMessage{
			Ver:         ProtocolVersion,
			Type:        "keyframe",
			Sequence:    legacy.Sequence,
			Tick:        legacy.Tick,
			Players:     append([]Player(nil), legacy.Players...),
			NPCs:        append([]NPC(nil), legacy.NPCs...),
			Obstacles:   append([]Obstacle(nil), legacy.Obstacles...),
			GroundItems: append([]GroundItem(nil), legacy.GroundItems...),
			Config:      legacy.Config,
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
			Type:     "keyframeNack",
			Sequence: sequence,
			Reason:   "rate_limited",
			Resync:   true,
			Config:   h.resyncConfigSnapshot(),
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
			Type:     "keyframeNack",
			Sequence: sequence,
			Reason:   "expired",
			Resync:   true,
			Config:   h.resyncConfigSnapshot(),
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
	data, entities, err := h.marshalState(players, npcs, triggers, groundItems, true, includeSnapshot)
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
		sub.mu.Lock()
		sub.conn.SetWriteDeadline(h.now().Add(writeWait))
		err := sub.conn.WriteMessage(websocket.TextMessage, data)
		sub.mu.Unlock()
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

func (h *Hub) enqueueCommand(cmd sim.Command) (bool, string) {
	var queueLen int
	var dropped bool
	var dropCount uint64
	h.commandsMu.Lock()
	if commandQueuePerActorLimit > 0 && cmd.ActorID != "" {
		if h.pendingCommandsByActor == nil {
			h.pendingCommandsByActor = make(map[string]int)
		}
		count := h.pendingCommandsByActor[cmd.ActorID]
		if count >= commandQueuePerActorLimit {
			if h.droppedCommandsByActor == nil {
				h.droppedCommandsByActor = make(map[string]uint64)
			}
			dropped = true
			dropCount = h.droppedCommandsByActor[cmd.ActorID] + 1
			h.droppedCommandsByActor[cmd.ActorID] = dropCount
		} else {
			h.pendingCommandsByActor[cmd.ActorID] = count + 1
			h.pendingCommands = append(h.pendingCommands, cmd)
			queueLen = len(h.pendingCommands)
		}
	} else {
		h.pendingCommands = append(h.pendingCommands, cmd)
		if commandQueuePerActorLimit > 0 && cmd.ActorID != "" {
			if h.pendingCommandsByActor == nil {
				h.pendingCommandsByActor = make(map[string]int)
			}
			h.pendingCommandsByActor[cmd.ActorID]++
		}
		queueLen = len(h.pendingCommands)
	}
	h.commandsMu.Unlock()

	if dropped {
		if h.telemetry != nil {
			h.telemetry.RecordCommandDropped("limit_exceeded", string(cmd.Type))
		}
		if dropCount > 0 && dropCount&(dropCount-1) == 0 {
			h.logf(
				"[backpressure] dropping command actor=%s type=%s count=%d limit=%d",
				cmd.ActorID,
				cmd.Type,
				dropCount,
				commandQueuePerActorLimit,
			)
		}
		return false, "queue_limit"
	}

	if commandQueueWarningStep > 0 && queueLen >= commandQueueWarningStep && queueLen%commandQueueWarningStep == 0 {
		h.logf("[backpressure] pendingCommands=%d; investigate tick latency or raise throttle thresholds", queueLen)
	}

	return true, ""
}

func (h *Hub) drainCommands() []sim.Command {
	h.commandsMu.Lock()
	cmds := h.pendingCommands
	h.pendingCommands = nil
	if len(h.pendingCommandsByActor) > 0 {
		h.pendingCommandsByActor = make(map[string]int)
	}
	h.commandsMu.Unlock()
	if len(cmds) == 0 {
		return nil
	}
	copied := make([]sim.Command, len(cmds))
	copy(copied, cmds)
	return copied
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

func playerFromView(view simpaches.PlayerView) Player {
	return legacyPlayerFromSim(view.Player)
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

func mergePlayersFromBaselines(players []Player, baselines map[string]simpaches.PlayerView) []Player {
	if len(baselines) == 0 {
		return players
	}
	if players == nil {
		players = make([]Player, 0, len(baselines))
	}

	seen := make(map[string]struct{}, len(players))
	for i := range players {
		id := players[i].ID
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
