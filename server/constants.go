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
	defaultSpawnX         = worldpkg.DefaultSpawnX
	defaultSpawnY         = worldpkg.DefaultSpawnY
	playerHalf            = worldpkg.PlayerHalf
	lavaDamagePerSecond   = 20.0
	heartbeatInterval     = 2 * time.Second
	disconnectAfter       = 3 * heartbeatInterval
	defaultObstacleCount  = 0
	obstacleMinWidth      = worldpkg.ObstacleMinWidth
	obstacleMaxWidth      = worldpkg.ObstacleMaxWidth
	obstacleMinHeight     = worldpkg.ObstacleMinHeight
	obstacleMaxHeight     = worldpkg.ObstacleMaxHeight
	obstacleSpawnMargin   = worldpkg.ObstacleSpawnMargin
	playerSpawnSafeRadius = worldpkg.PlayerSpawnSafeRadius
	defaultGoldMineCount  = 0
	defaultGoblinCount    = 0
	defaultRatCount       = 0
	defaultNPCCount       = defaultGoblinCount + defaultRatCount
	defaultLavaCount      = 0
	tileSize              = 40.0
	goldOreMinSize        = worldpkg.GoldOreMinSize
	goldOreMaxSize        = worldpkg.GoldOreMaxSize
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
