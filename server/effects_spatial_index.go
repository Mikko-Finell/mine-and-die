package main

import "math"

type effectCellKey struct {
	X int
	Y int
}

type effectSpatialEntry struct {
	cells []effectCellKey
}

const (
	effectSpatialCellSize      = tileSize
	effectSpatialMaxPerCell    = 16
	effectSpatialMinExtentFrac = 0.25
)

type effectSpatialIndex struct {
	cellSize    float64
	invCellSize float64
	maxPerCell  int
	cells       map[effectCellKey][]string
	entries     map[string]*effectSpatialEntry
}

func newEffectSpatialIndex(cellSize float64, maxPerCell int) *effectSpatialIndex {
	if cellSize <= 0 {
		cellSize = effectSpatialCellSize
	}
	if maxPerCell <= 0 {
		maxPerCell = effectSpatialMaxPerCell
	}
	return &effectSpatialIndex{
		cellSize:    cellSize,
		invCellSize: 1.0 / cellSize,
		maxPerCell:  maxPerCell,
		cells:       make(map[effectCellKey][]string),
		entries:     make(map[string]*effectSpatialEntry),
	}
}

func (idx *effectSpatialIndex) Upsert(effect *effectState) bool {
	if idx == nil || effect == nil || effect.ID == "" {
		return true
	}

	entry, existed := idx.entries[effect.ID]
	newCells := idx.cellsForEffect(effect)
	if !existed {
		if idx.maxPerCell > 0 {
			for _, cell := range newCells {
				bucket := idx.cells[cell]
				if len(bucket) >= idx.maxPerCell {
					return false
				}
			}
		}
	} else {
		idx.removeFromCells(effect.ID, entry.cells)
	}

	idx.entries[effect.ID] = &effectSpatialEntry{cells: newCells}
	for _, cell := range newCells {
		bucket := idx.cells[cell]
		bucket = append(bucket, effect.ID)
		idx.cells[cell] = bucket
	}
	return true
}

func (idx *effectSpatialIndex) Remove(effectID string) {
	if idx == nil || effectID == "" {
		return
	}
	entry, ok := idx.entries[effectID]
	if !ok {
		return
	}
	idx.removeFromCells(effectID, entry.cells)
	delete(idx.entries, effectID)
}

func (idx *effectSpatialIndex) removeFromCells(effectID string, cells []effectCellKey) {
	if idx == nil || effectID == "" || len(cells) == 0 {
		return
	}
	for _, cell := range cells {
		bucket := idx.cells[cell]
		if len(bucket) == 0 {
			continue
		}
		for i := range bucket {
			if bucket[i] != effectID {
				continue
			}
			bucket[i] = bucket[len(bucket)-1]
			bucket = bucket[:len(bucket)-1]
			break
		}
		if len(bucket) == 0 {
			delete(idx.cells, cell)
		} else {
			idx.cells[cell] = bucket
		}
	}
}

func (idx *effectSpatialIndex) cellsForEffect(effect *effectState) []effectCellKey {
	if idx == nil || effect == nil {
		return nil
	}
	minExtent := idx.cellSize * effectSpatialMinExtentFrac
	width := math.Abs(effect.Effect.Width)
	height := math.Abs(effect.Effect.Height)
	if width < minExtent {
		width = minExtent
	}
	if height < minExtent {
		height = minExtent
	}
	minX := idx.coordToCell(effect.Effect.X)
	minY := idx.coordToCell(effect.Effect.Y)
	maxX := idx.coordToCell(effect.Effect.X + width)
	maxY := idx.coordToCell(effect.Effect.Y + height)
	cellCount := (maxX - minX + 1) * (maxY - minY + 1)
	if cellCount <= 0 {
		cellCount = 1
	}
	cells := make([]effectCellKey, 0, cellCount)
	for row := minY; row <= maxY; row++ {
		for col := minX; col <= maxX; col++ {
			cells = append(cells, effectCellKey{X: col, Y: row})
		}
	}
	return cells
}

func (idx *effectSpatialIndex) coordToCell(value float64) int {
	if idx == nil {
		return 0
	}
	return int(math.Floor(value * idx.invCellSize))
}
