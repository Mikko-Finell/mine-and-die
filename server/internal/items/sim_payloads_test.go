package items

import (
	"testing"

	"mine-and-die/server/internal/sim"
)

type legacyInventorySlot struct {
	Slot int
	Qty  int
}

type legacyInventory struct {
	Slots []legacyInventorySlot
}

type legacyEquippedItem struct {
	Slot string
	Qty  int
}

type legacyEquipment struct {
	Slots []legacyEquippedItem
}

func TestInventoryFromSim(t *testing.T) {
	inv := sim.Inventory{Slots: []sim.InventorySlot{{
		Slot: 2,
		Item: sim.ItemStack{Quantity: 5},
	}}}

	converted := InventoryFromSim(inv, func(slot sim.InventorySlot) legacyInventorySlot {
		return legacyInventorySlot{Slot: slot.Slot, Qty: slot.Item.Quantity}
	}, func(slots []legacyInventorySlot) legacyInventory {
		return legacyInventory{Slots: slots}
	})

	if len(converted.Slots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(converted.Slots))
	}
	if converted.Slots[0].Slot != 2 || converted.Slots[0].Qty != 5 {
		t.Fatalf("unexpected slot conversion: %+v", converted.Slots[0])
	}
}

func TestInventoryFromSimEmpty(t *testing.T) {
	converted := InventoryFromSim(sim.Inventory{}, func(slot sim.InventorySlot) legacyInventorySlot {
		return legacyInventorySlot{Slot: slot.Slot, Qty: slot.Item.Quantity}
	}, func(slots []legacyInventorySlot) legacyInventory {
		return legacyInventory{Slots: slots}
	})

	if len(converted.Slots) != 0 {
		t.Fatalf("expected empty slots, got %d", len(converted.Slots))
	}
}

func TestInventoryFromSimNilHelpers(t *testing.T) {
	converted := InventoryFromSim[legacyInventorySlot, legacyInventory](sim.Inventory{}, nil, nil)
	if len(converted.Slots) != 0 {
		t.Fatalf("expected zero value inventory, got %+v", converted)
	}
}

func TestEquipmentFromSim(t *testing.T) {
	eq := sim.Equipment{Slots: []sim.EquippedItem{{
		Slot: "head",
		Item: sim.ItemStack{Quantity: 3},
	}}}

	converted := EquipmentFromSim(eq, func(slot sim.EquippedItem) legacyEquippedItem {
		return legacyEquippedItem{Slot: slot.Slot, Qty: slot.Item.Quantity}
	}, func(slots []legacyEquippedItem) legacyEquipment {
		return legacyEquipment{Slots: slots}
	})

	if len(converted.Slots) != 1 {
		t.Fatalf("expected 1 slot, got %d", len(converted.Slots))
	}
	if converted.Slots[0].Slot != "head" || converted.Slots[0].Qty != 3 {
		t.Fatalf("unexpected slot conversion: %+v", converted.Slots[0])
	}
}

func TestEquipmentFromSimEmpty(t *testing.T) {
	converted := EquipmentFromSim(sim.Equipment{}, func(slot sim.EquippedItem) legacyEquippedItem {
		return legacyEquippedItem{Slot: slot.Slot, Qty: slot.Item.Quantity}
	}, func(slots []legacyEquippedItem) legacyEquipment {
		return legacyEquipment{Slots: slots}
	})

	if len(converted.Slots) != 0 {
		t.Fatalf("expected empty slots, got %d", len(converted.Slots))
	}
}

func TestEquipmentFromSimNilHelpers(t *testing.T) {
	converted := EquipmentFromSim[legacyEquippedItem, legacyEquipment](sim.Equipment{}, nil, nil)
	if len(converted.Slots) != 0 {
		t.Fatalf("expected zero value equipment, got %+v", converted)
	}
}
