package items_test

import (
	"testing"

	"mine-and-die/server/internal/items"
	"mine-and-die/server/internal/sim"
)

type legacyInventorySlot struct {
	Slot int
	Item legacyItemStack
}

type legacyEquippedItem struct {
	Slot string
	Item legacyItemStack
}

type legacyItemStack struct {
	Type           string
	FungibilityKey string
	Quantity       int
}

func TestInventorySlotsFromConvertsEntries(t *testing.T) {
	slots := []legacyInventorySlot{{
		Slot: 2,
		Item: legacyItemStack{Type: "arrow", FungibilityKey: "stack", Quantity: 15},
	}}

	converted := items.InventorySlotsFrom(slots, func(slot legacyInventorySlot) sim.InventorySlot {
		return sim.InventorySlot{
			Slot: slot.Slot,
			Item: sim.ItemStack{
				Type:           sim.ItemType(slot.Item.Type),
				FungibilityKey: slot.Item.FungibilityKey,
				Quantity:       slot.Item.Quantity,
			},
		}
	})

	if len(converted) != 1 {
		t.Fatalf("expected 1 converted slot, got %d", len(converted))
	}
	result := converted[0]
	if result.Slot != 2 {
		t.Fatalf("expected slot index 2, got %d", result.Slot)
	}
	if result.Item.Type != sim.ItemType("arrow") {
		t.Fatalf("expected item type arrow, got %q", result.Item.Type)
	}
	if result.Item.FungibilityKey != "stack" {
		t.Fatalf("expected fungibility key stack, got %q", result.Item.FungibilityKey)
	}
	if result.Item.Quantity != 15 {
		t.Fatalf("expected quantity 15, got %d", result.Item.Quantity)
	}

	slots[0].Item.Quantity = 99
	if result.Item.Quantity != 15 {
		t.Fatalf("expected converted slot to remain 15, got %d", result.Item.Quantity)
	}
}

func TestInventorySlotsFromHandlesEmpty(t *testing.T) {
	converted := items.InventorySlotsFrom([]legacyInventorySlot{}, func(legacyInventorySlot) sim.InventorySlot {
		t.Fatalf("unexpected conversion for empty input")
		return sim.InventorySlot{}
	})
	if converted != nil {
		t.Fatalf("expected nil result for empty slice")
	}

	var nilInventory []legacyInventorySlot
	var nilInventoryConverter func(legacyInventorySlot) sim.InventorySlot
	converted = items.InventorySlotsFrom(nilInventory, nilInventoryConverter)
	if converted != nil {
		t.Fatalf("expected nil result for nil converter")
	}
}

func TestEquipmentSlotsFromConvertsEntries(t *testing.T) {
	slots := []*legacyEquippedItem{{
		Slot: "head",
		Item: legacyItemStack{Type: "helm", FungibilityKey: "unique", Quantity: 1},
	}}

	converted := items.EquipmentSlotsFrom(slots, func(slot *legacyEquippedItem) sim.EquippedItem {
		if slot == nil {
			t.Fatalf("nil slot passed to converter")
		}
		return sim.EquippedItem{
			Slot: sim.EquipSlot(slot.Slot),
			Item: sim.ItemStack{
				Type:           sim.ItemType(slot.Item.Type),
				FungibilityKey: slot.Item.FungibilityKey,
				Quantity:       slot.Item.Quantity,
			},
		}
	})

	if len(converted) != 1 {
		t.Fatalf("expected 1 converted slot, got %d", len(converted))
	}
	result := converted[0]
	if result.Slot != sim.EquipSlot("head") {
		t.Fatalf("unexpected equip slot %q", result.Slot)
	}
	if result.Item.Type != sim.ItemType("helm") {
		t.Fatalf("expected item type helm, got %q", result.Item.Type)
	}
	if result.Item.FungibilityKey != "unique" {
		t.Fatalf("expected fungibility key unique, got %q", result.Item.FungibilityKey)
	}
	if result.Item.Quantity != 1 {
		t.Fatalf("expected quantity 1, got %d", result.Item.Quantity)
	}

	slots[0].Item.Quantity = 7
	if result.Item.Quantity != 1 {
		t.Fatalf("expected converted slot to remain 1, got %d", result.Item.Quantity)
	}
}

func TestEquipmentSlotsFromHandlesEmpty(t *testing.T) {
	converted := items.EquipmentSlotsFrom([]legacyEquippedItem{}, func(legacyEquippedItem) sim.EquippedItem {
		t.Fatalf("unexpected conversion for empty input")
		return sim.EquippedItem{}
	})
	if converted != nil {
		t.Fatalf("expected nil result for empty slice")
	}

	var nilEquipment []*legacyEquippedItem
	var nilEquipmentConverter func(*legacyEquippedItem) sim.EquippedItem
	converted = items.EquipmentSlotsFrom(nilEquipment, nilEquipmentConverter)
	if converted != nil {
		t.Fatalf("expected nil result for nil converter")
	}
}

func TestInventorySlotsFromSupportsReverseConversion(t *testing.T) {
	slots := []sim.InventorySlot{{
		Slot: 4,
		Item: sim.ItemStack{Type: sim.ItemType("potion"), FungibilityKey: "k", Quantity: 3},
	}}

	converted := items.InventorySlotsFrom(slots, func(slot sim.InventorySlot) legacyInventorySlot {
		return legacyInventorySlot{
			Slot: slot.Slot,
			Item: legacyItemStack{
				Type:           string(slot.Item.Type),
				FungibilityKey: slot.Item.FungibilityKey,
				Quantity:       slot.Item.Quantity,
			},
		}
	})

	if len(converted) != 1 {
		t.Fatalf("expected 1 converted slot, got %d", len(converted))
	}
	result := converted[0]
	if result.Slot != 4 {
		t.Fatalf("expected slot 4, got %d", result.Slot)
	}
	if result.Item.Quantity != 3 {
		t.Fatalf("expected quantity 3, got %d", result.Item.Quantity)
	}

	slots[0].Item.Quantity = 8
	if converted[0].Item.Quantity != 3 {
		t.Fatalf("expected converted slot to remain 3, got %d", converted[0].Item.Quantity)
	}
}

func TestEquipmentSlotsFromSupportsReverseConversion(t *testing.T) {
	slots := []sim.EquippedItem{{
		Slot: sim.EquipSlot("main_hand"),
		Item: sim.ItemStack{Type: sim.ItemType("sword"), FungibilityKey: "rare", Quantity: 1},
	}}

	converted := items.EquipmentSlotsFrom(slots, func(slot sim.EquippedItem) legacyEquippedItem {
		return legacyEquippedItem{
			Slot: string(slot.Slot),
			Item: legacyItemStack{
				Type:           string(slot.Item.Type),
				FungibilityKey: slot.Item.FungibilityKey,
				Quantity:       slot.Item.Quantity,
			},
		}
	})

	if len(converted) != 1 {
		t.Fatalf("expected 1 converted slot, got %d", len(converted))
	}
	result := converted[0]
	if result.Slot != "main_hand" {
		t.Fatalf("expected main_hand slot, got %q", result.Slot)
	}
	if result.Item.FungibilityKey != "rare" {
		t.Fatalf("expected fungibility key rare, got %q", result.Item.FungibilityKey)
	}

	slots[0].Item.FungibilityKey = "changed"
	if converted[0].Item.FungibilityKey != "rare" {
		t.Fatalf("expected converted slot to remain rare, got %q", converted[0].Item.FungibilityKey)
	}
}

func TestInventoryToSimBuildsSnapshot(t *testing.T) {
	legacySlots := []legacyInventorySlot{{
		Slot: 1,
		Item: legacyItemStack{Type: "wand", FungibilityKey: "unique", Quantity: 5},
	}}

	inv := items.AssembleInventory(legacySlots, func(slot legacyInventorySlot) sim.InventorySlot {
		return sim.InventorySlot{
			Slot: slot.Slot,
			Item: sim.ItemStack{
				Type:           sim.ItemType(slot.Item.Type),
				FungibilityKey: slot.Item.FungibilityKey,
				Quantity:       slot.Item.Quantity,
			},
		}
	}, func(slots []sim.InventorySlot) sim.Inventory {
		return sim.Inventory{Slots: slots}
	})

	if len(inv.Slots) != 1 {
		t.Fatalf("expected 1 inventory slot, got %d", len(inv.Slots))
	}
	if inv.Slots[0].Item.Quantity != 5 {
		t.Fatalf("expected quantity 5, got %d", inv.Slots[0].Item.Quantity)
	}

	legacySlots[0].Item.Quantity = 99
	if inv.Slots[0].Item.Quantity != 5 {
		t.Fatalf("expected converted inventory to remain 5, got %d", inv.Slots[0].Item.Quantity)
	}

	empty := items.AssembleInventory([]legacyInventorySlot{}, func(slot legacyInventorySlot) sim.InventorySlot { return sim.InventorySlot{} }, func(slots []sim.InventorySlot) sim.Inventory {
		return sim.Inventory{Slots: slots}
	})
	if empty.Slots != nil {
		t.Fatalf("expected empty inventory to have nil slots")
	}

	nilConverter := items.AssembleInventory(legacySlots, nil, func(slots []sim.InventorySlot) sim.Inventory {
		return sim.Inventory{Slots: slots}
	})
	if nilConverter.Slots != nil {
		t.Fatalf("expected nil converter to produce nil slots")
	}
}

func TestInventoryFromSimConvertsSnapshot(t *testing.T) {
	inv := sim.Inventory{Slots: []sim.InventorySlot{{
		Slot: 3,
		Item: sim.ItemStack{Type: "salve", FungibilityKey: "stack", Quantity: 7},
	}}}

	converted := items.AssembleInventory(inv.Slots, func(slot sim.InventorySlot) legacyInventorySlot {
		return legacyInventorySlot{
			Slot: slot.Slot,
			Item: legacyItemStack{
				Type:           string(slot.Item.Type),
				FungibilityKey: slot.Item.FungibilityKey,
				Quantity:       slot.Item.Quantity,
			},
		}
	}, func(slots []legacyInventorySlot) []legacyInventorySlot {
		return slots
	})

	if len(converted) != 1 {
		t.Fatalf("expected 1 converted slot, got %d", len(converted))
	}
	if converted[0].Item.Quantity != 7 {
		t.Fatalf("expected quantity 7, got %d", converted[0].Item.Quantity)
	}

	inv.Slots[0].Item.Quantity = 12
	if converted[0].Item.Quantity != 7 {
		t.Fatalf("expected converted slot to remain 7, got %d", converted[0].Item.Quantity)
	}

	if result := items.AssembleInventory(sim.Inventory{}.Slots, func(slot sim.InventorySlot) legacyInventorySlot { return legacyInventorySlot{} }, func(slots []legacyInventorySlot) []legacyInventorySlot {
		return slots
	}); result != nil {
		t.Fatalf("expected empty inventory to return nil")
	}

	if result := items.AssembleInventory(inv.Slots, nil, func(slots []legacyInventorySlot) []legacyInventorySlot {
		return slots
	}); result != nil {
		t.Fatalf("expected nil converter to return nil")
	}
}

func TestEquipmentToSimBuildsSnapshot(t *testing.T) {
	legacySlots := []*legacyEquippedItem{{
		Slot: "Head",
		Item: legacyItemStack{Type: "helm", FungibilityKey: "rare", Quantity: 1},
	}}

	eq := items.AssembleEquipment(legacySlots, func(slot *legacyEquippedItem) sim.EquippedItem {
		if slot == nil {
			t.Fatalf("expected non-nil slot")
		}
		return sim.EquippedItem{
			Slot: sim.EquipSlot(slot.Slot),
			Item: sim.ItemStack{
				Type:           sim.ItemType(slot.Item.Type),
				FungibilityKey: slot.Item.FungibilityKey,
				Quantity:       slot.Item.Quantity,
			},
		}
	}, func(slots []sim.EquippedItem) sim.Equipment {
		return sim.Equipment{Slots: slots}
	})

	if len(eq.Slots) != 1 {
		t.Fatalf("expected 1 equipped slot, got %d", len(eq.Slots))
	}
	if eq.Slots[0].Slot != sim.EquipSlot("Head") {
		t.Fatalf("expected equip slot Head, got %q", eq.Slots[0].Slot)
	}

	legacySlots[0].Item.Quantity = 5
	if eq.Slots[0].Item.Quantity != 1 {
		t.Fatalf("expected converted equipment to remain 1, got %d", eq.Slots[0].Item.Quantity)
	}

	empty := items.AssembleEquipment([]*legacyEquippedItem{}, func(slot *legacyEquippedItem) sim.EquippedItem { return sim.EquippedItem{} }, func(slots []sim.EquippedItem) sim.Equipment {
		return sim.Equipment{Slots: slots}
	})
	if empty.Slots != nil {
		t.Fatalf("expected empty equipment to have nil slots")
	}

	nilConverter := items.AssembleEquipment(legacySlots, nil, func(slots []sim.EquippedItem) sim.Equipment {
		return sim.Equipment{Slots: slots}
	})
	if nilConverter.Slots != nil {
		t.Fatalf("expected nil converter to produce nil slots")
	}
}

func TestEquipmentFromSimConvertsSnapshot(t *testing.T) {
	eq := sim.Equipment{Slots: []sim.EquippedItem{{
		Slot: sim.EquipSlot("MainHand"),
		Item: sim.ItemStack{Type: "sword", FungibilityKey: "unique", Quantity: 1},
	}}}

	converted := items.AssembleEquipment(eq.Slots, func(slot sim.EquippedItem) legacyEquippedItem {
		return legacyEquippedItem{
			Slot: string(slot.Slot),
			Item: legacyItemStack{
				Type:           string(slot.Item.Type),
				FungibilityKey: slot.Item.FungibilityKey,
				Quantity:       slot.Item.Quantity,
			},
		}
	}, func(slots []legacyEquippedItem) []legacyEquippedItem {
		return slots
	})

	if len(converted) != 1 {
		t.Fatalf("expected 1 converted slot, got %d", len(converted))
	}
	if converted[0].Slot != "MainHand" {
		t.Fatalf("expected slot MainHand, got %q", converted[0].Slot)
	}

	eq.Slots[0].Slot = sim.EquipSlot("OffHand")
	if converted[0].Slot != "MainHand" {
		t.Fatalf("expected converted slot to remain MainHand, got %q", converted[0].Slot)
	}

	if result := items.AssembleEquipment(sim.Equipment{}.Slots, func(slot sim.EquippedItem) legacyEquippedItem { return legacyEquippedItem{} }, func(slots []legacyEquippedItem) []legacyEquippedItem {
		return slots
	}); result != nil {
		t.Fatalf("expected empty equipment to return nil")
	}

	if result := items.AssembleEquipment(eq.Slots, nil, func(slots []legacyEquippedItem) []legacyEquippedItem {
		return slots
	}); result != nil {
		t.Fatalf("expected nil converter to return nil")
	}
}
