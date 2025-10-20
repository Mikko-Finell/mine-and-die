package ws

import (
	"log"
	nethttp "net/http"
	"time"

	"github.com/gorilla/websocket"

	"mine-and-die/server"
	"mine-and-die/server/internal/net/proto"
	"mine-and-die/server/internal/sim"
)

type subscription interface {
	WriteMessage(messageType int, data []byte) error
	LastCommandSeq() uint64
	StoreLastCommandSeq(seq uint64)
}

type HandlerConfig struct {
	Logger *log.Logger
}

type Handler struct {
	hub      *server.Hub
	logger   *log.Logger
	upgrader websocket.Upgrader
}

func NewHandler(hub *server.Hub, cfg HandlerConfig) *Handler {
	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *nethttp.Request) bool {
			return true
		},
	}

	return &Handler{
		hub:      hub,
		logger:   logger,
		upgrader: upgrader,
	}
}

func (h *Handler) Handle(w nethttp.ResponseWriter, r *nethttp.Request) {
	playerID := r.URL.Query().Get("id")
	if playerID == "" {
		nethttp.Error(w, "missing id", nethttp.StatusBadRequest)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Printf("upgrade failed for %s: %v", playerID, err)
		return
	}

	sub, snapshotPlayers, snapshotNPCs, snapshotGroundItems, ok := h.hub.Subscribe(playerID, conn)
	if !ok {
		message := websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "unknown player")
		conn.WriteMessage(websocket.CloseMessage, message)
		conn.Close()
		return
	}

	session := subscription(sub)

	data, entities, err := h.hub.MarshalState(snapshotPlayers, snapshotNPCs, nil, snapshotGroundItems, false, true)
	if err != nil {
		h.logger.Printf("failed to marshal initial state for %s: %v", playerID, err)
		players, npcs := h.hub.Disconnect(playerID)
		if players != nil {
			h.hub.ForceKeyframe()
			go h.hub.BroadcastState(players, npcs, nil, nil)
		}
		return
	}

	if err := session.WriteMessage(websocket.TextMessage, data); err != nil {
		players, npcs := h.hub.Disconnect(playerID)
		if players != nil {
			h.hub.ForceKeyframe()
			go h.hub.BroadcastState(players, npcs, nil, nil)
		}
		return
	}
	h.hub.RecordTelemetryBroadcast(len(data), entities)

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			players, npcs := h.hub.Disconnect(playerID)
			if players != nil {
				h.hub.ForceKeyframe()
				go h.hub.BroadcastState(players, npcs, nil, nil)
			}
			return
		}

		msg, err := proto.DecodeClientMessage(payload)
		if err != nil {
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

		writeMessage := func(data []byte, err error) bool {
			if err != nil {
				h.logger.Printf("failed to marshal response for %s: %v", playerID, err)
				return true
			}
			if err := session.WriteMessage(websocket.TextMessage, data); err != nil {
				players, npcs := h.hub.Disconnect(playerID)
				if players != nil {
					h.hub.ForceKeyframe()
					go h.hub.BroadcastState(players, npcs, nil, nil)
				}
				return false
			}
			return true
		}

		sendDuplicateAck := func() bool {
			if normalizedSeq == 0 {
				return true
			}
			ack := proto.CommandAck{Seq: normalizedSeq}
			return writeMessage(proto.EncodeCommandAck(ack))
		}

		sendCommandAck := func(cmd sim.Command) bool {
			if normalizedSeq == 0 {
				return true
			}
			ack := proto.CommandAck{Seq: normalizedSeq}
			if cmd.OriginTick > 0 {
				ack.Tick = cmd.OriginTick
			}
			if !writeMessage(proto.EncodeCommandAck(ack)) {
				return false
			}
			session.StoreLastCommandSeq(normalizedSeq)
			return true
		}

		sendCommandReject := func(reason string, retry bool) bool {
			if normalizedSeq == 0 {
				return true
			}
			reject := proto.CommandReject{
				Seq:    normalizedSeq,
				Reason: reason,
			}
			if retry {
				reject.Retry = true
			}
			return writeMessage(proto.EncodeCommandReject(reject))
		}

		switch msg.Type {
		case "input":
			if normalizedSeq > 0 {
				if last := session.LastCommandSeq(); last > 0 && normalizedSeq <= last {
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
				if last := session.LastCommandSeq(); last > 0 && normalizedSeq <= last {
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
				if last := session.LastCommandSeq(); last > 0 && normalizedSeq <= last {
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
				if last := session.LastCommandSeq(); last > 0 && normalizedSeq <= last {
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

			heartbeat := proto.Heartbeat{
				ServerTime: now.UnixMilli(),
				ClientTime: msg.SentAt,
				RTTMillis:  rtt.Milliseconds(),
			}
			if !writeMessage(proto.EncodeHeartbeat(heartbeat)) {
				return
			}
		case "console":
			ack, handled := h.hub.HandleConsoleCommand(playerID, msg.Cmd, msg.Qty)
			if !handled {
				continue
			}
			if !writeMessage(proto.EncodeConsoleAck(ack)) {
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
			if nack != nil {
				data, err := proto.EncodeKeyframeNack(nack)
				if err != nil {
					h.logger.Printf("failed to marshal keyframe for %s: %v", playerID, err)
					continue
				}
				if err := session.WriteMessage(websocket.TextMessage, data); err != nil {
					players, npcs := h.hub.Disconnect(playerID)
					if players != nil {
						h.hub.ForceKeyframe()
						go h.hub.BroadcastState(players, npcs, nil, nil)
					}
					return
				}
				continue
			}
			data, err := proto.EncodeKeyframeSnapshot(snapshot)
			if err != nil {
				h.logger.Printf("failed to marshal keyframe for %s: %v", playerID, err)
				continue
			}
			if err := session.WriteMessage(websocket.TextMessage, data); err != nil {
				players, npcs := h.hub.Disconnect(playerID)
				if players != nil {
					h.hub.ForceKeyframe()
					go h.hub.BroadcastState(players, npcs, nil, nil)
				}
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
