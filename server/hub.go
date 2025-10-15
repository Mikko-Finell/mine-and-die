package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdlog "log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"mine-and-die/server/logging"
	loggingeconomy "mine-and-die/server/logging/economy"
	logginglifecycle "mine-and-die/server/logging/lifecycle"
	loggingnetwork "mine-and-die/server/logging/network"
	loggingsimulation "mine-and-die/server/logging/simulation"
	stats "mine-and-die/server/stats"
)

// Hub coordinates subscribers and orchestrates the deterministic world simulation.
type Hub struct {
	mu          sync.Mutex
	world       *World
	subscribers map[string]*subscriber
	config      worldConfig
	publisher   logging.Publisher
	telemetry   *telemetryCounters

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

	commandsMu      sync.Mutex // protects pendingCommands between network handlers and the tick loop
	pendingCommands []Command  // staged commands applied in order at the next simulation step
}

type subscriber struct {
	conn    *websocket.Conn
	mu      sync.Mutex
	lastAck atomic.Uint64
	limiter keyframeRateLimiter
}

const (
	keyframeLimiterCapacity  = 3
	keyframeLimiterRefillPer = 2.0 // tokens per second

	commandQueueWarningStep = 256

	tickBudgetCatchupMaxTicks = 2
	tickBudgetAlarmMinStreak  = 3
	tickBudgetAlarmMinRatio   = 2.0
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

type hubConfig struct {
	KeyframeInterval int
}

func defaultHubConfig() hubConfig {
	return hubConfig{KeyframeInterval: 30}
}

// newHub creates a hub with empty maps and a freshly generated world.
func newHub(pubs ...logging.Publisher) *Hub {
	return newHubWithConfig(defaultHubConfig(), pubs...)
}

func newHubWithConfig(hubCfg hubConfig, pubs ...logging.Publisher) *Hub {
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

	hub := &Hub{
		world:                   newWorld(cfg, pub),
		subscribers:             make(map[string]*subscriber),
		pendingCommands:         make([]Command, 0),
		config:                  cfg,
		publisher:               pub,
		telemetry:               newTelemetryCounters(),
		defaultKeyframeInterval: interval,
	}
	hub.world.telemetry = hub.telemetry
	hub.world.journal.AttachTelemetry(hub.telemetry)
	hub.keyframeInterval.Store(int64(interval))
	hub.forceKeyframe()
	return hub
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
	now := time.Now()

	player := h.seedPlayerState(playerID, now)

	h.mu.Lock()
	h.world.AddPlayer(player)
	players, npcs := h.world.Snapshot(now)
	groundItems := h.world.GroundItemsSnapshot()
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
		Ver:              ProtocolVersion,
		ID:               playerID,
		Players:          players,
		NPCs:             npcs,
		Obstacles:        obstacles,
		GroundItems:      groundItems,
		Config:           cfg,
		Resync:           true,
		KeyframeInterval: h.CurrentKeyframeInterval(),
	}
}

// ResetWorld replaces the current world with a freshly generated instance.
func (h *Hub) ResetWorld(cfg worldConfig) ([]Player, []NPC) {
	cfg = cfg.normalized()
	now := time.Now()

	h.commandsMu.Lock()
	h.pendingCommands = nil
	h.commandsMu.Unlock()

	h.mu.Lock()
	playerIDs := make([]string, 0, len(h.world.players))
	for id := range h.world.players {
		playerIDs = append(playerIDs, id)
	}

	newW := newWorld(cfg, h.publisher)
	newW.telemetry = h.telemetry
	newW.journal.AttachTelemetry(h.telemetry)
	for _, id := range playerIDs {
		newW.AddPlayer(h.seedPlayerState(id, now))
	}
	h.world = newW
	h.config = cfg
	players, npcs := h.world.Snapshot(now)
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

	state.lastHeartbeat = time.Now()

	if existing, ok := h.subscribers[playerID]; ok {
		existing.conn.Close()
	}

	sub := &subscriber{conn: conn, limiter: newKeyframeRateLimiter(keyframeLimiterCapacity, keyframeLimiterRefillPer)}
	h.subscribers[playerID] = sub
	now := time.Now()
	players, npcs := h.world.Snapshot(now)
	groundItems := h.world.GroundItemsSnapshot()
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
		now := time.Now()
		players, npcs = h.world.Snapshot(now)
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
func (h *Hub) UpdateIntent(playerID string, dx, dy float64, facing string) bool {
	parsedFacing := FacingDirection("")
	if facing != "" {
		if face, ok := parseFacing(facing); ok {
			parsedFacing = face
		}
	}

	if !h.playerExists(playerID) {
		return false
	}

	cmd := Command{
		OriginTick: h.tick.Load(),
		ActorID:    playerID,
		Type:       CommandMove,
		IssuedAt:   time.Now(),
		Move: &MoveCommand{
			DX:     dx,
			DY:     dy,
			Facing: parsedFacing,
		},
	}
	h.enqueueCommand(cmd)
	return true
}

// SetPlayerPath queues a command that asks the server to navigate the player toward a point.
func (h *Hub) SetPlayerPath(playerID string, x, y float64) bool {
	if !h.playerExists(playerID) {
		return false
	}
	cmd := Command{
		OriginTick: h.tick.Load(),
		ActorID:    playerID,
		Type:       CommandSetPath,
		IssuedAt:   time.Now(),
		Path: &PathCommand{
			TargetX: x,
			TargetY: y,
		},
	}
	h.enqueueCommand(cmd)
	return true
}

// ClearPlayerPath stops any server-driven navigation for the player.
func (h *Hub) ClearPlayerPath(playerID string) bool {
	if !h.playerExists(playerID) {
		return false
	}
	cmd := Command{
		OriginTick: h.tick.Load(),
		ActorID:    playerID,
		Type:       CommandClearPath,
		IssuedAt:   time.Now(),
	}
	h.enqueueCommand(cmd)
	return true
}

// HandleAction queues an action command for processing on the next tick.
func (h *Hub) HandleAction(playerID, action string) bool {
	switch action {
	case effectTypeAttack, effectTypeFireball:
	default:
		return false
	}
	if !h.playerExists(playerID) {
		return false
	}
	cmd := Command{
		OriginTick: h.tick.Load(),
		ActorID:    playerID,
		Type:       CommandAction,
		IssuedAt:   time.Now(),
		Action: &ActionCommand{
			Name: action,
		},
	}
	h.enqueueCommand(cmd)
	return true
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
		groundItems := h.world.GroundItemsSnapshot()
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
		groundItems := h.world.GroundItemsSnapshot()
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

	cmd := Command{
		OriginTick: h.tick.Load(),
		ActorID:    playerID,
		Type:       CommandHeartbeat,
		IssuedAt:   receivedAt,
		Heartbeat: &HeartbeatCommand{
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
	removed := h.world.Step(tick, now, dt, commands, nil)
	players, npcs := h.world.Snapshot(now)
	if h.telemetry != nil {
		h.telemetry.RecordEffectsActive(len(h.world.effects))
	}
	groundItems := h.world.GroundItemsSnapshot()
	triggers := h.world.flushEffectTriggersLocked()
	toClose := make([]*subscriber, 0, len(removed))
	for _, id := range removed {
		if sub, ok := h.subscribers[id]; ok {
			toClose = append(toClose, sub)
			delete(h.subscribers, id)
		}
	}
	h.mu.Unlock()

	return players, npcs, triggers, groundItems, toClose
}

// RunSimulation drives the fixed-rate tick loop until the stop channel closes.
func (h *Hub) RunSimulation(stop <-chan struct{}) {
	ticker := time.NewTicker(time.Second / tickRate)
	defer ticker.Stop()

	last := time.Now()
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
		case now := <-ticker.C:
			tickStart := time.Now()
			dt := now.Sub(last).Seconds()
			clamped := false
			if dt <= 0 {
				dt = budgetSeconds
			} else if dt > maxDtSeconds {
				dt = maxDtSeconds
				clamped = true
			}
			last = now

			players, npcs, triggers, groundItems, toClose := h.advance(now, dt)
			for _, sub := range toClose {
				sub.conn.Close()
			}
			h.broadcastState(players, npcs, triggers, groundItems)
			duration := time.Since(tickStart)
			if h.telemetry != nil {
				h.telemetry.RecordTickDuration(duration)
			}
			if tickBudget > 0 && duration > tickBudget {
				ratio := float64(duration) / float64(tickBudget)
				streak := uint64(0)
				if h.telemetry != nil {
					streak = h.telemetry.RecordTickBudgetOverrun(duration, tickBudget)
				}
				stdlog.Printf(
					"[tick] budget overrun: duration=%s budget=%s ratio=%.2f streak=%d",
					duration,
					tickBudget,
					ratio,
					streak,
				)
				if h.publisher != nil {
					extra := map[string]any{
						"ratio":        ratio,
						"dtSeconds":    dt,
						"clamped":      clamped,
						"maxDtSeconds": maxDtSeconds,
					}
					loggingsimulation.TickBudgetOverrun(
						context.Background(),
						h.publisher,
						h.tick.Load(),
						loggingsimulation.TickBudgetOverrunPayload{
							DurationMillis: duration.Milliseconds(),
							BudgetMillis:   tickBudget.Milliseconds(),
							Ratio:          ratio,
							Streak:         streak,
						},
						extra,
					)
				}
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
	stdlog.Printf(
		"[tick] budget alarm triggered: scheduling resync ratio=%.2f streak=%d dt=%.4f clamped=%t",
		ratio,
		streak,
		dt,
		clamped,
	)

	if h.telemetry != nil {
		h.telemetry.RecordTickBudgetAlarm(tick, ratio)
	}

	if h.publisher != nil {
		extra := map[string]any{
			"ratio":        ratio,
			"dtSeconds":    dt,
			"clamped":      clamped,
			"maxDtSeconds": maxDtSeconds,
			"alarm":        true,
		}
		loggingsimulation.TickBudgetAlarm(
			context.Background(),
			h.publisher,
			tick,
			loggingsimulation.TickBudgetAlarmPayload{
				DurationMillis:  duration.Milliseconds(),
				BudgetMillis:    budget.Milliseconds(),
				Ratio:           ratio,
				Streak:          streak,
				ResyncScheduled: true,
				ThresholdRatio:  tickBudgetAlarmMinRatio,
				ThresholdStreak: tickBudgetAlarmMinStreak,
			},
			extra,
		)
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
	if (players == nil || npcs == nil) && includeSnapshot {
		now := time.Now()
		players, npcs = h.world.Snapshot(now)
		if triggers == nil {
			triggers = h.world.flushEffectTriggersLocked()
		}
	} else if triggers == nil {
		triggers = make([]EffectTrigger, 0)
	}
	if groundItems == nil && includeSnapshot {
		groundItems = h.world.GroundItemsSnapshot()
	}
	var aliveEffectIDs []string
	if len(h.world.effects) > 0 {
		aliveEffectIDs = make([]string, 0, len(h.world.effects))
		for _, eff := range h.world.effects {
			if eff == nil || eff.ID == "" {
				continue
			}
			aliveEffectIDs = append(aliveEffectIDs, eff.ID)
		}
	}
	var patches []Patch
	if drainPatches {
		patches = h.world.drainPatchesLocked()
	} else {
		patches = h.world.snapshotPatchesLocked()
	}
	obstacles := []Obstacle(nil)
	if includeSnapshot {
		obstacles = append([]Obstacle(nil), h.world.obstacles...)
	}
	cfg := h.config
	tick := h.tick.Load()
	seq, resync := h.nextStateMeta(drainPatches)
	effectManagerPresent := h.world.effectManager != nil
	effectTransportEnabled := effectManagerPresent
	journal := &h.world.journal
	h.mu.Unlock()

	effectBatch := EffectEventBatch{}
	if effectManagerPresent {
		if drainPatches {
			effectBatch = journal.DrainEffectEvents()
		} else {
			effectBatch = journal.SnapshotEffectEvents()
		}
	}

	if patches == nil {
		patches = make([]Patch, 0)
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
		frame := keyframe{
			Tick:        tick,
			Sequence:    seq,
			Players:     players,
			NPCs:        npcs,
			Obstacles:   obstacles,
			GroundItems: groundItems,
			Config:      cfg,
		}
		record := journal.RecordKeyframe(frame)
		h.lastKeyframeSeq.Store(seq)
		h.lastKeyframeTick.Store(tick)
		keyframeSeq = seq
		if h.telemetry != nil {
			h.telemetry.RecordKeyframeJournal(record.Size, record.OldestSequence, record.NewestSequence)
		}
		if h.telemetry != nil && h.telemetry.DebugEnabled() {
			stdlog.Printf("[journal] add sequence=%d tick=%d size=%d", seq, tick, record.Size)
			for _, eviction := range record.Evicted {
				stdlog.Printf("[journal] evict sequence=%d tick=%d size=%d reason=%s", eviction.Sequence, eviction.Tick, record.Size, eviction.Reason)
			}
			stdlog.Printf("[journal] window size=%d oldest=%d newest=%d", record.Size, record.OldestSequence, record.NewestSequence)
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
		ServerTime:       time.Now().UnixMilli(),
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
	return data, entities, err
}

func (h *Hub) scheduleResyncIfNeeded() (bool, resyncSignal) {
	h.mu.Lock()
	journal := &h.world.journal
	h.mu.Unlock()

	signal, ok := journal.ConsumeResyncHint()
	if !ok {
		return false, resyncSignal{}
	}

	h.forceKeyframe()
	h.resyncNext.Store(true)

	summary := signal.summary()
	if summary == "" {
		stdlog.Printf("[effects] scheduling resync (journal hint)")
	} else {
		stdlog.Printf("[effects] scheduling resync (journal hint): %s", summary)
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
	journal := &h.world.journal
	h.mu.Unlock()

	frame, ok := journal.KeyframeBySequence(sequence)
	if ok {
		snapshot := keyframeMessage{
			Ver:         ProtocolVersion,
			Type:        "keyframe",
			Sequence:    frame.Sequence,
			Tick:        frame.Tick,
			Players:     append([]Player(nil), frame.Players...),
			NPCs:        append([]NPC(nil), frame.NPCs...),
			Obstacles:   append([]Obstacle(nil), frame.Obstacles...),
			GroundItems: append([]GroundItem(nil), frame.GroundItems...),
			Config:      frame.Config,
		}
		return snapshot, keyframeLookupFound
	}

	size, oldest, newest := journal.KeyframeWindow()
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

	now := time.Now()
	if sub != nil && !sub.limiter.allow(now) {
		if h.telemetry != nil {
			h.telemetry.RecordKeyframeRequest(0, false)
			h.telemetry.IncrementKeyframeRateLimited()
		}
		stdlog.Printf("[keyframe] rate_limited player=%s sequence=%d", playerID, sequence)
		nack := &keyframeNackMessage{
			Ver:      ProtocolVersion,
			Type:     "keyframeNack",
			Sequence: sequence,
			Reason:   "rate_limited",
		}
		return keyframeMessage{}, nack, true
	}

	snapshot, status := h.lookupKeyframe(sequence)
	latency := time.Since(now)
	switch status {
	case keyframeLookupFound:
		if h.telemetry != nil {
			h.telemetry.RecordKeyframeRequest(latency, true)
		}
		stdlog.Printf("[keyframe] served player=%s sequence=%d tick=%d latency_ms=%d", playerID, snapshot.Sequence, snapshot.Tick, latency.Milliseconds())
		return snapshot, nil, true
	case keyframeLookupExpired:
		if h.telemetry != nil {
			h.telemetry.RecordKeyframeRequest(latency, false)
			h.telemetry.IncrementKeyframeExpired()
		}
		stdlog.Printf("[keyframe] expired player=%s sequence=%d", playerID, sequence)
		nack := &keyframeNackMessage{
			Ver:      ProtocolVersion,
			Type:     "keyframeNack",
			Sequence: sequence,
			Reason:   "expired",
		}
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
		stdlog.Printf("failed to marshal state message: %v", err)
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
		stdlog.Printf(
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
		sub.conn.SetWriteDeadline(time.Now().Add(writeWait))
		err := sub.conn.WriteMessage(websocket.TextMessage, data)
		sub.mu.Unlock()
		if err != nil {
			stdlog.Printf("failed to send update to %s: %v", id, err)
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

func (h *Hub) TelemetrySnapshot() telemetrySnapshot {
	if h.telemetry == nil {
		return telemetrySnapshot{}
	}
	return h.telemetry.Snapshot()
}

func (h *Hub) enqueueCommand(cmd Command) {
	h.commandsMu.Lock()
	h.pendingCommands = append(h.pendingCommands, cmd)
	queueLen := len(h.pendingCommands)
	h.commandsMu.Unlock()

	if commandQueueWarningStep > 0 && queueLen >= commandQueueWarningStep && queueLen%commandQueueWarningStep == 0 {
		stdlog.Printf("[backpressure] pendingCommands=%d with no guardrails; add queue limits before wider testing", queueLen)
	}
}

func (h *Hub) drainCommands() []Command {
	h.commandsMu.Lock()
	cmds := h.pendingCommands
	h.pendingCommands = nil
	h.commandsMu.Unlock()
	if len(cmds) == 0 {
		return nil
	}
	copied := make([]Command, len(cmds))
	copy(copied, cmds)
	return copied
}

func (h *Hub) playerExists(playerID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.world.HasPlayer(playerID)
}
