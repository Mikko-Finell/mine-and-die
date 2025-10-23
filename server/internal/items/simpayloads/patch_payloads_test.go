package simpayloads_test

import (
	"testing"

	server "mine-and-die/server"
	"mine-and-die/server/internal/items/simpayloads"
	journal "mine-and-die/server/internal/journal"
	"mine-and-die/server/internal/sim"
)

func TestSimInventoryPayloadFromLegacyConvertsServerSlots(t *testing.T) {
	slots := []server.InventorySlot{{
		Slot: 3,
		Item: server.ItemStack{Type: server.ItemType("arrow"), FungibilityKey: "stack", Quantity: 20},
	}}
	payload := journal.InventoryPayload{Slots: slots}

	converted := simpayloads.SimInventoryPayloadFromLegacy(payload)

	if len(converted.Slots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(converted.Slots))
	}
	slot := converted.Slots[0]
	if slot.Slot != 3 {
		t.Fatalf("expected slot index 3, got %d", slot.Slot)
	}
	if slot.Item.Type != sim.ItemType("arrow") {
		t.Fatalf("expected item type arrow, got %q", slot.Item.Type)
	}
	if slot.Item.FungibilityKey != "stack" {
		t.Fatalf("expected fungibility key stack, got %q", slot.Item.FungibilityKey)
	}
	if slot.Item.Quantity != 20 {
		t.Fatalf("expected quantity 20, got %d", slot.Item.Quantity)
	}

	slots[0].Item.Quantity = 5
	if converted.Slots[0].Item.Quantity != 20 {
		t.Fatalf("expected converted quantity to remain 20, got %d", converted.Slots[0].Item.Quantity)
	}
}

func TestSimInventoryPayloadFromLegacyPtrNil(t *testing.T) {
	if simpayloads.SimInventoryPayloadFromLegacyPtr(nil) != nil {
		t.Fatalf("expected nil result for nil payload pointer")
	}
}

func TestLegacyInventoryPayloadFromSimClones(t *testing.T) {
	simPayload := sim.InventoryPayload{Slots: []sim.InventorySlot{{
		Slot: 1,
		Item: sim.ItemStack{Type: sim.ItemType("potion"), FungibilityKey: "k", Quantity: 2},
	}}}

	converted := simpayloads.LegacyInventoryPayloadFromSim(simPayload)

	slots, ok := converted.Slots.([]sim.InventorySlot)
	if !ok {
		t.Fatalf("expected legacy payload slots to be []sim.InventorySlot, got %T", converted.Slots)
	}
	if len(slots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(slots))
	}
	if slots[0].Item.Quantity != 2 {
		t.Fatalf("expected quantity 2, got %d", slots[0].Item.Quantity)
	}

	simPayload.Slots[0].Item.Quantity = 9
	if slots[0].Item.Quantity != 2 {
		t.Fatalf("expected cloned quantity 2, got %d", slots[0].Item.Quantity)
	}
}

func TestSimEquipmentPayloadFromLegacyConvertsServerSlots(t *testing.T) {
	slots := []server.EquippedItem{{
		Slot: server.EquipSlotHead,
		Item: server.ItemStack{Type: server.ItemType("helm"), FungibilityKey: "unique", Quantity: 1},
	}}
	payload := journal.EquipmentPayload{Slots: slots}

	converted := simpayloads.SimEquipmentPayloadFromLegacy(payload)

	if len(converted.Slots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(converted.Slots))
	}
	slot := converted.Slots[0]
	if slot.Slot != sim.EquipSlotHead {
		t.Fatalf("expected equip slot Head, got %q", slot.Slot)
	}
	if slot.Item.Type != sim.ItemType("helm") {
		t.Fatalf("expected item type helm, got %q", slot.Item.Type)
	}
	if slot.Item.FungibilityKey != "unique" {
		t.Fatalf("expected fungibility key unique, got %q", slot.Item.FungibilityKey)
	}
	if slot.Item.Quantity != 1 {
		t.Fatalf("expected quantity 1, got %d", slot.Item.Quantity)
	}

	slots[0].Item.FungibilityKey = "mutated"
	if converted.Slots[0].Item.FungibilityKey != "unique" {
		t.Fatalf("expected converted key unique, got %q", converted.Slots[0].Item.FungibilityKey)
	}
}

func TestLegacyEquipmentPayloadFromSimClones(t *testing.T) {
	simPayload := sim.EquipmentPayload{Slots: []sim.EquippedItem{{
		Slot: sim.EquipSlotMainHand,
		Item: sim.ItemStack{Type: sim.ItemType("sword"), FungibilityKey: "rare", Quantity: 1},
	}}}

	converted := simpayloads.LegacyEquipmentPayloadFromSim(simPayload)

	slots, ok := converted.Slots.([]sim.EquippedItem)
	if !ok {
		t.Fatalf("expected legacy payload slots to be []sim.EquippedItem, got %T", converted.Slots)
	}
	if len(slots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(slots))
	}
	if slots[0].Item.FungibilityKey != "rare" {
		t.Fatalf("expected fungibility key rare, got %q", slots[0].Item.FungibilityKey)
	}

	simPayload.Slots[0].Item.FungibilityKey = "changed"
	if slots[0].Item.FungibilityKey != "rare" {
		t.Fatalf("expected cloned key rare, got %q", slots[0].Item.FungibilityKey)
	}
}

func TestSimEquippedItemsFromAnyHandlesNil(t *testing.T) {
	if res := simpayloads.SimEquippedItemsFromAny(nil); res != nil {
		t.Fatalf("expected nil result for nil input")
	}
}
