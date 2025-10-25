package effects

import worldeffects "mine-and-die/server/internal/world/effects"

type (
	Runtime       = worldeffects.Runtime
	HookFunc      = worldeffects.HookFunc
	HookSet       = worldeffects.HookSet
	ManagerConfig = worldeffects.ManagerConfig
	Manager       = worldeffects.Manager
)

var (
	NewManager = worldeffects.NewManager
)
