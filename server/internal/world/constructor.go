package world

import "mine-and-die/server/logging"

// LegacyWorld captures the transitional world instance while the simulation
// package is migrated into this package.
//
// TEMPORARY SHIM REMOVE WHEN DONE: delete once the world implementation
//
//	lives under internal/world and callers no longer require the legacy
//	world type.
type LegacyWorld interface {
	LegacyWorldMarker()
}

// Constructor builds a world instance using the legacy implementation.
type Constructor func(Config, logging.Publisher) LegacyWorld

var registeredConstructor Constructor

// RegisterLegacyConstructor installs the constructor used by New.
func RegisterLegacyConstructor(fn Constructor) {
	if fn == nil {
		panic("world: nil constructor")
	}
	registeredConstructor = fn
}

// New constructs a world using the registered legacy constructor.
func New(cfg Config, publisher logging.Publisher) LegacyWorld {
	if registeredConstructor == nil {
		panic("world: legacy constructor not registered")
	}
	normalized := cfg.Normalized()
	return registeredConstructor(normalized, publisher)
}
