package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const writeWait = 10 * time.Second

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
	Type    string   `json:"type"`
	Players []Player `json:"players"`
}

type clientMessage struct {
	Type string  `json:"type"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
}

type subscriber struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

type Hub struct {
	mu          sync.Mutex
	players     map[string]Player
	subscribers map[string]*subscriber
	nextID      atomic.Uint64
}

func newHub() *Hub {
	return &Hub{
		players:     make(map[string]Player),
		subscribers: make(map[string]*subscriber),
	}
}

func (h *Hub) snapshotLocked() []Player {
	players := make([]Player, 0, len(h.players))
	for _, player := range h.players {
		players = append(players, player)
	}
	return players
}

func (h *Hub) broadcastState(players []Player) {
	if players == nil {
		h.mu.Lock()
		players = h.snapshotLocked()
		h.mu.Unlock()
	}

	data, err := json.Marshal(stateMessage{Type: "state", Players: players})
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
	player := Player{ID: playerID, X: 80, Y: 80}

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

	if _, ok := h.players[playerID]; !ok {
		return nil, nil, false
	}

	if existing, ok := h.subscribers[playerID]; ok {
		existing.conn.Close()
	}

	sub := &subscriber{conn: conn}
	h.subscribers[playerID] = sub
	players := h.snapshotLocked()
	return sub, players, true
}

func (h *Hub) UpdatePosition(playerID string, x, y float64) []Player {
	h.mu.Lock()
	defer h.mu.Unlock()

	player, ok := h.players[playerID]
	if !ok {
		return nil
	}

	player.X = x
	player.Y = y
	h.players[playerID] = player
	return h.snapshotLocked()
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

func main() {
	hub := newHub()

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("ok"))
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

		initial := stateMessage{Type: "state", Players: snapshot}
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
			case "move":
				players := hub.UpdatePosition(playerID, msg.X, msg.Y)
				if players != nil {
					go hub.broadcastState(players)
				}
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
