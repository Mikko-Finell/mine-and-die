package main

import "time"

const (
	ProtocolVersion       = 1
	writeWait             = 10 * time.Second
	tickRate              = 15    // ticks per second (10–20 Hz)
	moveSpeed             = 160.0 // pixels per second
	worldWidth            = 2400.0
	worldHeight           = 1800.0
	defaultSpawnX         = worldWidth / 2
	defaultSpawnY         = worldHeight / 2
	playerHalf            = 14.0
	lavaDamagePerSecond   = 20.0
	heartbeatInterval     = 2 * time.Second
	disconnectAfter       = 3 * heartbeatInterval
	defaultObstacleCount  = 2
	obstacleMinWidth      = 60.0
	obstacleMaxWidth      = 140.0
	obstacleMinHeight     = 60.0
	obstacleMaxHeight     = 140.0
	obstacleSpawnMargin   = 100.0
	playerSpawnSafeRadius = 120.0
	defaultGoldMineCount  = 1
	defaultGoblinCount    = 2
	defaultRatCount       = 1
	defaultNPCCount       = defaultGoblinCount + defaultRatCount
	defaultLavaCount      = 3
	tileSize              = 40.0
	goldOreMinSize        = 56.0
	goldOreMaxSize        = 96.0
)

// enableContractEffectManager gates the contract-backed EffectManager skeleton while
// it is incrementally wired into the simulation loop. Leave disabled to preserve
// current gameplay behaviour until the unified pipeline is ready.
var enableContractEffectManager = false

// enableContractEffectTransport gates the dual-write transport for contract
// effect events. Keep disabled until clients understand the `effect_spawned`,
// `effect_update`, `effect_ended`, and `effect_seq_cursors` fields on the state
// payload.
var enableContractEffectTransport = false
