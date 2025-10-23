package server

import (
	"encoding/json"
	"testing"

	itemspkg "mine-and-die/server/internal/items"
	"mine-and-die/server/internal/sim"
	logging "mine-and-die/server/logging"
)

func TestRemoveGroundItemRecordsQuantityPatch(t *testing.T) {
	w := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})

	def, ok := ItemDefinitionFor(ItemTypeGold)
	if !ok {
		t.Fatalf("expected gold definition to be registered")
	}

	tile := itemspkg.GroundTileKey{X: 1, Y: 2}
	item := &itemspkg.GroundItemState{
		GroundItem: itemspkg.GroundItem{
			ID:             "ground-1",
			Type:           string(ItemTypeGold),
			FungibilityKey: def.FungibilityKey,
			X:              4.5,
			Y:              9.25,
			Qty:            7,
		},
		Tile: tile,
	}

	w.groundItems[item.ID] = item
	w.groundItemsByTile[tile] = map[string]*itemspkg.GroundItemState{def.FungibilityKey: item}

	itemspkg.RemoveGroundItem(w.groundItems, w.groundItemsByTile, item, w.journal.AppendPatch)

	if _, exists := w.groundItems[item.ID]; exists {
		t.Fatalf("expected ground item %q to be removed from world", item.ID)
	}
	if _, exists := w.groundItemsByTile[tile]; exists {
		t.Fatalf("expected tile entry to be removed after deleting ground item")
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected exactly one patch, got %d", len(patches))
	}

	patch := patches[0]
	if patch.Kind != PatchGroundItemQty {
		t.Fatalf("expected patch kind %q, got %q", PatchGroundItemQty, patch.Kind)
	}
	qtyPayload, ok := patch.Payload.(GroundItemQtyPayload)
	if !ok {
		t.Fatalf("expected payload type GroundItemQtyPayload, got %T", patch.Payload)
	}
	if qtyPayload.Qty != 0 {
		t.Fatalf("expected removal patch to set quantity to 0, got %d", qtyPayload.Qty)
	}
}

func TestMarshalStateKeepsGroundItemRemovalPatch(t *testing.T) {
	hub := newHub()
	hub.world.drainPatchesLocked()

	def, ok := ItemDefinitionFor(ItemTypeGold)
	if !ok {
		t.Fatalf("expected gold definition to be registered")
	}

	tile := itemspkg.GroundTileKey{X: 0, Y: 0}
	item := &itemspkg.GroundItemState{
		GroundItem: itemspkg.GroundItem{
			ID:             "ground-42",
			Type:           string(ItemTypeGold),
			FungibilityKey: def.FungibilityKey,
			Qty:            3,
			X:              5.75,
			Y:              1.5,
		},
		Tile: tile,
	}

	hub.world.groundItems[item.ID] = item
	hub.world.groundItemsByTile[tile] = map[string]*itemspkg.GroundItemState{def.FungibilityKey: item}

	itemspkg.RemoveGroundItem(hub.world.groundItems, hub.world.groundItemsByTile, item, hub.world.journal.AppendPatch)

	data, _, err := hub.marshalState(nil, nil, nil, nil, true, false)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	var msg stateMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to decode state message: %v", err)
	}

	if len(msg.Patches) != 1 {
		t.Fatalf("expected one patch in state message, got %d", len(msg.Patches))
	}
	patch := msg.Patches[0]
	if patch.Kind != sim.PatchGroundItemQty {
		t.Fatalf("expected patch kind %q, got %q", sim.PatchGroundItemQty, patch.Kind)
	}
	if patch.EntityID != item.ID {
		t.Fatalf("expected patch entity %q, got %q", item.ID, patch.EntityID)
	}

	payload, ok := patch.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected payload to decode as map, got %T", patch.Payload)
	}
	qtyValue, ok := payload["qty"]
	if !ok {
		t.Fatalf("expected payload to contain qty field")
	}
	qtyFloat, ok := qtyValue.(float64)
	if !ok {
		t.Fatalf("expected qty to decode as float64, got %T", qtyValue)
	}
	if qtyFloat != 0 {
		t.Fatalf("expected qty to equal 0, got %f", qtyFloat)
	}
}

func TestMarshalStateOmitsGroundItemsFromDiffFrames(t *testing.T) {
	hub := newHub()
	hub.world.drainPatchesLocked()

	def, ok := ItemDefinitionFor(ItemTypeGold)
	if !ok {
		t.Fatalf("expected gold definition to be registered")
	}

	tile := itemspkg.GroundTileKey{X: 3, Y: 1}
	item := &itemspkg.GroundItemState{
		GroundItem: itemspkg.GroundItem{
			ID:             "ground-99",
			Type:           string(ItemTypeGold),
			FungibilityKey: def.FungibilityKey,
			Qty:            5,
			X:              2.5,
			Y:              6.25,
		},
		Tile: tile,
	}

	hub.world.groundItems[item.ID] = item
	hub.world.groundItemsByTile[tile] = map[string]*itemspkg.GroundItemState{def.FungibilityKey: item}

	snapshot := hub.world.GroundItemsSnapshot()
	if len(snapshot) != 1 {
		t.Fatalf("expected snapshot to contain one ground item, got %d", len(snapshot))
	}

	data, _, err := hub.marshalState(nil, nil, nil, append([]itemspkg.GroundItem(nil), snapshot...), true, false)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	var msg stateMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to decode state message: %v", err)
	}

	if msg.GroundItems != nil {
		t.Fatalf("expected ground items to be omitted for diff frames, got %d entries", len(msg.GroundItems))
	}
}
