package server

import (
	worldpkg "mine-and-die/server/internal/world"
	"mine-and-die/server/logging"
)

// newTestWorld constructs a world instance for tests via the internal/world constructor.
func newTestWorld(cfg worldConfig, publisher logging.Publisher) *World {
	return requireLegacyWorld(worldpkg.ConstructLegacy(cfg, publisher))
}
