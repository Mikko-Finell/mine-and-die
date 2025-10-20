package server

import (
	"time"

	"mine-and-die/server/internal/net/proto"
	worldpkg "mine-and-die/server/internal/world"
)

const (
	ProtocolVersion       = proto.Version
	writeWait             = 10 * time.Second
	tickRate              = 15    // ticks per second (10–20 Hz)
	moveSpeed             = 160.0 // pixels per second
	worldWidth            = worldpkg.DefaultWidth
	worldHeight           = worldpkg.DefaultHeight
	defaultSpawnX         = worldWidth / 2
	defaultSpawnY         = worldHeight / 2
	playerHalf            = 14.0
	lavaDamagePerSecond   = 20.0
	heartbeatInterval     = 2 * time.Second
	disconnectAfter       = 3 * heartbeatInterval
	defaultObstacleCount  = 0
	obstacleMinWidth      = 60.0
	obstacleMaxWidth      = 140.0
	obstacleMinHeight     = 60.0
	obstacleMaxHeight     = 140.0
	obstacleSpawnMargin   = 100.0
	playerSpawnSafeRadius = 120.0
	defaultGoldMineCount  = 0
	defaultGoblinCount    = 0
	defaultRatCount       = 0
	defaultNPCCount       = defaultGoblinCount + defaultRatCount
	defaultLavaCount      = 0
	tileSize              = 40.0
	goldOreMinSize        = 56.0
	goldOreMaxSize        = 96.0
)

// TickRate reports the server tick frequency in hertz.
func TickRate() int {
	return tickRate
}

// HeartbeatInterval reports how frequently clients must send heartbeats to stay connected.
func HeartbeatInterval() time.Duration {
	return heartbeatInterval
}

// WriteWaitDuration reports the timeout applied to outbound websocket writes.
func WriteWaitDuration() time.Duration {
	return writeWait
}
