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
	ID   string   `json:"id"`
	X    float64  `json:"x"`
	Y    float64  `json:"y"`
	Type ItemType `json:"type"`
	Qty  int      `json:"qty"`
}

type groundTileKey struct {
	X int
	Y int
}

type groundItemState struct {
	GroundItem
	tile groundTileKey
}

const groundPickupRadius = tileSize

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
	centerX, centerY := tileCenter(tile)
	if w.groundItemsByTile == nil {
		w.groundItemsByTile = make(map[groundTileKey]map[ItemType]*groundItemState)
	}
	if w.groundItemsByTile[tile] == nil {
		w.groundItemsByTile[tile] = make(map[ItemType]*groundItemState)
	}
	if existing := w.groundItemsByTile[tile][stack.Type]; existing != nil {
		existing.Qty += stack.Quantity
		existing.X = centerX
		existing.Y = centerY
		if stack.Type == ItemTypeGold {
			loggingeconomy.GoldDropped(
				context.Background(),
				w.publisher,
				w.currentTick,
				w.entityRef(actor.ID),
				loggingeconomy.GoldDroppedPayload{Quantity: stack.Quantity, Reason: reason},
				map[string]any{"stackId": existing.ID},
			)
		}
		return existing
	}
	w.nextGroundItemID++
	id := fmt.Sprintf("ground-%d", w.nextGroundItemID)
	item := &groundItemState{
		GroundItem: GroundItem{ID: id, X: centerX, Y: centerY, Type: stack.Type, Qty: stack.Quantity},
		tile:       tile,
	}
	w.groundItems[id] = item
	w.groundItemsByTile[tile][stack.Type] = item
	if stack.Type == ItemTypeGold {
		loggingeconomy.GoldDropped(
			context.Background(),
			w.publisher,
			w.currentTick,
			w.entityRef(actor.ID),
			loggingeconomy.GoldDroppedPayload{Quantity: stack.Quantity, Reason: reason},
			map[string]any{"stackId": id},
		)
	}
	return item
}

func (w *World) upsertGroundGold(actor *actorState, qty int, reason string) *groundItemState {
	if qty <= 0 {
		return nil
	}
	return w.upsertGroundItem(actor, ItemStack{Type: ItemTypeGold, Quantity: qty}, reason)
}

func (w *World) removeGroundItem(item *groundItemState) {
	if w == nil || item == nil {
		return
	}
	delete(w.groundItems, item.ID)
	if perTile := w.groundItemsByTile[item.tile]; perTile != nil {
		delete(perTile, item.Type)
		if len(perTile) == 0 {
			delete(w.groundItemsByTile, item.tile)
		}
	}
}

func (w *World) nearestGroundItem(actor *actorState, itemType ItemType) (*groundItemState, float64) {
	if w == nil || actor == nil || len(w.groundItems) == 0 {
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
	if w == nil || actor == nil {
		return 0
	}
	var total int
	if _, ok := w.players[actor.ID]; ok {
		_ = w.MutateInventory(actor.ID, func(inv *Inventory) error {
			total = inv.RemoveAllOf(ItemTypeGold)
			return nil
		})
	} else {
		total = actor.Inventory.RemoveAllOf(ItemTypeGold)
	}
	if total <= 0 {
		return 0
	}
	w.upsertGroundItem(actor, ItemStack{Type: ItemTypeGold, Quantity: total}, reason)
	return total
}
