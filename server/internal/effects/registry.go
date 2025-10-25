package effects

import runtime "mine-and-die/server/internal/effects/runtime"

type (
	Registry     = runtime.Registry
	SpatialIndex = runtime.SpatialIndex
)

var (
	RegisterEffect   = runtime.RegisterEffect
	UnregisterEffect = runtime.UnregisterEffect
	FindByID         = runtime.FindByID
	PruneExpired     = runtime.PruneExpired
)

func NewSpatialIndex(cellSize float64, maxPerCell int) *runtime.SpatialIndex {
	return runtime.NewSpatialIndex(cellSize, maxPerCell)
}

const (
	DefaultSpatialCellSize   = runtime.DefaultSpatialCellSize
	DefaultSpatialMaxPerCell = runtime.DefaultSpatialMaxPerCell
)
