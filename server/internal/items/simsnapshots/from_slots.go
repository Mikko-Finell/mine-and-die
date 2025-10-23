package simsnapshots

import "mine-and-die/server/internal/sim"

// InventoryFromSlots clones the provided simulation inventory slots and assembles
// a `sim.Inventory` snapshot.
func InventoryFromSlots(slots []sim.InventorySlot) sim.Inventory {
	cloned := cloneInventorySlots(slots)
	if len(cloned) == 0 {
		return sim.Inventory{}
	}
	return sim.Inventory{Slots: cloned}
}

// EquipmentFromSlots clones the provided simulation equipment slots and assembles
// a `sim.Equipment` snapshot.
func EquipmentFromSlots(slots []sim.EquippedItem) sim.Equipment {
	cloned := cloneEquippedItems(slots)
	if len(cloned) == 0 {
		return sim.Equipment{}
	}
	return sim.Equipment{Slots: cloned}
}

func cloneInventorySlots(slots []sim.InventorySlot) []sim.InventorySlot {
	if len(slots) == 0 {
		return nil
	}
	cloned := make([]sim.InventorySlot, len(slots))
	copy(cloned, slots)
	return cloned
}

func cloneEquippedItems(slots []sim.EquippedItem) []sim.EquippedItem {
	if len(slots) == 0 {
		return nil
	}
	cloned := make([]sim.EquippedItem, len(slots))
	copy(cloned, slots)
	return cloned
}
