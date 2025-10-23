package simpayloads

import (
	"reflect"

	journal "mine-and-die/server/internal/journal"
	"mine-and-die/server/internal/sim"
)

// CloneInventorySlots returns a deep copy of the provided inventory slots.
func CloneInventorySlots(slots []sim.InventorySlot) []sim.InventorySlot {
	if len(slots) == 0 {
		return nil
	}
	cloned := make([]sim.InventorySlot, len(slots))
	copy(cloned, slots)
	return cloned
}

// CloneEquippedItems returns a deep copy of the provided equipped item slots.
func CloneEquippedItems(slots []sim.EquippedItem) []sim.EquippedItem {
	if len(slots) == 0 {
		return nil
	}
	cloned := make([]sim.EquippedItem, len(slots))
	copy(cloned, slots)
	return cloned
}

// SimInventoryPayloadFromLegacy converts a legacy inventory payload into its
// simulation equivalent, cloning and normalizing the slot data so callers
// receive an independent slice backed by `sim` types.
func SimInventoryPayloadFromLegacy(payload journal.InventoryPayload) sim.InventoryPayload {
	return sim.InventoryPayload{Slots: SimInventorySlotsFromAny(payload.Slots)}
}

// SimInventoryPayloadFromLegacyPtr converts a legacy inventory payload pointer
// into its simulation equivalent. Nil pointers return nil.
func SimInventoryPayloadFromLegacyPtr(payload *journal.InventoryPayload) *sim.InventoryPayload {
	if payload == nil {
		return nil
	}
	converted := SimInventoryPayloadFromLegacy(*payload)
	return &converted
}

// SimEquipmentPayloadFromLegacy converts a legacy equipment payload into its
// simulation equivalent, cloning and normalizing the slot data so callers
// receive an independent slice backed by `sim` types.
func SimEquipmentPayloadFromLegacy(payload journal.EquipmentPayload) sim.EquipmentPayload {
	return sim.EquipmentPayload{Slots: SimEquippedItemsFromAny(payload.Slots)}
}

// SimEquipmentPayloadFromLegacyPtr converts a legacy equipment payload pointer
// into its simulation equivalent. Nil pointers return nil.
func SimEquipmentPayloadFromLegacyPtr(payload *journal.EquipmentPayload) *sim.EquipmentPayload {
	if payload == nil {
		return nil
	}
	converted := SimEquipmentPayloadFromLegacy(*payload)
	return &converted
}

// LegacyInventoryPayloadFromSim converts a simulation inventory payload into
// its legacy equivalent, cloning the slot slice so callers receive an
// independent copy.
func LegacyInventoryPayloadFromSim(payload sim.InventoryPayload) journal.InventoryPayload {
	return journal.InventoryPayload{Slots: CloneInventorySlots(payload.Slots)}
}

// LegacyInventoryPayloadFromSimPtr converts a simulation inventory payload
// pointer into its legacy equivalent. Nil pointers return nil.
func LegacyInventoryPayloadFromSimPtr(payload *sim.InventoryPayload) *journal.InventoryPayload {
	if payload == nil {
		return nil
	}
	converted := LegacyInventoryPayloadFromSim(*payload)
	return &converted
}

// LegacyEquipmentPayloadFromSim converts a simulation equipment payload into
// its legacy equivalent, cloning the slot slice so callers receive an
// independent copy.
func LegacyEquipmentPayloadFromSim(payload sim.EquipmentPayload) journal.EquipmentPayload {
	return journal.EquipmentPayload{Slots: CloneEquippedItems(payload.Slots)}
}

// LegacyEquipmentPayloadFromSimPtr converts a simulation equipment payload
// pointer into its legacy equivalent. Nil pointers return nil.
func LegacyEquipmentPayloadFromSimPtr(payload *sim.EquipmentPayload) *journal.EquipmentPayload {
	if payload == nil {
		return nil
	}
	converted := LegacyEquipmentPayloadFromSim(*payload)
	return &converted
}

// SimInventorySlotsFromAny converts an arbitrary collection of legacy
// inventory slots into their simulation equivalents. Supported shapes include
// `[]sim.InventorySlot`, `[]server.InventorySlot`, and pointer variations of
// those slices. Unrecognized values return nil.
func SimInventorySlotsFromAny(value any) []sim.InventorySlot {
	if value == nil {
		return nil
	}

	switch slots := value.(type) {
	case []sim.InventorySlot:
		return CloneInventorySlots(slots)
	case *[]sim.InventorySlot:
		if slots == nil {
			return nil
		}
		return CloneInventorySlots(*slots)
	}

	slice := reflect.ValueOf(value)
	if !slice.IsValid() {
		return nil
	}
	if slice.Kind() == reflect.Pointer {
		if slice.IsNil() {
			return nil
		}
		slice = slice.Elem()
	}
	if slice.Kind() != reflect.Slice {
		return nil
	}
	length := slice.Len()
	if length == 0 {
		return nil
	}

	converted := make([]sim.InventorySlot, length)
	for i := 0; i < length; i++ {
		entry := slice.Index(i)
		if entry.Kind() == reflect.Pointer {
			if entry.IsNil() {
				continue
			}
			entry = entry.Elem()
		}
		if entry.Kind() != reflect.Struct {
			return nil
		}
		converted[i] = sim.InventorySlot{
			Slot: int(intFromField(entry.FieldByName("Slot"))),
			Item: itemStackFromValue(entry.FieldByName("Item")),
		}
	}
	return converted
}

// SimEquippedItemsFromAny converts an arbitrary collection of legacy equipped
// item slots into their simulation equivalents. Supported shapes include
// `[]sim.EquippedItem`, `[]server.EquippedItem`, and pointer variations of
// those slices. Unrecognized values return nil.
func SimEquippedItemsFromAny(value any) []sim.EquippedItem {
	if value == nil {
		return nil
	}

	switch slots := value.(type) {
	case []sim.EquippedItem:
		return CloneEquippedItems(slots)
	case *[]sim.EquippedItem:
		if slots == nil {
			return nil
		}
		return CloneEquippedItems(*slots)
	}

	slice := reflect.ValueOf(value)
	if !slice.IsValid() {
		return nil
	}
	if slice.Kind() == reflect.Pointer {
		if slice.IsNil() {
			return nil
		}
		slice = slice.Elem()
	}
	if slice.Kind() != reflect.Slice {
		return nil
	}
	length := slice.Len()
	if length == 0 {
		return nil
	}

	converted := make([]sim.EquippedItem, length)
	for i := 0; i < length; i++ {
		entry := slice.Index(i)
		if entry.Kind() == reflect.Pointer {
			if entry.IsNil() {
				continue
			}
			entry = entry.Elem()
		}
		if entry.Kind() != reflect.Struct {
			return nil
		}
		converted[i] = sim.EquippedItem{
			Slot: sim.EquipSlot(stringFromField(entry.FieldByName("Slot"))),
			Item: itemStackFromValue(entry.FieldByName("Item")),
		}
	}
	return converted
}

func itemStackFromValue(value reflect.Value) sim.ItemStack {
	if !value.IsValid() {
		return sim.ItemStack{}
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return sim.ItemStack{}
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return sim.ItemStack{}
	}
	return sim.ItemStack{
		Type:           sim.ItemType(stringFromField(value.FieldByName("Type"))),
		FungibilityKey: stringFromField(value.FieldByName("FungibilityKey")),
		Quantity:       int(intFromField(value.FieldByName("Quantity"))),
	}
}

func stringFromField(value reflect.Value) string {
	if !value.IsValid() {
		return ""
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return ""
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.String {
		return ""
	}
	return value.String()
}

func intFromField(value reflect.Value) int64 {
	if !value.IsValid() {
		return 0
	}
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return 0
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return int64(value.Uint())
	default:
		return 0
	}
}
