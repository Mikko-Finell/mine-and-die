package main

import "testing"

func TestEquipFromInventoryRollsBackWhenReinsertionFails(t *testing.T) {
	hub := newHub()
	player := newTestPlayerState("equip-rollback")
	hub.world.players[player.ID] = player

	def, ok := ItemDefinitionFor(ItemTypeIronDagger)
	if !ok {
		t.Fatalf("expected definition for %q", ItemTypeIronDagger)
	}

	slot, err := player.Inventory.AddStack(ItemStack{Type: ItemTypeIronDagger, Quantity: 1})
	if err != nil {
		t.Fatalf("failed adding dagger to inventory: %v", err)
	}

	original := ItemStack{Type: ItemTypeIronDagger, FungibilityKey: def.FungibilityKey + "::custom", Quantity: 1}
	player.Equipment.Set(def.EquipSlot, original)

	returnedSlot, equipErr := hub.world.EquipFromInventory(player.ID, slot)
	if equipErr == nil {
		t.Fatalf("expected equip to fail, got success in slot %q", returnedSlot)
	}

	if qty := player.Inventory.QuantityOf(ItemTypeIronDagger); qty != 1 {
		t.Fatalf("expected inventory to keep dagger, have %d", qty)
	}

	restored, ok := player.Equipment.Get(def.EquipSlot)
	if !ok {
		t.Fatalf("expected equipment slot %q to remain populated", def.EquipSlot)
	}

	if restored.FungibilityKey != original.FungibilityKey {
		t.Fatalf("expected equipment to remain unchanged, got %q", restored.FungibilityKey)
	}
}
