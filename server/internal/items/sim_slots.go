package items

import (
	"reflect"

	"mine-and-die/server/internal/sim"
)

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
