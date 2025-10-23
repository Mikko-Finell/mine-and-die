package ai

import (
	"math/rand"

	worldpkg "mine-and-die/server/internal/world"
)

// WorldNPCSpawner adapts world callbacks to the world NPC seeding helpers.
type WorldNPCSpawner struct {
	ConfigFunc       func() worldpkg.Config
	DimensionsFunc   func() (float64, float64)
	SubsystemRNGFunc func(label string) *rand.Rand
	SpawnGoblinFunc  func(x, y float64, waypoints []worldpkg.Vec2, goldQty, potionQty int)
	SpawnRatFunc     func(x, y float64)
}

// Config returns the configured world settings or the defaults when unavailable.
func (s WorldNPCSpawner) Config() worldpkg.Config {
	if s.ConfigFunc == nil {
		return worldpkg.DefaultConfig()
	}
	return s.ConfigFunc()
}

// Dimensions reports the world size or defaults when the callback is missing.
func (s WorldNPCSpawner) Dimensions() (float64, float64) {
	if s.DimensionsFunc == nil {
		return worldpkg.DefaultWidth, worldpkg.DefaultHeight
	}
	return s.DimensionsFunc()
}

// SubsystemRNG returns the deterministic RNG for the supplied subsystem label.
func (s WorldNPCSpawner) SubsystemRNG(label string) *rand.Rand {
	if s.SubsystemRNGFunc == nil {
		return worldpkg.NewDeterministicRNG(worldpkg.DefaultSeed, label)
	}
	if rng := s.SubsystemRNGFunc(label); rng != nil {
		return rng
	}
	return worldpkg.NewDeterministicRNG(worldpkg.DefaultSeed, label)
}

// SpawnGoblinAt delegates goblin spawning to the configured callback when present.
func (s WorldNPCSpawner) SpawnGoblinAt(x, y float64, waypoints []worldpkg.Vec2, goldQty, potionQty int) {
	if s.SpawnGoblinFunc == nil {
		return
	}
	s.SpawnGoblinFunc(x, y, waypoints, goldQty, potionQty)
}

// SpawnRatAt delegates rat spawning to the configured callback when present.
func (s WorldNPCSpawner) SpawnRatAt(x, y float64) {
	if s.SpawnRatFunc == nil {
		return
	}
	s.SpawnRatFunc(x, y)
}

// SeedInitialNPCs mirrors the legacy spawn layout for goblins and rats.
func SeedInitialNPCs(spawner WorldNPCSpawner) {
	worldpkg.SeedInitialNPCs(spawner)
}

// SpawnExtraGoblins distributes extra goblins around the map perimeter.
func SpawnExtraGoblins(spawner WorldNPCSpawner, count int) {
	worldpkg.SpawnExtraGoblins(spawner, count)
}

// SpawnExtraRats distributes extra rats through the central spawn area.
func SpawnExtraRats(spawner WorldNPCSpawner, count int) {
	worldpkg.SpawnExtraRats(spawner, count)
}
