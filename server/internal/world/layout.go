package world

const (
	// DefaultSpawnX and DefaultSpawnY represent the center of the map used
	// by legacy spawn logic. These remain fixed so constructor helpers can
	// reproduce the historical layout regardless of configured dimensions.
	DefaultSpawnX = DefaultWidth / 2
	DefaultSpawnY = DefaultHeight / 2

	PlayerHalf            = 14.0
	ObstacleSpawnMargin   = 100.0
	PlayerSpawnSafeRadius = 120.0

	ObstacleMinWidth  = 60.0
	ObstacleMaxWidth  = 140.0
	ObstacleMinHeight = 60.0
	ObstacleMaxHeight = 140.0

	GoldOreMinSize = 56.0
	GoldOreMaxSize = 96.0
)

const (
	ObstacleTypeGoldOre = "gold-ore"
	ObstacleTypeLava    = "lava"
)
