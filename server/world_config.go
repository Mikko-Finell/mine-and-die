package server

import worldpkg "mine-and-die/server/internal/world"

type worldConfig = worldpkg.Config

const defaultWorldSeed = worldpkg.DefaultSeed

func defaultWorldConfig() worldConfig {
	return worldpkg.DefaultConfig()
}
