package server

import worldpkg "mine-and-die/server/internal/world"

type navGrid struct {
	*worldpkg.NavigationGrid
	cols     int
	rows     int
	cellSize float64
}

func newNavGrid(obstacles []Obstacle, width, height float64) *navGrid {
	grid := worldpkg.NewNavigationGrid(obstacles, width, height)
	if grid == nil {
		return nil
	}
	return &navGrid{
		NavigationGrid: grid,
		cols:           grid.Cols(),
		rows:           grid.Rows(),
		cellSize:       grid.CellSize(),
	}
}

func (g *navGrid) worldPos(col, row int) vec2 {
	if g == nil {
		return vec2{}
	}
	pos := g.NavigationGrid.WorldPos(col, row)
	return vec2{X: pos.X, Y: pos.Y}
}
