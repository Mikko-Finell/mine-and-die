package server

import "mine-and-die/server/internal/state"

type (
	ItemType             = state.ItemType
	ItemClass            = state.ItemClass
	EquipSlot            = state.EquipSlot
	ItemAction           = state.ItemAction
	ItemModifier         = state.ItemModifier
	ItemDefinition       = state.ItemDefinition
	ItemDefinitionParams = state.ItemDefinitionParams
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
)

func NewItemDefinition(params ItemDefinitionParams) (ItemDefinition, error) {
	return state.NewItemDefinition(params)
}

func ComposeFungibilityKey(id ItemType, tier int, qualityTags ...string) string {
	return state.ComposeFungibilityKey(id, tier, qualityTags...)
}

func MarshalItemDefinitions(defs []ItemDefinition) ([]byte, error) {
	return state.MarshalItemDefinitions(defs)
}
