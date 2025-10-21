package world

import (
	"errors"
	"math"
	"testing"
)

func TestNearestGroundItemReturnsClosestStack(t *testing.T) {
	items := map[string]*GroundItemState{
		"near": {
			GroundItem: GroundItem{ID: "near", Type: "gold", FungibilityKey: "gold-key", X: 3, Y: 4, Qty: 5},
		},
		"far": {
			GroundItem: GroundItem{ID: "far", Type: "gold", FungibilityKey: "gold-key", X: 6, Y: 8, Qty: 2},
		},
		"other": {
			GroundItem: GroundItem{ID: "other", Type: "wood", FungibilityKey: "wood-key", X: 1, Y: 1, Qty: 7},
		},
		"empty": {
			GroundItem: GroundItem{ID: "empty", Type: "gold", FungibilityKey: "gold-key", X: 10, Y: 10, Qty: 0},
		},
	}

	actor := &Actor{X: 0, Y: 0}

	item, distance := NearestGroundItem(items, actor, "gold")

	if item == nil {
		t.Fatalf("expected ground item to be returned")
	}
	if item.ID != "near" {
		t.Fatalf("expected nearest stack to be 'near', got %q", item.ID)
	}

	expected := math.Hypot(3, 4)
	if math.Abs(distance-expected) > 1e-9 {
		t.Fatalf("expected distance %.2f, got %.2f", expected, distance)
	}
}

func TestNearestGroundItemReturnsNilWhenNoMatch(t *testing.T) {
	items := map[string]*GroundItemState{
		"other": {
			GroundItem: GroundItem{ID: "other", Type: "wood", FungibilityKey: "wood-key", X: 1, Y: 1, Qty: 3},
		},
	}

	actor := &Actor{X: 5, Y: 5}

	item, distance := NearestGroundItem(items, actor, "gold")

	if item != nil || distance != 0 {
		t.Fatalf("expected no matching ground item, got %#v with distance %.2f", item, distance)
	}

	if result, dist := NearestGroundItem(items, nil, "wood"); result != nil || dist != 0 {
		t.Fatalf("expected nil result when actor missing, got %#v with distance %.2f", result, dist)
	}

	if result, dist := NearestGroundItem(nil, actor, "wood"); result != nil || dist != 0 {
		t.Fatalf("expected nil result when item map missing, got %#v with distance %.2f", result, dist)
	}
}

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

func TestDropAllItemsOfTypePlacesStacksOnGround(t *testing.T) {
	items := make(map[string]*GroundItemState)
	byTile := make(map[GroundTileKey]map[string]*GroundItemState)
	var nextID uint64

	actor := &Actor{ID: "actor-1", X: 3, Y: 7}
	cfg := ScatterConfig{TileSize: 10}

	ensureCalls := 0
	total := DropAllItemsOfType(
		items,
		byTile,
		&nextID,
		actor,
		"gold",
		"loot",
		cfg,
		nil,
		nil,
		func(s *ItemStack) bool {
			ensureCalls++
			if s.FungibilityKey == "" {
				s.FungibilityKey = "gold-key"
			}
			return true
		},
		func(item *GroundItemState, qty int) { item.Qty = qty },
		func(item *GroundItemState, x, y float64) { item.X, item.Y = x, y },
		nil,
		func(itemType string) []ItemStack {
			if itemType != "gold" {
				t.Fatalf("expected gold item type, got %q", itemType)
			}
			return []ItemStack{
				{Type: "gold", Quantity: 2},
				{Type: "gold", FungibilityKey: "gold-key", Quantity: 3},
			}
		},
	)

	if total != 5 {
		t.Fatalf("expected total quantity 5, got %d", total)
	}
	if ensureCalls == 0 {
		t.Fatalf("expected ensureKey callback to be invoked")
	}
	if len(items) != 1 {
		t.Fatalf("expected merged gold stack, got %d entries", len(items))
	}

	stack := items["ground-1"]
	if stack == nil {
		t.Fatalf("expected ground item to be created")
	}
	if stack.Qty != 5 {
		t.Fatalf("expected merged quantity 5, got %d", stack.Qty)
	}
	if stack.FungibilityKey != "gold-key" {
		t.Fatalf("expected fungibility key 'gold-key', got %q", stack.FungibilityKey)
	}

	tile := TileForPosition(actor.X, actor.Y, cfg.TileSize)
	if len(byTile[tile]) != 1 {
		t.Fatalf("expected single entry in tile index, got %d", len(byTile[tile]))
	}
}

func TestDropAllInventoryDropsCombinedStacks(t *testing.T) {
	items := make(map[string]*GroundItemState)
	byTile := make(map[GroundTileKey]map[string]*GroundItemState)
	var nextID uint64

	actor := &Actor{ID: "actor-2", X: 1, Y: 1}
	cfg := ScatterConfig{TileSize: 8}

	inventoryCalled := false
	equipmentCalled := false
	logCalls := 0

	total := DropAllInventory(
		items,
		byTile,
		&nextID,
		actor,
		"death",
		cfg,
		nil,
		nil,
		func(s *ItemStack) bool {
			if s.FungibilityKey == "" {
				s.FungibilityKey = s.Type + "-key"
			}
			return true
		},
		func(item *GroundItemState, qty int) { item.Qty = qty },
		func(item *GroundItemState, x, y float64) { item.X, item.Y = x, y },
		func(_ *Actor, stack ItemStack, reason, stackID string) {
			logCalls++
			if reason != "death" {
				t.Fatalf("expected log reason 'death', got %q", reason)
			}
			if stackID == "" {
				t.Fatalf("expected stackID to be populated")
			}
			if stack.Quantity <= 0 {
				t.Fatalf("expected logged stack quantity to be positive")
			}
		},
		func() []ItemStack {
			inventoryCalled = true
			return []ItemStack{{Type: "gold", FungibilityKey: "gold-key", Quantity: 4}}
		},
		func() []ItemStack {
			equipmentCalled = true
			return []ItemStack{{Type: "iron_dagger", FungibilityKey: "dagger-key", Quantity: 1}}
		},
	)

	if !inventoryCalled || !equipmentCalled {
		t.Fatalf("expected both drain callbacks to run (inventory=%v, equipment=%v)", inventoryCalled, equipmentCalled)
	}
	if total != 5 {
		t.Fatalf("expected total quantity 5, got %d", total)
	}
	if logCalls != 2 {
		t.Fatalf("expected log drop to run for each stack, got %d", logCalls)
	}
	if len(items) != 2 {
		t.Fatalf("expected two ground stacks, got %d", len(items))
	}
}

func TestDropAllGoldUsesGoldItemType(t *testing.T) {
	items := make(map[string]*GroundItemState)
	byTile := make(map[GroundTileKey]map[string]*GroundItemState)
	var nextID uint64

	actor := &Actor{ID: "actor-3", X: 0, Y: 0}
	cfg := ScatterConfig{TileSize: 6}

	var requestedType string
	calls := 0

	total := DropAllGold(
		items,
		byTile,
		&nextID,
		actor,
		"manual",
		cfg,
		nil,
		nil,
		func(s *ItemStack) bool {
			if s.FungibilityKey == "" {
				s.FungibilityKey = "gold-key"
			}
			return true
		},
		func(item *GroundItemState, qty int) { item.Qty = qty },
		func(item *GroundItemState, x, y float64) { item.X, item.Y = x, y },
		nil,
		func(itemType string) []ItemStack {
			requestedType = itemType
			calls++
			return []ItemStack{{Type: "gold", Quantity: 3}}
		},
	)

	if total != 3 {
		t.Fatalf("expected total quantity 3, got %d", total)
	}
	if requestedType != "gold" {
		t.Fatalf("expected drop helper to request gold stacks, got %q", requestedType)
	}
	if calls != 1 {
		t.Fatalf("expected removeStacks to be invoked once, got %d", calls)
	}
	if len(items) != 1 {
		t.Fatalf("expected single ground stack, got %d", len(items))
	}
}

func TestDropGoldQuantityPlacesStack(t *testing.T) {
	items := make(map[string]*GroundItemState)
	byTile := make(map[GroundTileKey]map[string]*GroundItemState)
	var nextID uint64

	actor := &Actor{ID: "player-1", X: 1, Y: 2}
	cfg := ScatterConfig{TileSize: 4}

	availableCalls := 0
	removeCalls := 0
	logged := false

	result, failure := DropGoldQuantity(
		items,
		byTile,
		&nextID,
		actor,
		3,
		"manual",
		cfg,
		nil,
		nil,
		func(s *ItemStack) bool {
			if s.FungibilityKey == "" {
				s.FungibilityKey = "gold-key"
			}
			return true
		},
		func(item *GroundItemState, qty int) { item.Qty = qty },
		func(item *GroundItemState, x, y float64) { item.X, item.Y = x, y },
		func(*Actor, ItemStack, string, string) { logged = true },
		func() int {
			availableCalls++
			return 5
		},
		func(qty int) (int, error) {
			removeCalls++
			if qty != 3 {
				t.Fatalf("expected removal request for 3, got %d", qty)
			}
			return qty, nil
		},
	)

	if failure != nil {
		t.Fatalf("expected drop to succeed, got failure %#v", failure)
	}
	if result == nil {
		t.Fatalf("expected drop result")
	}
	if result.Quantity != 3 {
		t.Fatalf("expected quantity 3, got %d", result.Quantity)
	}
	if result.StackID == "" {
		t.Fatalf("expected stack ID to be populated")
	}
	if availableCalls != 1 {
		t.Fatalf("expected available callback to run once, got %d", availableCalls)
	}
	if removeCalls != 1 {
		t.Fatalf("expected removeQuantity callback to run once, got %d", removeCalls)
	}
	if !logged {
		t.Fatalf("expected drop to be logged")
	}
	if len(items) != 1 {
		t.Fatalf("expected one ground item, got %d", len(items))
	}
}

func TestDropGoldQuantityInsufficientGold(t *testing.T) {
	items := make(map[string]*GroundItemState)
	byTile := make(map[GroundTileKey]map[string]*GroundItemState)
	var nextID uint64

	actor := &Actor{ID: "player-2", X: 0, Y: 0}
	cfg := ScatterConfig{TileSize: 4}

	removeCalled := false

	result, failure := DropGoldQuantity(
		items,
		byTile,
		&nextID,
		actor,
		5,
		"manual",
		cfg,
		nil,
		nil,
		func(*ItemStack) bool { return true },
		func(*GroundItemState, int) {},
		func(*GroundItemState, float64, float64) {},
		nil,
		func() int { return 3 },
		func(int) (int, error) {
			removeCalled = true
			return 0, nil
		},
	)

	if result != nil {
		t.Fatalf("expected no drop result, got %#v", result)
	}
	if failure == nil || failure.Reason != DropFailureReasonInsufficientGold {
		t.Fatalf("expected insufficient_gold failure, got %#v", failure)
	}
	if removeCalled {
		t.Fatalf("did not expect removeQuantity to be called when insufficient gold")
	}
	if len(items) != 0 {
		t.Fatalf("expected no ground items to be placed, got %d", len(items))
	}
}

func TestDropGoldQuantityInventoryError(t *testing.T) {
	items := make(map[string]*GroundItemState)
	byTile := make(map[GroundTileKey]map[string]*GroundItemState)
	var nextID uint64

	actor := &Actor{ID: "player-3", X: 0, Y: 0}
	cfg := ScatterConfig{TileSize: 4}

	result, failure := DropGoldQuantity(
		items,
		byTile,
		&nextID,
		actor,
		4,
		"manual",
		cfg,
		nil,
		nil,
		func(*ItemStack) bool { return true },
		func(*GroundItemState, int) {},
		func(*GroundItemState, float64, float64) {},
		nil,
		func() int { return 6 },
		func(qty int) (int, error) {
			if qty != 4 {
				t.Fatalf("expected removal request for 4, got %d", qty)
			}
			return 2, errors.New("mutation failed")
		},
	)

	if result != nil {
		t.Fatalf("expected no drop result, got %#v", result)
	}
	if failure == nil || failure.Reason != DropFailureReasonInventoryError {
		t.Fatalf("expected inventory_error failure, got %#v", failure)
	}
	if failure.Err == "" {
		t.Fatalf("expected failure error message to be populated")
	}
	if len(items) != 0 {
		t.Fatalf("expected no ground items to be placed, got %d", len(items))
	}
}

func TestPickupNearestItemTransfersStack(t *testing.T) {
	item := &GroundItemState{GroundItem: GroundItem{ID: "ground-1", Type: "gold", FungibilityKey: "gold-key", Qty: 5, X: 3, Y: 4}}
	items := map[string]*GroundItemState{item.ID: item}
	actor := &Actor{ID: "player-1", X: 0, Y: 0}

	added := false
	removed := false

	result, failure := PickupNearestItem(
		items,
		actor,
		"gold",
		10,
		func(stack ItemStack) error {
			if stack.Type != "gold" || stack.Quantity != 5 {
				t.Fatalf("unexpected stack transfer %#v", stack)
			}
			added = true
			return nil
		},
		func(target *GroundItemState) {
			if target != item {
				t.Fatalf("expected removal of ground-1, got %#v", target)
			}
			removed = true
		},
	)

	if failure != nil {
		t.Fatalf("expected pickup to succeed, got failure %#v", failure)
	}
	if result == nil {
		t.Fatalf("expected pickup result")
	}
	if result.StackID != item.ID || result.Quantity != 5 {
		t.Fatalf("unexpected pickup result %#v", result)
	}
	expectedDistance := math.Hypot(item.X-actor.X, item.Y-actor.Y)
	if math.Abs(result.Distance-expectedDistance) > 1e-9 {
		t.Fatalf("expected distance %.2f, got %.2f", expectedDistance, result.Distance)
	}
	if !added {
		t.Fatalf("expected addToInventory callback to run")
	}
	if !removed {
		t.Fatalf("expected removeItem callback to run")
	}
}

func TestPickupNearestItemOutOfRange(t *testing.T) {
	item := &GroundItemState{GroundItem: GroundItem{ID: "ground-2", Type: "gold", Qty: 1, X: 10, Y: 0}}
	items := map[string]*GroundItemState{item.ID: item}
	actor := &Actor{ID: "player-2", X: 0, Y: 0}

	addCalled := false
	removeCalled := false

	result, failure := PickupNearestItem(
		items,
		actor,
		"gold",
		5,
		func(ItemStack) error {
			addCalled = true
			return nil
		},
		func(*GroundItemState) {
			removeCalled = true
		},
	)

	if result != nil {
		t.Fatalf("expected no result when out of range, got %#v", result)
	}
	if failure == nil || failure.Reason != PickupFailureReasonOutOfRange {
		t.Fatalf("expected out_of_range failure, got %#v", failure)
	}
	if failure.StackID != item.ID {
		t.Fatalf("expected stack ID %q, got %q", item.ID, failure.StackID)
	}
	if failure.Distance <= 5 {
		t.Fatalf("expected recorded distance greater than radius, got %.2f", failure.Distance)
	}
	if addCalled {
		t.Fatalf("expected addToInventory not to be called for out of range stack")
	}
	if removeCalled {
		t.Fatalf("expected removeItem not to be called for out of range stack")
	}
}

func TestPickupNearestItemHandlesEmptyStacks(t *testing.T) {
	item := &GroundItemState{GroundItem: GroundItem{ID: "ground-3", Type: "gold", Qty: 0}}
	items := map[string]*GroundItemState{item.ID: item}
	actor := &Actor{ID: "player-3", X: 0, Y: 0}

	removed := false

	result, failure := PickupNearestItem(
		items,
		actor,
		"gold",
		2,
		func(ItemStack) error {
			t.Fatalf("did not expect inventory mutation for empty stack")
			return nil
		},
		func(target *GroundItemState) {
			if target != item {
				t.Fatalf("expected empty stack to be removed")
			}
			removed = true
		},
	)

	if result != nil {
		t.Fatalf("expected no result when stack is empty, got %#v", result)
	}
	if failure == nil || failure.Reason != PickupFailureReasonNotFound {
		t.Fatalf("expected not_found failure, got %#v", failure)
	}
	if failure.StackID != item.ID {
		t.Fatalf("expected failure to include stack ID %q, got %q", item.ID, failure.StackID)
	}
	if !removed {
		t.Fatalf("expected removeItem to be called for empty stack")
	}
}

func TestPickupNearestItemInventoryError(t *testing.T) {
	item := &GroundItemState{GroundItem: GroundItem{ID: "ground-4", Type: "gold", Qty: 3}}
	items := map[string]*GroundItemState{item.ID: item}
	actor := &Actor{ID: "player-4", X: 0, Y: 0}

	invErr := errors.New("inventory full")
	removeCalled := false

	result, failure := PickupNearestItem(
		items,
		actor,
		"gold",
		5,
		func(ItemStack) error { return invErr },
		func(*GroundItemState) { removeCalled = true },
	)

	if result != nil {
		t.Fatalf("expected no result when inventory mutation fails, got %#v", result)
	}
	if failure == nil || failure.Reason != PickupFailureReasonInventoryError {
		t.Fatalf("expected inventory_error failure, got %#v", failure)
	}
	if failure.StackID != item.ID {
		t.Fatalf("expected failure to include stack ID %q, got %q", item.ID, failure.StackID)
	}
	if failure.Err != invErr.Error() {
		t.Fatalf("expected failure error message %q, got %q", invErr.Error(), failure.Err)
	}
	if removeCalled {
		t.Fatalf("expected removeItem not to be called when inventory mutation fails")
	}
}

func TestPickupNearestItemNilInputs(t *testing.T) {
	result, failure := PickupNearestItem(nil, nil, "gold", 5, nil, nil)
	if result != nil {
		t.Fatalf("expected nil result when inputs missing, got %#v", result)
	}
	if failure == nil || failure.Reason != PickupFailureReasonNotFound {
		t.Fatalf("expected not_found failure for missing inputs, got %#v", failure)
	}
}
