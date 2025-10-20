package world

import "math/rand"

// NPCSpawner exposes the legacy world surface required for deterministic NPC seeding.
type NPCSpawner interface {
	Config() Config
	Dimensions() (float64, float64)
	SubsystemRNG(label string) *rand.Rand
	SpawnGoblinAt(x, y float64, waypoints []Vec2, goldQty, potionQty int)
	SpawnRatAt(x, y float64)
}

// SeedInitialNPCs mirrors the legacy spawn layout for goblins and rats.
func SeedInitialNPCs(spawner NPCSpawner) {
	if spawner == nil {
		return
	}

	cfg := spawner.Config()
	if !cfg.NPCs {
		return
	}

	goblinTarget := cfg.GoblinCount
	ratTarget := cfg.RatCount
	if goblinTarget <= 0 && ratTarget <= 0 {
		return
	}

	centerX := DefaultSpawnX
	centerY := DefaultSpawnY

	goblinsSpawned := 0
	if goblinTarget >= 1 {
		patrolOffset := 160.0
		spawner.SpawnGoblinAt(centerX-patrolOffset, centerY-patrolOffset, []Vec2{
			{X: centerX - patrolOffset, Y: centerY - patrolOffset},
			{X: centerX + patrolOffset, Y: centerY - patrolOffset},
			{X: centerX + patrolOffset, Y: centerY + patrolOffset},
			{X: centerX - patrolOffset, Y: centerY + patrolOffset},
		}, 12, 1)
		goblinsSpawned++
	}
	if goblinTarget >= 2 {
		topLeftX := centerX + 120.0
		height := 220.0
		width := 220.0
		topLeftY := centerY - height/2
		spawner.SpawnGoblinAt(topLeftX, topLeftY, []Vec2{
			{X: topLeftX, Y: topLeftY},
			{X: topLeftX + width, Y: topLeftY},
			{X: topLeftX + width, Y: topLeftY + height},
			{X: topLeftX, Y: topLeftY + height},
		}, 8, 1)
		goblinsSpawned++
	}
	extraGoblins := goblinTarget - goblinsSpawned
	if extraGoblins > 0 {
		SpawnExtraGoblins(spawner, extraGoblins)
	}

	ratsSpawned := 0
	if ratTarget >= 1 {
		spawner.SpawnRatAt(centerX-200, centerY+240)
		ratsSpawned++
	}
	extraRats := ratTarget - ratsSpawned
	if extraRats > 0 {
		SpawnExtraRats(spawner, extraRats)
	}
}

// SpawnExtraGoblins distributes extra goblins around the map perimeter.
func SpawnExtraGoblins(spawner NPCSpawner, count int) {
	if spawner == nil || count <= 0 {
		return
	}

	rng := spawner.SubsystemRNG("npcs.extraGoblin")
	if rng == nil {
		return
	}

	width, height := spawner.Dimensions()

	const patrolRadius = 60.0

	minX := ObstacleSpawnMargin + patrolRadius
	maxX := width - ObstacleSpawnMargin - patrolRadius
	if maxX <= minX {
		minX = PlayerHalf + patrolRadius
		maxX = width - PlayerHalf - patrolRadius
	}

	minY := ObstacleSpawnMargin + patrolRadius
	maxY := height - ObstacleSpawnMargin - patrolRadius
	if maxY <= minY {
		minY = PlayerHalf + patrolRadius
		maxY = height - PlayerHalf - patrolRadius
	}

	centralMinX, centralMaxX := CentralCenterRange(width, DefaultSpawnX, ObstacleSpawnMargin, patrolRadius)
	if centralMaxX >= centralMinX {
		minX = centralMinX
		maxX = centralMaxX
	}
	centralMinY, centralMaxY := CentralCenterRange(height, DefaultSpawnY, ObstacleSpawnMargin, patrolRadius)
	if centralMaxY >= centralMinY {
		minY = centralMinY
		maxY = centralMaxY
	}

	for i := 0; i < count; i++ {
		x := minX
		if maxX > minX {
			x = minX + rng.Float64()*(maxX-minX)
		}
		y := minY
		if maxY > minY {
			y = minY + rng.Float64()*(maxY-minY)
		}

		topLeftX := Clamp(x-patrolRadius, PlayerHalf, width-PlayerHalf)
		topLeftY := Clamp(y-patrolRadius, PlayerHalf, height-PlayerHalf)
		topRightX := Clamp(x+patrolRadius, PlayerHalf, width-PlayerHalf)
		bottomY := Clamp(y+patrolRadius, PlayerHalf, height-PlayerHalf)

		waypoints := []Vec2{
			{X: topLeftX, Y: topLeftY},
			{X: topRightX, Y: topLeftY},
			{X: topRightX, Y: bottomY},
			{X: topLeftX, Y: bottomY},
		}

		spawner.SpawnGoblinAt(topLeftX, topLeftY, waypoints, 10, 1)
	}
}

// SpawnExtraRats distributes extra rats through the central spawn area.
func SpawnExtraRats(spawner NPCSpawner, count int) {
	if spawner == nil || count <= 0 {
		return
	}

	rng := spawner.SubsystemRNG("npcs.extra")
	if rng == nil {
		return
	}

	width, height := spawner.Dimensions()
	minX, maxX := CentralCenterRange(width, DefaultSpawnX, ObstacleSpawnMargin, PlayerHalf)
	minY, maxY := CentralCenterRange(height, DefaultSpawnY, ObstacleSpawnMargin, PlayerHalf)

	for i := 0; i < count; i++ {
		x := minX
		if maxX > minX {
			x = minX + rng.Float64()*(maxX-minX)
		}
		y := minY
		if maxY > minY {
			y = minY + rng.Float64()*(maxY-minY)
		}
		spawner.SpawnRatAt(x, y)
	}
}
