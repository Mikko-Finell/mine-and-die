package server

import (
	effectcontract "mine-and-die/server/effects/contract"
	"mine-and-die/server/internal/sim"
)

type joinResponse struct {
	Ver               int                 `json:"ver"`
	ID                string              `json:"id"`
	Players           []sim.Player        `json:"players"`
	NPCs              []sim.NPC           `json:"npcs"`
	Obstacles         []sim.Obstacle      `json:"obstacles"`
	EffectTriggers    []sim.EffectTrigger `json:"effectTriggers,omitempty"`
	GroundItems       []sim.GroundItem    `json:"groundItems,omitempty"`
	Patches           []sim.Patch         `json:"patches,omitempty"`
	Config            sim.WorldConfig     `json:"config"`
	Resync            bool                `json:"resync"`
	KeyframeInterval  int                 `json:"keyframeInterval,omitempty"`
	EffectCatalogHash string              `json:"effectCatalogHash"`
}

func (joinResponse) ProtoJoinResponse() {}

type stateMessage struct {
	Ver              int                                `json:"ver"`
	Type             string                             `json:"type"`
	Players          []sim.Player                       `json:"players,omitempty"`
	NPCs             []sim.NPC                          `json:"npcs,omitempty"`
	Obstacles        []sim.Obstacle                     `json:"obstacles,omitempty"`
	EffectTriggers   []sim.EffectTrigger                `json:"effectTriggers,omitempty"`
	EffectSpawns     []effectcontract.EffectSpawnEvent  `json:"effect_spawned,omitempty"`
	EffectUpdates    []effectcontract.EffectUpdateEvent `json:"effect_update,omitempty"`
	EffectEnds       []effectcontract.EffectEndEvent    `json:"effect_ended,omitempty"`
	EffectSeqCursors map[string]effectcontract.Seq      `json:"effect_seq_cursors,omitempty"`
	GroundItems      []sim.GroundItem                   `json:"groundItems,omitempty"`
	Patches          []sim.Patch                        `json:"patches"`
	Tick             uint64                             `json:"t"`
	Sequence         uint64                             `json:"sequence"`
	KeyframeSeq      uint64                             `json:"keyframeSeq"`
	ServerTime       int64                              `json:"serverTime"`
	Config           sim.WorldConfig                    `json:"config"`
	Resync           bool                               `json:"resync,omitempty"`
	KeyframeInterval int                                `json:"keyframeInterval,omitempty"`
}

type keyframeMessage struct {
	Ver         int              `json:"ver"`
	Type        string           `json:"type"`
	Sequence    uint64           `json:"sequence"`
	Tick        uint64           `json:"t"`
	Players     []sim.Player     `json:"players"`
	NPCs        []sim.NPC        `json:"npcs"`
	Obstacles   []sim.Obstacle   `json:"obstacles"`
	GroundItems []sim.GroundItem `json:"groundItems"`
	Config      sim.WorldConfig  `json:"config"`
}

type keyframeNackMessage struct {
	Ver      int             `json:"ver"`
	Type     string          `json:"type"`
	Sequence uint64          `json:"sequence"`
	Reason   string          `json:"reason"`
	Resync   bool            `json:"resync,omitempty"`
	Config   sim.WorldConfig `json:"config,omitempty"`
}

func (stateMessage) ProtoStateSnapshot()        {}
func (keyframeMessage) ProtoKeyframeSnapshot()  {}
func (*keyframeNackMessage) ProtoKeyframeNack() {}

type diagnosticsPlayer struct {
	Ver           int    `json:"ver"`
	ID            string `json:"id"`
	LastHeartbeat int64  `json:"lastHeartbeat"`
	RTTMillis     int64  `json:"rttMillis"`
	LastAck       uint64 `json:"lastAck"`
}
