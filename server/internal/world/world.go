package world

import (
	"math/rand"

	state "mine-and-die/server/internal/state"
	"mine-and-die/server/logging"
)

// RNGFactory produces deterministic RNG instances for world subsystems.
type RNGFactory func(rootSeed, label string) *rand.Rand

// Deps bundles runtime dependencies required to construct a World instance.
type Deps struct {
	Publisher logging.Publisher
	RNG       RNGFactory
}

// World owns the deterministic RNG root and configuration for the simulation.
type World struct {
	config Config
	seed   string

	publisher  logging.Publisher
	rngFactory RNGFactory
	rng        *rand.Rand

	players map[string]*state.PlayerState
	npcs    map[string]*state.NPCState
}

// New constructs a world instance with normalized configuration and seeded RNG.
func New(cfg Config, deps Deps) (*World, error) {
	normalized := cfg.normalized()

	factory := deps.RNG
	if factory == nil {
		factory = NewDeterministicRNG
	}

	publisher := deps.Publisher
	if publisher == nil {
		publisher = logging.NopPublisher{}
	}

	seed := normalized.Seed
	if seed == "" {
		seed = DefaultSeed
	}

	world := &World{
		config:     normalized,
		seed:       seed,
		publisher:  publisher,
		rngFactory: factory,
		rng:        factory(seed, "world"),
		players:    make(map[string]*state.PlayerState),
		npcs:       make(map[string]*state.NPCState),
	}

	return world, nil
}

// Config returns the normalized configuration captured at construction time.
func (w *World) Config() Config {
	if w == nil {
		return Config{}
	}
	return w.config
}

// Seed reports the deterministic seed applied to the world RNG hierarchy.
func (w *World) Seed() string {
	if w == nil {
		return ""
	}
	return w.seed
}

// RNG exposes the root RNG instance seeded for the world.
func (w *World) RNG() *rand.Rand {
	if w == nil {
		return nil
	}
	if w.rng == nil {
		w.rng = w.ensureFactory()(w.seed, "world")
	}
	return w.rng
}

// SubsystemRNG returns a deterministic RNG derived from the world seed.
func (w *World) SubsystemRNG(label string) *rand.Rand {
	if w == nil {
		return NewDeterministicRNG(DefaultSeed, label)
	}
	seed := w.seed
	if seed == "" {
		seed = DefaultSeed
	}
	return w.ensureFactory()(seed, label)
}

func (w *World) ensureFactory() RNGFactory {
	if w == nil || w.rngFactory == nil {
		return NewDeterministicRNG
	}
	return w.rngFactory
}
