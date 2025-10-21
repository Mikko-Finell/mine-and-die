package effects

import "math"

// SpatialCellKey identifies a grid cell occupied by an effect bounding box.
type SpatialCellKey struct {
	X int
	Y int
}

// spatialEntry tracks the cells occupied by an effect instance.
type spatialEntry struct {
	cells []SpatialCellKey
}

const (
	// DefaultSpatialCellSize mirrors the legacy tile size used for effects.
	DefaultSpatialCellSize = 40.0
	// DefaultSpatialMaxPerCell mirrors the historical per-cell capacity guard.
	DefaultSpatialMaxPerCell = 16
	// SpatialMinExtentFraction clamps narrow bounds to ensure occupancy.
	SpatialMinExtentFraction = 0.25
)

// SpatialIndex maintains the grid occupancy bookkeeping for effect instances.
type SpatialIndex struct {
	cellSize    float64
	invCellSize float64
	maxPerCell  int
	cells       map[SpatialCellKey][]string
	entries     map[string]*spatialEntry
}

// NewSpatialIndex constructs a SpatialIndex with optional custom parameters.
func NewSpatialIndex(cellSize float64, maxPerCell int) *SpatialIndex {
	if cellSize <= 0 {
		cellSize = DefaultSpatialCellSize
	}
	if maxPerCell <= 0 {
		maxPerCell = DefaultSpatialMaxPerCell
	}
	return &SpatialIndex{
		cellSize:    cellSize,
		invCellSize: 1.0 / cellSize,
		maxPerCell:  maxPerCell,
		cells:       make(map[SpatialCellKey][]string),
		entries:     make(map[string]*spatialEntry),
	}
}

// Upsert inserts or updates an effect's occupied cells, returning false when
// the operation would exceed the configured cell capacity limits.
func (idx *SpatialIndex) Upsert(effect *State) bool {
	if idx == nil || effect == nil || effect.ID == "" {
		return true
	}

	entry, existed := idx.entries[effect.ID]
	newCells := idx.cellsForEffect(effect)
	if idx.maxPerCell > 0 {
		var existingCellCounts map[SpatialCellKey]int
		if existed {
			existingCellCounts = make(map[SpatialCellKey]int, len(entry.cells))
			for _, cell := range entry.cells {
				existingCellCounts[cell]++
			}
		}

		checked := make(map[SpatialCellKey]struct{}, len(newCells))
		for _, cell := range newCells {
			if _, seen := checked[cell]; seen {
				continue
			}
			checked[cell] = struct{}{}

			bucket := idx.cells[cell]
			occupancy := len(bucket)
			if existed {
				if count := existingCellCounts[cell]; count > 0 {
					occupancy -= count
				}
			}
			if occupancy >= idx.maxPerCell {
				return false
			}
		}
	}

	if existed {
		idx.removeFromCells(effect.ID, entry.cells)
	}

	idx.entries[effect.ID] = &spatialEntry{cells: newCells}
	for _, cell := range newCells {
		bucket := idx.cells[cell]
		bucket = append(bucket, effect.ID)
		idx.cells[cell] = bucket
	}
	return true
}

// Remove deletes an effect from the spatial index.
func (idx *SpatialIndex) Remove(effectID string) {
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

func (idx *SpatialIndex) removeFromCells(effectID string, cells []SpatialCellKey) {
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

func (idx *SpatialIndex) cellsForEffect(effect *State) []SpatialCellKey {
	if idx == nil || effect == nil {
		return nil
	}
	minExtent := idx.cellSize * SpatialMinExtentFraction
	width := math.Abs(effect.Width)
	height := math.Abs(effect.Height)
	if width < minExtent {
		width = minExtent
	}
	if height < minExtent {
		height = minExtent
	}
	minX := idx.coordToCell(effect.X)
	minY := idx.coordToCell(effect.Y)
	maxX := idx.coordToCell(effect.X + width)
	maxY := idx.coordToCell(effect.Y + height)
	cellCount := (maxX - minX + 1) * (maxY - minY + 1)
	if cellCount <= 0 {
		cellCount = 1
	}
	cells := make([]SpatialCellKey, 0, cellCount)
	for row := minY; row <= maxY; row++ {
		for col := minX; col <= maxX; col++ {
			cells = append(cells, SpatialCellKey{X: col, Y: row})
		}
	}
	return cells
}

func (idx *SpatialIndex) coordToCell(value float64) int {
	if idx == nil {
		return 0
	}
	return int(math.Floor(value * idx.invCellSize))
}
