package server

import internaleffects "mine-and-die/server/internal/effects"

type effectSpatialIndex = internaleffects.SpatialIndex

const (
	effectSpatialCellSize   = internaleffects.DefaultSpatialCellSize
	effectSpatialMaxPerCell = internaleffects.DefaultSpatialMaxPerCell
)

func newEffectSpatialIndex(cellSize float64, maxPerCell int) *effectSpatialIndex {
	return internaleffects.NewSpatialIndex(cellSize, maxPerCell)
}
