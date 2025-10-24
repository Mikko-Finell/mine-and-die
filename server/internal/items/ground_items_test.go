package items

import (
	"errors"
	"math"
	"testing"

	simpatches "mine-and-die/server/internal/sim/patches/typed"
)

func mustBuildGroundDropDelegates(t *testing.T, cfg GroundDropConfig) GroundDropDelegates {
	t.Helper()
	delegates, ok := BuildGroundDropDelegates(cfg)
	if !ok {
		t.Fatalf("expected ground drop delegates to be constructed")
	}
	return delegates
}

func TestBuildGroundDropDelegatesRequiresAppendPatch(t *testing.T) {
	items := make(map[string]*GroundItemState)
	byTile := make(map[GroundTileKey]map[string]*GroundItemState)
	var nextID uint64

	cfg := GroundDropConfig{
		Items:       items,
		ItemsByTile: byTile,
		NextID:      &nextID,
		Actor:       &Actor{ID: "actor-append"},
		Scatter:     ScatterConfig{},
		EnsureKey:   func(*ItemStack) bool { return true },
	}

	if _, ok := BuildGroundDropDelegates(cfg); ok {
		t.Fatalf("expected build to fail when appendPatch missing")
	}

	cfg.AppendPatch = func(simpatches.Patch) {}

	if _, ok := BuildGroundDropDelegates(cfg); !ok {
		t.Fatalf("expected build to succeed once appendPatch provided")
	}
}

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

func TestGroundItemQuantityJournalSetterRecordsPatch(t *testing.T) {
	var patches []simpatches.Patch
	setter := GroundItemQuantityJournalSetter(func(p simpatches.Patch) {
		patches = append(patches, p)
	})

	item := &GroundItemState{GroundItem: GroundItem{ID: "ground-1", Qty: 2}}

	setter(item, 5)

	if item.Qty != 5 {
		t.Fatalf("expected quantity to update to 5, got %d", item.Qty)
	}
	if item.Version != 1 {
		t.Fatalf("expected version to increment to 1, got %d", item.Version)
	}
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}
	patch := patches[0]
	if patch.Kind != simpatches.PatchGroundItemQty {
		t.Fatalf("expected patch kind %q, got %q", simpatches.PatchGroundItemQty, patch.Kind)
	}
	payload, ok := patch.Payload.(simpatches.GroundItemQtyPayload)
	if !ok {
		t.Fatalf("expected payload type GroundItemQtyPayload, got %T", patch.Payload)
	}
	if payload.Qty != 5 {
		t.Fatalf("expected payload quantity 5, got %d", payload.Qty)
	}

	setter(item, 5)

	if item.Version != 1 {
		t.Fatalf("expected version to remain 1 after noop, got %d", item.Version)
	}
	if len(patches) != 1 {
		t.Fatalf("expected patch count to remain 1 after noop, got %d", len(patches))
	}
}

func TestGroundItemPositionJournalSetterRecordsPatch(t *testing.T) {
	var patches []simpatches.Patch
	setter := GroundItemPositionJournalSetter(func(p simpatches.Patch) {
		patches = append(patches, p)
	})

	item := &GroundItemState{GroundItem: GroundItem{ID: "ground-2", X: 1, Y: 2}}

	setter(item, 3, 4)

	if item.X != 3 || item.Y != 4 {
		t.Fatalf("expected position to update to (3,4), got (%.2f, %.2f)", item.X, item.Y)
	}
	if item.Version != 1 {
		t.Fatalf("expected version to increment to 1, got %d", item.Version)
	}
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}
	patch := patches[0]
	if patch.Kind != simpatches.PatchGroundItemPos {
		t.Fatalf("expected patch kind %q, got %q", simpatches.PatchGroundItemPos, patch.Kind)
	}
	payload, ok := patch.Payload.(simpatches.GroundItemPosPayload)
	if !ok {
		t.Fatalf("expected payload type GroundItemPosPayload, got %T", patch.Payload)
	}
	if payload.X != 3 || payload.Y != 4 {
		t.Fatalf("expected payload coordinates (3,4), got (%.2f, %.2f)", payload.X, payload.Y)
	}

	setter(item, 3, 4)

	if item.Version != 1 {
		t.Fatalf("expected version to remain 1 after noop, got %d", item.Version)
	}
	if len(patches) != 1 {
		t.Fatalf("expected patch count to remain 1 after noop, got %d", len(patches))
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
	setQuantity := GroundItemQuantityJournalSetter(nil)
	setPosition := GroundItemPositionJournalSetter(nil)

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
		setQuantity,
		setPosition,
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

	setQuantity := GroundItemQuantityJournalSetter(nil)
	setPosition := GroundItemPositionJournalSetter(nil)

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
		setQuantity,
		setPosition,
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

func TestUpsertGroundItemJournalSettersRecordPatches(t *testing.T) {
	tile := GroundTileKey{X: 1, Y: 2}
	existing := &GroundItemState{GroundItem: GroundItem{ID: "ground-7", Type: "gold", FungibilityKey: "gold-key", Qty: 4, X: 1, Y: 1}}
	existing.Tile = tile

	items := map[string]*GroundItemState{existing.ID: existing}
	byTile := map[GroundTileKey]map[string]*GroundItemState{tile: {existing.FungibilityKey: existing}}
	nextID := uint64(7)

	actor := &Actor{ID: "actor-merge", X: 15.5, Y: 25.25}
	stack := ItemStack{Type: "gold", FungibilityKey: "gold-key", Quantity: 2}
	cfg := ScatterConfig{TileSize: 10, MinDistance: 0, MaxDistance: 0, Padding: 0}

	var patches []simpatches.Patch
	merged := UpsertGroundItem(
		items,
		byTile,
		&nextID,
		actor,
		stack,
		"test",
		cfg,
		nil,
		nil,
		nil,
		GroundItemQuantityJournalSetter(func(p simpatches.Patch) {
			patches = append(patches, p)
		}),
		GroundItemPositionJournalSetter(func(p simpatches.Patch) {
			patches = append(patches, p)
		}),
		nil,
	)

	if merged != existing {
		t.Fatalf("expected existing stack to be returned")
	}
	if merged.Qty != 6 {
		t.Fatalf("expected merged quantity 6, got %d", merged.Qty)
	}
	if merged.X != actor.X || merged.Y != actor.Y {
		t.Fatalf("expected merged position to match actor (%.2f, %.2f), got (%.2f, %.2f)", actor.X, actor.Y, merged.X, merged.Y)
	}
	if merged.Version != 2 {
		t.Fatalf("expected merge to bump version twice, got %d", merged.Version)
	}
	if len(patches) != 2 {
		t.Fatalf("expected patches for quantity and position, got %d", len(patches))
	}
	if patches[0].Kind != simpatches.PatchGroundItemQty {
		t.Fatalf("expected first patch kind %q, got %q", simpatches.PatchGroundItemQty, patches[0].Kind)
	}
	qtyPayload, ok := patches[0].Payload.(simpatches.GroundItemQtyPayload)
	if !ok || qtyPayload.Qty != 6 {
		t.Fatalf("expected quantity payload 6, got %#v", patches[0].Payload)
	}
	if patches[1].Kind != simpatches.PatchGroundItemPos {
		t.Fatalf("expected second patch kind %q, got %q", simpatches.PatchGroundItemPos, patches[1].Kind)
	}
	posPayload, ok := patches[1].Payload.(simpatches.GroundItemPosPayload)
	if !ok || posPayload.X != actor.X || posPayload.Y != actor.Y {
		t.Fatalf("expected position payload (%.2f, %.2f), got %#v", actor.X, actor.Y, patches[1].Payload)
	}
}

func TestUpsertGroundItemReturnsNilWhenFungibilityMissing(t *testing.T) {
	items := make(map[string]*GroundItemState)
	byTile := make(map[GroundTileKey]map[string]*GroundItemState)
	var nextID uint64

	actor := &Actor{ID: "npc-1", X: 1, Y: 1}
	stack := ItemStack{Type: "mystery", Quantity: 1}
	cfg := ScatterConfig{TileSize: 5}

	setQuantity := GroundItemQuantityJournalSetter(nil)
	setPosition := GroundItemPositionJournalSetter(nil)

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
		setQuantity,
		setPosition,
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

	RemoveGroundItem(items, byTile, item, func(simpatches.Patch) {})

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

func TestRemoveGroundItemRequiresAppendPatch(t *testing.T) {
	tile := GroundTileKey{X: 1, Y: 2}
	item := &GroundItemState{GroundItem: GroundItem{ID: "ground-required", Type: "gold", FungibilityKey: "gold-key", Qty: 4}, Tile: tile}

	items := map[string]*GroundItemState{item.ID: item}
	byTile := map[GroundTileKey]map[string]*GroundItemState{tile: {"gold-key": item}}

	RemoveGroundItem(items, byTile, item, nil)

	if len(items) != 1 {
		t.Fatalf("expected items map to remain unchanged when appendPatch missing")
	}
	if len(byTile) != 1 {
		t.Fatalf("expected tile index to remain unchanged when appendPatch missing")
	}
	if item.Qty != 4 {
		t.Fatalf("expected quantity to remain at 4 when appendPatch missing, got %d", item.Qty)
	}
}

func TestRemoveGroundItemJournalSetterRecordsPatch(t *testing.T) {
	tile := GroundTileKey{X: 4, Y: 1}
	item := &GroundItemState{GroundItem: GroundItem{ID: "ground-6", Type: "gold", FungibilityKey: "gold-key", Qty: 3}, Tile: tile}

	items := map[string]*GroundItemState{item.ID: item}
	byTile := map[GroundTileKey]map[string]*GroundItemState{tile: {"gold-key": item}}

	var patches []simpatches.Patch
	RemoveGroundItem(
		items,
		byTile,
		item,
		func(p simpatches.Patch) {
			patches = append(patches, p)
		},
	)

	if item.Version != 1 {
		t.Fatalf("expected removal to increment version to 1, got %d", item.Version)
	}
	if len(patches) != 1 {
		t.Fatalf("expected removal to record 1 patch, got %d", len(patches))
	}
	patch := patches[0]
	if patch.Kind != simpatches.PatchGroundItemQty {
		t.Fatalf("expected removal patch kind %q, got %q", simpatches.PatchGroundItemQty, patch.Kind)
	}
	payload, ok := patch.Payload.(simpatches.GroundItemQtyPayload)
	if !ok {
		t.Fatalf("expected payload type GroundItemQtyPayload, got %T", patch.Payload)
	}
	if payload.Qty != 0 {
		t.Fatalf("expected removal patch quantity 0, got %d", payload.Qty)
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
	delegates := mustBuildGroundDropDelegates(t, GroundDropConfig{
		Items:       items,
		ItemsByTile: byTile,
		NextID:      &nextID,
		Actor:       actor,
		Scatter:     cfg,
		EnsureKey: func(s *ItemStack) bool {
			ensureCalls++
			if s.FungibilityKey == "" {
				s.FungibilityKey = "gold-key"
			}
			return true
		},
		AppendPatch: func(simpatches.Patch) {},
	})

	total := DropAllItemsOfType(
		delegates,
		"gold",
		"loot",
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

	delegates := mustBuildGroundDropDelegates(t, GroundDropConfig{
		Items:       items,
		ItemsByTile: byTile,
		NextID:      &nextID,
		Actor:       actor,
		Scatter:     cfg,
		EnsureKey: func(s *ItemStack) bool {
			if s.FungibilityKey == "" {
				s.FungibilityKey = s.Type + "-key"
			}
			return true
		},
		AppendPatch: func(simpatches.Patch) {},
		LogDrop: func(_ *Actor, stack ItemStack, reason, stackID string) {
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
	})

	total := DropAllInventory(
		delegates,
		"death",
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

	delegates := mustBuildGroundDropDelegates(t, GroundDropConfig{
		Items:       items,
		ItemsByTile: byTile,
		NextID:      &nextID,
		Actor:       actor,
		Scatter:     cfg,
		EnsureKey: func(s *ItemStack) bool {
			if s.FungibilityKey == "" {
				s.FungibilityKey = "gold-key"
			}
			return true
		},
		AppendPatch: func(simpatches.Patch) {},
	})

	total := DropAllGold(
		delegates,
		"manual",
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

	delegates := mustBuildGroundDropDelegates(t, GroundDropConfig{
		Items:       items,
		ItemsByTile: byTile,
		NextID:      &nextID,
		Actor:       actor,
		Scatter:     cfg,
		EnsureKey: func(s *ItemStack) bool {
			if s.FungibilityKey == "" {
				s.FungibilityKey = "gold-key"
			}
			return true
		},
		AppendPatch: func(simpatches.Patch) {},
		LogDrop:     func(*Actor, ItemStack, string, string) { logged = true },
	})

	result, failure := DropGoldQuantity(
		delegates,
		3,
		"manual",
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

	delegates := mustBuildGroundDropDelegates(t, GroundDropConfig{
		Items:       items,
		ItemsByTile: byTile,
		NextID:      &nextID,
		Actor:       actor,
		Scatter:     cfg,
		EnsureKey:   func(*ItemStack) bool { return true },
		AppendPatch: func(simpatches.Patch) {},
	})

	result, failure := DropGoldQuantity(
		delegates,
		5,
		"manual",
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

	delegates := mustBuildGroundDropDelegates(t, GroundDropConfig{
		Items:       items,
		ItemsByTile: byTile,
		NextID:      &nextID,
		Actor:       actor,
		Scatter:     cfg,
		EnsureKey:   func(*ItemStack) bool { return true },
		AppendPatch: func(simpatches.Patch) {},
	})

	result, failure := DropGoldQuantity(
		delegates,
		4,
		"manual",
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
	tile := GroundTileKey{X: 0, Y: 0}
	item := &GroundItemState{GroundItem: GroundItem{ID: "ground-1", Type: "gold", FungibilityKey: "gold-key", Qty: 5, X: 3, Y: 4}, Tile: tile}
	items := map[string]*GroundItemState{item.ID: item}
	byTile := map[GroundTileKey]map[string]*GroundItemState{tile: {item.FungibilityKey: item}}
	actor := &Actor{ID: "player-1", X: 0, Y: 0}

	added := false
	var patches []simpatches.Patch

	result, failure := PickupNearestItem(
		items,
		byTile,
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
		func(p simpatches.Patch) {
			patches = append(patches, p)
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
	if len(items) != 0 {
		t.Fatalf("expected items map to be cleared after pickup")
	}
	if len(byTile) != 0 {
		t.Fatalf("expected tile index to be cleared after pickup")
	}
	if len(patches) != 1 {
		t.Fatalf("expected removal to append a quantity patch, got %d", len(patches))
	}
}

func TestPickupNearestItemOutOfRange(t *testing.T) {
	tile := GroundTileKey{X: 1, Y: 0}
	item := &GroundItemState{GroundItem: GroundItem{ID: "ground-2", Type: "gold", FungibilityKey: "gold-key", Qty: 1, X: 10, Y: 0}, Tile: tile}
	items := map[string]*GroundItemState{item.ID: item}
	byTile := map[GroundTileKey]map[string]*GroundItemState{tile: {item.FungibilityKey: item}}
	actor := &Actor{ID: "player-2", X: 0, Y: 0}

	addCalled := false
	removeCalled := false
	var patches []simpatches.Patch

	result, failure := PickupNearestItem(
		items,
		byTile,
		actor,
		"gold",
		5,
		func(ItemStack) error {
			addCalled = true
			return nil
		},
		func(p simpatches.Patch) {
			removeCalled = true
			patches = append(patches, p)
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
	if len(items) != 1 {
		t.Fatalf("expected items map to remain unchanged when out of range")
	}
	if len(byTile) != 1 {
		t.Fatalf("expected tile index to remain unchanged when out of range")
	}
	if len(patches) != 0 {
		t.Fatalf("expected no patches when out of range, got %d", len(patches))
	}
}

func TestPickupNearestItemHandlesEmptyStacks(t *testing.T) {
	tile := GroundTileKey{X: 0, Y: 1}
	item := &GroundItemState{GroundItem: GroundItem{ID: "ground-3", Type: "gold", FungibilityKey: "gold-key", Qty: 0}, Tile: tile}
	items := map[string]*GroundItemState{item.ID: item}
	byTile := map[GroundTileKey]map[string]*GroundItemState{tile: {item.FungibilityKey: item}}
	actor := &Actor{ID: "player-3", X: 0, Y: 0}

	var patches []simpatches.Patch

	result, failure := PickupNearestItem(
		items,
		byTile,
		actor,
		"gold",
		2,
		func(ItemStack) error {
			t.Fatalf("did not expect inventory mutation for empty stack")
			return nil
		},
		func(p simpatches.Patch) {
			patches = append(patches, p)
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
	if len(items) != 0 {
		t.Fatalf("expected items map to be cleared after removing empty stack")
	}
	if len(byTile) != 0 {
		t.Fatalf("expected tile index to be cleared after removing empty stack")
	}
	if len(patches) != 0 {
		t.Fatalf("expected no patches when empty stack removed, got %d", len(patches))
	}
}

func TestPickupNearestItemInventoryError(t *testing.T) {
	tile := GroundTileKey{X: 2, Y: 0}
	item := &GroundItemState{GroundItem: GroundItem{ID: "ground-4", Type: "gold", FungibilityKey: "gold-key", Qty: 3}, Tile: tile}
	items := map[string]*GroundItemState{item.ID: item}
	byTile := map[GroundTileKey]map[string]*GroundItemState{tile: {item.FungibilityKey: item}}
	actor := &Actor{ID: "player-4", X: 0, Y: 0}

	invErr := errors.New("inventory full")
	removeCalled := false
	var patches []simpatches.Patch

	result, failure := PickupNearestItem(
		items,
		byTile,
		actor,
		"gold",
		5,
		func(ItemStack) error { return invErr },
		func(p simpatches.Patch) {
			removeCalled = true
			patches = append(patches, p)
		},
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
	if len(items) != 1 {
		t.Fatalf("expected items map to remain unchanged when inventory fails")
	}
	if len(byTile) != 1 {
		t.Fatalf("expected tile index to remain unchanged when inventory fails")
	}
	if len(patches) != 0 {
		t.Fatalf("expected no patches when inventory mutation fails, got %d", len(patches))
	}
}

func TestPickupNearestItemNilInputs(t *testing.T) {
	result, failure := PickupNearestItem(nil, nil, nil, "gold", 5, nil, nil)
	if result != nil {
		t.Fatalf("expected nil result when inputs missing, got %#v", result)
	}
	if failure == nil || failure.Reason != PickupFailureReasonNotFound {
		t.Fatalf("expected not_found failure for missing inputs, got %#v", failure)
	}
}

func TestGroundDropRemoveStacksFuncPrefersHandledProvider(t *testing.T) {
	cfg := GroundDropActorConfig{
		RemovePlayerStacks: func(itemType string) ([]ItemStack, bool) {
			if itemType != "gold" {
				t.Fatalf("expected gold item type, got %q", itemType)
			}
			return []ItemStack{{Type: itemType, Quantity: 2}}, true
		},
		RemoveFallbackStacks: func(string) ([]ItemStack, bool) {
			t.Fatalf("fallback should not be invoked when player handler succeeds")
			return nil, true
		},
	}

	remover := GroundDropRemoveStacksFunc(cfg)
	if remover == nil {
		t.Fatalf("expected remover to be constructed")
	}

	stacks := remover("gold")
	if len(stacks) != 1 || stacks[0].Quantity != 2 {
		t.Fatalf("expected handled stacks, got %#v", stacks)
	}
}

func TestGroundDropRemoveStacksFuncFallsBackWhenNotHandled(t *testing.T) {
	fallbackCalled := false
	cfg := GroundDropActorConfig{
		RemovePlayerStacks: func(string) ([]ItemStack, bool) {
			return nil, false
		},
		RemoveFallbackStacks: func(itemType string) ([]ItemStack, bool) {
			fallbackCalled = true
			return []ItemStack{{Type: itemType, Quantity: 1}}, true
		},
	}

	remover := GroundDropRemoveStacksFunc(cfg)
	if remover == nil {
		t.Fatalf("expected remover to be constructed")
	}

	stacks := remover("iron")
	if !fallbackCalled {
		t.Fatalf("expected fallback provider to run")
	}
	if len(stacks) != 1 || stacks[0].Type != "iron" {
		t.Fatalf("expected fallback stacks, got %#v", stacks)
	}
}

func TestGroundDropRemoveStacksFuncNilWhenNoProviders(t *testing.T) {
	if GroundDropRemoveStacksFunc(GroundDropActorConfig{}) != nil {
		t.Fatalf("expected nil remover when no providers configured")
	}
}

func TestGroundDropInventoryDrainFuncPrefersHandled(t *testing.T) {
	cfg := GroundDropActorConfig{
		DrainPlayerInventory: func() ([]ItemStack, bool) {
			return []ItemStack{{Type: "gold", Quantity: 3}}, true
		},
		DrainFallbackInventory: func() ([]ItemStack, bool) {
			t.Fatalf("expected fallback inventory to remain unused")
			return nil, true
		},
	}

	drain := GroundDropInventoryDrainFunc(cfg)
	if drain == nil {
		t.Fatalf("expected inventory drain to be constructed")
	}

	stacks := drain()
	if len(stacks) != 1 || stacks[0].Quantity != 3 {
		t.Fatalf("expected player inventory stacks, got %#v", stacks)
	}
}

func TestGroundDropInventoryDrainFuncFallsBack(t *testing.T) {
	fallbackCalled := false
	cfg := GroundDropActorConfig{
		DrainPlayerInventory: func() ([]ItemStack, bool) {
			return nil, false
		},
		DrainFallbackInventory: func() ([]ItemStack, bool) {
			fallbackCalled = true
			return []ItemStack{{Type: "potion", Quantity: 1}}, true
		},
	}

	drain := GroundDropInventoryDrainFunc(cfg)
	if drain == nil {
		t.Fatalf("expected inventory drain to be constructed")
	}

	stacks := drain()
	if !fallbackCalled {
		t.Fatalf("expected fallback inventory to run")
	}
	if len(stacks) != 1 || stacks[0].Type != "potion" {
		t.Fatalf("expected fallback inventory stack, got %#v", stacks)
	}
}

func TestGroundDropInventoryDrainFuncNilWhenNoProviders(t *testing.T) {
	if GroundDropInventoryDrainFunc(GroundDropActorConfig{}) != nil {
		t.Fatalf("expected nil inventory drain when no providers configured")
	}
}

func TestGroundDropEquipmentDrainFuncPrefersHandled(t *testing.T) {
	cfg := GroundDropActorConfig{
		DrainPlayerEquipment: func() ([]ItemStack, bool) {
			return []ItemStack{{Type: "sword", Quantity: 1}}, true
		},
		DrainFallbackEquipment: func() ([]ItemStack, bool) {
			t.Fatalf("expected fallback equipment to remain unused")
			return nil, true
		},
	}

	drain := GroundDropEquipmentDrainFunc(cfg)
	if drain == nil {
		t.Fatalf("expected equipment drain to be constructed")
	}

	stacks := drain()
	if len(stacks) != 1 || stacks[0].Type != "sword" {
		t.Fatalf("expected player equipment stack, got %#v", stacks)
	}
}

func TestGroundDropEquipmentDrainFuncFallsBack(t *testing.T) {
	fallbackCalled := false
	cfg := GroundDropActorConfig{
		DrainNPCEquipment: func() ([]ItemStack, bool) {
			return nil, false
		},
		DrainFallbackEquipment: func() ([]ItemStack, bool) {
			fallbackCalled = true
			return []ItemStack{{Type: "shield", Quantity: 1}}, true
		},
	}

	drain := GroundDropEquipmentDrainFunc(cfg)
	if drain == nil {
		t.Fatalf("expected equipment drain to be constructed")
	}

	stacks := drain()
	if !fallbackCalled {
		t.Fatalf("expected fallback equipment to run")
	}
	if len(stacks) != 1 || stacks[0].Type != "shield" {
		t.Fatalf("expected fallback equipment stack, got %#v", stacks)
	}
}

func TestGroundDropEquipmentDrainFuncNilWhenNoProviders(t *testing.T) {
	if GroundDropEquipmentDrainFunc(GroundDropActorConfig{}) != nil {
		t.Fatalf("expected nil equipment drain when no providers configured")
	}
}

func TestGroundDropRemoveGoldQuantityFuncNilWhenUnconfigured(t *testing.T) {
	if GroundDropRemoveGoldQuantityFunc(GroundDropActorConfig{}) != nil {
		t.Fatalf("expected nil gold quantity remover when not configured")
	}
}

func TestGroundDropRemoveGoldQuantityFuncDelegates(t *testing.T) {
	called := false
	cfg := GroundDropActorConfig{
		RemoveGoldQuantity: func(quantity int) (int, error) {
			called = true
			if quantity != 4 {
				t.Fatalf("expected quantity 4, got %d", quantity)
			}
			return quantity, nil
		},
	}

	remove := GroundDropRemoveGoldQuantityFunc(cfg)
	if remove == nil {
		t.Fatalf("expected gold quantity remover to be constructed")
	}

	qty, err := remove(4)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if qty != 4 {
		t.Fatalf("expected quantity 4, got %d", qty)
	}
	if !called {
		t.Fatalf("expected gold removal provider to be invoked")
	}
}
