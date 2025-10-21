package world

import (
	"fmt"
	"math"
	"sort"
)

// GroundTileKey identifies the map tile that currently contains a ground item stack.
type GroundTileKey struct {
	X int
	Y int
}

// GroundItem mirrors the legacy ground-item snapshot exposed to callers.
type GroundItem struct {
	ID             string  `json:"id"`
	Type           string  `json:"type"`
	FungibilityKey string  `json:"fungibility_key"`
	X              float64 `json:"x"`
	Y              float64 `json:"y"`
	Qty            int     `json:"qty"`
}

// GroundItemState tracks a ground item along with its tile metadata.
type GroundItemState struct {
	GroundItem
	Tile    GroundTileKey
	Version uint64
}

// Actor captures the minimal actor metadata required for ground item placement.
type Actor struct {
	ID string
	X  float64
	Y  float64
}

// ItemStack describes a fungible stack that can be moved to the ground.
type ItemStack struct {
	Type           string
	FungibilityKey string
	Quantity       int
}

// ScatterConfig carries the geometry parameters used when scattering items inside a tile.
type ScatterConfig struct {
	TileSize    float64
	MinDistance float64
	MaxDistance float64
	Padding     float64
}

// TileForPosition computes the grid coordinate for the provided point.
func TileForPosition(x, y, tileSize float64) GroundTileKey {
	return GroundTileKey{X: int(math.Floor(x / tileSize)), Y: int(math.Floor(y / tileSize))}
}

// TileCenter returns the midpoint for the provided tile.
func TileCenter(tile GroundTileKey, tileSize float64) (float64, float64) {
	return float64(tile.X)*tileSize + tileSize/2, float64(tile.Y)*tileSize + tileSize/2
}

// ScatterGroundItemPosition selects a deterministic position for a ground item inside the tile.
func ScatterGroundItemPosition(
	actor *Actor,
	tile GroundTileKey,
	cfg ScatterConfig,
	randomAngle func() float64,
	randomDistance func(min, max float64) float64,
) (float64, float64) {
	if actor == nil {
		return TileCenter(tile, cfg.TileSize)
	}

	angle := 0.0
	if randomAngle != nil {
		angle = randomAngle()
	}

	distance := cfg.MinDistance
	if randomDistance != nil {
		distance = randomDistance(cfg.MinDistance, cfg.MaxDistance)
	}

	baseX := actor.X
	baseY := actor.Y
	x := baseX + math.Cos(angle)*distance
	y := baseY + math.Sin(angle)*distance

	left := float64(tile.X) * cfg.TileSize
	top := float64(tile.Y) * cfg.TileSize
	right := left + cfg.TileSize
	bottom := top + cfg.TileSize

	padding := cfg.Padding
	if padding*2 >= cfg.TileSize {
		padding = 0
	}

	minX := left + padding
	maxX := right - padding
	minY := top + padding
	maxY := bottom - padding

	return clampFloat(x, minX, maxX), clampFloat(y, minY, maxY)
}

// UpsertGroundItem merges the provided stack into the store, creating a new entry when required.
// The ensureKey callback should populate the stack's fungibility key when missing, returning true on success.
// Setters and logDrop mirror the legacy world helpers so wrappers can record patches and telemetry.
func UpsertGroundItem(
	items map[string]*GroundItemState,
	itemsByTile map[GroundTileKey]map[string]*GroundItemState,
	nextID *uint64,
	actor *Actor,
	stack ItemStack,
	reason string,
	cfg ScatterConfig,
	randomAngle func() float64,
	randomDistance func(min, max float64) float64,
	ensureKey func(*ItemStack) bool,
	setQuantity func(*GroundItemState, int),
	setPosition func(*GroundItemState, float64, float64),
	logDrop func(*Actor, ItemStack, string, string),
) *GroundItemState {
	if items == nil || itemsByTile == nil || nextID == nil || actor == nil {
		return nil
	}
	if stack.Quantity <= 0 || stack.Type == "" {
		return nil
	}

	tile := TileForPosition(actor.X, actor.Y, cfg.TileSize)
	x, y := ScatterGroundItemPosition(actor, tile, cfg, randomAngle, randomDistance)

	itemsByType := itemsByTile[tile]
	if itemsByType == nil {
		itemsByType = make(map[string]*GroundItemState)
		itemsByTile[tile] = itemsByType
	}

	if stack.FungibilityKey == "" {
		if ensureKey == nil || !ensureKey(&stack) {
			return nil
		}
	}
	if stack.FungibilityKey == "" {
		return nil
	}

	if existing := itemsByType[stack.FungibilityKey]; existing != nil {
		if setQuantity != nil {
			setQuantity(existing, existing.Qty+stack.Quantity)
		} else {
			existing.Qty += stack.Quantity
		}
		existing.Tile = tile
		if setPosition != nil {
			setPosition(existing, x, y)
		} else {
			existing.X = x
			existing.Y = y
		}
		if logDrop != nil {
			logDrop(actor, stack, reason, existing.ID)
		}
		return existing
	}

	*nextID = *nextID + 1
	id := fmt.Sprintf("ground-%d", *nextID)

	item := &GroundItemState{
		GroundItem: GroundItem{
			ID:             id,
			Type:           stack.Type,
			FungibilityKey: stack.FungibilityKey,
			X:              x,
			Y:              y,
			Qty:            stack.Quantity,
		},
		Tile: tile,
	}

	items[id] = item
	itemsByType[stack.FungibilityKey] = item

	if logDrop != nil {
		logDrop(actor, stack, reason, id)
	}

	return item
}

// RemoveGroundItem deletes the provided ground item from the store and tile index.
func RemoveGroundItem(
	items map[string]*GroundItemState,
	itemsByTile map[GroundTileKey]map[string]*GroundItemState,
	item *GroundItemState,
	setQuantity func(*GroundItemState, int),
) {
	if items == nil || item == nil {
		return
	}

	if item.Qty > 0 {
		if setQuantity != nil {
			setQuantity(item, 0)
		} else {
			item.Qty = 0
		}
	}

	delete(items, item.ID)

	if itemsByTile == nil {
		return
	}

	if itemsByType, ok := itemsByTile[item.Tile]; ok {
		delete(itemsByType, item.FungibilityKey)
		if len(itemsByType) == 0 {
			delete(itemsByTile, item.Tile)
		}
	}
}

// SetGroundItemPosition updates the ground item's coordinates when they change.
// Returns true when the mutation was applied.
func SetGroundItemPosition(x, y *float64, newX, newY float64) bool {
	if x == nil || y == nil {
		return false
	}

	if PositionsEqual(*x, *y, newX, newY) {
		return false
	}

	*x = newX
	*y = newY
	return true
}

// SetGroundItemQuantity clamps the quantity to zero or greater and updates the
// stored value when it changes. Returns true when the mutation was applied.
func SetGroundItemQuantity(qty *int, newQty int) bool {
	if qty == nil {
		return false
	}

	if newQty < 0 {
		newQty = 0
	}

	if *qty == newQty {
		return false
	}

	*qty = newQty
	return true
}

func groundItemsSnapshot(items map[string]*GroundItemState) []GroundItem {
	if len(items) == 0 {
		return make([]GroundItem, 0)
	}

	snapshot := make([]GroundItem, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		snapshot = append(snapshot, item.GroundItem)
	}

	sort.Slice(snapshot, func(i, j int) bool {
		return snapshot[i].ID < snapshot[j].ID
	})

	return snapshot
}

// GroundItemsSnapshot returns a broadcast-friendly copy of the provided ground items.
func GroundItemsSnapshot(items map[string]*GroundItemState) []GroundItem {
	snapshot := groundItemsSnapshot(items)
	if snapshot == nil {
		return make([]GroundItem, 0)
	}
	return snapshot
}

func clampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
