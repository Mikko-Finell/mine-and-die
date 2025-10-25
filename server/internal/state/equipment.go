package state

import "sort"

// EquippedItem stores the item occupying a specific equipment slot.
type EquippedItem struct {
	Slot EquipSlot `json:"slot"`
	Item ItemStack `json:"item"`
}

// Equipment holds the deterministic equipped item list for an actor.
type Equipment struct {
	Slots []EquippedItem `json:"slots,omitempty"`
}

// NewEquipment returns an empty equipment container.
func NewEquipment() Equipment {
	return Equipment{Slots: nil}
}

func (e Equipment) Clone() Equipment {
	if len(e.Slots) == 0 {
		return Equipment{}
	}
	cloned := make([]EquippedItem, len(e.Slots))
	copy(cloned, e.Slots)
	return Equipment{Slots: cloned}
}

func (e *Equipment) Get(slot EquipSlot) (ItemStack, bool) {
	if e == nil {
		return ItemStack{}, false
	}
	for _, entry := range e.Slots {
		if entry.Slot == slot {
			return entry.Item, true
		}
	}
	return ItemStack{}, false
}

func (e *Equipment) Set(slot EquipSlot, stack ItemStack) {
	if e == nil {
		return
	}
	if stack.Quantity <= 0 {
		stack.Quantity = 1
	}
	for i := range e.Slots {
		if e.Slots[i].Slot == slot {
			e.Slots[i].Item = stack
			return
		}
	}
	e.Slots = append(e.Slots, EquippedItem{Slot: slot, Item: stack})
	e.sortSlots()
}

func (e *Equipment) Remove(slot EquipSlot) (ItemStack, bool) {
	if e == nil || len(e.Slots) == 0 {
		return ItemStack{}, false
	}
	for i := range e.Slots {
		if e.Slots[i].Slot != slot {
			continue
		}
		removed := e.Slots[i].Item
		e.Slots = append(e.Slots[:i], e.Slots[i+1:]...)
		return removed, true
	}
	return ItemStack{}, false
}

func (e *Equipment) DrainAll() []EquippedItem {
	if e == nil || len(e.Slots) == 0 {
		return nil
	}
	drained := make([]EquippedItem, len(e.Slots))
	copy(drained, e.Slots)
	e.Slots = nil
	return drained
}

func (e *Equipment) sortSlots() {
	if len(e.Slots) <= 1 {
		return
	}
	sort.Slice(e.Slots, func(i, j int) bool {
		ai := EquipSlotRank(e.Slots[i].Slot)
		bj := EquipSlotRank(e.Slots[j].Slot)
		if ai == bj {
			return string(e.Slots[i].Slot) < string(e.Slots[j].Slot)
		}
		return ai < bj
	})
}

func EquipmentsEqual(a, b Equipment) bool {
	if len(a.Slots) != len(b.Slots) {
		return false
	}
	for i := range a.Slots {
		if a.Slots[i].Slot != b.Slots[i].Slot {
			return false
		}
		if a.Slots[i].Item != b.Slots[i].Item {
			return false
		}
	}
	return true
}

func CloneEquipmentSlots(slots []EquippedItem) []EquippedItem {
	if len(slots) == 0 {
		return nil
	}
	cloned := make([]EquippedItem, len(slots))
	copy(cloned, slots)
	return cloned
}

var orderedEquipSlots = []EquipSlot{
	EquipSlotMainHand,
	EquipSlotOffHand,
	EquipSlotHead,
	EquipSlotBody,
	EquipSlotGloves,
	EquipSlotBoots,
	EquipSlotAccessory,
}

var equipSlotToRank = func() map[EquipSlot]int {
	ranks := make(map[EquipSlot]int, len(orderedEquipSlots))
	for idx, slot := range orderedEquipSlots {
		ranks[slot] = idx
	}
	return ranks
}()

func EquipSlotRank(slot EquipSlot) int {
	if rank, ok := equipSlotToRank[slot]; ok {
		return rank
	}
	return len(orderedEquipSlots)
}

func EquipSlotFromOrdinal(idx int) (EquipSlot, bool) {
	if idx < 0 || idx >= len(orderedEquipSlots) {
		return "", false
	}
	return orderedEquipSlots[idx], true
}
