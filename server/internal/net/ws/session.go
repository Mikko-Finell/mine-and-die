package ws

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"

	"mine-and-die/server"
	"mine-and-die/server/internal/sim"
)

// SessionHub defines the hub operations required to orchestrate a websocket session.
type SessionHub interface {
	Subscribe(playerID string, conn *websocket.Conn) (server.SessionSubscriber, []server.Player, []server.NPC, []server.GroundItem, bool)
	MarshalState(players []server.Player, npcs []server.NPC, triggers []server.EffectTrigger, groundItems []server.GroundItem, drainPatches bool, includeSnapshot bool) ([]byte, int, error)
	Disconnect(playerID string) ([]server.Player, []server.NPC)
	ForceKeyframe()
	BroadcastState(players []server.Player, npcs []server.NPC, triggers []server.EffectTrigger, groundItems []server.GroundItem)
	RecordTelemetryBroadcast(bytes, entities int)
	RecordAck(playerID string, ack uint64)
	UpdateIntent(playerID string, dx, dy float64, facing string) (sim.Command, bool, string)
	SetPlayerPath(playerID string, x, y float64) (sim.Command, bool, string)
	ClearPlayerPath(playerID string) (sim.Command, bool, string)
	HandleAction(playerID, action string) (sim.Command, bool, string)
	UpdateHeartbeat(playerID string, receivedAt time.Time, clientSent int64) (time.Duration, bool)
	HandleConsoleCommand(playerID, cmd string, qty int) (any, bool)
	HandleKeyframeRequest(playerID string, sub server.SessionSubscriber, sequence uint64) (any, any, bool)
	SetKeyframeInterval(requested int) int
}

type hubAdapter struct {
	hub *server.Hub
}

// NewHubAdapter wraps a server Hub in a SessionHub interface.
func NewHubAdapter(h *server.Hub) SessionHub {
	if h == nil {
		return nil
	}
	return &hubAdapter{hub: h}
}

func (a *hubAdapter) Subscribe(playerID string, conn *websocket.Conn) (server.SessionSubscriber, []server.Player, []server.NPC, []server.GroundItem, bool) {
	return a.hub.Subscribe(playerID, conn)
}

func (a *hubAdapter) MarshalState(players []server.Player, npcs []server.NPC, triggers []server.EffectTrigger, groundItems []server.GroundItem, drainPatches bool, includeSnapshot bool) ([]byte, int, error) {
	return a.hub.MarshalState(players, npcs, triggers, groundItems, drainPatches, includeSnapshot)
}

func (a *hubAdapter) Disconnect(playerID string) ([]server.Player, []server.NPC) {
	return a.hub.Disconnect(playerID)
}

func (a *hubAdapter) ForceKeyframe() {
	a.hub.ForceKeyframe()
}

func (a *hubAdapter) BroadcastState(players []server.Player, npcs []server.NPC, triggers []server.EffectTrigger, groundItems []server.GroundItem) {
	a.hub.BroadcastState(players, npcs, triggers, groundItems)
}

func (a *hubAdapter) RecordTelemetryBroadcast(bytes, entities int) {
	a.hub.RecordTelemetryBroadcast(bytes, entities)
}

func (a *hubAdapter) RecordAck(playerID string, ack uint64) {
	a.hub.RecordAck(playerID, ack)
}

func (a *hubAdapter) UpdateIntent(playerID string, dx, dy float64, facing string) (sim.Command, bool, string) {
	return a.hub.UpdateIntent(playerID, dx, dy, facing)
}

func (a *hubAdapter) SetPlayerPath(playerID string, x, y float64) (sim.Command, bool, string) {
	return a.hub.SetPlayerPath(playerID, x, y)
}

func (a *hubAdapter) ClearPlayerPath(playerID string) (sim.Command, bool, string) {
	return a.hub.ClearPlayerPath(playerID)
}

func (a *hubAdapter) HandleAction(playerID, action string) (sim.Command, bool, string) {
	return a.hub.HandleAction(playerID, action)
}

func (a *hubAdapter) UpdateHeartbeat(playerID string, receivedAt time.Time, clientSent int64) (time.Duration, bool) {
	return a.hub.UpdateHeartbeat(playerID, receivedAt, clientSent)
}

func (a *hubAdapter) HandleConsoleCommand(playerID, cmd string, qty int) (any, bool) {
	ack, handled := a.hub.HandleConsoleCommand(playerID, cmd, qty)
	return ack, handled
}

func (a *hubAdapter) HandleKeyframeRequest(playerID string, sub server.SessionSubscriber, sequence uint64) (any, any, bool) {
	snapshot, nack, ok := a.hub.HandleKeyframeRequest(playerID, sub, sequence)
	return snapshot, nack, ok
}

func (a *hubAdapter) SetKeyframeInterval(requested int) int {
	return a.hub.SetKeyframeInterval(requested)
}

// SessionConfig contains the inputs required to run a websocket session loop.
type SessionConfig struct {
	PlayerID string
	Conn     *websocket.Conn
	Hub      SessionHub
	Logger   *log.Logger
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

// Serve handles the websocket session lifecycle for a single player.
func Serve(cfg SessionConfig) {
	if cfg.Conn == nil || cfg.Hub == nil {
		if cfg.Conn != nil {
			cfg.Conn.Close()
		}
		return
	}

	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}

	sub, snapshotPlayers, snapshotNPCs, snapshotGroundItems, ok := cfg.Hub.Subscribe(cfg.PlayerID, cfg.Conn)
	if !ok {
		message := websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "unknown player")
		cfg.Conn.WriteMessage(websocket.CloseMessage, message)
		cfg.Conn.Close()
		return
	}

	data, entities, err := cfg.Hub.MarshalState(snapshotPlayers, snapshotNPCs, nil, snapshotGroundItems, false, true)
	if err != nil {
		logger.Printf("failed to marshal initial state for %s: %v", cfg.PlayerID, err)
		disconnectAndBroadcast(cfg.Hub, cfg.PlayerID)
		return
	}

	if err := sub.WriteMessage(websocket.TextMessage, data); err != nil {
		disconnectAndBroadcast(cfg.Hub, cfg.PlayerID)
		return
	}
	cfg.Hub.RecordTelemetryBroadcast(len(data), entities)

	for {
		_, payload, err := cfg.Conn.ReadMessage()
		if err != nil {
			disconnectAndBroadcast(cfg.Hub, cfg.PlayerID)
			return
		}

		var msg clientMessage
		if err := json.Unmarshal(payload, &msg); err != nil {
			logger.Printf("discarding malformed message from %s: %v", cfg.PlayerID, err)
			continue
		}

		if msg.Ack != nil {
			cfg.Hub.RecordAck(cfg.PlayerID, *msg.Ack)
		}

		normalizedSeq := uint64(0)
		if msg.CommandSeq != nil && *msg.CommandSeq > 0 {
			normalizedSeq = *msg.CommandSeq
		}

		writeJSON := func(payload any) bool {
			data, err := json.Marshal(payload)
			if err != nil {
				logger.Printf("failed to marshal response for %s: %v", cfg.PlayerID, err)
				return true
			}
			if err := sub.WriteMessage(websocket.TextMessage, data); err != nil {
				disconnectAndBroadcast(cfg.Hub, cfg.PlayerID)
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
			cmd, ok, reason := cfg.Hub.UpdateIntent(cfg.PlayerID, msg.DX, msg.DY, msg.Facing)
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
				logger.Printf("input ignored for unknown player %s", cfg.PlayerID)
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
			cmd, ok, reason := cfg.Hub.SetPlayerPath(cfg.PlayerID, msg.X, msg.Y)
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
				logger.Printf("path request ignored for unknown player %s", cfg.PlayerID)
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
			cmd, ok, reason := cfg.Hub.ClearPlayerPath(cfg.PlayerID)
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
				logger.Printf("cancelPath ignored for unknown player %s", cfg.PlayerID)
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
			cmd, ok, reason := cfg.Hub.HandleAction(cfg.PlayerID, msg.Action)
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
					logger.Printf("unknown action %q from %s", msg.Action, cfg.PlayerID)
				} else if reason == server.CommandRejectUnknownActor {
					logger.Printf("action ignored for unknown player %s", cfg.PlayerID)
				}
			}
		case "heartbeat":
			now := time.Now()
			rtt, ok := cfg.Hub.UpdateHeartbeat(cfg.PlayerID, now, msg.SentAt)
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
				logger.Printf("failed to marshal heartbeat ack for %s: %v", cfg.PlayerID, err)
				continue
			}

			if err := sub.WriteMessage(websocket.TextMessage, data); err != nil {
				disconnectAndBroadcast(cfg.Hub, cfg.PlayerID)
				return
			}
		case "console":
			ack, handled := cfg.Hub.HandleConsoleCommand(cfg.PlayerID, msg.Cmd, msg.Qty)
			if !handled {
				continue
			}
			data, err := json.Marshal(ack)
			if err != nil {
				logger.Printf("failed to marshal console ack for %s: %v", cfg.PlayerID, err)
				continue
			}
			if err := sub.WriteMessage(websocket.TextMessage, data); err != nil {
				disconnectAndBroadcast(cfg.Hub, cfg.PlayerID)
				return
			}
		case "keyframeRequest":
			if msg.KeyframeSeq == nil {
				continue
			}
			snapshot, nack, ok := cfg.Hub.HandleKeyframeRequest(cfg.PlayerID, sub, *msg.KeyframeSeq)
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
				logger.Printf("failed to marshal keyframe for %s: %v", cfg.PlayerID, err)
				continue
			}
			if err := sub.WriteMessage(websocket.TextMessage, data); err != nil {
				disconnectAndBroadcast(cfg.Hub, cfg.PlayerID)
				return
			}
		case "keyframeCadence":
			requested := 0
			if msg.KeyframeInterval != nil {
				requested = *msg.KeyframeInterval
			}
			applied := cfg.Hub.SetKeyframeInterval(requested)
			logger.Printf("[keyframe] player=%s requested cadence=%d", cfg.PlayerID, applied)
		default:
			logger.Printf("unknown message type %q from %s", msg.Type, cfg.PlayerID)
		}
	}
}

func disconnectAndBroadcast(h SessionHub, playerID string) {
	if h == nil {
		return
	}
	players, npcs := h.Disconnect(playerID)
	if players != nil {
		h.ForceKeyframe()
		go h.BroadcastState(players, npcs, nil, nil)
	}
}
