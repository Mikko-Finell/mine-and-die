package proto

import (
	"encoding/json"

	"mine-and-die/server/internal/sim"
)

const (
	// Version tracks the wire-protocol revision expected by clients.
	Version = 1

	// Type identifiers for websocket payloads.
	typeCommandAck    = "commandAck"
	typeCommandReject = "commandReject"
	typeHeartbeat     = "heartbeat"
	typeConsoleAck    = "console_ack"
	typeState         = "state"
	typeKeyframe      = "keyframe"
	typeKeyframeNack  = "keyframeNack"
)

// Client message type identifiers.
const (
	TypeInput           = "input"
	TypePath            = "path"
	TypeCancelPath      = "cancelPath"
	TypeAction          = "action"
	TypeHeartbeat       = "heartbeat"
	TypeConsole         = "console"
	TypeKeyframeReq     = "keyframeRequest"
	TypeKeyframeCadence = "keyframeCadence"
)

// Exported aliases for outbound message type identifiers.
const (
	TypeState        = typeState
	TypeKeyframe     = typeKeyframe
	TypeKeyframeNack = typeKeyframeNack
)

type stateSnapshot interface {
	ProtoStateSnapshot()
}

// EncodeStateSnapshot renders a state snapshot payload.
func EncodeStateSnapshot(msg stateSnapshot) ([]byte, error) {
	return json.Marshal(msg)
}

type joinResponse interface {
	ProtoJoinResponse()
}

// EncodeJoinResponse renders a join response payload.
func EncodeJoinResponse(msg joinResponse) ([]byte, error) {
	return json.Marshal(msg)
}

type keyframeSnapshot interface {
	ProtoKeyframeSnapshot()
}

// EncodeKeyframeSnapshot renders a keyframe payload.
func EncodeKeyframeSnapshot(msg keyframeSnapshot) ([]byte, error) {
	return json.Marshal(msg)
}

type keyframeNack interface {
	ProtoKeyframeNack()
}

// EncodeKeyframeNack renders a keyframe nack payload.
func EncodeKeyframeNack(msg keyframeNack) ([]byte, error) {
	return json.Marshal(msg)
}

// ClientMessage captures an inbound websocket message from the client.
type ClientMessage struct {
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

// DecodeClientMessage converts raw websocket payloads into a structured message.
func DecodeClientMessage(payload []byte) (ClientMessage, error) {
	var msg ClientMessage
	return msg, json.Unmarshal(payload, &msg)
}

// ClientCommand captures the structured simulation command carried by a
// websocket message. Origin metadata is populated by the hub when the command
// is accepted for processing.
func ClientCommand(msg ClientMessage) (sim.Command, bool) {
	switch msg.Type {
	case TypeInput:
		return sim.Command{
			Type: sim.CommandMove,
			Move: &sim.MoveCommand{
				DX:     msg.DX,
				DY:     msg.DY,
				Facing: parseFacing(msg.Facing),
			},
		}, true
	case TypePath:
		return sim.Command{
			Type: sim.CommandSetPath,
			Path: &sim.PathCommand{
				TargetX: msg.X,
				TargetY: msg.Y,
			},
		}, true
	case TypeCancelPath:
		return sim.Command{Type: sim.CommandClearPath}, true
	case TypeAction:
		if msg.Action == "" {
			return sim.Command{}, false
		}
		return sim.Command{
			Type: sim.CommandAction,
			Action: &sim.ActionCommand{
				Name: msg.Action,
			},
		}, true
	default:
		return sim.Command{}, false
	}
}

func parseFacing(value string) sim.FacingDirection {
	switch sim.FacingDirection(value) {
	case sim.FacingUp, sim.FacingDown, sim.FacingLeft, sim.FacingRight:
		return sim.FacingDirection(value)
	default:
		return ""
	}
}

// CommandAck describes an acknowledgement of a processed command.
type CommandAck struct {
	Seq  uint64
	Tick uint64
}

// EncodeCommandAck renders a command acknowledgement response.
func EncodeCommandAck(msg CommandAck) ([]byte, error) {
	frame := struct {
		Ver  int    `json:"ver"`
		Type string `json:"type"`
		Seq  uint64 `json:"seq"`
		Tick uint64 `json:"tick,omitempty"`
	}{
		Ver:  Version,
		Type: typeCommandAck,
		Seq:  msg.Seq,
	}
	if msg.Tick > 0 {
		frame.Tick = msg.Tick
	}
	return json.Marshal(frame)
}

// CommandReject notifies the client that a command was refused.
type CommandReject struct {
	Seq    uint64
	Reason string
	Retry  bool
	Tick   uint64
}

// EncodeCommandReject renders a command rejection response.
func EncodeCommandReject(msg CommandReject) ([]byte, error) {
	frame := struct {
		Ver    int    `json:"ver"`
		Type   string `json:"type"`
		Seq    uint64 `json:"seq"`
		Reason string `json:"reason"`
		Retry  bool   `json:"retry,omitempty"`
		Tick   uint64 `json:"tick,omitempty"`
	}{
		Ver:    Version,
		Type:   typeCommandReject,
		Seq:    msg.Seq,
		Reason: msg.Reason,
	}
	if msg.Retry {
		frame.Retry = true
	}
	if msg.Tick > 0 {
		frame.Tick = msg.Tick
	}
	return json.Marshal(frame)
}

// Heartbeat echoes timing metadata back to the client.
type Heartbeat struct {
	ServerTime int64
	ClientTime int64
	RTTMillis  int64
}

// EncodeHeartbeat renders a heartbeat acknowledgement payload.
func EncodeHeartbeat(msg Heartbeat) ([]byte, error) {
	frame := struct {
		Ver        int    `json:"ver"`
		Type       string `json:"type"`
		ServerTime int64  `json:"serverTime"`
		ClientTime int64  `json:"clientTime"`
		RTTMillis  int64  `json:"rtt"`
	}{
		Ver:        Version,
		Type:       typeHeartbeat,
		ServerTime: msg.ServerTime,
		ClientTime: msg.ClientTime,
		RTTMillis:  msg.RTTMillis,
	}
	return json.Marshal(frame)
}

// ConsoleAck captures the outcome of a console command.
type ConsoleAck struct {
	Cmd     string
	Status  string
	Reason  string
	Qty     int
	StackID string
	Slot    string
}

// NewConsoleAck constructs a baseline acknowledgement for the given command.
func NewConsoleAck(cmd string) ConsoleAck {
	return ConsoleAck{Cmd: cmd}
}

// EncodeConsoleAck renders a console command acknowledgement payload.
func EncodeConsoleAck(msg ConsoleAck) ([]byte, error) {
	frame := struct {
		Ver     int    `json:"ver"`
		Type    string `json:"type"`
		Cmd     string `json:"cmd"`
		Status  string `json:"status"`
		Reason  string `json:"reason,omitempty"`
		Qty     int    `json:"qty,omitempty"`
		StackID string `json:"stackId,omitempty"`
		Slot    string `json:"slot,omitempty"`
	}{
		Ver:     Version,
		Type:    typeConsoleAck,
		Cmd:     msg.Cmd,
		Status:  msg.Status,
		Reason:  msg.Reason,
		Qty:     msg.Qty,
		StackID: msg.StackID,
		Slot:    msg.Slot,
	}
	return json.Marshal(frame)
}
