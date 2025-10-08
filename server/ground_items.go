package main

import (
	"context"
	"fmt"
	"math"

	loggingeconomy "mine-and-die/server/logging/economy"
)

// GroundItem represents a stack of gold dropped on the ground.
type GroundItem struct {
	ID    string  `json:"id"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Qty   int     `json:"qty"`
	tileX int
	tileY int
}

func groundTileKey(tileX, tileY int) string {
	return fmt.Sprintf("%d:%d", tileX, tileY)
}

func quantizeToTile(x, y float64) (int, int, float64, float64) {
	tileX := int(math.Floor(x / tileSize))
	tileY := int(math.Floor(y / tileSize))
	centerX := (float64(tileX) + 0.5) * tileSize
	centerY := (float64(tileY) + 0.5) * tileSize
	return tileX, tileY, centerX, centerY
}

func (w *World) spawnGroundGold(x, y float64, qty int) *GroundItem {
	if w == nil || qty <= 0 {
		return nil
	}
	tileX, tileY, centerX, centerY := quantizeToTile(x, y)
	key := groundTileKey(tileX, tileY)
	if w.groundIndex == nil {
		w.groundIndex = make(map[string]string)
	}
	if w.groundItems == nil {
		w.groundItems = make(map[string]*GroundItem)
	}
	if existingID, ok := w.groundIndex[key]; ok {
		if existing, ok := w.groundItems[existingID]; ok && existing != nil {
			existing.Qty += qty
			existing.X = centerX
			existing.Y = centerY
			existing.tileX = tileX
			existing.tileY = tileY
			w.groundIndex[key] = existing.ID
			return existing
		}
	}

	w.nextGroundItemID++
	id := fmt.Sprintf("ground-%d", w.nextGroundItemID)
	item := &GroundItem{ID: id, X: centerX, Y: centerY, Qty: qty, tileX: tileX, tileY: tileY}
	w.groundItems[id] = item
	w.groundIndex[key] = id
	return item
}

func (w *World) closestGroundItem(x, y float64) (*GroundItem, float64) {
	if w == nil || len(w.groundItems) == 0 {
		return nil, math.MaxFloat64
	}
	bestDistance := math.MaxFloat64
	var bestItem *GroundItem
	bestID := ""
	for id, item := range w.groundItems {
		if item == nil || item.Qty <= 0 {
			continue
		}
		dx := item.X - x
		dy := item.Y - y
		dist := math.Hypot(dx, dy)
		if dist+1e-6 < bestDistance {
			bestDistance = dist
			bestItem = item
			bestID = id
			continue
		}
		if math.Abs(dist-bestDistance) <= 1e-6 && bestItem != nil && id < bestID {
			bestItem = item
			bestID = id
		}
	}
	if bestItem == nil {
		return nil, math.MaxFloat64
	}
	return bestItem, bestDistance
}

func (w *World) removeGroundItem(id string) {
	if w == nil || id == "" {
		return
	}
	item, ok := w.groundItems[id]
	if !ok {
		return
	}
	delete(w.groundItems, id)
	key := groundTileKey(item.tileX, item.tileY)
	if existing, ok := w.groundIndex[key]; ok && existing == id {
		delete(w.groundIndex, key)
	}
}

func (w *World) dropAllGold(actor *actorState, reason string) int {
	if w == nil || actor == nil {
		return 0
	}
	total := 0
	for i := len(actor.Inventory.Slots) - 1; i >= 0; i-- {
		slot := actor.Inventory.Slots[i]
		if slot.Item.Type != ItemTypeGold {
			continue
		}
		qty := slot.Item.Quantity
		if qty <= 0 {
			continue
		}
		if _, err := actor.Inventory.RemoveQuantity(i, qty); err != nil {
			continue
		}
		total += qty
	}
	if total > 0 {
		w.spawnGroundGold(actor.X, actor.Y, total)
		loggingeconomy.GoldDropped(
			context.Background(),
			w.publisher,
			w.currentTick,
			w.entityRef(actor.ID),
			loggingeconomy.GoldDroppedPayload{Quantity: total, Reason: reason},
			nil,
		)
	}
	return total
}
