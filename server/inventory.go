package server

import "mine-and-die/server/internal/state"

type (
	ItemStack     = state.ItemStack
	InventorySlot = state.InventorySlot
	Inventory     = state.Inventory
)

func NewInventory() Inventory {
	return state.NewInventory()
}
