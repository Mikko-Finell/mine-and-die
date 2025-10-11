package main

import (
	"container/heap"
	"math"
)

type navNeighbor struct {
	col      int
	row      int
	cost     float64
	diagonal bool
}

var navNeighborOffsets = [...]navNeighbor{
	{col: 0, row: -1, cost: 1, diagonal: false},
	{col: 1, row: 0, cost: 1, diagonal: false},
	{col: 0, row: 1, cost: 1, diagonal: false},
	{col: -1, row: 0, cost: 1, diagonal: false},
	{col: 1, row: -1, cost: math.Sqrt2, diagonal: true},
	{col: 1, row: 1, cost: math.Sqrt2, diagonal: true},
	{col: -1, row: 1, cost: math.Sqrt2, diagonal: true},
	{col: -1, row: -1, cost: math.Sqrt2, diagonal: true},
}

const (
	navCellSize             = 32.0
	pathNodeReachedEpsilon  = playerHalf * 0.75
	pathPushRecalcThreshold = 4.0
	pathStallThresholdTicks = 6
	pathRecalcCooldownTicks = 8
)

type navGrid struct {
	cols, rows int
	cellSize   float64
	walkable   []bool
	width      float64
	height     float64
}

func newNavGrid(obstacles []Obstacle, width, height float64) *navGrid {
	cols := int(math.Ceil(width / navCellSize))
	rows := int(math.Ceil(height / navCellSize))
	if cols <= 0 {
		cols = 1
	}
	if rows <= 0 {
		rows = 1
	}
	grid := &navGrid{
		cols:     cols,
		rows:     rows,
		cellSize: navCellSize,
		walkable: make([]bool, cols*rows),
		width:    width,
		height:   height,
	}

	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			cx := (float64(col) + 0.5) * grid.cellSize
			cy := (float64(row) + 0.5) * grid.cellSize
			if cx < playerHalf || cx > width-playerHalf || cy < playerHalf || cy > height-playerHalf {
				continue
			}
			blocked := false
			for _, obs := range obstacles {
				if obs.Type == obstacleTypeLava {
					continue
				}
				if circleRectOverlap(cx, cy, playerHalf, obs) {
					blocked = true
					break
				}
			}
			if !blocked {
				grid.walkable[row*cols+col] = true
			}
		}
	}

	return grid
}

func (g *navGrid) inBounds(col, row int) bool {
	return g != nil && col >= 0 && row >= 0 && col < g.cols && row < g.rows
}

func (g *navGrid) index(col, row int) int {
	return row*g.cols + col
}

func (g *navGrid) isWalkable(col, row int) bool {
	if !g.inBounds(col, row) {
		return false
	}
	return g.walkable[g.index(col, row)]
}

func (g *navGrid) worldPos(col, row int) vec2 {
	return vec2{
		X: (float64(col) + 0.5) * g.cellSize,
		Y: (float64(row) + 0.5) * g.cellSize,
	}
}

func (g *navGrid) canTraverseDiagonal(current navPoint, delta navNeighbor, blocked map[int]struct{}) bool {
	if g == nil || !delta.diagonal {
		return true
	}
	horizCol := current.col + delta.col
	horizRow := current.row
	vertCol := current.col
	vertRow := current.row + delta.row
	if !g.inBounds(horizCol, horizRow) || !g.inBounds(vertCol, vertRow) {
		return false
	}
	if !g.walkable[g.index(horizCol, horizRow)] || !g.walkable[g.index(vertCol, vertRow)] {
		return false
	}
	if blocked == nil {
		return true
	}
	if _, exists := blocked[g.index(horizCol, horizRow)]; exists {
		return false
	}
	if _, exists := blocked[g.index(vertCol, vertRow)]; exists {
		return false
	}
	return true
}

func (g *navGrid) locate(x, y float64) (int, int, bool) {
	if g == nil || g.cols == 0 || g.rows == 0 {
		return 0, 0, false
	}
	maxX := g.width - 1
	if maxX < 0 {
		maxX = 0
	}
	maxY := g.height - 1
	if maxY < 0 {
		maxY = 0
	}
	clampedX := clamp(x, 0, maxX)
	clampedY := clamp(y, 0, maxY)
	col := int(clampedX / g.cellSize)
	row := int(clampedY / g.cellSize)
	if !g.inBounds(col, row) {
		return 0, 0, false
	}
	return col, row, true
}

func (g *navGrid) closestWalkable(col, row int, blocked map[int]struct{}) (int, int, bool) {
	if !g.inBounds(col, row) {
		return 0, 0, false
	}
	type node struct {
		col int
		row int
	}
	startIdx := g.index(col, row)
	visited := make(map[int]struct{})
	queue := []node{{col: col, row: row}}
	visited[startIdx] = struct{}{}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		idx := g.index(current.col, current.row)
		if g.walkable[idx] {
			if blocked == nil {
				return current.col, current.row, true
			}
			if _, exists := blocked[idx]; !exists {
				return current.col, current.row, true
			}
		}
		for _, delta := range navNeighborOffsets {
			nc := current.col + delta.col
			nr := current.row + delta.row
			if delta.diagonal && !g.canTraverseDiagonal(navPoint{col: current.col, row: current.row}, delta, blocked) {
				continue
			}
			if !g.inBounds(nc, nr) {
				continue
			}
			nIdx := g.index(nc, nr)
			if _, seen := visited[nIdx]; seen {
				continue
			}
			visited[nIdx] = struct{}{}
			queue = append(queue, node{col: nc, row: nr})
		}
	}
	return 0, 0, false
}

type navPoint struct {
	col int
	row int
}

func (g *navGrid) heuristic(a, b navPoint) float64 {
	dx := math.Abs(float64(a.col - b.col))
	dy := math.Abs(float64(a.row - b.row))
	if dx > dy {
		return dx + (math.Sqrt2-1)*dy
	}
	return dy + (math.Sqrt2-1)*dx
}

type pathNode struct {
	point  navPoint
	g      float64
	f      float64
	index  int
	parent *pathNode
}

type pathQueue []*pathNode

func (pq pathQueue) Len() int { return len(pq) }

func (pq pathQueue) Less(i, j int) bool { return pq[i].f < pq[j].f }

func (pq pathQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *pathQueue) Push(x any) {
	n := len(*pq)
	item := x.(*pathNode)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *pathQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[:n-1]
	return item
}

func (g *navGrid) astar(start, goal navPoint, blocked map[int]struct{}) ([]navPoint, bool) {
	open := &pathQueue{}
	heap.Init(open)
	startNode := &pathNode{point: start, g: 0, f: g.heuristic(start, goal)}
	heap.Push(open, startNode)
	gScore := map[int]float64{g.index(start.col, start.row): 0}
	closed := make(map[int]struct{})

	for open.Len() > 0 {
		current := heap.Pop(open).(*pathNode)
		currIdx := g.index(current.point.col, current.point.row)
		if _, seen := closed[currIdx]; seen {
			continue
		}
		closed[currIdx] = struct{}{}
		if current.point == goal {
			return reconstructPath(current), true
		}

		for _, delta := range navNeighborOffsets {
			if delta.diagonal && !g.canTraverseDiagonal(current.point, delta, blocked) {
				continue
			}
			nc := current.point.col + delta.col
			nr := current.point.row + delta.row
			if !g.inBounds(nc, nr) {
				continue
			}
			idx := g.index(nc, nr)
			if !g.walkable[idx] {
				continue
			}
			if blocked != nil {
				if _, blocked := blocked[idx]; blocked && !(nc == goal.col && nr == goal.row) {
					continue
				}
			}
			if _, seen := closed[idx]; seen {
				continue
			}
			tentativeG := current.g + delta.cost
			if prev, ok := gScore[idx]; ok && tentativeG >= prev {
				continue
			}
			gScore[idx] = tentativeG
			heap.Push(open, &pathNode{
				point:  navPoint{col: nc, row: nr},
				g:      tentativeG,
				f:      tentativeG + g.heuristic(navPoint{col: nc, row: nr}, goal),
				parent: current,
			})
		}
	}
	return nil, false
}

func reconstructPath(end *pathNode) []navPoint {
	if end == nil {
		return nil
	}
	path := make([]navPoint, 0)
	for node := end; node != nil; node = node.parent {
		path = append(path, node.point)
	}
	for i := 0; i < len(path)/2; i++ {
		j := len(path) - 1 - i
		path[i], path[j] = path[j], path[i]
	}
	return path
}

func (g *navGrid) findPath(startX, startY float64, target vec2, blocked map[int]struct{}) ([]vec2, bool) {
	if g == nil {
		return nil, false
	}
	startCol, startRow, ok := g.locate(startX, startY)
	if !ok {
		return nil, false
	}
	goalCol, goalRow, ok := g.locate(target.X, target.Y)
	if !ok {
		return nil, false
	}
	startIdx := g.index(startCol, startRow)
	goalIdx := g.index(goalCol, goalRow)
	if !g.walkable[startIdx] {
		sc, sr, ok := g.closestWalkable(startCol, startRow, blocked)
		if !ok {
			return nil, false
		}
		startCol, startRow = sc, sr
		startIdx = g.index(startCol, startRow)
	}
	if blocked != nil {
		if _, exists := blocked[startIdx]; exists {
			sc, sr, ok := g.closestWalkable(startCol, startRow, blocked)
			if !ok {
				return nil, false
			}
			startCol, startRow = sc, sr
			startIdx = g.index(startCol, startRow)
		}
	}
	if !g.walkable[goalIdx] {
		return nil, false
	}
	if blocked != nil {
		if _, exists := blocked[goalIdx]; exists {
			return nil, false
		}
	}
	start := navPoint{col: startCol, row: startRow}
	goal := navPoint{col: goalCol, row: goalRow}
	nodes, ok := g.astar(start, goal, blocked)
	if !ok || len(nodes) == 0 {
		return nil, false
	}
	if len(nodes) == 1 {
		return []vec2{target}, true
	}
	path := make([]vec2, 0, len(nodes))
	for i := 1; i < len(nodes); i++ {
		path = append(path, g.worldPos(nodes[i].col, nodes[i].row))
	}
	if len(path) == 0 {
		path = append(path, target)
		return path, true
	}
	last := path[len(path)-1]
	if math.Hypot(last.X-target.X, last.Y-target.Y) > 1 {
		path = append(path, target)
	} else {
		path[len(path)-1] = target
	}
	return path, true
}
