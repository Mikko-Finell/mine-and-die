package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"sync"
	"sync/atomic"
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

type moveRequest struct {
	ID string  `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
}

type stateMessage struct {
	Type    string   `json:"type"`
	Players []Player `json:"players"`
}

type Hub struct {
	mu          sync.Mutex
	players     map[string]Player
	subscribers map[string]chan []byte
	nextID      atomic.Uint64
}

func newHub() *Hub {
	return &Hub{
		players:     make(map[string]Player),
		subscribers: make(map[string]chan []byte),
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
	subs := make(map[string]chan []byte, len(h.subscribers))
	for id, ch := range h.subscribers {
		subs[id] = ch
	}
	h.mu.Unlock()

	for id, ch := range subs {
		select {
		case ch <- data:
		default:
			h.mu.Lock()
			if current, ok := h.subscribers[id]; ok && current == ch {
				close(current)
				delete(h.subscribers, id)
			}
			h.mu.Unlock()
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

func (h *Hub) Subscribe(playerID string) (chan []byte, []Player, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.players[playerID]; !ok {
		return nil, nil, false
	}

	ch := make(chan []byte, 16)
	h.subscribers[playerID] = ch
	players := h.snapshotLocked()
	return ch, players, true
}

func (h *Hub) UpdatePosition(req moveRequest) []Player {
	h.mu.Lock()
	defer h.mu.Unlock()

	player, ok := h.players[req.ID]
	if !ok {
		return nil
	}

	player.X = req.X
	player.Y = req.Y
	h.players[req.ID] = player
	return h.snapshotLocked()
}

func (h *Hub) Disconnect(playerID string) []Player {
	h.mu.Lock()
	defer h.mu.Unlock()

	if ch, ok := h.subscribers[playerID]; ok {
		close(ch)
		delete(h.subscribers, playerID)
	}

	if _, ok := h.players[playerID]; !ok {
		return nil
	}

	delete(h.players, playerID)
	return h.snapshotLocked()
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

	http.HandleFunc("/move", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req moveRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}

		players := hub.UpdatePosition(req)
		if players == nil {
			http.Error(w, "unknown player", http.StatusBadRequest)
			return
		}

		go hub.broadcastState(players)
		w.WriteHeader(http.StatusNoContent)
	})

	http.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		playerID := r.URL.Query().Get("id")
		if playerID == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		ch, snapshot, ok := hub.Subscribe(playerID)
		if !ok {
			http.Error(w, "unknown player", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		writeEvent := func(data []byte) error {
			if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
				return err
			}
			flusher.Flush()
			return nil
		}

		initial := stateMessage{Type: "state", Players: snapshot}
		if data, err := json.Marshal(initial); err == nil {
			_ = writeEvent(data)
		}

		notify := r.Context().Done()

		for {
			select {
			case data, ok := <-ch:
				if !ok {
					return
				}
				if err := writeEvent(data); err != nil {
					players := hub.Disconnect(playerID)
					if players != nil {
						go hub.broadcastState(players)
					}
					return
				}
			case <-notify:
				players := hub.Disconnect(playerID)
				if players != nil {
					go hub.broadcastState(players)
				}
				return
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
