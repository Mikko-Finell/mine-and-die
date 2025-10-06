package main

import (
	"container/heap"
	"math"
)

const navCellSize = 20.0

type navGrid struct {
	cellSize float64
	width    int
	height   int
	walkable []bool
}

func newNavGrid(cellSize float64, obstacles []Obstacle) *navGrid {
	if cellSize <= 0 {
		cellSize = playerHalf
	}
	width := int(math.Ceil(worldWidth / cellSize))
	height := int(math.Ceil(worldHeight / cellSize))
	if width <= 0 || height <= 0 {
		return nil
	}
	walkable := make([]bool, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			idx := y*width + x
			cx := (float64(x) + 0.5) * cellSize
			cy := (float64(y) + 0.5) * cellSize
			walkable[idx] = isNavCellWalkable(cx, cy, obstacles)
		}
	}
	return &navGrid{cellSize: cellSize, width: width, height: height, walkable: walkable}
}

func isNavCellWalkable(cx, cy float64, obstacles []Obstacle) bool {
	if cx < playerHalf || cy < playerHalf || cx > worldWidth-playerHalf || cy > worldHeight-playerHalf {
		return false
	}
	for _, obs := range obstacles {
		if obs.Type == obstacleTypeLava {
			continue
		}
		if circleRectOverlap(cx, cy, playerHalf, obs) {
			return false
		}
	}
	return true
}

func (g *navGrid) inBounds(x, y int) bool {
	return g != nil && x >= 0 && y >= 0 && x < g.width && y < g.height
}

func (g *navGrid) index(x, y int) int {
	return y*g.width + x
}

func (g *navGrid) isWalkable(x, y int) bool {
	if !g.inBounds(x, y) {
		return false
	}
	return g.walkable[g.index(x, y)]
}

func (g *navGrid) worldToCell(x, y float64) (int, int, bool) {
	if g == nil || x < 0 || y < 0 || x > worldWidth || y > worldHeight {
		return 0, 0, false
	}
	cx := int(x / g.cellSize)
	cy := int(y / g.cellSize)
	if cx < 0 {
		cx = 0
	}
	if cy < 0 {
		cy = 0
	}
	if cx >= g.width {
		cx = g.width - 1
	}
	if cy >= g.height {
		cy = g.height - 1
	}
	return cx, cy, true
}

func (g *navGrid) cellCenter(x, y int) vec2 {
	return vec2{
		X: (float64(x) + 0.5) * g.cellSize,
		Y: (float64(y) + 0.5) * g.cellSize,
	}
}

type navNeighbor struct {
	x, y int
	cost float64
}

func (g *navGrid) neighbors(x, y int) []navNeighbor {
	if g == nil {
		return nil
	}
	directions := []struct {
		dx, dy int
		cost   float64
		diag   bool
	}{
		{dx: 0, dy: -1, cost: g.cellSize},
		{dx: 1, dy: 0, cost: g.cellSize},
		{dx: 0, dy: 1, cost: g.cellSize},
		{dx: -1, dy: 0, cost: g.cellSize},
		{dx: 1, dy: -1, cost: g.cellSize * math.Sqrt2, diag: true},
		{dx: 1, dy: 1, cost: g.cellSize * math.Sqrt2, diag: true},
		{dx: -1, dy: 1, cost: g.cellSize * math.Sqrt2, diag: true},
		{dx: -1, dy: -1, cost: g.cellSize * math.Sqrt2, diag: true},
	}
	neighbors := make([]navNeighbor, 0, len(directions))
	for _, dir := range directions {
		nx := x + dir.dx
		ny := y + dir.dy
		if !g.inBounds(nx, ny) {
			continue
		}
		if !g.isWalkable(nx, ny) {
			continue
		}
		if dir.diag {
			if !g.isWalkable(x, ny) || !g.isWalkable(nx, y) {
				continue
			}
		}
		neighbors = append(neighbors, navNeighbor{x: nx, y: ny, cost: dir.cost})
	}
	return neighbors
}

type pathNode struct {
	x, y  int
	g     float64
	h     float64
	f     float64
	index int
	prev  *pathNode
}

type nodeHeap []*pathNode

func (h nodeHeap) Len() int { return len(h) }

func (h nodeHeap) Less(i, j int) bool {
	if h[i].f == h[j].f {
		return h[i].h < h[j].h
	}
	return h[i].f < h[j].f
}

func (h nodeHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *nodeHeap) Push(x any) {
	node := x.(*pathNode)
	node.index = len(*h)
	*h = append(*h, node)
}

func (h *nodeHeap) Pop() any {
	old := *h
	n := len(old)
	node := old[n-1]
	node.index = -1
	*h = old[0 : n-1]
	return node
}

func (g *navGrid) heuristic(x1, y1, x2, y2 int) float64 {
	dx := float64(x1 - x2)
	dy := float64(y1 - y2)
	return math.Hypot(dx, dy) * g.cellSize
}

func (g *navGrid) FindPath(start, goal vec2) []vec2 {
	if g == nil {
		return nil
	}
	sx, sy, ok := g.worldToCell(start.X, start.Y)
	if !ok {
		return nil
	}
	gx, gy, ok := g.worldToCell(goal.X, goal.Y)
	if !ok {
		return nil
	}
	if !g.isWalkable(gx, gy) {
		return nil
	}
	startNode := &pathNode{x: sx, y: sy, g: 0, h: g.heuristic(sx, sy, gx, gy)}
	startNode.f = startNode.h
	startNode.index = -1
	open := &nodeHeap{startNode}
	heap.Init(open)
	gScore := map[int]float64{g.index(sx, sy): 0}
	nodes := map[int]*pathNode{g.index(sx, sy): startNode}
	closed := make(map[int]struct{})

	for open.Len() > 0 {
		current := heap.Pop(open).(*pathNode)
		if current.x == gx && current.y == gy {
			return g.buildPath(current, goal)
		}
		closed[g.index(current.x, current.y)] = struct{}{}
		for _, nb := range g.neighbors(current.x, current.y) {
			key := g.index(nb.x, nb.y)
			if _, seen := closed[key]; seen {
				continue
			}
			tentative := current.g + nb.cost
			if existing, ok := gScore[key]; ok && tentative >= existing {
				continue
			}
			neighbor := nodes[key]
			if neighbor == nil {
				neighbor = &pathNode{x: nb.x, y: nb.y, index: -1}
				nodes[key] = neighbor
			}
			neighbor.prev = current
			neighbor.g = tentative
			neighbor.h = g.heuristic(nb.x, nb.y, gx, gy)
			neighbor.f = neighbor.g + neighbor.h
			gScore[key] = tentative
			if neighbor.index >= 0 {
				heap.Fix(open, neighbor.index)
			} else {
				heap.Push(open, neighbor)
			}
		}
	}
	return nil
}

func (g *navGrid) buildPath(goal *pathNode, target vec2) []vec2 {
	if g == nil || goal == nil {
		return nil
	}
	cells := make([]vec2, 0)
	node := goal
	for node != nil {
		cells = append(cells, g.cellCenter(node.x, node.y))
		node = node.prev
	}
	if len(cells) == 0 {
		return nil
	}
	for i, j := 0, len(cells)-1; i < j; i, j = i+1, j-1 {
		cells[i], cells[j] = cells[j], cells[i]
	}
	if len(cells) > 0 {
		cells[len(cells)-1] = target
	}
	if len(cells) > 1 {
		cells = cells[1:]
	}
	if len(cells) == 0 {
		return []vec2{target}
	}
	return cells
}

func (g *navGrid) nearestWalkable(goal vec2, maxRadius int) (vec2, bool) {
	if g == nil || maxRadius <= 0 {
		return vec2{}, false
	}
	gx, gy, ok := g.worldToCell(goal.X, goal.Y)
	if !ok {
		gx = int(clamp(goal.X/g.cellSize, 0, float64(g.width-1)))
		gy = int(clamp(goal.Y/g.cellSize, 0, float64(g.height-1)))
	}
	bestDist := math.MaxFloat64
	best := vec2{}
	found := false
	for radius := 1; radius <= maxRadius; radius++ {
		for dy := -radius; dy <= radius; dy++ {
			for dx := -radius; dx <= radius; dx++ {
				nx := gx + dx
				ny := gy + dy
				if !g.inBounds(nx, ny) {
					continue
				}
				if !g.isWalkable(nx, ny) {
					continue
				}
				center := g.cellCenter(nx, ny)
				dist := math.Hypot(center.X-goal.X, center.Y-goal.Y)
				if dist < bestDist {
					bestDist = dist
					best = center
					found = true
				}
			}
		}
		if found {
			break
		}
	}
	return best, found
}
