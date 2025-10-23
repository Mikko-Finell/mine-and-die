package items

// InventorySlotsFrom converts the provided inventory slots into the desired
// representation using the supplied converter. Nil converters return nil.
func InventorySlotsFrom[T any, Slot any](slots []T, convert func(T) Slot) []Slot {
	return mapSlots(slots, convert)
}

// EquipmentSlotsFrom converts the provided equipment slots into the desired
// representation using the supplied converter. Nil converters return nil.
func EquipmentSlotsFrom[T any, Slot any](slots []T, convert func(T) Slot) []Slot {
	return mapSlots(slots, convert)
}

// AssembleInventory maps the provided slots and hands the converted slice to
// the assemble function. Nil assemblers return the zero value.
func AssembleInventory[From any, To any, Inventory any](
	slots []From,
	convert func(From) To,
	assemble func([]To) Inventory,
) Inventory {
	if assemble == nil {
		var zero Inventory
		return zero
	}
	return assemble(InventorySlotsFrom(slots, convert))
}

// AssembleEquipment maps the provided slots and hands the converted slice to
// the assemble function. Nil assemblers return the zero value.
func AssembleEquipment[From any, To any, Equipment any](
	slots []From,
	convert func(From) To,
	assemble func([]To) Equipment,
) Equipment {
	if assemble == nil {
		var zero Equipment
		return zero
	}
	return assemble(EquipmentSlotsFrom(slots, convert))
}

func mapSlots[T any, Slot any](slots []T, convert func(T) Slot) []Slot {
	if len(slots) == 0 || convert == nil {
		return nil
	}
	converted := make([]Slot, len(slots))
	for i, slot := range slots {
		converted[i] = convert(slot)
	}
	return converted
}
