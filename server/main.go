package main

import (
	"context"
	"encoding/json"
	"io"
	stdlog "log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gorilla/websocket"

	"mine-and-die/server/logging"
	loggingSinks "mine-and-die/server/logging/sinks"
)

// main wires up HTTP handlers, starts the simulation, and serves the client.
func main() {
	logConfig := logging.DefaultConfig()
	sinks := map[string]logging.Sink{
		"console": loggingSinks.NewConsole(os.Stdout),
	}
	router, err := logging.NewRouter(logConfig, logging.SystemClock{}, stdlog.Default(), sinks)
	if err != nil {
		stdlog.Fatalf("failed to construct logging router: %v", err)
	}
	defer func() {
		if cerr := router.Close(context.Background()); cerr != nil {
			stdlog.Printf("failed to close logging router: %v", cerr)
		}
	}()

	hubCfg := defaultHubConfig()
	if raw := os.Getenv("KEYFRAME_INTERVAL_TICKS"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			hubCfg.KeyframeInterval = value
		} else {
			stdlog.Printf("invalid KEYFRAME_INTERVAL_TICKS=%q: %v", raw, err)
		}
	}

	hub := newHubWithConfig(hubCfg, router)
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
			Telemetry  telemetrySnapshot   `json:"telemetry"`
		}{
			Status:     "ok",
			ServerTime: time.Now().UnixMilli(),
			Players:    hub.DiagnosticsSnapshot(),
			TickRate:   tickRate,
			Heartbeat:  heartbeatInterval.Milliseconds(),
			Telemetry:  hub.TelemetrySnapshot(),
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
			GoblinCount    *int    `json:"goblinCount"`
			RatCount       *int    `json:"ratCount"`
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
			if req.GoblinCount != nil {
				cfg.GoblinCount = *req.GoblinCount
			}
			if req.RatCount != nil {
				cfg.RatCount = *req.RatCount
			}
			if req.NPCCount != nil {
				cfg.NPCCount = *req.NPCCount
				if req.GoblinCount == nil && req.RatCount == nil {
					goblins := cfg.NPCCount
					if goblins > 2 {
						goblins = 2
					}
					if goblins < 0 {
						goblins = 0
					}
					cfg.GoblinCount = goblins
					rats := cfg.NPCCount - goblins
					if rats < 0 {
						rats = 0
					}
					cfg.RatCount = rats
				}
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

		players, npcs := hub.ResetWorld(cfg)
		hub.forceKeyframe()
		go hub.broadcastState(players, npcs, nil, nil)

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
			stdlog.Printf("upgrade failed for %s: %v", playerID, err)
			return
		}

		sub, snapshotPlayers, snapshotNPCs, snapshotGroundItems, ok := hub.Subscribe(playerID, conn)
		if !ok {
			message := websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "unknown player")
			conn.WriteMessage(websocket.CloseMessage, message)
			conn.Close()
			return
		}

		data, entities, err := hub.marshalState(snapshotPlayers, snapshotNPCs, nil, snapshotGroundItems, false, true)
		if err != nil {
			stdlog.Printf("failed to marshal initial state for %s: %v", playerID, err)
			players, npcs := hub.Disconnect(playerID)
			if players != nil {
				hub.forceKeyframe()
				go hub.broadcastState(players, npcs, nil, nil)
			}
			return
		}

		sub.mu.Lock()
		conn.SetWriteDeadline(time.Now().Add(writeWait))
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			sub.mu.Unlock()
			players, npcs := hub.Disconnect(playerID)
			if players != nil {
				hub.forceKeyframe()
				go hub.broadcastState(players, npcs, nil, nil)
			}
			return
		}
		sub.mu.Unlock()
		if hub.telemetry != nil {
			hub.telemetry.RecordBroadcast(len(data), entities)
		}

		for {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				players, npcs := hub.Disconnect(playerID)
				if players != nil {
					hub.forceKeyframe()
					go hub.broadcastState(players, npcs, nil, nil)
				}
				return
			}

			var msg clientMessage
			if err := json.Unmarshal(payload, &msg); err != nil {
				stdlog.Printf("discarding malformed message from %s: %v", playerID, err)
				continue
			}

			if msg.Ack != nil {
				hub.RecordAck(playerID, *msg.Ack)
			}

			switch msg.Type {
			case "input":
				if !hub.UpdateIntent(playerID, msg.DX, msg.DY, msg.Facing) {
					stdlog.Printf("input ignored for unknown player %s", playerID)
				}
			case "path":
				if !hub.SetPlayerPath(playerID, msg.X, msg.Y) {
					stdlog.Printf("path request ignored for unknown player %s", playerID)
				}
			case "cancelPath":
				if !hub.ClearPlayerPath(playerID) {
					stdlog.Printf("cancelPath ignored for unknown player %s", playerID)
				}
			case "action":
				if msg.Action == "" {
					continue
				}
				if !hub.HandleAction(playerID, msg.Action) {
					stdlog.Printf("unknown action %q from %s", msg.Action, playerID)
				}
			case "heartbeat":
				now := time.Now()
				rtt, ok := hub.UpdateHeartbeat(playerID, now, msg.SentAt)
				if !ok {
					continue
				}

				ack := heartbeatMessage{
					Ver:        ProtocolVersion,
					Type:       "heartbeat",
					ServerTime: now.UnixMilli(),
					ClientTime: msg.SentAt,
					RTTMillis:  rtt.Milliseconds(),
				}

				data, err := json.Marshal(ack)
				if err != nil {
					stdlog.Printf("failed to marshal heartbeat ack for %s: %v", playerID, err)
					continue
				}

				sub.mu.Lock()
				conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					sub.mu.Unlock()
					players, npcs := hub.Disconnect(playerID)
					if players != nil {
						hub.forceKeyframe()
						go hub.broadcastState(players, npcs, nil, nil)
					}
					return
				}
				sub.mu.Unlock()
			case "console":
				ack, handled := hub.HandleConsoleCommand(playerID, msg.Cmd, msg.Qty)
				if !handled {
					continue
				}
				data, err := json.Marshal(ack)
				if err != nil {
					stdlog.Printf("failed to marshal console ack for %s: %v", playerID, err)
					continue
				}
				sub.mu.Lock()
				conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					sub.mu.Unlock()
					players, npcs := hub.Disconnect(playerID)
					if players != nil {
						hub.forceKeyframe()
						go hub.broadcastState(players, npcs, nil, nil)
					}
					return
				}
				sub.mu.Unlock()
			case "keyframeRequest":
				if msg.KeyframeSeq == nil {
					continue
				}
				snapshot, nack, ok := hub.HandleKeyframeRequest(playerID, sub, *msg.KeyframeSeq)
				if !ok {
					continue
				}
				var data []byte
				var err error
				if nack != nil {
					data, err = json.Marshal(nack)
				} else {
					data, err = json.Marshal(snapshot)
				}
				if err != nil {
					stdlog.Printf("failed to marshal keyframe for %s: %v", playerID, err)
					continue
				}
				sub.mu.Lock()
				conn.SetWriteDeadline(time.Now().Add(writeWait))
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					sub.mu.Unlock()
					players, npcs := hub.Disconnect(playerID)
					if players != nil {
						hub.forceKeyframe()
						go hub.broadcastState(players, npcs, nil, nil)
					}
					return
				}
				sub.mu.Unlock()
			case "keyframeCadence":
				requested := 0
				if msg.KeyframeInterval != nil {
					requested = *msg.KeyframeInterval
				}
				applied := hub.SetKeyframeInterval(requested)
				stdlog.Printf("[keyframe] player=%s requested cadence=%d", playerID, applied)
			default:
				stdlog.Printf("unknown message type %q from %s", msg.Type, playerID)
			}
		}
	})

	clientDir := filepath.Clean(filepath.Join("..", "client"))
	fs := http.FileServer(http.Dir(clientDir))
	http.Handle("/", fs)

	addr := ":8080"
	stdlog.Printf("server listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		stdlog.Fatalf("server failed: %v", err)
	}
}
