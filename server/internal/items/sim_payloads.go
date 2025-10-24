package items

import "mine-and-die/server/internal/sim"

// InventoryFromSim assembles an inventory value from the provided simulation
// inventory using the supplied slot converter and assembler. Nil converters or
// assemblers return the zero value.
func InventoryFromSim[Slot any, Inventory any](
	inv sim.Inventory,
	convert func(sim.InventorySlot) Slot,
	assemble func([]Slot) Inventory,
) Inventory {
	if convert == nil || assemble == nil {
		var zero Inventory
		return zero
	}
	slots := mapSlots(inv.Slots, convert)
	if len(slots) == 0 {
		var zero Inventory
		return zero
	}
	return assemble(slots)
}

// EquipmentFromSim assembles an equipment value from the provided simulation
// equipment using the supplied slot converter and assembler. Nil converters or
// assemblers return the zero value.
func EquipmentFromSim[Slot any, Equipment any](
	eq sim.Equipment,
	convert func(sim.EquippedItem) Slot,
	assemble func([]Slot) Equipment,
) Equipment {
	if convert == nil || assemble == nil {
		var zero Equipment
		return zero
	}
	slots := mapSlots(eq.Slots, convert)
	if len(slots) == 0 {
		var zero Equipment
		return zero
	}
	return assemble(slots)
}
