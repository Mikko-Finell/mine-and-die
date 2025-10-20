package server

import worldpkg "mine-and-die/server/internal/world"

const (
	navCellSize               = worldpkg.NavCellSize
	pathNodeReachedEpsilon    = worldpkg.PathNodeReachedEpsilon
	pathPushRecalcThreshold   = worldpkg.PathPushRecalcThreshold
	pathStallThresholdTicks   = worldpkg.PathStallThresholdTicks
	pathRecalcCooldownTicks   = worldpkg.PathRecalcCooldownTicks
	defaultPlayerArriveRadius = worldpkg.DefaultPlayerArriveRadius
)
