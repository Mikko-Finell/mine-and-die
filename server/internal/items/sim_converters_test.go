package items_test

import (
	"testing"

	itemspkg "mine-and-die/server/internal/items"
	"mine-and-die/server/internal/sim"
)

type converterInventorySlot struct {
	Slot int
	Item converterItemStack
}

type converterEquippedItem struct {
	Slot converterEquipSlot
	Item converterItemStack
}

type converterItemStack struct {
	Type           string
	FungibilityKey string
	Quantity       int
}

type converterEquipSlot string

func TestInventorySlotFromSimConverter(t *testing.T) {
	convert := itemspkg.InventorySlotFromSimConverter[converterInventorySlot, converterItemStack](
		func(index int, item converterItemStack) converterInventorySlot {
			return converterInventorySlot{Slot: index, Item: item}
		},
		func(stack sim.ItemStack) converterItemStack {
			return converterItemStack{
				Type:           string(stack.Type),
				FungibilityKey: stack.FungibilityKey,
				Quantity:       stack.Quantity,
			}
		},
	)

	slot := convert(sim.InventorySlot{
		Slot: 3,
		Item: sim.ItemStack{Type: "arrow", FungibilityKey: "stack", Quantity: 5},
	})

	if slot.Slot != 3 {
		t.Fatalf("expected slot index 3, got %d", slot.Slot)
	}
	if slot.Item.Type != "arrow" || slot.Item.FungibilityKey != "stack" || slot.Item.Quantity != 5 {
		t.Fatalf("unexpected item conversion: %#v", slot.Item)
	}
}

func TestInventorySlotFromSimConverterNil(t *testing.T) {
	convert := itemspkg.InventorySlotFromSimConverter[converterInventorySlot, converterItemStack](nil, nil)

	slot := convert(sim.InventorySlot{Slot: 1})
	if slot != (converterInventorySlot{}) {
		t.Fatalf("expected zero value, got %#v", slot)
	}
}

func TestEquippedItemFromSimConverter(t *testing.T) {
	convert := itemspkg.EquippedItemFromSimConverter[converterEquippedItem, converterEquipSlot, converterItemStack](
		func(slot converterEquipSlot, item converterItemStack) converterEquippedItem {
			return converterEquippedItem{Slot: slot, Item: item}
		},
		func(slot sim.EquipSlot) converterEquipSlot { return converterEquipSlot(slot) },
		func(stack sim.ItemStack) converterItemStack {
			return converterItemStack{
				Type:           string(stack.Type),
				FungibilityKey: stack.FungibilityKey,
				Quantity:       stack.Quantity,
			}
		},
	)

	item := convert(sim.EquippedItem{
		Slot: sim.EquipSlot("head"),
		Item: sim.ItemStack{Type: "helm", FungibilityKey: "unique", Quantity: 1},
	})

	if item.Slot != converterEquipSlot("head") {
		t.Fatalf("expected slot 'head', got %q", item.Slot)
	}
	if item.Item.Type != "helm" || item.Item.FungibilityKey != "unique" || item.Item.Quantity != 1 {
		t.Fatalf("unexpected item conversion: %#v", item.Item)
	}
}

func TestEquippedItemFromSimConverterNil(t *testing.T) {
	convert := itemspkg.EquippedItemFromSimConverter[converterEquippedItem, converterEquipSlot, converterItemStack](nil, nil, nil)

	item := convert(sim.EquippedItem{Slot: sim.EquipSlot("head")})
	if item != (converterEquippedItem{}) {
		t.Fatalf("expected zero value, got %#v", item)
	}
}
