package main

import (
	"context"
	"encoding/json"
	"fmt"
	stdlog "log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"mine-and-die/server/logging"
	loggingeconomy "mine-and-die/server/logging/economy"
	logginglifecycle "mine-and-die/server/logging/lifecycle"
)

// Hub coordinates subscribers and orchestrates the deterministic world simulation.
type Hub struct {
	mu          sync.Mutex
	world       *World
	subscribers map[string]*subscriber
	config      worldConfig
	publisher   logging.Publisher

	nextID atomic.Uint64
	tick   atomic.Uint64

	commandsMu      sync.Mutex // protects pendingCommands between network handlers and the tick loop
	pendingCommands []Command  // staged commands applied in order at the next simulation step
}

type subscriber struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// newHub creates a hub with empty maps and a freshly generated world.
func newHub(pubs ...logging.Publisher) *Hub {
	cfg := defaultWorldConfig().normalized()
	var pub logging.Publisher
	if len(pubs) > 0 && pubs[0] != nil {
		pub = pubs[0]
	}
	if pub == nil {
		pub = logging.NopPublisher{}
	}
	return &Hub{
		world:           newWorld(cfg, pub),
		subscribers:     make(map[string]*subscriber),
		pendingCommands: make([]Command, 0),
		config:          cfg,
		publisher:       pub,
	}
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

	return &playerState{
		actorState: actorState{
			Actor: Actor{
				ID:        playerID,
				X:         defaultSpawnX,
				Y:         defaultSpawnY,
				Facing:    defaultFacing,
				Health:    playerMaxHealth,
				MaxHealth: playerMaxHealth,
				Inventory: inventory,
			},
		},
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
	players, npcs, effects, groundItems := h.world.Snapshot(now)
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

	go h.broadcastState(players, npcs, effects, groundItems, nil)

	return joinResponse{ID: playerID, Players: players, NPCs: npcs, Obstacles: obstacles, Effects: effects, GroundItems: groundItems, Config: cfg}
}

// ResetWorld replaces the current world with a freshly generated instance.
func (h *Hub) ResetWorld(cfg worldConfig) ([]Player, []NPC, []Effect, []GroundItem) {
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
	for _, id := range playerIDs {
		newW.AddPlayer(h.seedPlayerState(id, now))
	}
	h.world = newW
	h.config = cfg
	players, npcs, effects, groundItems := h.world.Snapshot(now)
	h.mu.Unlock()

	h.tick.Store(0)

	return players, npcs, effects, groundItems
}

// CurrentConfig returns a copy of the active world configuration.
func (h *Hub) CurrentConfig() worldConfig {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.config
}

// Subscribe associates a WebSocket connection with an existing player.
func (h *Hub) Subscribe(playerID string, conn *websocket.Conn) (*subscriber, []Player, []NPC, []Effect, []GroundItem, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	state, ok := h.world.players[playerID]
	if !ok {
		return nil, nil, nil, nil, nil, false
	}

	state.lastHeartbeat = time.Now()

	if existing, ok := h.subscribers[playerID]; ok {
		existing.conn.Close()
	}

	sub := &subscriber{conn: conn}
	h.subscribers[playerID] = sub
	now := time.Now()
	players, npcs, effects, groundItems := h.world.Snapshot(now)
	return sub, players, npcs, effects, groundItems, true
}

// Disconnect removes a player and closes any active subscriber connection.
func (h *Hub) Disconnect(playerID string) ([]Player, []NPC, []Effect, []GroundItem) {
	h.mu.Lock()
	sub, subOK := h.subscribers[playerID]
	if subOK {
		delete(h.subscribers, playerID)
	}

	removed := h.world.RemovePlayer(playerID)
	var players []Player
	var npcs []NPC
	var effects []Effect
	var groundItems []GroundItem
	if removed {
		now := time.Now()
		players, npcs, effects, groundItems = h.world.Snapshot(now)
	}
	h.mu.Unlock()

	if subOK {
		sub.conn.Close()
	}

	if !removed {
		return nil, nil, nil, nil
	}

	logginglifecycle.PlayerDisconnected(
		context.Background(),
		h.publisher,
		h.tick.Load(),
		logging.EntityRef{ID: playerID, Kind: logging.EntityKind("player")},
		logginglifecycle.PlayerDisconnectedPayload{Reason: "manual"},
		nil,
	)

	return players, npcs, effects, groundItems
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

func (h *Hub) QueueDropGold(playerID string, qty int) (<-chan consoleCommandResult, bool) {
	result := make(chan consoleCommandResult, 1)
	if !h.playerExists(playerID) {
		result <- consoleCommandResult{Command: "drop_gold", Status: consoleStatusError, Reason: consoleReasonActorMissing}
		close(result)
		return result, false
	}
	cmd := Command{
		OriginTick:    h.tick.Load(),
		ActorID:       playerID,
		Type:          CommandDropGold,
		IssuedAt:      time.Now(),
		DropGold:      &DropGoldCommand{Quantity: qty},
		ConsoleResult: result,
	}
	h.enqueueCommand(cmd)
	return result, true
}

func (h *Hub) QueuePickupGold(playerID string) (<-chan consoleCommandResult, bool) {
	result := make(chan consoleCommandResult, 1)
	if !h.playerExists(playerID) {
		result <- consoleCommandResult{Command: "pickup_gold", Status: consoleStatusError, Reason: consoleReasonActorMissing}
		close(result)
		return result, false
	}
	cmd := Command{
		OriginTick:    h.tick.Load(),
		ActorID:       playerID,
		Type:          CommandPickupGold,
		IssuedAt:      time.Now(),
		PickupGold:    &PickupGoldCommand{},
		ConsoleResult: result,
	}
	h.enqueueCommand(cmd)
	return result, true
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
func (h *Hub) advance(now time.Time, dt float64) ([]Player, []NPC, []Effect, []GroundItem, []EffectTrigger, []*subscriber) {
	tick := h.tick.Add(1)
	commands := h.drainCommands()

	h.mu.Lock()
	removed := h.world.Step(tick, now, dt, commands)
	players, npcs, effects, groundItems := h.world.Snapshot(now)
	triggers := h.world.flushEffectTriggersLocked()
	toClose := make([]*subscriber, 0, len(removed))
	for _, id := range removed {
		if sub, ok := h.subscribers[id]; ok {
			toClose = append(toClose, sub)
			delete(h.subscribers, id)
		}
	}
	h.mu.Unlock()

	return players, npcs, effects, groundItems, triggers, toClose
}

// RunSimulation drives the fixed-rate tick loop until the stop channel closes.
func (h *Hub) RunSimulation(stop <-chan struct{}) {
	ticker := time.NewTicker(time.Second / tickRate)
	defer ticker.Stop()

	last := time.Now()
	for {
		select {
		case <-stop:
			return
		case now := <-ticker.C:
			dt := now.Sub(last).Seconds()
			if dt <= 0 {
				dt = 1.0 / float64(tickRate)
			}
			last = now

			players, npcs, effects, groundItems, triggers, toClose := h.advance(now, dt)
			for _, sub := range toClose {
				sub.conn.Close()
			}
			h.broadcastState(players, npcs, effects, groundItems, triggers)
		}
	}
}

// DiagnosticsSnapshot exposes heartbeat data for the diagnostics endpoint.
func (h *Hub) DiagnosticsSnapshot() []diagnosticsPlayer {
	h.mu.Lock()
	defer h.mu.Unlock()

	players := make([]diagnosticsPlayer, 0, len(h.world.players))
	for _, state := range h.world.players {
		players = append(players, diagnosticsPlayer{
			ID:            state.ID,
			LastHeartbeat: state.lastHeartbeat.UnixMilli(),
			RTTMillis:     state.lastRTT.Milliseconds(),
		})
	}
	return players
}

// broadcastState sends the latest world snapshot to every subscriber.
func (h *Hub) broadcastState(players []Player, npcs []NPC, effects []Effect, groundItems []GroundItem, triggers []EffectTrigger) {
	h.mu.Lock()
	shouldFlushTriggers := false
	if players == nil || npcs == nil || effects == nil || groundItems == nil {
		now := time.Now()
		players, npcs, effects, groundItems = h.world.Snapshot(now)
		shouldFlushTriggers = true
	}
	if shouldFlushTriggers && triggers == nil {
		triggers = h.world.flushEffectTriggersLocked()
	}
	obstacles := append([]Obstacle(nil), h.world.obstacles...)
	cfg := h.config
	h.mu.Unlock()

	msg := stateMessage{
		Type:           "state",
		Players:        players,
		NPCs:           npcs,
		Obstacles:      obstacles,
		Effects:        effects,
		GroundItems:    groundItems,
		EffectTriggers: triggers,
		ServerTime:     time.Now().UnixMilli(),
		Config:         cfg,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		stdlog.Printf("failed to marshal state message: %v", err)
		return
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
			players, npcs, effects, groundItems := h.Disconnect(id)
			if players != nil {
				go h.broadcastState(players, npcs, effects, groundItems, nil)
			}
		}
	}
}

func (h *Hub) enqueueCommand(cmd Command) {
	h.commandsMu.Lock()
	h.pendingCommands = append(h.pendingCommands, cmd)
	h.commandsMu.Unlock()
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
