package simsnapshots_test

import (
	"testing"

	"mine-and-die/server/internal/items/simsnapshots"
	"mine-and-die/server/internal/sim"
)

func TestInventoryFromSlotsClonesSnapshot(t *testing.T) {
	slots := []sim.InventorySlot{{
		Slot: 2,
		Item: sim.ItemStack{Type: "arrow", FungibilityKey: "stack", Quantity: 5},
	}}

	inv := simsnapshots.InventoryFromSlots(slots)
	if len(inv.Slots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(inv.Slots))
	}
	if inv.Slots[0].Item.Quantity != 5 {
		t.Fatalf("expected quantity 5, got %d", inv.Slots[0].Item.Quantity)
	}

	slots[0].Item.Quantity = 9
	if inv.Slots[0].Item.Quantity != 5 {
		t.Fatalf("expected cloned inventory to remain 5, got %d", inv.Slots[0].Item.Quantity)
	}

	if empty := simsnapshots.InventoryFromSlots(nil); len(empty.Slots) != 0 {
		t.Fatalf("expected empty inventory to have no slots")
	}
}

func TestEquipmentFromSlotsClonesSnapshot(t *testing.T) {
	slots := []sim.EquippedItem{{
		Slot: sim.EquipSlot("Head"),
		Item: sim.ItemStack{Type: "helm", FungibilityKey: "unique", Quantity: 1},
	}}

	eq := simsnapshots.EquipmentFromSlots(slots)
	if len(eq.Slots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(eq.Slots))
	}
	if eq.Slots[0].Slot != sim.EquipSlot("Head") {
		t.Fatalf("expected slot Head, got %q", eq.Slots[0].Slot)
	}

	slots[0].Item.Quantity = 3
	if eq.Slots[0].Item.Quantity != 1 {
		t.Fatalf("expected cloned equipment to remain 1, got %d", eq.Slots[0].Item.Quantity)
	}

	if empty := simsnapshots.EquipmentFromSlots(nil); len(empty.Slots) != 0 {
		t.Fatalf("expected empty equipment to have no slots")
	}
}
