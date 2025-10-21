package world

import (
	"math"
	"testing"
)

func TestSetGroundItemPositionUpdatesCoordinates(t *testing.T) {
	x := 1.0
	y := 2.0

	if !SetGroundItemPosition(&x, &y, 3, 4) {
		t.Fatalf("expected position mutation to be applied")
	}

	if x != 3 || y != 4 {
		t.Fatalf("expected coordinates (3,4), got (%.2f, %.2f)", x, y)
	}
}

func TestSetGroundItemPositionSkipsWhenUnchanged(t *testing.T) {
	x := 5.0
	y := 6.0

	if SetGroundItemPosition(&x, &y, 5, 6) {
		t.Fatalf("expected mutation to be ignored when coordinates match")
	}
}

func TestSetGroundItemPositionNilPointers(t *testing.T) {
	if SetGroundItemPosition(nil, nil, 1, 2) {
		t.Fatalf("expected mutation to fail for nil coordinates")
	}
}

func TestSetGroundItemQuantityUpdatesValue(t *testing.T) {
	qty := 3

	if !SetGroundItemQuantity(&qty, 5) {
		t.Fatalf("expected quantity mutation to be applied")
	}

	if qty != 5 {
		t.Fatalf("expected quantity to equal 5, got %d", qty)
	}
}

func TestSetGroundItemQuantityClampsNegative(t *testing.T) {
	qty := 7

	if !SetGroundItemQuantity(&qty, -4) {
		t.Fatalf("expected mutation to be applied when clamping negative values")
	}

	if qty != 0 {
		t.Fatalf("expected quantity to clamp to 0, got %d", qty)
	}
}

func TestSetGroundItemQuantitySkipsWhenUnchanged(t *testing.T) {
	qty := 9

	if SetGroundItemQuantity(&qty, 9) {
		t.Fatalf("expected mutation to be ignored when quantity is unchanged")
	}
}

func TestSetGroundItemQuantityNilPointer(t *testing.T) {
	if SetGroundItemQuantity(nil, 1) {
		t.Fatalf("expected mutation to fail for nil quantity pointer")
	}
}

func TestScatterGroundItemPositionDefaultsToTileCenterWhenActorMissing(t *testing.T) {
	cfg := ScatterConfig{TileSize: 10, Padding: 1, MinDistance: 0.5, MaxDistance: 2}
	tile := GroundTileKey{X: 3, Y: 4}

	x, y := ScatterGroundItemPosition(nil, tile, cfg, nil, nil)

	expectedX, expectedY := TileCenter(tile, cfg.TileSize)
	if x != expectedX || y != expectedY {
		t.Fatalf("expected tile center (%.2f, %.2f), got (%.2f, %.2f)", expectedX, expectedY, x, y)
	}
}

func TestScatterGroundItemPositionClampsToTilePadding(t *testing.T) {
	cfg := ScatterConfig{TileSize: 20, Padding: 2, MinDistance: 0, MaxDistance: 5}
	tile := GroundTileKey{X: 0, Y: 0}
	actor := &Actor{X: 0, Y: 0}

	angle := math.Pi / 4
	distance := 100.0

	x, y := ScatterGroundItemPosition(actor, tile, cfg, func() float64 { return angle }, func(_, _ float64) float64 { return distance })

	if x < cfg.Padding || x > cfg.TileSize-cfg.Padding || y < cfg.Padding || y > cfg.TileSize-cfg.Padding {
		t.Fatalf("expected scattered position within tile padding, got (%.2f, %.2f)", x, y)
	}
}

func TestUpsertGroundItemCreatesNewEntry(t *testing.T) {
	items := make(map[string]*GroundItemState)
	byTile := make(map[GroundTileKey]map[string]*GroundItemState)
	var nextID uint64

	actor := &Actor{ID: "player-1", X: 15, Y: 25}
	stack := ItemStack{Type: "gold", Quantity: 3}
	cfg := ScatterConfig{TileSize: 10, MinDistance: 0, MaxDistance: 0, Padding: 0}

	ensured := false
	created := UpsertGroundItem(
		items,
		byTile,
		&nextID,
		actor,
		stack,
		"test",
		cfg,
		func() float64 { return 0 },
		func(_, _ float64) float64 { return 0 },
		func(s *ItemStack) bool {
			ensured = true
			s.FungibilityKey = "gold-key"
			return true
		},
		func(item *GroundItemState, qty int) { item.Qty = qty },
		func(item *GroundItemState, x, y float64) { item.X, item.Y = x, y },
		nil,
	)

	if created == nil {
		t.Fatalf("expected ground item to be created")
	}
	if !ensured {
		t.Fatalf("expected ensureKey callback to run")
	}
	if created.FungibilityKey != "gold-key" {
		t.Fatalf("expected fungibility key to be applied, got %q", created.FungibilityKey)
	}
	tile := TileForPosition(actor.X, actor.Y, cfg.TileSize)
	if created.Tile != tile {
		t.Fatalf("expected tile %#v, got %#v", tile, created.Tile)
	}
	if len(byTile[tile]) != 1 {
		t.Fatalf("expected tile index to contain new item")
	}
}

func TestUpsertGroundItemMergesExistingStack(t *testing.T) {
	tile := GroundTileKey{X: 0, Y: 0}
	existing := &GroundItemState{GroundItem: GroundItem{ID: "ground-1", Type: "gold", FungibilityKey: "gold-key", Qty: 5}}
	existing.Tile = tile

	items := map[string]*GroundItemState{"ground-1": existing}
	byTile := map[GroundTileKey]map[string]*GroundItemState{tile: {"gold-key": existing}}
	var nextID uint64 = 1

	actor := &Actor{ID: "player-1", X: 2, Y: 3}
	stack := ItemStack{Type: "gold", FungibilityKey: "gold-key", Quantity: 4}
	cfg := ScatterConfig{TileSize: 10}

	merged := UpsertGroundItem(
		items,
		byTile,
		&nextID,
		actor,
		stack,
		"merge",
		cfg,
		func() float64 { return 0 },
		func(_, _ float64) float64 { return 0 },
		nil,
		func(item *GroundItemState, qty int) { item.Qty = qty },
		func(item *GroundItemState, x, y float64) { item.X, item.Y = x, y },
		nil,
	)

	if merged != existing {
		t.Fatalf("expected existing stack to be returned")
	}
	if merged.Qty != 9 {
		t.Fatalf("expected merged quantity 9, got %d", merged.Qty)
	}
	if nextID != 1 {
		t.Fatalf("expected nextID to remain unchanged, got %d", nextID)
	}
}

func TestUpsertGroundItemReturnsNilWhenFungibilityMissing(t *testing.T) {
	items := make(map[string]*GroundItemState)
	byTile := make(map[GroundTileKey]map[string]*GroundItemState)
	var nextID uint64

	actor := &Actor{ID: "npc-1", X: 1, Y: 1}
	stack := ItemStack{Type: "mystery", Quantity: 1}
	cfg := ScatterConfig{TileSize: 5}

	created := UpsertGroundItem(
		items,
		byTile,
		&nextID,
		actor,
		stack,
		"unknown",
		cfg,
		nil,
		nil,
		func(*ItemStack) bool { return false },
		nil,
		nil,
		nil,
	)

	if created != nil {
		t.Fatalf("expected upsert to fail when fungibility key cannot be resolved")
	}
	if len(items) != 0 {
		t.Fatalf("expected item map to remain empty when upsert fails")
	}
	if len(byTile) != 0 {
		tile := TileForPosition(actor.X, actor.Y, cfg.TileSize)
		if len(byTile[tile]) != 0 {
			t.Fatalf("expected tile index to remain empty when upsert fails")
		}
	}
}

func TestRemoveGroundItemClearsStoreAndTile(t *testing.T) {
	tile := GroundTileKey{X: 2, Y: 3}
	item := &GroundItemState{GroundItem: GroundItem{ID: "ground-5", Type: "gold", FungibilityKey: "gold-key", Qty: 7}, Tile: tile}

	items := map[string]*GroundItemState{item.ID: item}
	byTile := map[GroundTileKey]map[string]*GroundItemState{tile: {"gold-key": item}}

	RemoveGroundItem(items, byTile, item, func(target *GroundItemState, qty int) { target.Qty = qty })

	if len(items) != 0 {
		t.Fatalf("expected items map to be empty after removal")
	}
	if len(byTile) != 0 {
		t.Fatalf("expected tile index to be cleared after removal")
	}
	if item.Qty != 0 {
		t.Fatalf("expected removal to zero the quantity, got %d", item.Qty)
	}
}

func TestGroundItemsSnapshotReturnsSortedCopy(t *testing.T) {
	items := map[string]*GroundItemState{
		"ground-b": {GroundItem: GroundItem{ID: "ground-b", Qty: 1}},
		"ground-a": {GroundItem: GroundItem{ID: "ground-a", Qty: 2}},
		"nil":      nil,
	}

	snapshot := GroundItemsSnapshot(items)

	if len(snapshot) != 2 {
		t.Fatalf("expected snapshot to contain 2 items, got %d", len(snapshot))
	}
	if snapshot[0].ID != "ground-a" || snapshot[1].ID != "ground-b" {
		t.Fatalf("expected snapshot order [ground-a ground-b], got [%s %s]", snapshot[0].ID, snapshot[1].ID)
	}

	snapshot[0].Qty = 99
	if items["ground-a"].Qty != 2 {
		t.Fatalf("expected snapshot to copy values, got %d", items["ground-a"].Qty)
	}
}

func TestGroundItemsSnapshotHandlesEmptyInput(t *testing.T) {
	if snapshot := GroundItemsSnapshot(nil); len(snapshot) != 0 {
		t.Fatalf("expected nil input to produce empty slice, got %d", len(snapshot))
	}

	if snapshot := GroundItemsSnapshot(map[string]*GroundItemState{}); len(snapshot) != 0 {
		t.Fatalf("expected empty map to produce empty slice, got %d", len(snapshot))
	}
}
