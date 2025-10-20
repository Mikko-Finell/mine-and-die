package server

import effectcontract "mine-and-die/server/effects/contract"

type joinResponse struct {
	Ver               int             `json:"ver"`
	ID                string          `json:"id"`
	Players           []Player        `json:"players"`
	NPCs              []NPC           `json:"npcs"`
	Obstacles         []Obstacle      `json:"obstacles"`
	EffectTriggers    []EffectTrigger `json:"effectTriggers,omitempty"`
	GroundItems       []GroundItem    `json:"groundItems,omitempty"`
	Patches           []Patch         `json:"patches,omitempty"`
	Config            worldConfig     `json:"config"`
	Resync            bool            `json:"resync"`
	KeyframeInterval  int             `json:"keyframeInterval,omitempty"`
	EffectCatalogHash string          `json:"effectCatalogHash"`
}

type stateMessage struct {
	Ver              int                                `json:"ver"`
	Type             string                             `json:"type"`
	Players          []Player                           `json:"players,omitempty"`
	NPCs             []NPC                              `json:"npcs,omitempty"`
	Obstacles        []Obstacle                         `json:"obstacles,omitempty"`
	EffectTriggers   []EffectTrigger                    `json:"effectTriggers,omitempty"`
	EffectSpawns     []effectcontract.EffectSpawnEvent  `json:"effect_spawned,omitempty"`
	EffectUpdates    []effectcontract.EffectUpdateEvent `json:"effect_update,omitempty"`
	EffectEnds       []effectcontract.EffectEndEvent    `json:"effect_ended,omitempty"`
	EffectSeqCursors map[string]effectcontract.Seq      `json:"effect_seq_cursors,omitempty"`
	GroundItems      []GroundItem                       `json:"groundItems,omitempty"`
	Patches          []Patch                            `json:"patches"`
	Tick             uint64                             `json:"t"`
	Sequence         uint64                             `json:"sequence"`
	KeyframeSeq      uint64                             `json:"keyframeSeq"`
	ServerTime       int64                              `json:"serverTime"`
	Config           worldConfig                        `json:"config"`
	Resync           bool                               `json:"resync,omitempty"`
	KeyframeInterval int                                `json:"keyframeInterval,omitempty"`
}

type keyframeMessage struct {
	Ver         int          `json:"ver"`
	Type        string       `json:"type"`
	Sequence    uint64       `json:"sequence"`
	Tick        uint64       `json:"t"`
	Players     []Player     `json:"players"`
	NPCs        []NPC        `json:"npcs"`
	Obstacles   []Obstacle   `json:"obstacles"`
	GroundItems []GroundItem `json:"groundItems"`
	Config      worldConfig  `json:"config"`
}

type keyframeNackMessage struct {
	Ver      int         `json:"ver"`
	Type     string      `json:"type"`
	Sequence uint64      `json:"sequence"`
	Reason   string      `json:"reason"`
	Resync   bool        `json:"resync,omitempty"`
	Config   worldConfig `json:"config,omitempty"`
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
