package sim

// ItemType represents a unique identifier for an item kind.
type ItemType = string

// ItemStack represents a quantity of a specific item type and fungibility key.
type ItemStack struct {
	Type           ItemType `json:"type"`
	FungibilityKey string   `json:"fungibility_key"`
	Quantity       int      `json:"quantity"`
}

// InventorySlot stores an item stack at a specific position.
type InventorySlot struct {
	Slot int       `json:"slot"`
	Item ItemStack `json:"item"`
}

// Inventory maintains an ordered list of slots.
type Inventory struct {
	Slots []InventorySlot `json:"slots"`
}
