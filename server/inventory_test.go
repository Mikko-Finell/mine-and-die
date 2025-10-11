package main

import "testing"

func TestInventoryAddStackMergesByFungibilityKey(t *testing.T) {
	inv := NewInventory()

	def, ok := ItemDefinitionFor(ItemTypeGold)
	if !ok {
		t.Fatalf("missing item definition for gold")
	}

	slot, err := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 10})
	if err != nil {
		t.Fatalf("unexpected error adding first stack: %v", err)
	}
	if slot != 0 {
		t.Fatalf("expected first stack to occupy slot 0, got %d", slot)
	}
	if len(inv.Slots) != 1 {
		t.Fatalf("expected inventory to have 1 slot, got %d", len(inv.Slots))
	}
	if inv.Slots[slot].Item.FungibilityKey != def.FungibilityKey {
		t.Fatalf("expected fungibility key %q, got %q", def.FungibilityKey, inv.Slots[slot].Item.FungibilityKey)
	}

	mergedSlot, err := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 5})
	if err != nil {
		t.Fatalf("unexpected error merging stack: %v", err)
	}
	if mergedSlot != slot {
		t.Fatalf("expected merged stack to use original slot %d, got %d", slot, mergedSlot)
	}
	if inv.Slots[slot].Item.Quantity != 15 {
		t.Fatalf("expected merged quantity of 15, got %d", inv.Slots[slot].Item.Quantity)
	}
}

func TestInventoryAddStackRejectsMismatchedFungibilityKey(t *testing.T) {
	inv := NewInventory()
	if _, err := inv.AddStack(ItemStack{Type: ItemTypeGold, FungibilityKey: "custom", Quantity: 1}); err == nil {
		t.Fatalf("expected error when fungibility key does not match definition")
	}
}

func TestInventoryNonStackableCreatesNewSlot(t *testing.T) {
	inv := NewInventory()
	if _, err := inv.AddStack(ItemStack{Type: ItemTypeIronDagger, Quantity: 1}); err != nil {
		t.Fatalf("unexpected error adding dagger: %v", err)
	}
	if _, err := inv.AddStack(ItemStack{Type: ItemTypeIronDagger, Quantity: 1}); err != nil {
		t.Fatalf("unexpected error adding second dagger: %v", err)
	}
	if len(inv.Slots) != 2 {
		t.Fatalf("expected non-stackable item to occupy separate slots, got %d", len(inv.Slots))
	}
}

func TestInventoryMoveSlotUpdatesOrder(t *testing.T) {
	inv := NewInventory()
	if _, err := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 1}); err != nil {
		t.Fatalf("unexpected error adding gold: %v", err)
	}
	if _, err := inv.AddStack(ItemStack{Type: ItemTypeHealthPotion, Quantity: 1}); err != nil {
		t.Fatalf("unexpected error adding potion: %v", err)
	}

	if err := inv.MoveSlot(0, 1); err != nil {
		t.Fatalf("unexpected error moving slot: %v", err)
	}

	if inv.Slots[0].Item.Type != ItemTypeHealthPotion {
		t.Fatalf("expected potion in slot 0 after move, found %s", inv.Slots[0].Item.Type)
	}
	if inv.Slots[0].Slot != 0 || inv.Slots[1].Slot != 1 {
		t.Fatalf("expected slots to be renumbered to 0 and 1, got %d and %d", inv.Slots[0].Slot, inv.Slots[1].Slot)
	}
}

func TestInventoryRemoveQuantityRemovesEmptySlot(t *testing.T) {
	inv := NewInventory()
	if _, err := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 3}); err != nil {
		t.Fatalf("unexpected error adding stack: %v", err)
	}

	removed, err := inv.RemoveQuantity(0, 2)
	if err != nil {
		t.Fatalf("unexpected error removing quantity: %v", err)
	}
	if removed.Quantity != 2 {
		t.Fatalf("expected to remove quantity 2, removed %d", removed.Quantity)
	}
	if inv.Slots[0].Item.Quantity != 1 {
		t.Fatalf("expected remaining quantity 1, got %d", inv.Slots[0].Item.Quantity)
	}

	removed, err = inv.RemoveQuantity(0, 1)
	if err != nil {
		t.Fatalf("unexpected error removing final quantity: %v", err)
	}
	if len(inv.Slots) != 0 {
		t.Fatalf("expected inventory to be empty after removing final item, have %d slots", len(inv.Slots))
	}
	if removed.Quantity != 1 {
		t.Fatalf("expected final removal quantity 1, got %d", removed.Quantity)
	}
}

func TestInventoryCloneCreatesDeepCopy(t *testing.T) {
	inv := NewInventory()
	if _, err := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 5}); err != nil {
		t.Fatalf("unexpected error adding stack: %v", err)
	}

	clone := inv.Clone()
	clone.Slots[0].Item.Quantity = 99

	if inv.Slots[0].Item.Quantity != 5 {
		t.Fatalf("expected original inventory to remain unchanged, got %d", inv.Slots[0].Item.Quantity)
	}
}

func TestInventoryDrainAllClearsSlots(t *testing.T) {
	inv := NewInventory()
	if _, err := inv.AddStack(ItemStack{Type: ItemTypeGold, Quantity: 5}); err != nil {
		t.Fatalf("unexpected error adding gold: %v", err)
	}
	if _, err := inv.AddStack(ItemStack{Type: ItemTypeHealthPotion, Quantity: 2}); err != nil {
		t.Fatalf("unexpected error adding potion: %v", err)
	}

	drained := inv.DrainAll()
	if len(inv.Slots) != 0 {
		t.Fatalf("expected inventory to be empty after drain, have %d slots", len(inv.Slots))
	}
	if len(drained) != 2 {
		t.Fatalf("expected two drained stacks, got %d", len(drained))
	}
	totals := map[ItemType]int{}
	for _, stack := range drained {
		totals[stack.Type] += stack.Quantity
		if stack.FungibilityKey == "" {
			t.Fatalf("expected drained stack to include fungibility key")
		}
	}
	if totals[ItemTypeGold] != 5 {
		t.Fatalf("expected drained gold quantity 5, got %d", totals[ItemTypeGold])
	}
	if totals[ItemTypeHealthPotion] != 2 {
		t.Fatalf("expected drained potion quantity 2, got %d", totals[ItemTypeHealthPotion])
	}
}
