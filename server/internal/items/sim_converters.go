package items

import "mine-and-die/server/internal/sim"

// InventorySlotFromSimConverter returns a converter that maps simulation
// inventory slots into the caller-provided representation using the supplied
// item conversion and assembler. Nil converters produce the zero value.
func InventorySlotFromSimConverter[Slot any, Item any](
	assemble func(int, Item) Slot,
	convertItem func(sim.ItemStack) Item,
) func(sim.InventorySlot) Slot {
	if assemble == nil || convertItem == nil {
		return func(sim.InventorySlot) Slot {
			var zero Slot
			return zero
		}
	}
	return func(slot sim.InventorySlot) Slot {
		return assemble(slot.Slot, convertItem(slot.Item))
	}
}

// EquippedItemFromSimConverter returns a converter that maps simulation
// equipped item slots into the caller-provided representation using the
// supplied slot and item converters. Nil converters produce the zero value.
func EquippedItemFromSimConverter[Result any, Slot any, Item any](
	assemble func(Slot, Item) Result,
	convertSlot func(sim.EquipSlot) Slot,
	convertItem func(sim.ItemStack) Item,
) func(sim.EquippedItem) Result {
	if assemble == nil || convertSlot == nil || convertItem == nil {
		return func(sim.EquippedItem) Result {
			var zero Result
			return zero
		}
	}
	return func(slot sim.EquippedItem) Result {
		return assemble(convertSlot(slot.Slot), convertItem(slot.Item))
	}
}
