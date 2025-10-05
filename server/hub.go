package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// Hub owns all live players, subscribers, obstacles, and active effects.
type Hub struct {
	mu              sync.Mutex
	players         map[string]*playerState
	subscribers     map[string]*subscriber
	nextID          atomic.Uint64
	obstacles       []Obstacle
	effects         []*effectState
	nextEffect      atomic.Uint64
	effectBehaviors map[string]effectBehavior
}

type subscriber struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// newHub creates a hub with empty maps and a freshly generated obstacle set.
func newHub() *Hub {
	hub := &Hub{
		players:         make(map[string]*playerState),
		subscribers:     make(map[string]*subscriber),
		effects:         make([]*effectState, 0),
		effectBehaviors: newEffectBehaviors(),
	}
	hub.obstacles = hub.generateObstacles(obstacleCount)
	return hub
}

// Join registers a new player and returns the latest snapshot.
func (h *Hub) Join() joinResponse {
	id := h.nextID.Add(1)
	playerID := fmt.Sprintf("player-%d", id)
	now := time.Now()
	inventory := NewInventory()
	if _, err := inventory.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 50}); err != nil {
		log.Printf("failed to seed gold for %s: %v", playerID, err)
	}
	if _, err := inventory.AddStack(ItemStack{Type: ItemTypeHealthPotion, Quantity: 2}); err != nil {
		log.Printf("failed to seed potions for %s: %v", playerID, err)
	}

	player := &playerState{
		Player: Player{
			ID:        playerID,
			X:         80,
			Y:         80,
			Facing:    defaultFacing,
			Health:    playerMaxHealth,
			MaxHealth: playerMaxHealth,
			Inventory: inventory,
		},
		lastHeartbeat: now,
		cooldowns:     make(map[string]time.Time),
	}

	h.mu.Lock()
	h.pruneEffectsLocked(now)
	h.players[playerID] = player
	players, effects := h.snapshotLocked(now)
	h.mu.Unlock()

	go h.broadcastState(players, effects)

	return joinResponse{ID: playerID, Players: players, Obstacles: h.obstacles, Effects: effects}
}

// Subscribe associates a WebSocket connection with an existing player.
func (h *Hub) Subscribe(playerID string, conn *websocket.Conn) (*subscriber, []Player, []Effect, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	state, ok := h.players[playerID]
	if !ok {
		return nil, nil, nil, false
	}

	state.lastHeartbeat = time.Now()

	if existing, ok := h.subscribers[playerID]; ok {
		existing.conn.Close()
	}

	sub := &subscriber{conn: conn}
	h.subscribers[playerID] = sub
	now := time.Now()
	h.pruneEffectsLocked(now)
	players, effects := h.snapshotLocked(now)
	return sub, players, effects, true
}

// Disconnect removes a player and closes any active subscriber connection.
func (h *Hub) Disconnect(playerID string) ([]Player, []Effect) {
	h.mu.Lock()
	sub, subOK := h.subscribers[playerID]
	if subOK {
		delete(h.subscribers, playerID)
	}

	_, playerOK := h.players[playerID]
	if playerOK {
		delete(h.players, playerID)
	}

	var players []Player
	var effects []Effect
	if playerOK {
		now := time.Now()
		h.pruneEffectsLocked(now)
		players, effects = h.snapshotLocked(now)
	}
	h.mu.Unlock()

	if subOK {
		sub.conn.Close()
	}

	if !playerOK {
		return nil, nil
	}

	return players, effects
}

// UpdateIntent stores the latest movement vector and facing for a player.
func (h *Hub) UpdateIntent(playerID string, dx, dy float64, facing string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	state, ok := h.players[playerID]
	if !ok {
		return false
	}

	if state.Facing == "" {
		state.Facing = defaultFacing
	}

	length := math.Hypot(dx, dy)
	if length > 1 {
		dx /= length
		dy /= length
	}

	state.intentX = dx
	state.intentY = dy

	state.Facing = deriveFacing(dx, dy, state.Facing)
	if dx == 0 && dy == 0 {
		if face, ok := parseFacing(facing); ok {
			state.Facing = face
		}
	}

	state.lastInput = time.Now()
	return true
}

// UpdateHeartbeat records the most recent heartbeat time and RTT for a player.
func (h *Hub) UpdateHeartbeat(playerID string, receivedAt time.Time, clientSent int64) (time.Duration, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	state, ok := h.players[playerID]
	if !ok {
		return 0, false
	}

	state.lastHeartbeat = receivedAt

	var rtt time.Duration
	if clientSent > 0 {
		clientTime := time.UnixMilli(clientSent)
		if clientTime.Before(receivedAt.Add(5 * time.Second)) {
			rtt = receivedAt.Sub(clientTime)
			if rtt < 0 {
				rtt = 0
			}
			state.lastRTT = rtt
		}
	}

	return state.lastRTT, true
}

// advance runs a single simulation step and returns updated snapshots plus stale subscribers.
func (h *Hub) advance(now time.Time, dt float64) ([]Player, []Effect, []*subscriber) {
	h.mu.Lock()

	toClose := make([]*subscriber, 0)
	activeStates := make([]*playerState, 0, len(h.players))
	for id, state := range h.players {
		if now.Sub(state.lastHeartbeat) > disconnectAfter {
			if sub, ok := h.subscribers[id]; ok {
				toClose = append(toClose, sub)
				delete(h.subscribers, id)
			}
			delete(h.players, id)
			log.Printf("disconnecting %s due to heartbeat timeout", id)
			continue
		}

		activeStates = append(activeStates, state)

		if state.intentX != 0 || state.intentY != 0 {
			movePlayerWithObstacles(state, dt, h.obstacles)
		}
	}

	resolvePlayerCollisions(activeStates, h.obstacles)
	h.applyEnvironmentalDamageLocked(activeStates, dt)

	h.advanceEffectsLocked(now, dt)
	h.pruneEffectsLocked(now)
	players, effects := h.snapshotLocked(now)
	h.mu.Unlock()

	return players, effects, toClose
}

// applyEnvironmentalDamageLocked processes hazard areas that deal damage over time.
func (h *Hub) applyEnvironmentalDamageLocked(states []*playerState, dt float64) {
	if dt <= 0 || len(states) == 0 {
		return
	}
	damage := lavaDamagePerSecond * dt
	if damage <= 0 {
		return
	}
	for _, state := range states {
		for _, obs := range h.obstacles {
			if obs.Type != obstacleTypeLava {
				continue
			}
			if circleRectOverlap(state.X, state.Y, playerHalf, obs) {
				state.applyHealthDelta(-damage)
				break
			}
		}
	}
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

			players, effects, toClose := h.advance(now, dt)
			for _, sub := range toClose {
				sub.conn.Close()
			}
			h.broadcastState(players, effects)
		}
	}
}

// DiagnosticsSnapshot exposes heartbeat data for the diagnostics endpoint.
func (h *Hub) DiagnosticsSnapshot() []diagnosticsPlayer {
	h.mu.Lock()
	defer h.mu.Unlock()

	players := make([]diagnosticsPlayer, 0, len(h.players))
	for _, state := range h.players {
		players = append(players, diagnosticsPlayer{
			ID:            state.ID,
			LastHeartbeat: state.lastHeartbeat.UnixMilli(),
			RTTMillis:     state.lastRTT.Milliseconds(),
		})
	}
	return players
}

// snapshotLocked copies players/effects for broadcasting while holding the mutex.
func (h *Hub) snapshotLocked(now time.Time) ([]Player, []Effect) {
	players := make([]Player, 0, len(h.players))
	for _, player := range h.players {
		if player.Facing == "" {
			player.Facing = defaultFacing
		}
		players = append(players, player.snapshot())
	}
	effects := make([]Effect, 0, len(h.effects))
	for _, eff := range h.effects {
		if now.Before(eff.expiresAt) {
			effects = append(effects, eff.Effect)
		}
	}
	return players, effects
}

// broadcastState sends the latest world snapshot to every subscriber.
func (h *Hub) broadcastState(players []Player, effects []Effect) {
	if players == nil || effects == nil {
		h.mu.Lock()
		now := time.Now()
		if players == nil || effects == nil {
			players, effects = h.snapshotLocked(now)
		}
		h.mu.Unlock()
	}

	msg := stateMessage{
		Type:       "state",
		Players:    players,
		Obstacles:  h.obstacles,
		Effects:    effects,
		ServerTime: time.Now().UnixMilli(),
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("failed to marshal state message: %v", err)
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
			log.Printf("failed to send update to %s: %v", id, err)
			players, effects := h.Disconnect(id)
			if players != nil {
				go h.broadcastState(players, effects)
			}
		}
	}
}
