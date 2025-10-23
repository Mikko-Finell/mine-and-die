package items

import "mine-and-die/server/internal/sim"

// InventoryFromSimSlots clones the provided simulation inventory slots and assembles
// a `sim.Inventory` snapshot.
func InventoryFromSimSlots(slots []sim.InventorySlot) sim.Inventory {
	cloned := cloneSimInventorySlots(slots)
	if len(cloned) == 0 {
		return sim.Inventory{}
	}
	return sim.Inventory{Slots: cloned}
}

// EquipmentFromSimSlots clones the provided simulation equipment slots and assembles
// a `sim.Equipment` snapshot.
func EquipmentFromSimSlots(slots []sim.EquippedItem) sim.Equipment {
	cloned := cloneSimEquippedItems(slots)
	if len(cloned) == 0 {
		return sim.Equipment{}
	}
	return sim.Equipment{Slots: cloned}
}

func cloneSimInventorySlots(slots []sim.InventorySlot) []sim.InventorySlot {
	if len(slots) == 0 {
		return nil
	}
	cloned := make([]sim.InventorySlot, len(slots))
	copy(cloned, slots)
	return cloned
}

func cloneSimEquippedItems(slots []sim.EquippedItem) []sim.EquippedItem {
	if len(slots) == 0 {
		return nil
	}
	cloned := make([]sim.EquippedItem, len(slots))
	copy(cloned, slots)
	return cloned
}
