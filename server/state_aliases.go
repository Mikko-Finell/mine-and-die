package server

import state "mine-and-die/server/internal/state"

type (
	ItemType               = state.ItemType
	ItemClass              = state.ItemClass
	EquipSlot              = state.EquipSlot
	ItemAction             = state.ItemAction
	ItemModifier           = state.ItemModifier
	ItemDefinition         = state.ItemDefinition
	ItemDefinitionParams   = state.ItemDefinitionParams
	ItemStack              = state.ItemStack
	InventorySlot          = state.InventorySlot
	Inventory              = state.Inventory
	EquippedItem           = state.EquippedItem
	Equipment              = state.Equipment
	FacingDirection        = state.FacingDirection
	Actor                  = state.Actor
	Player                 = state.Player
	NPCType                = state.NPCType
	NPC                    = state.NPC
	StatusEffectType       = state.StatusEffectType
	StatusEffectDefinition = state.StatusEffectDefinition
)

const (
	ItemClassWeapon            = state.ItemClassWeapon
	ItemClassShield            = state.ItemClassShield
	ItemClassArmor             = state.ItemClassArmor
	ItemClassAccessory         = state.ItemClassAccessory
	ItemClassCoating           = state.ItemClassCoating
	ItemClassConsumable        = state.ItemClassConsumable
	ItemClassThrowable         = state.ItemClassThrowable
	ItemClassTrap              = state.ItemClassTrap
	ItemClassBlock             = state.ItemClassBlock
	ItemClassContainer         = state.ItemClassContainer
	ItemClassTool              = state.ItemClassTool
	ItemClassProcessedMaterial = state.ItemClassProcessedMaterial

	EquipSlotMainHand  = state.EquipSlotMainHand
	EquipSlotOffHand   = state.EquipSlotOffHand
	EquipSlotHead      = state.EquipSlotHead
	EquipSlotBody      = state.EquipSlotBody
	EquipSlotGloves    = state.EquipSlotGloves
	EquipSlotBoots     = state.EquipSlotBoots
	EquipSlotAccessory = state.EquipSlotAccessory

	ItemActionAttack   = state.ItemActionAttack
	ItemActionSpecial  = state.ItemActionSpecial
	ItemActionCharge   = state.ItemActionCharge
	ItemActionBlock    = state.ItemActionBlock
	ItemActionBash     = state.ItemActionBash
	ItemActionActivate = state.ItemActionActivate
	ItemActionConsume  = state.ItemActionConsume
	ItemActionApply    = state.ItemActionApply
	ItemActionThrow    = state.ItemActionThrow
	ItemActionPlace    = state.ItemActionPlace
	ItemActionMine     = state.ItemActionMine
	ItemActionHarvest  = state.ItemActionHarvest
	ItemActionOpen     = state.ItemActionOpen
	ItemActionLock     = state.ItemActionLock
	ItemActionUnlock   = state.ItemActionUnlock
	ItemActionPickup   = state.ItemActionPickup
	ItemActionRemove   = state.ItemActionRemove
	ItemActionRepair   = state.ItemActionRepair

	ItemTypeGold          = state.ItemTypeGold
	ItemTypeHealthPotion  = state.ItemTypeHealthPotion
	ItemTypeRatTail       = state.ItemTypeRatTail
	ItemTypeIronDagger    = state.ItemTypeIronDagger
	ItemTypeLeatherJerkin = state.ItemTypeLeatherJerkin
	ItemTypeTravelerCharm = state.ItemTypeTravelerCharm
	ItemTypeVenomCoating  = state.ItemTypeVenomCoating
	ItemTypeBlastingOrb   = state.ItemTypeBlastingOrb
	ItemTypeRefinedOre    = state.ItemTypeRefinedOre

	FacingUp    = state.FacingUp
	FacingDown  = state.FacingDown
	FacingLeft  = state.FacingLeft
	FacingRight = state.FacingRight

	defaultFacing = state.DefaultFacing

	NPCTypeGoblin = state.NPCTypeGoblin
	NPCTypeRat    = state.NPCTypeRat
)

var (
	NewItemDefinition      = state.NewItemDefinition
	ComposeFungibilityKey  = state.ComposeFungibilityKey
	MarshalItemDefinitions = state.MarshalItemDefinitions
	ItemDefinitionFor      = state.ItemDefinitionFor
	ItemDefinitions        = state.ItemDefinitions

	NewInventory = state.NewInventory
	NewEquipment = state.NewEquipment

	parseFacing    = state.ParseFacing
	deriveFacing   = state.DeriveFacing
	facingToVector = state.FacingToVector

	equipmentDeltaForDefinition = state.EquipmentDeltaForDefinition
	equipSlotFromOrdinal        = state.EquipSlotFromOrdinal
	equipmentsEqual             = state.EquipmentsEqual
	cloneEquipmentSlots         = state.CloneEquipmentSlots
)

type statusEffectInstance = state.StatusEffectInstance
