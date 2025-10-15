package main

import (
	"context"
	"fmt"
	"math"
	"sort"

	loggingeconomy "mine-and-die/server/logging/economy"
)

// GroundItem represents a stack of items that exists in the world.
type GroundItem struct {
	ID             string   `json:"id"`
	Type           ItemType `json:"type"`
	FungibilityKey string   `json:"fungibility_key"`
	X              float64  `json:"x"`
	Y              float64  `json:"y"`
	Qty            int      `json:"qty"`
}

type groundTileKey struct {
	X int
	Y int
}

type groundItemState struct {
	GroundItem
	tile    groundTileKey
	version uint64
}

const groundPickupRadius = tileSize

const (
	groundScatterMinDistance = tileSize * 0.1
	groundScatterMaxDistance = tileSize * 0.35
	groundScatterPadding     = tileSize * 0.1
)

func (w *World) groundItemsSnapshot() []GroundItem {
	if w == nil || len(w.groundItems) == 0 {
		return nil
	}
	items := make([]GroundItem, 0, len(w.groundItems))
	for _, item := range w.groundItems {
		if item == nil {
			continue
		}
		items = append(items, item.GroundItem)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items
}

// GroundItemsSnapshot returns a copy of the ground items for broadcasting.
func (w *World) GroundItemsSnapshot() []GroundItem {
	return w.groundItemsSnapshot()
}

func tileForPosition(x, y float64) groundTileKey {
	return groundTileKey{X: int(math.Floor(x / tileSize)), Y: int(math.Floor(y / tileSize))}
}

func tileCenter(key groundTileKey) (float64, float64) {
	return float64(key.X)*tileSize + tileSize/2, float64(key.Y)*tileSize + tileSize/2
}

func (w *World) upsertGroundItem(actor *actorState, stack ItemStack, reason string) *groundItemState {
	if w == nil || actor == nil || stack.Quantity <= 0 || stack.Type == "" {
		return nil
	}
	tile := tileForPosition(actor.X, actor.Y)
	x, y := w.scatterGroundItemPosition(actor, tile)
	if w.groundItemsByTile == nil {
		w.groundItemsByTile = make(map[groundTileKey]map[string]*groundItemState)
	}
	itemsByType := w.groundItemsByTile[tile]
	if itemsByType == nil {
		itemsByType = make(map[string]*groundItemState)
		w.groundItemsByTile[tile] = itemsByType
	}
	key := stack.FungibilityKey
	if key == "" {
		if def, ok := ItemDefinitionFor(stack.Type); ok {
			stack.FungibilityKey = def.FungibilityKey
			key = stack.FungibilityKey
		}
	}
	if key == "" {
		return nil
	}
	if existing := itemsByType[key]; existing != nil {
		w.SetGroundItemQuantity(existing, existing.Qty+stack.Quantity)
		existing.tile = tile
		w.SetGroundItemPosition(existing, x, y)
		w.logGoldDrop(actor, stack, reason, existing.ID)
		return existing
	}
	w.nextGroundItemID++
	id := fmt.Sprintf("ground-%d", w.nextGroundItemID)
	item := &groundItemState{
		GroundItem: GroundItem{ID: id, Type: stack.Type, FungibilityKey: stack.FungibilityKey, X: x, Y: y, Qty: stack.Quantity},
		tile:       tile,
	}
	w.groundItems[id] = item
	itemsByType[key] = item
	w.logGoldDrop(actor, stack, reason, id)
	return item
}

func (w *World) scatterGroundItemPosition(actor *actorState, tile groundTileKey) (float64, float64) {
	if actor == nil {
		return tileCenter(tile)
	}

	angle := w.randomAngle()
	distance := w.randomDistance(groundScatterMinDistance, groundScatterMaxDistance)
	baseX := actor.X
	baseY := actor.Y
	x := baseX + math.Cos(angle)*distance
	y := baseY + math.Sin(angle)*distance

	left := float64(tile.X) * tileSize
	top := float64(tile.Y) * tileSize
	right := left + tileSize
	bottom := top + tileSize

	padding := groundScatterPadding
	if padding*2 >= tileSize {
		padding = 0
	}

	minX := left + padding
	maxX := right - padding
	minY := top + padding
	maxY := bottom - padding

	return clampFloat(x, minX, maxX), clampFloat(y, minY, maxY)
}

func clampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func (w *World) removeGroundItem(item *groundItemState) {
	if w == nil || item == nil {
		return
	}
	if item.Qty > 0 {
		item.Qty = 0
		incrementVersion(&item.version)
		if item.ID != "" {
			w.appendPatch(PatchGroundItemQty, item.ID, GroundItemQtyPayload{Qty: 0})
		}
	}
	delete(w.groundItems, item.ID)
	if itemsByType, ok := w.groundItemsByTile[item.tile]; ok {
		delete(itemsByType, item.FungibilityKey)
		if len(itemsByType) == 0 {
			delete(w.groundItemsByTile, item.tile)
		}
	}
}

func (w *World) nearestGroundItem(actor *actorState, itemType ItemType) (*groundItemState, float64) {
	if w == nil || actor == nil || itemType == "" || len(w.groundItems) == 0 {
		return nil, 0
	}
	var best *groundItemState
	bestDist := math.MaxFloat64
	for _, item := range w.groundItems {
		if item == nil || item.Qty <= 0 || item.Type != itemType {
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
