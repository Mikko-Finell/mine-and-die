package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gorilla/websocket"
)

// main wires up HTTP handlers, starts the simulation, and serves the client.
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

	http.HandleFunc("/world/reset", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		cfg := hub.CurrentConfig()

		type resetRequest struct {
			Obstacles      *bool   `json:"obstacles"`
			ObstaclesCount *int    `json:"obstaclesCount"`
			GoldMines      *bool   `json:"goldMines"`
			GoldMineCount  *int    `json:"goldMineCount"`
			NPCs           *bool   `json:"npcs"`
			NPCCount       *int    `json:"npcCount"`
			Lava           *bool   `json:"lava"`
			LavaCount      *int    `json:"lavaCount"`
			Seed           *string `json:"seed"`
		}

		if r.Body != nil {
			defer r.Body.Close()
			var req resetRequest
			decoder := json.NewDecoder(r.Body)
			if err := decoder.Decode(&req); err != nil && err != io.EOF {
				http.Error(w, "invalid payload", http.StatusBadRequest)
				return
			}
			if req.Obstacles != nil {
				cfg.Obstacles = *req.Obstacles
			}
			if req.ObstaclesCount != nil {
				cfg.ObstaclesCount = *req.ObstaclesCount
			}
			if req.GoldMines != nil {
				cfg.GoldMines = *req.GoldMines
			}
			if req.GoldMineCount != nil {
				cfg.GoldMineCount = *req.GoldMineCount
			}
			if req.NPCs != nil {
				cfg.NPCs = *req.NPCs
			}
			if req.NPCCount != nil {
				cfg.NPCCount = *req.NPCCount
			}
			if req.Lava != nil {
				cfg.Lava = *req.Lava
			}
			if req.LavaCount != nil {
				cfg.LavaCount = *req.LavaCount
			}
			if req.Seed != nil {
				cfg.Seed = *req.Seed
			}
		}

		cfg = cfg.normalized()

		players, npcs, effects := hub.ResetWorld(cfg)
		go hub.broadcastState(players, npcs, effects)

		response := struct {
			Status string      `json:"status"`
			Config worldConfig `json:"config"`
		}{
			Status: "ok",
			Config: cfg,
		}

		data, err := json.Marshal(response)
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

		sub, snapshotPlayers, snapshotNPCs, snapshotEffects, ok := hub.Subscribe(playerID, conn)
		if !ok {
			message := websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "unknown player")
			conn.WriteMessage(websocket.CloseMessage, message)
			conn.Close()
			return
		}

		cfg := hub.CurrentConfig()

		initial := stateMessage{
			Type:        "state",
			Players:     snapshotPlayers,
			NPCs:        snapshotNPCs,
			Obstacles:   append([]Obstacle(nil), hub.world.obstacles...),
			Effects:     snapshotEffects,
			ServerTime:  time.Now().UnixMilli(),
			Config:      cfg,
			WorldWidth:  worldWidth,
			WorldHeight: worldHeight,
		}
		data, err := json.Marshal(initial)
		if err != nil {
			log.Printf("failed to marshal initial state for %s: %v", playerID, err)
			players, npcs, effects := hub.Disconnect(playerID)
			if players != nil {
				go hub.broadcastState(players, npcs, effects)
			}
			return
		}

		sub.mu.Lock()
		conn.SetWriteDeadline(time.Now().Add(writeWait))
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			sub.mu.Unlock()
			players, npcs, effects := hub.Disconnect(playerID)
			if players != nil {
				go hub.broadcastState(players, npcs, effects)
			}
			return
		}
		sub.mu.Unlock()

		for {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				players, npcs, effects := hub.Disconnect(playerID)
				if players != nil {
					go hub.broadcastState(players, npcs, effects)
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
				if !hub.UpdateIntent(playerID, msg.DX, msg.DY, msg.Facing) {
					log.Printf("input ignored for unknown player %s", playerID)
				}
			case "path":
				if !hub.SetPlayerPath(playerID, msg.X, msg.Y) {
					log.Printf("path request ignored for unknown player %s", playerID)
				}
			case "cancelPath":
				if !hub.ClearPlayerPath(playerID) {
					log.Printf("cancelPath ignored for unknown player %s", playerID)
				}
			case "action":
				if msg.Action == "" {
					continue
				}
				if !hub.HandleAction(playerID, msg.Action) {
					log.Printf("unknown action %q from %s", msg.Action, playerID)
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
					players, npcs, effects := hub.Disconnect(playerID)
					if players != nil {
						go hub.broadcastState(players, npcs, effects)
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
