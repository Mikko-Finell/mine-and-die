package server

import (
	"context"
	"math"

	worldpkg "mine-and-die/server/internal/world"
	loggingeconomy "mine-and-die/server/logging/economy"
)

const groundPickupRadius = tileSize

const (
	groundScatterMinDistance = tileSize * 0.1
	groundScatterMaxDistance = tileSize * 0.35
	groundScatterPadding     = tileSize * 0.1
)

func toWorldItemStack(stack ItemStack) worldpkg.ItemStack {
	return worldpkg.ItemStack{Type: string(stack.Type), FungibilityKey: stack.FungibilityKey, Quantity: stack.Quantity}
}

func fromWorldItemStack(stack worldpkg.ItemStack) ItemStack {
	return ItemStack{Type: ItemType(stack.Type), FungibilityKey: stack.FungibilityKey, Quantity: stack.Quantity}
}

func toWorldActor(actor *actorState) *worldpkg.Actor {
	if actor == nil {
		return nil
	}
	return &worldpkg.Actor{ID: actor.ID, X: actor.X, Y: actor.Y}
}

func scatterConfig() worldpkg.ScatterConfig {
	return worldpkg.ScatterConfig{
		TileSize:    tileSize,
		MinDistance: groundScatterMinDistance,
		MaxDistance: groundScatterMaxDistance,
		Padding:     groundScatterPadding,
	}
}

// GroundItemsSnapshot returns a copy of the ground items for broadcasting.
func (w *World) GroundItemsSnapshot() []GroundItem {
	if w == nil {
		return make([]GroundItem, 0)
	}
	return worldpkg.GroundItemsSnapshot(w.groundItems)
}

func tileForPosition(x, y float64) groundTileKey {
	return worldpkg.TileForPosition(x, y, tileSize)
}

func tileCenter(key groundTileKey) (float64, float64) {
	return worldpkg.TileCenter(key, tileSize)
}

func (w *World) upsertGroundItem(actor *actorState, stack ItemStack, reason string) *groundItemState {
	if w == nil || actor == nil || stack.Quantity <= 0 || stack.Type == "" {
		return nil
	}

	if w.groundItems == nil {
		w.groundItems = make(map[string]*groundItemState)
	}
	if w.groundItemsByTile == nil {
		w.groundItemsByTile = make(map[groundTileKey]map[string]*groundItemState)
	}

	cfg := scatterConfig()
	worldActor := toWorldActor(actor)

	var angleFn func() float64
	var distanceFn func(min, max float64) float64
	if w != nil {
		w.ensureRNG()
		angleFn = func() float64 { return w.randomAngle() }
		distanceFn = func(min, max float64) float64 { return w.randomDistance(min, max) }
	}

	worldStack := toWorldItemStack(stack)

	return worldpkg.UpsertGroundItem(
		w.groundItems,
		w.groundItemsByTile,
		&w.nextGroundItemID,
		worldActor,
		worldStack,
		reason,
		cfg,
		angleFn,
		distanceFn,
		func(s *worldpkg.ItemStack) bool {
			if s == nil || s.Type == "" {
				return false
			}
			if s.FungibilityKey != "" {
				return true
			}
			if def, ok := ItemDefinitionFor(ItemType(s.Type)); ok {
				s.FungibilityKey = def.FungibilityKey
				return s.FungibilityKey != ""
			}
			return false
		},
		func(item *groundItemState, qty int) {
			w.SetGroundItemQuantity(item, qty)
		},
		func(item *groundItemState, x, y float64) {
			w.SetGroundItemPosition(item, x, y)
		},
		func(_ *worldpkg.Actor, stack worldpkg.ItemStack, reason, stackID string) {
			w.logGoldDrop(actor, fromWorldItemStack(stack), reason, stackID)
		},
	)
}

func (w *World) scatterGroundItemPosition(actor *actorState, tile groundTileKey) (float64, float64) {
	cfg := scatterConfig()
	worldActor := toWorldActor(actor)

	var angleFn func() float64
	var distanceFn func(min, max float64) float64
	if w != nil {
		w.ensureRNG()
		angleFn = func() float64 { return w.randomAngle() }
		distanceFn = func(min, max float64) float64 { return w.randomDistance(min, max) }
	}

	return worldpkg.ScatterGroundItemPosition(worldActor, tile, cfg, angleFn, distanceFn)
}

func (w *World) removeGroundItem(item *groundItemState) {
	if w == nil || item == nil {
		return
	}

	worldpkg.RemoveGroundItem(
		w.groundItems,
		w.groundItemsByTile,
		item,
		func(target *groundItemState, qty int) {
			w.SetGroundItemQuantity(target, qty)
		},
	)
}

func (w *World) nearestGroundItem(actor *actorState, itemType ItemType) (*groundItemState, float64) {
	if w == nil || actor == nil || itemType == "" || len(w.groundItems) == 0 {
		return nil, 0
	}
	var best *groundItemState
	bestDist := math.MaxFloat64
	for _, item := range w.groundItems {
		if item == nil || item.Qty <= 0 || ItemType(item.Type) != itemType {
			continue
		}
		dx := item.X - actor.X
		dy := item.Y - actor.Y
		dist := math.Hypot(dx, dy)
		if dist < bestDist {
			bestDist = dist
			best = item
		}
	}
	if best == nil {
		return nil, 0
	}
	return best, bestDist
}

func (w *World) dropAllGold(actor *actorState, reason string) int {
	return w.dropAllItemsOfType(actor, ItemTypeGold, reason)
}

func (w *World) dropAllInventory(actor *actorState, reason string) int {
	if w == nil || actor == nil {
		return 0
	}
	var stacks []ItemStack
	var equipmentStacks []ItemStack
	if _, ok := w.players[actor.ID]; ok {
		_ = w.MutateInventory(actor.ID, func(inv *Inventory) error {
			stacks = inv.DrainAll()
			return nil
		})
		if player, ok := w.players[actor.ID]; ok {
			equipmentStacks = w.drainEquipment(&player.actorState, &player.version, player.ID, PatchPlayerEquipment, PatchPlayerHealth, &player.stats)
		}
	} else if npc, ok := w.npcs[actor.ID]; ok {
		_ = w.MutateNPCInventory(npc.ID, func(inv *Inventory) error {
			stacks = inv.DrainAll()
			return nil
		})
		equipmentStacks = w.drainEquipment(&npc.actorState, &npc.version, npc.ID, PatchNPCEquipment, PatchNPCHealth, &npc.stats)
	} else {
		stacks = actor.Inventory.DrainAll()
		drained := actor.Equipment.DrainAll()
		if len(drained) > 0 {
			equipmentStacks = make([]ItemStack, 0, len(drained))
			for _, entry := range drained {
				if entry.Item.Type == "" || entry.Item.Quantity <= 0 {
					continue
				}
				equipmentStacks = append(equipmentStacks, entry.Item)
			}
		}
	}
	if len(equipmentStacks) > 0 {
		stacks = append(stacks, equipmentStacks...)
	}
	if len(stacks) == 0 {
		return 0
	}
	total := 0
	for _, stack := range stacks {
		if stack.Type == "" || stack.Quantity <= 0 {
			continue
		}
		w.upsertGroundItem(actor, stack, reason)
		total += stack.Quantity
	}
	return total
}

func (w *World) dropAllItemsOfType(actor *actorState, itemType ItemType, reason string) int {
	if w == nil || actor == nil || itemType == "" {
		return 0
	}
	var removed []ItemStack
	if _, ok := w.players[actor.ID]; ok {
		_ = w.MutateInventory(actor.ID, func(inv *Inventory) error {
			removed = inv.RemoveAllOf(itemType)
			return nil
		})
	} else if npc, ok := w.npcs[actor.ID]; ok {
		_ = w.MutateNPCInventory(npc.ID, func(inv *Inventory) error {
			removed = inv.RemoveAllOf(itemType)
			return nil
		})
	} else {
		removed = actor.Inventory.RemoveAllOf(itemType)
	}
	if len(removed) == 0 {
		return 0
	}
	total := 0
	for _, stack := range removed {
		if stack.Type == "" || stack.Quantity <= 0 {
			continue
		}
		w.upsertGroundItem(actor, stack, reason)
		total += stack.Quantity
	}
	return total
}

func (w *World) logGoldDrop(actor *actorState, stack ItemStack, reason, stackID string) {
	if w == nil || actor == nil {
		return
	}
	if stack.Type != ItemTypeGold || stack.Quantity <= 0 {
		return
	}
	loggingeconomy.GoldDropped(
		context.Background(),
		w.publisher,
		w.currentTick,
		w.entityRef(actor.ID),
		loggingeconomy.GoldDroppedPayload{Quantity: stack.Quantity, Reason: reason},
		map[string]any{"stackId": stackID},
	)
}
