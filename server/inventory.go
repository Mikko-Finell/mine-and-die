package server

import "fmt"

// ItemStack represents a quantity of a specific item type and fungibility key.
type ItemStack struct {
	Type           ItemType `json:"type"`
	FungibilityKey string   `json:"fungibility_key"`
	Quantity       int      `json:"quantity"`
}

// InventorySlot stores an item stack at a specific position. The slot index is
// always kept in sync with the position inside the inventory slice.
type InventorySlot struct {
	Slot int       `json:"slot"`
	Item ItemStack `json:"item"`
}

// Inventory maintains an ordered list of slots. Order matters to allow players
// to arrange their equipment however they prefer.
type Inventory struct {
	Slots []InventorySlot `json:"slots"`
}

// NewInventory returns an empty inventory with no slots.
func NewInventory() Inventory {
	return Inventory{Slots: make([]InventorySlot, 0)}
}

// Clone performs a deep copy of the inventory and all slots.
func (inv Inventory) Clone() Inventory {
	if len(inv.Slots) == 0 {
		return Inventory{Slots: nil}
	}
	slots := make([]InventorySlot, len(inv.Slots))
	copy(slots, inv.Slots)
	return Inventory{Slots: slots}
}

// AddStack merges stackable items and returns the slot index that was affected.
func (inv *Inventory) AddStack(stack ItemStack) (int, error) {
	if stack.Quantity <= 0 {
		return -1, fmt.Errorf("quantity must be positive, got %d", stack.Quantity)
	}
	def, ok := ItemDefinitionFor(stack.Type)
	if !ok {
		return -1, fmt.Errorf("unknown item type %q", stack.Type)
	}

	if stack.FungibilityKey == "" {
		stack.FungibilityKey = def.FungibilityKey
	}
	if stack.FungibilityKey != def.FungibilityKey {
		return -1, fmt.Errorf("fungibility key %q does not match definition %q", stack.FungibilityKey, def.FungibilityKey)
	}

	if def.Stackable {
		for i := range inv.Slots {
			if inv.Slots[i].Item.FungibilityKey != stack.FungibilityKey {
				continue
			}
			inv.Slots[i].Item.Quantity += stack.Quantity
			return inv.Slots[i].Slot, nil
		}
	}

	slot := InventorySlot{Slot: len(inv.Slots), Item: stack}
	inv.Slots = append(inv.Slots, slot)
	return slot.Slot, nil
}

// MoveSlot reorders an item to a new index while preserving slot metadata.
func (inv *Inventory) MoveSlot(from, to int) error {
	if from < 0 || from >= len(inv.Slots) {
		return fmt.Errorf("from index %d out of range", from)
	}
	if to < 0 || to >= len(inv.Slots) {
		return fmt.Errorf("to index %d out of range", to)
	}
	if from == to {
		return nil
	}

	slot := inv.Slots[from]
	inv.Slots = append(inv.Slots[:from], inv.Slots[from+1:]...)

	// Insert at the new position.
	if to >= len(inv.Slots) {
		inv.Slots = append(inv.Slots, slot)
	} else {
		inv.Slots = append(inv.Slots[:to], append([]InventorySlot{slot}, inv.Slots[to:]...)...)
	}

	for i := range inv.Slots {
		inv.Slots[i].Slot = i
	}

	return nil
}

// RemoveQuantity subtracts an amount from the given slot. If the stack reaches
// zero it is removed entirely.
func (inv *Inventory) RemoveQuantity(slotIndex int, quantity int) (ItemStack, error) {
	if slotIndex < 0 || slotIndex >= len(inv.Slots) {
		return ItemStack{}, fmt.Errorf("slot %d out of range", slotIndex)
	}
	if quantity <= 0 {
		return ItemStack{}, fmt.Errorf("quantity must be positive, got %d", quantity)
	}

	slot := &inv.Slots[slotIndex]
	if quantity > slot.Item.Quantity {
		return ItemStack{}, fmt.Errorf("not enough quantity in slot %d", slotIndex)
	}

	slot.Item.Quantity -= quantity
	removed := ItemStack{Type: slot.Item.Type, FungibilityKey: slot.Item.FungibilityKey, Quantity: quantity}

	if slot.Item.Quantity == 0 {
		inv.Slots = append(inv.Slots[:slotIndex], inv.Slots[slotIndex+1:]...)
		for i := range inv.Slots {
			inv.Slots[i].Slot = i
		}
	}

	return removed, nil
}

// QuantityOf returns the total quantity of an item type across all slots.
func (inv Inventory) QuantityOf(itemType ItemType) int {
	total := 0
	for _, slot := range inv.Slots {
		if slot.Item.Type != itemType {
			continue
		}
		if slot.Item.Quantity > 0 {
			total += slot.Item.Quantity
		}
	}
	return total
}

// RemoveAllOf removes every stack of the provided item type and returns the stacks removed.
func (inv *Inventory) RemoveAllOf(itemType ItemType) []ItemStack {
	if inv == nil {
		return nil
	}
	var removed []ItemStack
	for i := len(inv.Slots) - 1; i >= 0; i-- {
		slot := inv.Slots[i]
		if slot.Item.Type != itemType {
			continue
		}
		qty := slot.Item.Quantity
		if qty <= 0 {
			continue
		}
		if stack, err := inv.RemoveQuantity(i, qty); err == nil && stack.Quantity > 0 {
			removed = append(removed, stack)
		}
	}
	return removed
}

// DrainAll removes every stack from the inventory, returning the collected items.
func (inv *Inventory) DrainAll() []ItemStack {
	if inv == nil {
		return nil
	}
	if len(inv.Slots) == 0 {
		inv.Slots = nil
		return nil
	}
	drained := make([]ItemStack, 0, len(inv.Slots))
	for _, slot := range inv.Slots {
		stack := slot.Item
		if stack.Type == "" || stack.Quantity <= 0 {
			continue
		}
		drained = append(drained, stack)
	}
	inv.Slots = nil
	return drained
}

// RemoveItemTypeQuantity subtracts a specific quantity of the given item type across slots.
func (inv *Inventory) RemoveItemTypeQuantity(itemType ItemType, quantity int) (int, error) {
	if inv == nil {
		return 0, fmt.Errorf("inventory is nil")
	}
	if quantity <= 0 {
		return 0, fmt.Errorf("quantity must be positive, got %d", quantity)
	}
	remaining := quantity
	for i := len(inv.Slots) - 1; i >= 0 && remaining > 0; i-- {
		slot := inv.Slots[i]
		if slot.Item.Type != itemType {
			continue
		}
		available := slot.Item.Quantity
		if available <= 0 {
			continue
		}
		take := available
		if take > remaining {
			take = remaining
		}
		if _, err := inv.RemoveQuantity(i, take); err != nil {
			return quantity - remaining, err
		}
		remaining -= take
	}
	if remaining > 0 {
		return quantity - remaining, fmt.Errorf("not enough quantity of %s", itemType)
	}
	return quantity, nil
}
