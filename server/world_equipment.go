package server

import (
	"errors"
	"fmt"

	stats "mine-and-die/server/stats"
)

var (
	errEquipUnknownActor         = errors.New("unknown_actor")
	errEquipInvalidInventorySlot = errors.New("invalid_inventory_slot")
	errEquipEmptySlot            = errors.New("empty_slot")
	errEquipNotEquippable        = errors.New("not_equippable")
	errUnequipInvalidSlot        = errors.New("invalid_equip_slot")
	errUnequipEmptySlot          = errors.New("slot_empty")
)

func (w *World) EquipFromInventory(playerID string, inventorySlot int) (EquipSlot, error) {
	if w == nil {
		return "", fmt.Errorf("world not initialised")
	}
	player, ok := w.players[playerID]
	if !ok {
		return "", errEquipUnknownActor
	}
	if inventorySlot < 0 || inventorySlot >= len(player.Inventory.Slots) {
		return "", errEquipInvalidInventorySlot
	}
	slot := player.Inventory.Slots[inventorySlot]
	if slot.Item.Quantity <= 0 || slot.Item.Type == "" {
		return "", errEquipEmptySlot
	}
	def, ok := ItemDefinitionFor(slot.Item.Type)
	if !ok {
		return "", fmt.Errorf("unknown item type %q", slot.Item.Type)
	}
	if def.EquipSlot == "" {
		return "", errEquipNotEquippable
	}

	var removed ItemStack
	if err := w.mutateActorInventory(&player.actorState, &player.version, playerID, PatchPlayerInventory, func(inv *Inventory) error {
		var innerErr error
		removed, innerErr = inv.RemoveQuantity(inventorySlot, 1)
		return innerErr
	}); err != nil {
		return "", err
	}
	if removed.FungibilityKey == "" {
		removed.FungibilityKey = def.FungibilityKey
	}
	removed.Quantity = 1

	slotKey := stats.SourceKey{Kind: stats.SourceKindEquipment, ID: string(def.EquipSlot)}

	restoreRemoved := func() {
		_ = w.mutateActorInventory(&player.actorState, &player.version, playerID, PatchPlayerInventory, func(inv *Inventory) error {
			_, addErr := inv.AddStack(removed)
			return addErr
		})
	}

	var (
		reinsertionSlot   int
		reinsertionQty    int
		reinsertionActive bool
		previous          ItemStack
	)

	if current, ok := player.Equipment.Get(def.EquipSlot); ok && current.Type != "" {
		previous = current
		if previous.Quantity <= 0 {
			previous.Quantity = 1
		}
		reinsertionQty = previous.Quantity
		if prevDef, ok := ItemDefinitionFor(previous.Type); ok {
			if previous.FungibilityKey == "" {
				previous.FungibilityKey = prevDef.FungibilityKey
			}
		}

		if err := w.mutateActorInventory(&player.actorState, &player.version, playerID, PatchPlayerInventory, func(inv *Inventory) error {
			var addErr error
			reinsertionSlot, addErr = inv.AddStack(previous)
			return addErr
		}); err != nil {
			restoreRemoved()
			return "", err
		}
		reinsertionActive = true

		if err := w.MutateEquipment(playerID, func(eq *Equipment) error {
			if _, ok := eq.Remove(def.EquipSlot); !ok {
				return fmt.Errorf("slot %s empty during equip", def.EquipSlot)
			}
			return nil
		}); err != nil {
			if reinsertionActive {
				_ = w.mutateActorInventory(&player.actorState, &player.version, playerID, PatchPlayerInventory, func(inv *Inventory) error {
					_, remErr := inv.RemoveQuantity(reinsertionSlot, reinsertionQty)
					return remErr
				})
			}
			restoreRemoved()
			return "", err
		}

	}

	if err := w.MutateEquipment(playerID, func(eq *Equipment) error {
		eq.Set(def.EquipSlot, removed)
		return nil
	}); err != nil {
		restoreRemoved()
		if reinsertionActive {
			_ = w.mutateActorInventory(&player.actorState, &player.version, playerID, PatchPlayerInventory, func(inv *Inventory) error {
				_, remErr := inv.RemoveQuantity(reinsertionSlot, reinsertionQty)
				return remErr
			})
			_ = w.MutateEquipment(playerID, func(eq *Equipment) error {
				eq.Set(def.EquipSlot, previous)
				return nil
			})
		}
		return "", err
	}

	if reinsertionActive {
		player.stats.Apply(stats.CommandStatChange{Layer: stats.LayerEquipment, Source: slotKey, Remove: true})
	}

	delta, err := equipmentDeltaForDefinition(def)
	if err != nil {
		return "", err
	}
	player.stats.Apply(stats.CommandStatChange{Layer: stats.LayerEquipment, Source: slotKey, Delta: delta})
	player.stats.Resolve(w.currentTick)
	w.syncMaxHealth(&player.actorState, &player.version, player.ID, PatchPlayerHealth, &player.stats)
	return def.EquipSlot, nil
}

func (w *World) UnequipToInventory(playerID string, slot EquipSlot) (ItemStack, error) {
	if w == nil {
		return ItemStack{}, fmt.Errorf("world not initialised")
	}
	if slot == "" {
		return ItemStack{}, errUnequipInvalidSlot
	}
	player, ok := w.players[playerID]
	if !ok {
		return ItemStack{}, errEquipUnknownActor
	}
	stack, ok := player.Equipment.Get(slot)
	if !ok || stack.Type == "" {
		return ItemStack{}, errUnequipEmptySlot
	}

	slotKey := stats.SourceKey{Kind: stats.SourceKindEquipment, ID: string(slot)}
	player.stats.Apply(stats.CommandStatChange{Layer: stats.LayerEquipment, Source: slotKey, Remove: true})

	if err := w.MutateEquipment(playerID, func(eq *Equipment) error {
		_, _ = eq.Remove(slot)
		return nil
	}); err != nil {
		return ItemStack{}, err
	}

	if err := w.mutateActorInventory(&player.actorState, &player.version, playerID, PatchPlayerInventory, func(inv *Inventory) error {
		_, addErr := inv.AddStack(stack)
		return addErr
	}); err != nil {
		return ItemStack{}, err
	}

	player.stats.Resolve(w.currentTick)
	w.syncMaxHealth(&player.actorState, &player.version, player.ID, PatchPlayerHealth, &player.stats)
	return stack, nil
}

func (w *World) drainEquipment(actor *actorState, version *uint64, entityID string, equipPatchKind PatchKind, healthPatchKind PatchKind, comp *stats.Component) []ItemStack {
	if w == nil || actor == nil || version == nil || entityID == "" || comp == nil {
		return nil
	}

	var drained []EquippedItem
	_ = w.mutateActorEquipment(actor, version, entityID, equipPatchKind, func(eq *Equipment) error {
		drained = eq.DrainAll()
		return nil
	})

	if len(drained) == 0 {
		return nil
	}

	for _, entry := range drained {
		slotKey := stats.SourceKey{Kind: stats.SourceKindEquipment, ID: string(entry.Slot)}
		comp.Apply(stats.CommandStatChange{Layer: stats.LayerEquipment, Source: slotKey, Remove: true})
	}

	comp.Resolve(w.currentTick)
	w.syncMaxHealth(actor, version, entityID, healthPatchKind, comp)

	items := make([]ItemStack, 0, len(drained))
	for _, entry := range drained {
		if entry.Item.Type == "" || entry.Item.Quantity <= 0 {
			continue
		}
		items = append(items, entry.Item)
	}
	return items
}
