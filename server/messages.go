package main

type joinResponse struct {
	ID        string     `json:"id"`
	Players   []Player   `json:"players"`
	Obstacles []Obstacle `json:"obstacles"`
	Effects   []Effect   `json:"effects"`
}

type stateMessage struct {
	Type       string     `json:"type"`
	Players    []Player   `json:"players"`
	Obstacles  []Obstacle `json:"obstacles"`
	Effects    []Effect   `json:"effects"`
	ServerTime int64      `json:"serverTime"`
}

type clientMessage struct {
	Type   string  `json:"type"`
	DX     float64 `json:"dx"`
	DY     float64 `json:"dy"`
	Facing string  `json:"facing"`
	SentAt int64   `json:"sentAt"`
	Action string  `json:"action"`
}

type heartbeatMessage struct {
	Type       string `json:"type"`
	ServerTime int64  `json:"serverTime"`
	ClientTime int64  `json:"clientTime"`
	RTTMillis  int64  `json:"rtt"`
}

type diagnosticsPlayer struct {
	ID            string `json:"id"`
	LastHeartbeat int64  `json:"lastHeartbeat"`
	RTTMillis     int64  `json:"rttMillis"`
}
