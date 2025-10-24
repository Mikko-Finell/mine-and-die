package items_test

import (
	"testing"

	server "mine-and-die/server"
	itemspkg "mine-and-die/server/internal/items"
	"mine-and-die/server/internal/sim"
)

func TestSimInventorySlotsFromAnyConvertsServerSlots(t *testing.T) {
	slots := []server.InventorySlot{{
		Slot: 3,
		Item: server.ItemStack{Type: server.ItemType("arrow"), FungibilityKey: "stack", Quantity: 4},
	}}

	converted := itemspkg.SimInventorySlotsFromAny(slots)
	if len(converted) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(converted))
	}
	if converted[0].Slot != 3 {
		t.Fatalf("expected slot index 3, got %d", converted[0].Slot)
	}
	if converted[0].Item.Quantity != 4 {
		t.Fatalf("expected quantity 4, got %d", converted[0].Item.Quantity)
	}

	slots[0].Item.Quantity = 9
	if converted[0].Item.Quantity != 4 {
		t.Fatalf("expected clone to remain 4, got %d", converted[0].Item.Quantity)
	}
}

func TestSimInventorySlotsFromAnyHandlesNil(t *testing.T) {
	if res := itemspkg.SimInventorySlotsFromAny(nil); res != nil {
		t.Fatalf("expected nil result for nil input")
	}
}

func TestSimInventorySlotsFromAnyConvertsPointerSlice(t *testing.T) {
	slots := []server.InventorySlot{{
		Slot: 5,
		Item: server.ItemStack{Type: server.ItemType("potion"), FungibilityKey: "stack", Quantity: 7},
	}}

	converted := itemspkg.SimInventorySlotsFromAny(&slots)
	if len(converted) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(converted))
	}
	if converted[0].Slot != 5 {
		t.Fatalf("expected slot index 5, got %d", converted[0].Slot)
	}
}

func TestSimEquippedItemsFromAnyConvertsServerSlots(t *testing.T) {
	slots := []server.EquippedItem{{
		Slot: server.EquipSlotHead,
		Item: server.ItemStack{Type: server.ItemType("helm"), FungibilityKey: "unique", Quantity: 1},
	}}

	converted := itemspkg.SimEquippedItemsFromAny(slots)
	if len(converted) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(converted))
	}
	if converted[0].Slot != sim.EquipSlotHead {
		t.Fatalf("expected equip slot Head, got %q", converted[0].Slot)
	}
	if converted[0].Item.FungibilityKey != "unique" {
		t.Fatalf("expected fungibility key unique, got %q", converted[0].Item.FungibilityKey)
	}

	slots[0].Item.FungibilityKey = "mutated"
	if converted[0].Item.FungibilityKey != "unique" {
		t.Fatalf("expected clone to remain unique, got %q", converted[0].Item.FungibilityKey)
	}
}

func TestSimEquippedItemsFromAnyHandlesNil(t *testing.T) {
	if res := itemspkg.SimEquippedItemsFromAny(nil); res != nil {
		t.Fatalf("expected nil result for nil input")
	}
}

func TestSimEquippedItemsFromAnyConvertsPointerSlice(t *testing.T) {
	slots := []server.EquippedItem{{
		Slot: server.EquipSlotMainHand,
		Item: server.ItemStack{Type: server.ItemType("sword"), FungibilityKey: "rare", Quantity: 1},
	}}

	converted := itemspkg.SimEquippedItemsFromAny(&slots)
	if len(converted) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(converted))
	}
	if converted[0].Slot != sim.EquipSlotMainHand {
		t.Fatalf("expected equip slot MainHand, got %q", converted[0].Slot)
	}
}
