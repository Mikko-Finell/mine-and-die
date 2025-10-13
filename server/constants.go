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

// enableContractEffectManager gates the contract-backed EffectManager skeleton.
// With the unified pipeline rolled out, it now defaults to enabled so the
// contract journal remains authoritative for all effect lifecycles.
var enableContractEffectManager = true

// enableContractEffectTransport gates the transport for contract effect events.
// Unified clients rely on these batches, so keep the transport enabled unless a
// targeted rollback is required for debugging.
var enableContractEffectTransport = true

// enableContractMeleeDefinitions hands melee behaviour over to the contract
// EffectManager hooks. Default it to enabled so the unified definitions remain
// authoritative.
var enableContractMeleeDefinitions = true

// enableContractProjectileDefinitions hands projectile behaviour over to the
// contract EffectManager hooks. Default it to enabled for unified execution.
var enableContractProjectileDefinitions = true

// enableContractBurningDefinitions hands burning status visuals and damage
// ticks over to the contract EffectManager hooks. Default it to enabled for the
// unified runtime.
var enableContractBurningDefinitions = true

// enableContractBloodDecalDefinitions hands blood decal visuals over to the
// contract EffectManager hooks. Default it to enabled to keep the unified
// transport authoritative.
var enableContractBloodDecalDefinitions = true
