package world

import (
	"container/heap"
	"math"
)

const (
	NavCellSize             = 32.0
	PathNodeReachedEpsilon  = PlayerHalf * 0.75
	PathPushRecalcThreshold = 4.0
	PathStallThresholdTicks = 6
	PathRecalcCooldownTicks = 8
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

type navGrid struct {
	cols, rows int
	cellSize   float64
	walkable   []bool
	width      float64
	height     float64
}

func newNavGrid(obstacles []Obstacle, width, height float64) *navGrid {
	cols := int(math.Ceil(width / NavCellSize))
	rows := int(math.Ceil(height / NavCellSize))
	if cols <= 0 {
		cols = 1
	}
	if rows <= 0 {
		rows = 1
	}
	grid := &navGrid{
		cols:     cols,
		rows:     rows,
		cellSize: NavCellSize,
		walkable: make([]bool, cols*rows),
		width:    width,
		height:   height,
	}

	for row := 0; row < rows; row++ {
		for col := 0; col < cols; col++ {
			cx := (float64(col) + 0.5) * grid.cellSize
			cy := (float64(row) + 0.5) * grid.cellSize
			if cx < PlayerHalf || cx > width-PlayerHalf || cy < PlayerHalf || cy > height-PlayerHalf {
				continue
			}
			blocked := false
			for _, obs := range obstacles {
				if obs.Type == ObstacleTypeLava {
					continue
				}
				if CircleRectOverlap(cx, cy, PlayerHalf, obs) {
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

func (g *navGrid) worldPos(col, row int) Vec2 {
	return Vec2{
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
	clampedX := Clamp(x, 0, maxX)
	clampedY := Clamp(y, 0, maxY)
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

func (g *navGrid) findPath(start Vec2, target Vec2, blocked map[int]struct{}) ([]Vec2, bool) {
	if g == nil {
		return nil, false
	}
	startCol, startRow, ok := g.locate(start.X, start.Y)
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
	startPoint := navPoint{col: startCol, row: startRow}
	goalPoint := navPoint{col: goalCol, row: goalRow}
	nodes, ok := g.astar(startPoint, goalPoint, blocked)
	if !ok || len(nodes) == 0 {
		return nil, false
	}
	if len(nodes) == 1 {
		return []Vec2{target}, true
	}
	path := make([]Vec2, 0, len(nodes))
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

func pathTravelCost(start Vec2, path []Vec2) float64 {
	if len(path) == 0 {
		return 0
	}
	cost := 0.0
	prevX := start.X
	prevY := start.Y
	for _, node := range path {
		cost += math.Hypot(node.X-prevX, node.Y-prevY)
		prevX = node.X
		prevY = node.Y
	}
	return cost
}

func buildDynamicBlockers(grid *navGrid, blockers []Vec2) map[int]struct{} {
	if grid == nil || len(blockers) == 0 {
		return nil
	}
	blocked := make(map[int]struct{})
	mark := func(x, y float64) {
		minCol := int(math.Floor((x - PlayerHalf) / grid.cellSize))
		maxCol := int(math.Ceil((x + PlayerHalf) / grid.cellSize))
		minRow := int(math.Floor((y - PlayerHalf) / grid.cellSize))
		maxRow := int(math.Ceil((y + PlayerHalf) / grid.cellSize))
		for row := minRow; row <= maxRow; row++ {
			for col := minCol; col <= maxCol; col++ {
				if !grid.inBounds(col, row) {
					continue
				}
				idx := grid.index(col, row)
				if !grid.walkable[idx] {
					continue
				}
				cx := (float64(col) + 0.5) * grid.cellSize
				cy := (float64(row) + 0.5) * grid.cellSize
				if math.Hypot(cx-x, cy-y) <= PlayerHalf {
					blocked[idx] = struct{}{}
				}
			}
		}
	}
	for _, pos := range blockers {
		mark(pos.X, pos.Y)
	}
	if len(blocked) == 0 {
		return nil
	}
	return blocked
}

// ComputePathRequest captures the inputs required to reproduce the legacy
// navigation behaviour.
type ComputePathRequest struct {
	Start     Vec2
	Target    Vec2
	Width     float64
	Height    float64
	Obstacles []Obstacle
	Blockers  []Vec2
}

// ComputePathFrom reproduces the legacy pathfinding helpers used by the world
// package while remaining agnostic of the legacy world implementation.
func ComputePathFrom(req ComputePathRequest) ([]Vec2, Vec2, bool) {
	grid := newNavGrid(req.Obstacles, req.Width, req.Height)
	if grid == nil {
		return nil, Vec2{}, false
	}
	target := Vec2{
		X: Clamp(req.Target.X, PlayerHalf, req.Width-PlayerHalf),
		Y: Clamp(req.Target.Y, PlayerHalf, req.Height-PlayerHalf),
	}
	blocked := buildDynamicBlockers(grid, req.Blockers)
	path, ok := grid.findPath(req.Start, target, blocked)
	if ok {
		return append([]Vec2(nil), path...), target, true
	}
	step := grid.cellSize
	offsets := []Vec2{
		{X: step, Y: 0},
		{X: -step, Y: 0},
		{X: 0, Y: step},
		{X: 0, Y: -step},
		{X: step, Y: step},
		{X: step, Y: -step},
		{X: -step, Y: step},
		{X: -step, Y: -step},
		{X: 2 * step, Y: 0},
		{X: -2 * step, Y: 0},
		{X: 0, Y: 2 * step},
		{X: 0, Y: -2 * step},
	}
	bestScore := math.MaxFloat64
	var bestPath []Vec2
	var bestGoal Vec2
	for _, offset := range offsets {
		alt := Vec2{
			X: Clamp(target.X+offset.X, PlayerHalf, req.Width-PlayerHalf),
			Y: Clamp(target.Y+offset.Y, PlayerHalf, req.Height-PlayerHalf),
		}
		if math.Hypot(alt.X-target.X, alt.Y-target.Y) < 1 {
			continue
		}
		candidate, ok := grid.findPath(req.Start, alt, blocked)
		if !ok {
			continue
		}
		score := math.Hypot(alt.X-target.X, alt.Y-target.Y) + pathTravelCost(req.Start, candidate)
		if score < bestScore {
			bestScore = score
			bestGoal = alt
			bestPath = append([]Vec2(nil), candidate...)
		}
	}
	if len(bestPath) == 0 {
		return nil, Vec2{}, false
	}
	return bestPath, bestGoal, true
}

// PathActor captures the minimal information required to treat an entity as a
// navigation blocker when computing a path.
type PathActor struct {
	ID       string
	Position Vec2
}

// DynamicBlockerPositions converts the provided actor set into positions while
// skipping the ignored actor identifier. The returned slice mirrors the legacy
// world helper by producing nil when no blockers remain after filtering.
func DynamicBlockerPositions(actors []PathActor, ignoreID string) []Vec2 {
	if len(actors) == 0 {
		return nil
	}
	positions := make([]Vec2, 0, len(actors))
	for _, actor := range actors {
		if actor.ID == ignoreID {
			continue
		}
		positions = append(positions, actor.Position)
	}
	if len(positions) == 0 {
		return nil
	}
	return positions
}

// ConvertPath clones the provided path so callers receive a slice with
// independent backing storage, matching the historical server helper.
func ConvertPath(path []Vec2) []Vec2 {
	if len(path) == 0 {
		return nil
	}
	cloned := make([]Vec2, len(path))
	copy(cloned, path)
	return cloned
}

// ComputeNavigationPath mirrors the legacy world helper that assembled a path
// request from the active world snapshot, including dynamic blockers.
func ComputeNavigationPath(req ComputePathRequest, actors []PathActor, ignoreID string) ([]Vec2, Vec2, bool) {
	req.Blockers = DynamicBlockerPositions(actors, ignoreID)
	path, goal, ok := ComputePathFrom(req)
	if !ok {
		return nil, Vec2{}, false
	}
	return ConvertPath(path), goal, true
}

// NavigationGrid exposes a minimal view of the legacy navigation grid for
// callers that need to inspect the generated layout (primarily tests during the
// migration).
type NavigationGrid struct {
	grid *navGrid
}

// NewNavigationGrid constructs a navigation grid using the legacy parameters.
func NewNavigationGrid(obstacles []Obstacle, width, height float64) *NavigationGrid {
	grid := newNavGrid(obstacles, width, height)
	if grid == nil {
		return nil
	}
	return &NavigationGrid{grid: grid}
}

// Cols reports the number of columns in the grid.
func (g *NavigationGrid) Cols() int {
	if g == nil || g.grid == nil {
		return 0
	}
	return g.grid.cols
}

// Rows reports the number of rows in the grid.
func (g *NavigationGrid) Rows() int {
	if g == nil || g.grid == nil {
		return 0
	}
	return g.grid.rows
}

// CellSize reports the size of each navigation cell in world units.
func (g *NavigationGrid) CellSize() float64 {
	if g == nil || g.grid == nil {
		return 0
	}
	return g.grid.cellSize
}

// WorldPos reports the world coordinates corresponding to the provided grid
// indices.
func (g *NavigationGrid) WorldPos(col, row int) Vec2 {
	if g == nil || g.grid == nil {
		return Vec2{}
	}
	return g.grid.worldPos(col, row)
}

func (g *NavigationGrid) underlying() *navGrid {
	if g == nil {
		return nil
	}
	return g.grid
}
