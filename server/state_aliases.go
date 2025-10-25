package server

import (
       state "mine-and-die/server/internal/world/state"
)

type (
	ItemType             = state.ItemType
	ItemClass            = state.ItemClass
	EquipSlot            = state.EquipSlot
	ItemAction           = state.ItemAction
	ItemModifier         = state.ItemModifier
	ItemDefinition       = state.ItemDefinition
	ItemDefinitionParams = state.ItemDefinitionParams
	ItemStack            = state.ItemStack
	InventorySlot        = state.InventorySlot
	Inventory            = state.Inventory
	EquippedItem         = state.EquippedItem
	Equipment            = state.Equipment
	Actor                = state.Actor
	Player               = state.Player
	FacingDirection      = state.FacingDirection
	actorState           = state.ActorState
	playerState          = state.PlayerState
	playerPathState      = state.PlayerPathState
	npcState             = state.NPCState
	NPC                  = state.NPC
	NPCType              = state.NPCType
	statusEffectInstance = state.StatusEffectInstance
	StatusEffectType     = state.StatusEffectType
)

const (
	ItemClassWeapon            ItemClass = state.ItemClassWeapon
	ItemClassShield            ItemClass = state.ItemClassShield
	ItemClassArmor             ItemClass = state.ItemClassArmor
	ItemClassAccessory         ItemClass = state.ItemClassAccessory
	ItemClassCoating           ItemClass = state.ItemClassCoating
	ItemClassConsumable        ItemClass = state.ItemClassConsumable
	ItemClassThrowable         ItemClass = state.ItemClassThrowable
	ItemClassTrap              ItemClass = state.ItemClassTrap
	ItemClassBlock             ItemClass = state.ItemClassBlock
	ItemClassContainer         ItemClass = state.ItemClassContainer
	ItemClassTool              ItemClass = state.ItemClassTool
	ItemClassProcessedMaterial ItemClass = state.ItemClassProcessedMaterial

	EquipSlotMainHand  EquipSlot = state.EquipSlotMainHand
	EquipSlotOffHand   EquipSlot = state.EquipSlotOffHand
	EquipSlotHead      EquipSlot = state.EquipSlotHead
	EquipSlotBody      EquipSlot = state.EquipSlotBody
	EquipSlotGloves    EquipSlot = state.EquipSlotGloves
	EquipSlotBoots     EquipSlot = state.EquipSlotBoots
	EquipSlotAccessory EquipSlot = state.EquipSlotAccessory

	ItemActionAttack   ItemAction = state.ItemActionAttack
	ItemActionSpecial  ItemAction = state.ItemActionSpecial
	ItemActionCharge   ItemAction = state.ItemActionCharge
	ItemActionBlock    ItemAction = state.ItemActionBlock
	ItemActionBash     ItemAction = state.ItemActionBash
	ItemActionActivate ItemAction = state.ItemActionActivate
	ItemActionConsume  ItemAction = state.ItemActionConsume
	ItemActionApply    ItemAction = state.ItemActionApply
	ItemActionThrow    ItemAction = state.ItemActionThrow
	ItemActionPlace    ItemAction = state.ItemActionPlace
	ItemActionMine     ItemAction = state.ItemActionMine
	ItemActionHarvest  ItemAction = state.ItemActionHarvest
	ItemActionOpen     ItemAction = state.ItemActionOpen
	ItemActionLock     ItemAction = state.ItemActionLock
	ItemActionUnlock   ItemAction = state.ItemActionUnlock
	ItemActionPickup   ItemAction = state.ItemActionPickup
	ItemActionRemove   ItemAction = state.ItemActionRemove
	ItemActionRepair   ItemAction = state.ItemActionRepair

	FacingUp      FacingDirection = state.FacingUp
	FacingDown    FacingDirection = state.FacingDown
	FacingLeft    FacingDirection = state.FacingLeft
	FacingRight   FacingDirection = state.FacingRight
	defaultFacing FacingDirection = state.DefaultFacing

	NPCTypeGoblin NPCType = state.NPCTypeGoblin
	NPCTypeRat    NPCType = state.NPCTypeRat
)

const (
	ItemTypeGold          ItemType = state.ItemTypeGold
	ItemTypeHealthPotion  ItemType = state.ItemTypeHealthPotion
	ItemTypeRatTail       ItemType = state.ItemTypeRatTail
	ItemTypeIronDagger    ItemType = state.ItemTypeIronDagger
	ItemTypeLeatherJerkin ItemType = state.ItemTypeLeatherJerkin
	ItemTypeTravelerCharm ItemType = state.ItemTypeTravelerCharm
	ItemTypeVenomCoating  ItemType = state.ItemTypeVenomCoating
	ItemTypeBlastingOrb   ItemType = state.ItemTypeBlastingOrb
	ItemTypeRefinedOre    ItemType = state.ItemTypeRefinedOre
)

var (
	NewItemDefinition           = state.NewItemDefinition
	NewInventory                = state.NewInventory
	NewEquipment                = state.NewEquipment
	ComposeFungibilityKey       = state.ComposeFungibilityKey
	MarshalItemDefinitions      = state.MarshalItemDefinitions
	ItemDefinitionFor           = state.ItemDefinitionFor
	ItemDefinitions             = state.ItemDefinitions
	parseFacing                 = state.ParseFacing
	deriveFacing                = state.DeriveFacing
	facingToVector              = state.FacingToVector
	equipmentsEqual             = state.EquipmentsEqual
	cloneEquipmentSlots         = state.CloneEquipmentSlots
	equipSlotRank               = state.EquipSlotRank
	equipSlotFromOrdinal        = state.EquipSlotFromOrdinal
	equipmentDeltaForDefinition = state.EquipmentDeltaForDefinition
)
