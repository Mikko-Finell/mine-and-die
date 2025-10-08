package main

type joinResponse struct {
	ID             string          `json:"id"`
	Players        []Player        `json:"players"`
	NPCs           []NPC           `json:"npcs"`
	Obstacles      []Obstacle      `json:"obstacles"`
	Effects        []Effect        `json:"effects"`
	GroundItems    []GroundItem    `json:"groundItems"`
	EffectTriggers []EffectTrigger `json:"effectTriggers,omitempty"`
	Config         worldConfig     `json:"config"`
}

type stateMessage struct {
	Type           string          `json:"type"`
	Players        []Player        `json:"players"`
	NPCs           []NPC           `json:"npcs"`
	Obstacles      []Obstacle      `json:"obstacles"`
	Effects        []Effect        `json:"effects"`
	GroundItems    []GroundItem    `json:"groundItems"`
	EffectTriggers []EffectTrigger `json:"effectTriggers,omitempty"`
	ServerTime     int64           `json:"serverTime"`
	Config         worldConfig     `json:"config"`
}

type clientMessage struct {
	Type   string  `json:"type"`
	DX     float64 `json:"dx"`
	DY     float64 `json:"dy"`
	Facing string  `json:"facing"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	SentAt int64   `json:"sentAt"`
	Action string  `json:"action"`
	Cmd    string  `json:"cmd"`
	Qty    int     `json:"qty"`
}

type heartbeatMessage struct {
	Type       string `json:"type"`
	ServerTime int64  `json:"serverTime"`
	ClientTime int64  `json:"clientTime"`
	RTTMillis  int64  `json:"rtt"`
}

type consoleAckMessage struct {
	Type   string `json:"type"`
	Cmd    string `json:"cmd"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
	Qty    int    `json:"qty,omitempty"`
}

type diagnosticsPlayer struct {
	ID            string `json:"id"`
	LastHeartbeat int64  `json:"lastHeartbeat"`
	RTTMillis     int64  `json:"rttMillis"`
}
