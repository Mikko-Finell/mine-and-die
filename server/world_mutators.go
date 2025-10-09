package main

import "math"

const positionEpsilon = 1e-6
const healthEpsilon = 1e-6

// positionsEqual reports whether two coordinate pairs are effectively the same.
func positionsEqual(ax, ay, bx, by float64) bool {
	return math.Abs(ax-bx) <= positionEpsilon && math.Abs(ay-by) <= positionEpsilon
}

// SetPosition updates a player's position, bumps the version, and records a patch.
// All player position writes must flow through this helper so snapshot versions
// and patch journals stay authoritative.
func (w *World) SetPosition(playerID string, x, y float64) {
	if w == nil {
		return
	}

	player, ok := w.players[playerID]
	if !ok {
		return
	}

	if positionsEqual(player.X, player.Y, x, y) {
		return
	}

	player.X = x
	player.Y = y
	player.version++

	w.journal.AppendPatch(Patch{
		Kind:     PatchPlayerPos,
		EntityID: playerID,
		Payload: PlayerPosPayload{
			X: x,
			Y: y,
		},
	})
}

// SetFacing updates a player's facing, bumps the version, and records a patch.
// All player facing writes must flow through this helper so snapshot versions
// and patch journals stay authoritative.
func (w *World) SetFacing(playerID string, facing FacingDirection) {
	if w == nil {
		return
	}

	if facing == "" {
		facing = defaultFacing
	}

	player, ok := w.players[playerID]
	if !ok {
		return
	}

	if player.Facing == facing {
		return
	}

	player.Facing = facing
	player.version++

	w.journal.AppendPatch(Patch{
		Kind:     PatchPlayerFacing,
		EntityID: playerID,
		Payload: PlayerFacingPayload{
			Facing: facing,
		},
	})
}

// SetHealth updates a player's health, bumps the version, and records a patch.
// All player health writes must flow through this helper so snapshot versions
// and patch journals stay authoritative.
func (w *World) SetHealth(playerID string, health float64) {
	if w == nil {
		return
	}

	if math.IsNaN(health) || math.IsInf(health, 0) {
		return
	}

	player, ok := w.players[playerID]
	if !ok {
		return
	}

	max := player.MaxHealth
	if max <= 0 {
		max = playerMaxHealth
	}

	if health < 0 {
		health = 0
	}
	if health > max {
		health = max
	}

	if math.Abs(player.Health-health) < healthEpsilon {
		return
	}

	player.Health = health
	player.version++

	w.journal.AppendPatch(Patch{
		Kind:     PatchPlayerHealth,
		EntityID: playerID,
		Payload: PlayerHealthPayload{
			Health:    health,
			MaxHealth: max,
		},
	})
}

// MutateInventory applies the provided mutation to a player's inventory while
// preserving snapshot versioning and patch emission. All inventory writes for
// players must flow through this helper.
func (w *World) MutateInventory(playerID string, mutate func(inv *Inventory) error) error {
	if w == nil || mutate == nil {
		return nil
	}

	player, ok := w.players[playerID]
	if !ok {
		return nil
	}

	before := player.Inventory.Clone()
	if err := mutate(&player.Inventory); err != nil {
		player.Inventory = before
		return err
	}

	if inventoriesEqual(before, player.Inventory) {
		return nil
	}

	player.version++
	w.journal.AppendPatch(Patch{
		Kind:     PatchPlayerInventory,
		EntityID: playerID,
		Payload: PlayerInventoryPayload{
			Slots: cloneInventorySlots(player.Inventory.Slots),
		},
	})

	return nil
}

func inventoriesEqual(a, b Inventory) bool {
	if len(a.Slots) != len(b.Slots) {
		return false
	}
	for i := range a.Slots {
		as := a.Slots[i]
		bs := b.Slots[i]
		if as.Slot != bs.Slot {
			return false
		}
		if as.Item.Type != bs.Item.Type {
			return false
		}
		if as.Item.Quantity != bs.Item.Quantity {
			return false
		}
	}
	return true
}

func cloneInventorySlots(slots []InventorySlot) []InventorySlot {
	if len(slots) == 0 {
		return nil
	}
	cloned := make([]InventorySlot, len(slots))
	copy(cloned, slots)
	return cloned
}
