package net

import (
	"encoding/json"
	"io"
	"log"
	nethttp "net/http"
	"time"

	"github.com/gorilla/websocket"

	"mine-and-die/server"
	"mine-and-die/server/internal/sim"
)

type HTTPHandlerConfig struct {
	ClientDir string
	Logger    *log.Logger
}

type clientMessage struct {
	Ver              int     `json:"ver,omitempty"`
	Type             string  `json:"type"`
	DX               float64 `json:"dx"`
	DY               float64 `json:"dy"`
	Facing           string  `json:"facing"`
	X                float64 `json:"x"`
	Y                float64 `json:"y"`
	SentAt           int64   `json:"sentAt"`
	Action           string  `json:"action"`
	Cmd              string  `json:"cmd"`
	Qty              int     `json:"qty"`
	Ack              *uint64 `json:"ack"`
	KeyframeSeq      *uint64 `json:"keyframeSeq"`
	KeyframeInterval *int    `json:"keyframeInterval,omitempty"`
	CommandSeq       *uint64 `json:"seq,omitempty"`
}

type commandAckMessage struct {
	Ver  int    `json:"ver"`
	Type string `json:"type"`
	Seq  uint64 `json:"seq"`
	Tick uint64 `json:"tick,omitempty"`
}

type commandRejectMessage struct {
	Ver    int    `json:"ver"`
	Type   string `json:"type"`
	Seq    uint64 `json:"seq"`
	Reason string `json:"reason"`
	Retry  bool   `json:"retry,omitempty"`
	Tick   uint64 `json:"tick,omitempty"`
}

type heartbeatMessage struct {
	Ver        int    `json:"ver"`
	Type       string `json:"type"`
	ServerTime int64  `json:"serverTime"`
	ClientTime int64  `json:"clientTime"`
	RTTMillis  int64  `json:"rtt"`
}

func NewHTTPHandler(hub *server.Hub, cfg HTTPHandlerConfig) nethttp.Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}

	mux := nethttp.NewServeMux()

	mux.HandleFunc("/health", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("ok"))
	})

	mux.HandleFunc("/diagnostics", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		payload := struct {
			Status     string `json:"status"`
			ServerTime int64  `json:"serverTime"`
			Players    any    `json:"players"`
			TickRate   int    `json:"tickRate"`
			Heartbeat  int64  `json:"heartbeatMillis"`
			Telemetry  any    `json:"telemetry"`
		}{
			Status:     "ok",
			ServerTime: time.Now().UnixMilli(),
			Players:    hub.DiagnosticsSnapshot(),
			TickRate:   server.TickRate(),
			Heartbeat:  server.HeartbeatInterval().Milliseconds(),
			Telemetry:  hub.TelemetrySnapshot(),
		}

		data, err := json.Marshal(payload)
		if err != nil {
			httpError(w, "failed to encode", nethttp.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	mux.HandleFunc("/world/reset", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		if r.Method != nethttp.MethodPost {
			httpError(w, "method not allowed", nethttp.StatusMethodNotAllowed)
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
				httpError(w, "invalid payload", nethttp.StatusBadRequest)
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

		cfg = cfg.Normalized()

		players, npcs := hub.ResetWorld(cfg)
		hub.ForceKeyframe()
		go hub.BroadcastState(players, npcs, nil, nil)

		response := struct {
			Status string `json:"status"`
			Config any    `json:"config"`
		}{
			Status: "ok",
			Config: cfg,
		}

		data, err := json.Marshal(response)
		if err != nil {
			httpError(w, "failed to encode", nethttp.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	mux.HandleFunc("/join", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		if r.Method != nethttp.MethodPost {
			httpError(w, "method not allowed", nethttp.StatusMethodNotAllowed)
			return
		}

		join := hub.Join()
		data, err := json.Marshal(join)
		if err != nil {
			httpError(w, "failed to encode", nethttp.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	mux.HandleFunc("/effects/catalog", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		if r.Method != nethttp.MethodGet {
			httpError(w, "method not allowed", nethttp.StatusMethodNotAllowed)
			return
		}

		catalog := hub.EffectCatalogSnapshot()
		var payloadCatalog any = catalog
		if payloadCatalog == nil {
			payloadCatalog = map[string]any{}
		}

		payload := struct {
			Catalog any `json:"effectCatalog"`
		}{Catalog: payloadCatalog}

		data, err := json.Marshal(payload)
		if err != nil {
			httpError(w, "failed to encode", nethttp.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	})

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *nethttp.Request) bool {
			return true
		},
	}

	mux.HandleFunc("/ws", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		playerID := r.URL.Query().Get("id")
		if playerID == "" {
			httpError(w, "missing id", nethttp.StatusBadRequest)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Printf("upgrade failed for %s: %v", playerID, err)
			return
		}

		sub, snapshotPlayers, snapshotNPCs, snapshotGroundItems, ok := hub.Subscribe(playerID, conn)
		if !ok {
			message := websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "unknown player")
			conn.WriteMessage(websocket.CloseMessage, message)
			conn.Close()
			return
		}

		data, entities, err := hub.MarshalState(snapshotPlayers, snapshotNPCs, nil, snapshotGroundItems, false, true)
		if err != nil {
			logger.Printf("failed to marshal initial state for %s: %v", playerID, err)
			players, npcs := hub.Disconnect(playerID)
			if players != nil {
				hub.ForceKeyframe()
				go hub.BroadcastState(players, npcs, nil, nil)
			}
			return
		}

		if err := sub.WriteMessage(websocket.TextMessage, data); err != nil {
			players, npcs := hub.Disconnect(playerID)
			if players != nil {
				hub.ForceKeyframe()
				go hub.BroadcastState(players, npcs, nil, nil)
			}
			return
		}
		hub.RecordTelemetryBroadcast(len(data), entities)

		for {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				players, npcs := hub.Disconnect(playerID)
				if players != nil {
					hub.ForceKeyframe()
					go hub.BroadcastState(players, npcs, nil, nil)
				}
				return
			}

			var msg clientMessage
			if err := json.Unmarshal(payload, &msg); err != nil {
				logger.Printf("discarding malformed message from %s: %v", playerID, err)
				continue
			}

			if msg.Ack != nil {
				hub.RecordAck(playerID, *msg.Ack)
			}

			normalizedSeq := uint64(0)
			if msg.CommandSeq != nil && *msg.CommandSeq > 0 {
				normalizedSeq = *msg.CommandSeq
			}

			writeJSON := func(payload any) bool {
				data, err := json.Marshal(payload)
				if err != nil {
					logger.Printf("failed to marshal response for %s: %v", playerID, err)
					return true
				}
				if err := sub.WriteMessage(websocket.TextMessage, data); err != nil {
					players, npcs := hub.Disconnect(playerID)
					if players != nil {
						hub.ForceKeyframe()
						go hub.BroadcastState(players, npcs, nil, nil)
					}
					return false
				}
				return true
			}

			sendDuplicateAck := func() bool {
				if normalizedSeq == 0 {
					return true
				}
				ack := commandAckMessage{Ver: server.ProtocolVersion, Type: "commandAck", Seq: normalizedSeq}
				return writeJSON(ack)
			}

			sendCommandAck := func(cmd sim.Command) bool {
				if normalizedSeq == 0 {
					return true
				}
				ack := commandAckMessage{Ver: server.ProtocolVersion, Type: "commandAck", Seq: normalizedSeq}
				if cmd.OriginTick > 0 {
					ack.Tick = cmd.OriginTick
				}
				if !writeJSON(ack) {
					return false
				}
				sub.StoreLastCommandSeq(normalizedSeq)
				return true
			}

			sendCommandReject := func(reason string, retry bool) bool {
				if normalizedSeq == 0 {
					return true
				}
				reject := commandRejectMessage{
					Ver:    server.ProtocolVersion,
					Type:   "commandReject",
					Seq:    normalizedSeq,
					Reason: reason,
				}
				if retry {
					reject.Retry = true
				}
				return writeJSON(reject)
			}

			switch msg.Type {
			case "input":
				if normalizedSeq > 0 {
					if last := sub.LastCommandSeq(); last > 0 && normalizedSeq <= last {
						if !sendDuplicateAck() {
							return
						}
						continue
					}
				}
				cmd, ok, reason := hub.UpdateIntent(playerID, msg.DX, msg.DY, msg.Facing)
				if normalizedSeq > 0 {
					if ok {
						if !sendCommandAck(cmd) {
							return
						}
					} else {
						retry := reason == server.CommandRejectQueueLimit
						if !sendCommandReject(reason, retry) {
							return
						}
					}
				}
				if !ok {
					if reason == server.CommandRejectUnknownActor {
						logger.Printf("input ignored for unknown player %s", playerID)
					}
				}
			case "path":
				if normalizedSeq > 0 {
					if last := sub.LastCommandSeq(); last > 0 && normalizedSeq <= last {
						if !sendDuplicateAck() {
							return
						}
						continue
					}
				}
				cmd, ok, reason := hub.SetPlayerPath(playerID, msg.X, msg.Y)
				if normalizedSeq > 0 {
					if ok {
						if !sendCommandAck(cmd) {
							return
						}
					} else {
						retry := reason == server.CommandRejectQueueLimit
						if !sendCommandReject(reason, retry) {
							return
						}
					}
				}
				if !ok && reason == server.CommandRejectUnknownActor {
					logger.Printf("path request ignored for unknown player %s", playerID)
				}
			case "cancelPath":
				if normalizedSeq > 0 {
					if last := sub.LastCommandSeq(); last > 0 && normalizedSeq <= last {
						if !sendDuplicateAck() {
							return
						}
						continue
					}
				}
				cmd, ok, reason := hub.ClearPlayerPath(playerID)
				if normalizedSeq > 0 {
					if ok {
						if !sendCommandAck(cmd) {
							return
						}
					} else {
						retry := reason == server.CommandRejectQueueLimit
						if !sendCommandReject(reason, retry) {
							return
						}
					}
				}
				if !ok && reason == server.CommandRejectUnknownActor {
					logger.Printf("cancelPath ignored for unknown player %s", playerID)
				}
			case "action":
				if msg.Action == "" {
					continue
				}
				if normalizedSeq > 0 {
					if last := sub.LastCommandSeq(); last > 0 && normalizedSeq <= last {
						if !sendDuplicateAck() {
							return
						}
						continue
					}
				}
				cmd, ok, reason := hub.HandleAction(playerID, msg.Action)
				if normalizedSeq > 0 {
					if ok {
						if !sendCommandAck(cmd) {
							return
						}
					} else {
						retry := reason == server.CommandRejectQueueLimit
						if !sendCommandReject(reason, retry) {
							return
						}
					}
				}
				if !ok {
					if reason == server.CommandRejectInvalidAction {
						logger.Printf("unknown action %q from %s", msg.Action, playerID)
					} else if reason == server.CommandRejectUnknownActor {
						logger.Printf("action ignored for unknown player %s", playerID)
					}
				}
			case "heartbeat":
				now := time.Now()
				rtt, ok := hub.UpdateHeartbeat(playerID, now, msg.SentAt)
				if !ok {
					continue
				}

				ack := heartbeatMessage{
					Ver:        server.ProtocolVersion,
					Type:       "heartbeat",
					ServerTime: now.UnixMilli(),
					ClientTime: msg.SentAt,
					RTTMillis:  rtt.Milliseconds(),
				}

				data, err := json.Marshal(ack)
				if err != nil {
					logger.Printf("failed to marshal heartbeat ack for %s: %v", playerID, err)
					continue
				}

				if err := sub.WriteMessage(websocket.TextMessage, data); err != nil {
					players, npcs := hub.Disconnect(playerID)
					if players != nil {
						hub.ForceKeyframe()
						go hub.BroadcastState(players, npcs, nil, nil)
					}
					return
				}
			case "console":
				ack, handled := hub.HandleConsoleCommand(playerID, msg.Cmd, msg.Qty)
				if !handled {
					continue
				}
				data, err := json.Marshal(ack)
				if err != nil {
					logger.Printf("failed to marshal console ack for %s: %v", playerID, err)
					continue
				}
				if err := sub.WriteMessage(websocket.TextMessage, data); err != nil {
					players, npcs := hub.Disconnect(playerID)
					if players != nil {
						hub.ForceKeyframe()
						go hub.BroadcastState(players, npcs, nil, nil)
					}
					return
				}
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
					logger.Printf("failed to marshal keyframe for %s: %v", playerID, err)
					continue
				}
				if err := sub.WriteMessage(websocket.TextMessage, data); err != nil {
					players, npcs := hub.Disconnect(playerID)
					if players != nil {
						hub.ForceKeyframe()
						go hub.BroadcastState(players, npcs, nil, nil)
					}
					return
				}
			case "keyframeCadence":
				requested := 0
				if msg.KeyframeInterval != nil {
					requested = *msg.KeyframeInterval
				}
				applied := hub.SetKeyframeInterval(requested)
				logger.Printf("[keyframe] player=%s requested cadence=%d", playerID, applied)
			default:
				logger.Printf("unknown message type %q from %s", msg.Type, playerID)
			}
		}
	})

	if cfg.ClientDir != "" {
		fs := nethttp.FileServer(nethttp.Dir(cfg.ClientDir))
		mux.Handle("/", fs)
	}

	return mux
}

func httpError(w nethttp.ResponseWriter, msg string, code int) {
	nethttp.Error(w, msg, code)
}
