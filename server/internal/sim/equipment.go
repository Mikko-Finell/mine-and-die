package sim

// EquipSlot represents the seven equipable slots defined in the itemization plan.
type EquipSlot string

const (
	EquipSlotMainHand  EquipSlot = "MainHand"
	EquipSlotOffHand   EquipSlot = "OffHand"
	EquipSlotHead      EquipSlot = "Head"
	EquipSlotBody      EquipSlot = "Body"
	EquipSlotGloves    EquipSlot = "Gloves"
	EquipSlotBoots     EquipSlot = "Boots"
	EquipSlotAccessory EquipSlot = "Accessory"
)

// EquippedItem stores the item occupying a specific equipment slot.
type EquippedItem struct {
	Slot EquipSlot `json:"slot"`
	Item ItemStack `json:"item"`
}

// Equipment holds the deterministic equipped item list for an actor.
type Equipment struct {
	Slots []EquippedItem `json:"slots,omitempty"`
}
