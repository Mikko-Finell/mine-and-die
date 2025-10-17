package main

import effectcontract "mine-and-die/server/effects/contract"

type joinResponse struct {
	Ver              int                              `json:"ver"`
	ID               string                           `json:"id"`
	Players          []Player                         `json:"players"`
	NPCs             []NPC                            `json:"npcs"`
	Obstacles        []Obstacle                       `json:"obstacles"`
	EffectTriggers   []EffectTrigger                  `json:"effectTriggers,omitempty"`
	GroundItems      []GroundItem                     `json:"groundItems,omitempty"`
	Patches          []Patch                          `json:"patches,omitempty"`
	Config           worldConfig                      `json:"config"`
	Resync           bool                             `json:"resync"`
	KeyframeInterval int                              `json:"keyframeInterval,omitempty"`
	EffectCatalog    map[string]effectCatalogMetadata `json:"effectCatalog,omitempty"`
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
}

type consoleAckMessage struct {
	Ver     int    `json:"ver"`
	Type    string `json:"type"`
	Cmd     string `json:"cmd"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Qty     int    `json:"qty,omitempty"`
	StackID string `json:"stackId,omitempty"`
	Slot    string `json:"slot,omitempty"`
}

type heartbeatMessage struct {
	Ver        int    `json:"ver"`
	Type       string `json:"type"`
	ServerTime int64  `json:"serverTime"`
	ClientTime int64  `json:"clientTime"`
	RTTMillis  int64  `json:"rtt"`
}

type diagnosticsPlayer struct {
	Ver           int    `json:"ver"`
	ID            string `json:"id"`
	LastHeartbeat int64  `json:"lastHeartbeat"`
	RTTMillis     int64  `json:"rttMillis"`
	LastAck       uint64 `json:"lastAck"`
}
