package items

import "mine-and-die/server/internal/sim"

// InventoryFromSimSlots clones the provided simulation inventory slots and assembles
// a `sim.Inventory` snapshot.
func InventoryFromSimSlots(slots []sim.InventorySlot) sim.Inventory {
	cloned := CloneInventorySlots(slots)
	if len(cloned) == 0 {
		return sim.Inventory{}
	}
	return sim.Inventory{Slots: cloned}
}

// EquipmentFromSimSlots clones the provided simulation equipment slots and assembles
// a `sim.Equipment` snapshot.
func EquipmentFromSimSlots(slots []sim.EquippedItem) sim.Equipment {
	cloned := CloneEquippedItems(slots)
	if len(cloned) == 0 {
		return sim.Equipment{}
	}
	return sim.Equipment{Slots: cloned}
}

// CloneInventorySlots returns a deep copy of the provided inventory slot slice.
func CloneInventorySlots(slots []sim.InventorySlot) []sim.InventorySlot {
	if len(slots) == 0 {
		return nil
	}
	cloned := make([]sim.InventorySlot, len(slots))
	copy(cloned, slots)
	return cloned
}

// CloneEquippedItems returns a deep copy of the provided equipped item slice.
func CloneEquippedItems(slots []sim.EquippedItem) []sim.EquippedItem {
	if len(slots) == 0 {
		return nil
	}
	cloned := make([]sim.EquippedItem, len(slots))
	copy(cloned, slots)
	return cloned
}
