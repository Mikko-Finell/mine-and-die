package server

import "mine-and-die/server/internal/state"

const (
	ItemTypeGold          = state.ItemTypeGold
	ItemTypeHealthPotion  = state.ItemTypeHealthPotion
	ItemTypeRatTail       = state.ItemTypeRatTail
	ItemTypeIronDagger    = state.ItemTypeIronDagger
	ItemTypeLeatherJerkin = state.ItemTypeLeatherJerkin
	ItemTypeTravelerCharm = state.ItemTypeTravelerCharm
	ItemTypeVenomCoating  = state.ItemTypeVenomCoating
	ItemTypeBlastingOrb   = state.ItemTypeBlastingOrb
	ItemTypeRefinedOre    = state.ItemTypeRefinedOre
)

var (
	ItemDefinitionFor = state.ItemDefinitionFor
	ItemDefinitions   = state.ItemDefinitions
)
