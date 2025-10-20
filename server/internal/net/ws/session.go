package ws

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"

	"mine-and-die/server"
	"mine-and-die/server/internal/sim"
)

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

// Handler coordinates a websocket session for a player.
type Handler struct {
	hub    *server.Hub
	logger *log.Logger
}

// NewHandler constructs a websocket session handler for the given hub.
func NewHandler(hub *server.Hub, logger *log.Logger) *Handler {
	if logger == nil {
		logger = log.Default()
	}
	return &Handler{hub: hub, logger: logger}
}

// Serve orchestrates a websocket session for the provided player connection.
func (h *Handler) Serve(playerID string, conn *websocket.Conn) {
	if h == nil || h.hub == nil || conn == nil {
		return
	}

	sub, snapshotPlayers, snapshotNPCs, snapshotGroundItems, ok := h.hub.Subscribe(playerID, conn)
	if !ok {
		message := websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "unknown player")
		conn.WriteMessage(websocket.CloseMessage, message)
		conn.Close()
		return
	}

	data, entities, err := h.hub.MarshalState(snapshotPlayers, snapshotNPCs, nil, snapshotGroundItems, false, true)
	if err != nil {
		h.logger.Printf("failed to marshal initial state for %s: %v", playerID, err)
		h.disconnect(playerID)
		return
	}

	if err := sub.WriteMessage(websocket.TextMessage, data); err != nil {
		h.disconnect(playerID)
		return
	}
	h.hub.RecordTelemetryBroadcast(len(data), entities)

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			h.disconnect(playerID)
			return
		}

		var msg clientMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			h.logger.Printf("discarding malformed message from %s: %v", playerID, err)
			continue
		}

		if msg.Ack != nil {
			h.hub.RecordAck(playerID, *msg.Ack)
		}

		normalizedSeq := uint64(0)
		if msg.CommandSeq != nil && *msg.CommandSeq > 0 {
			normalizedSeq = *msg.CommandSeq
		}

		writeJSON := func(payload any) bool {
			data, err := json.Marshal(payload)
			if err != nil {
				h.logger.Printf("failed to marshal response for %s: %v", playerID, err)
				return true
			}
			if err := sub.WriteMessage(websocket.TextMessage, data); err != nil {
				h.disconnect(playerID)
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
			cmd, ok, reason := h.hub.UpdateIntent(playerID, msg.DX, msg.DY, msg.Facing)
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
					h.logger.Printf("input ignored for unknown player %s", playerID)
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
			cmd, ok, reason := h.hub.SetPlayerPath(playerID, msg.X, msg.Y)
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
				h.logger.Printf("path request ignored for unknown player %s", playerID)
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
			cmd, ok, reason := h.hub.ClearPlayerPath(playerID)
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
				h.logger.Printf("cancelPath ignored for unknown player %s", playerID)
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
			cmd, ok, reason := h.hub.HandleAction(playerID, msg.Action)
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
					h.logger.Printf("unknown action %q from %s", msg.Action, playerID)
				} else if reason == server.CommandRejectUnknownActor {
					h.logger.Printf("action ignored for unknown player %s", playerID)
				}
			}
		case "heartbeat":
			now := time.Now()
			rtt, ok := h.hub.UpdateHeartbeat(playerID, now, msg.SentAt)
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
				h.logger.Printf("failed to marshal heartbeat ack for %s: %v", playerID, err)
				continue
			}

			if err := sub.WriteMessage(websocket.TextMessage, data); err != nil {
				h.disconnect(playerID)
				return
			}
		case "console":
			ack, handled := h.hub.HandleConsoleCommand(playerID, msg.Cmd, msg.Qty)
			if !handled {
				continue
			}
			data, err := json.Marshal(ack)
			if err != nil {
				h.logger.Printf("failed to marshal console ack for %s: %v", playerID, err)
				continue
			}
			if err := sub.WriteMessage(websocket.TextMessage, data); err != nil {
				h.disconnect(playerID)
				return
			}
		case "keyframeRequest":
			if msg.KeyframeSeq == nil {
				continue
			}
			snapshot, nack, ok := h.hub.HandleKeyframeRequest(playerID, sub, *msg.KeyframeSeq)
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
				h.logger.Printf("failed to marshal keyframe for %s: %v", playerID, err)
				continue
			}
			if err := sub.WriteMessage(websocket.TextMessage, data); err != nil {
				h.disconnect(playerID)
				return
			}
		case "keyframeCadence":
			requested := 0
			if msg.KeyframeInterval != nil {
				requested = *msg.KeyframeInterval
			}
			applied := h.hub.SetKeyframeInterval(requested)
			h.logger.Printf("[keyframe] player=%s requested cadence=%d", playerID, applied)
		default:
			h.logger.Printf("unknown message type %q from %s", msg.Type, playerID)
		}
	}
}

func (h *Handler) disconnect(playerID string) {
	players, npcs := h.hub.Disconnect(playerID)
	if players != nil {
		h.hub.ForceKeyframe()
		go h.hub.BroadcastState(players, npcs, nil, nil)
	}
}
