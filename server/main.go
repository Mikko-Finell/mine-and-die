package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait         = 10 * time.Second
	tickRate          = 15    // ticks per second (10–20 Hz)
	moveSpeed         = 160.0 // pixels per second
	worldWidth        = 800.0
	worldHeight       = 600.0
	playerHalf        = 14.0
	heartbeatInterval = 2 * time.Second
	disconnectAfter   = 3 * heartbeatInterval
)

type Player struct {
	ID string  `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
}

type joinResponse struct {
	ID      string   `json:"id"`
	Players []Player `json:"players"`
}

type stateMessage struct {
	Type       string   `json:"type"`
	Players    []Player `json:"players"`
	ServerTime int64    `json:"serverTime"`
}

type clientMessage struct {
	Type   string  `json:"type"`
	DX     float64 `json:"dx"`
	DY     float64 `json:"dy"`
	SentAt int64   `json:"sentAt"`
}

type heartbeatMessage struct {
	Type       string `json:"type"`
	ServerTime int64  `json:"serverTime"`
	ClientTime int64  `json:"clientTime"`
	RTTMillis  int64  `json:"rtt"`
}

type subscriber struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

type playerState struct {
	Player
	intentX       float64
	intentY       float64
	lastInput     time.Time
	lastHeartbeat time.Time
	lastRTT       time.Duration
}

type diagnosticsPlayer struct {
	ID            string `json:"id"`
	LastHeartbeat int64  `json:"lastHeartbeat"`
	RTTMillis     int64  `json:"rttMillis"`
}

type Hub struct {
	mu          sync.Mutex
	players     map[string]*playerState
	subscribers map[string]*subscriber
	nextID      atomic.Uint64
}

func newHub() *Hub {
	return &Hub{
		players:     make(map[string]*playerState),
		subscribers: make(map[string]*subscriber),
	}
}

func (h *Hub) snapshotLocked() []Player {
	players := make([]Player, 0, len(h.players))
	for _, player := range h.players {
		players = append(players, player.Player)
	}
	return players
}

func (h *Hub) broadcastState(players []Player) {
	if players == nil {
		h.mu.Lock()
		players = h.snapshotLocked()
		h.mu.Unlock()
	}

	msg := stateMessage{Type: "state", Players: players, ServerTime: time.Now().UnixMilli()}
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
			players := h.Disconnect(id)
			if players != nil {
				go h.broadcastState(players)
			}
		}
	}
}

func (h *Hub) Join() joinResponse {
	id := h.nextID.Add(1)
	playerID := fmt.Sprintf("player-%d", id)
	now := time.Now()
	player := &playerState{Player: Player{ID: playerID, X: 80, Y: 80}, lastHeartbeat: now}

	h.mu.Lock()
	h.players[playerID] = player
	players := h.snapshotLocked()
	h.mu.Unlock()

	go h.broadcastState(players)

	return joinResponse{ID: playerID, Players: players}
}

func (h *Hub) Subscribe(playerID string, conn *websocket.Conn) (*subscriber, []Player, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	state, ok := h.players[playerID]
	if !ok {
		return nil, nil, false
	}

	state.lastHeartbeat = time.Now()

	if existing, ok := h.subscribers[playerID]; ok {
		existing.conn.Close()
	}

	sub := &subscriber{conn: conn}
	h.subscribers[playerID] = sub
	players := h.snapshotLocked()
	return sub, players, true
}

func (h *Hub) Disconnect(playerID string) []Player {
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
	if playerOK {
		players = h.snapshotLocked()
	}
	h.mu.Unlock()

	if subOK {
		sub.conn.Close()
	}

	if !playerOK {
		return nil
	}

	return players
}

func (h *Hub) UpdateIntent(playerID string, dx, dy float64) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	state, ok := h.players[playerID]
	if !ok {
		return false
	}

	length := math.Hypot(dx, dy)
	if length > 1 {
		dx /= length
		dy /= length
	}

	state.intentX = dx
	state.intentY = dy
	state.lastInput = time.Now()
	return true
}

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

func (h *Hub) advance(now time.Time, dt float64) ([]Player, []*subscriber) {
	h.mu.Lock()

	toClose := make([]*subscriber, 0)
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

		if state.intentX != 0 || state.intentY != 0 {
			dx, dy := state.intentX, state.intentY
			length := math.Hypot(dx, dy)
			if length != 0 {
				dx /= length
				dy /= length
			}

			state.X += dx * moveSpeed * dt
			state.Y += dy * moveSpeed * dt

			state.X = math.Max(playerHalf, math.Min(worldWidth-playerHalf, state.X))
			state.Y = math.Max(playerHalf, math.Min(worldHeight-playerHalf, state.Y))
		}
	}

	players := h.snapshotLocked()
	h.mu.Unlock()

	return players, toClose
}

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

			players, toClose := h.advance(now, dt)
			for _, sub := range toClose {
				sub.conn.Close()
			}
			h.broadcastState(players)
		}
	}
}

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

func main() {
	hub := newHub()
	stop := make(chan struct{})
	go hub.RunSimulation(stop)
	defer close(stop)

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("ok"))
	})

	http.HandleFunc("/diagnostics", func(w http.ResponseWriter, r *http.Request) {
		payload := struct {
			Status     string              `json:"status"`
			ServerTime int64               `json:"serverTime"`
			Players    []diagnosticsPlayer `json:"players"`
			TickRate   int                 `json:"tickRate"`
			Heartbeat  int64               `json:"heartbeatMillis"`
		}{
			Status:     "ok",
			ServerTime: time.Now().UnixMilli(),
			Players:    hub.DiagnosticsSnapshot(),
			TickRate:   tickRate,
			Heartbeat:  heartbeatInterval.Milliseconds(),
		}

		data, err := json.Marshal(payload)
		if err != nil {
			http.Error(w, "failed to encode", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	http.HandleFunc("/join", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		join := hub.Join()
		data, err := json.Marshal(join)
		if err != nil {
			http.Error(w, "failed to encode", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		playerID := r.URL.Query().Get("id")
		if playerID == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("upgrade failed for %s: %v", playerID, err)
			return
		}

		sub, snapshot, ok := hub.Subscribe(playerID, conn)
		if !ok {
			message := websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "unknown player")
			conn.WriteMessage(websocket.CloseMessage, message)
			conn.Close()
			return
		}

		initial := stateMessage{Type: "state", Players: snapshot, ServerTime: time.Now().UnixMilli()}
		data, err := json.Marshal(initial)
		if err != nil {
			log.Printf("failed to marshal initial state for %s: %v", playerID, err)
			players := hub.Disconnect(playerID)
			if players != nil {
				go hub.broadcastState(players)
			}
			return
		}

		sub.mu.Lock()
		conn.SetWriteDeadline(time.Now().Add(writeWait))
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			sub.mu.Unlock()
			players := hub.Disconnect(playerID)
			if players != nil {
				go hub.broadcastState(players)
			}
			return
		}
		sub.mu.Unlock()

		for {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				players := hub.Disconnect(playerID)
				if players != nil {
					go hub.broadcastState(players)
				}
				return
			}

			var msg clientMessage
			if err := json.Unmarshal(payload, &msg); err != nil {
				log.Printf("discarding malformed message from %s: %v", playerID, err)
				continue
			}

			switch msg.Type {
			case "input":
				if !hub.UpdateIntent(playerID, msg.DX, msg.DY) {
					log.Printf("input ignored for unknown player %s", playerID)
				}
			case "heartbeat":
				now := time.Now()
				rtt, ok := hub.UpdateHeartbeat(playerID, now, msg.SentAt)
				if !ok {
					continue
				}

				ack := heartbeatMessage{
					Type:       "heartbeat",
					ServerTime: now.UnixMilli(),
					ClientTime: msg.SentAt,
					RTTMillis:  rtt.Milliseconds(),
				}

				data, err := json.Marshal(ack)
				if err != nil {
					log.Printf("failed to marshal heartbeat ack for %s: %v", playerID, err)
					continue
				}

				sub.mu.Lock()
				conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					sub.mu.Unlock()
					players := hub.Disconnect(playerID)
					if players != nil {
						go hub.broadcastState(players)
					}
					return
				}
				sub.mu.Unlock()
			default:
				log.Printf("unknown message type %q from %s", msg.Type, playerID)
			}
		}
	})

	clientDir := filepath.Clean(filepath.Join("..", "client"))
	fs := http.FileServer(http.Dir(clientDir))
	http.Handle("/", fs)

	addr := ":8080"
	log.Printf("server listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
