package server

import (
	"mine-and-die/server/internal/net/proto"
	"mine-and-die/server/internal/sim"
)

type joinResponse = proto.JoinResponseV1

type stateMessage = proto.StateSnapshotV1

type keyframeMessage = proto.KeyframeSnapshotV1

type keyframeNackMessage struct {
	Ver      int             `json:"ver"`
	Type     string          `json:"type"`
	Sequence uint64          `json:"sequence"`
	Reason   string          `json:"reason"`
	Resync   bool            `json:"resync,omitempty"`
	Config   sim.WorldConfig `json:"config,omitempty"`
}

func (*keyframeNackMessage) ProtoKeyframeNack() {}

type diagnosticsPlayer struct {
	Ver           int    `json:"ver"`
	ID            string `json:"id"`
	LastHeartbeat int64  `json:"lastHeartbeat"`
	RTTMillis     int64  `json:"rttMillis"`
	LastAck       uint64 `json:"lastAck"`
}
