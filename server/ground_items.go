package server

import (
	"context"

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

func toWorldStacks(stacks []ItemStack) []worldpkg.ItemStack {
	if len(stacks) == 0 {
		return nil
	}
	converted := make([]worldpkg.ItemStack, 0, len(stacks))
	for _, stack := range stacks {
		converted = append(converted, toWorldItemStack(stack))
	}
	return converted
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

func (w *World) ensureGroundItemStorage() {
	if w == nil {
		return
	}
	if w.groundItems == nil {
		w.groundItems = make(map[string]*groundItemState)
	}
	if w.groundItemsByTile == nil {
		w.groundItemsByTile = make(map[groundTileKey]map[string]*groundItemState)
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
	if w == nil {
		return nil, 0
	}
	return worldpkg.NearestGroundItem(w.groundItems, toWorldActor(actor), string(itemType))
}

func (w *World) pickupNearestGold(actor *actorState) (*worldpkg.PickupResult, *worldpkg.PickupFailure) {
	if w == nil || actor == nil {
		return nil, &worldpkg.PickupFailure{Reason: worldpkg.PickupFailureReasonNotFound}
	}

	worldActor := toWorldActor(actor)
	if worldActor == nil {
		return nil, &worldpkg.PickupFailure{Reason: worldpkg.PickupFailureReasonNotFound}
	}

	return worldpkg.PickupNearestItem(
		w.groundItems,
		worldActor,
		string(ItemTypeGold),
		groundPickupRadius,
		func(stack worldpkg.ItemStack) error {
			return w.MutateInventory(actor.ID, func(inv *Inventory) error {
				_, addErr := inv.AddStack(ItemStack{
					Type:           ItemType(stack.Type),
					FungibilityKey: stack.FungibilityKey,
					Quantity:       stack.Quantity,
				})
				return addErr
			})
		},
		func(item *worldpkg.GroundItemState) {
			w.removeGroundItem(item)
		},
	)
}

type groundDropDelegates struct {
	items       map[string]*groundItemState
	itemsByTile map[groundTileKey]map[string]*groundItemState
	nextID      *uint64
	actor       *worldpkg.Actor
	cfg         worldpkg.ScatterConfig
	angleFn     func() float64
	distanceFn  func(min, max float64) float64
	ensureKey   func(*worldpkg.ItemStack) bool
	setQuantity func(*worldpkg.GroundItemState, int)
	setPosition func(*worldpkg.GroundItemState, float64, float64)
	logDrop     func(*worldpkg.Actor, worldpkg.ItemStack, string, string)
}

func (w *World) invokeGroundDrop(actor *actorState, reason string, call func(groundDropDelegates) int) int {
	if w == nil || actor == nil || call == nil {
		return 0
	}
	delegates, ok := w.buildGroundDropDelegates(actor, reason)
	if !ok {
		return 0
	}
	return call(delegates)
}

func (w *World) buildGroundDropDelegates(actor *actorState, reason string) (groundDropDelegates, bool) {
	if w == nil || actor == nil {
		return groundDropDelegates{}, false
	}

	w.ensureGroundItemStorage()

	cfg := scatterConfig()
	worldActor := toWorldActor(actor)

	var angleFn func() float64
	var distanceFn func(min, max float64) float64
	w.ensureRNG()
	angleFn = func() float64 { return w.randomAngle() }
	distanceFn = func(min, max float64) float64 { return w.randomDistance(min, max) }

	delegates := groundDropDelegates{
		items:       w.groundItems,
		itemsByTile: w.groundItemsByTile,
		nextID:      &w.nextGroundItemID,
		actor:       worldActor,
		cfg:         cfg,
		angleFn:     angleFn,
		distanceFn:  distanceFn,
	}

	delegates.ensureKey = func(s *worldpkg.ItemStack) bool {
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
	}

	delegates.setQuantity = func(item *worldpkg.GroundItemState, qty int) {
		w.SetGroundItemQuantity(item, qty)
	}
	delegates.setPosition = func(item *worldpkg.GroundItemState, x, y float64) {
		w.SetGroundItemPosition(item, x, y)
	}
	delegates.logDrop = func(_ *worldpkg.Actor, stack worldpkg.ItemStack, dropReason, stackID string) {
		w.logGoldDrop(actor, fromWorldItemStack(stack), dropReason, stackID)
	}

	return delegates, true
}

func (w *World) removeStacksFunc(actor *actorState) func(string) []worldpkg.ItemStack {
	if w == nil || actor == nil {
		return nil
	}
	return func(itemType string) []worldpkg.ItemStack {
		if itemType == "" {
			return nil
		}
		var removed []ItemStack
		if _, ok := w.players[actor.ID]; ok {
			_ = w.MutateInventory(actor.ID, func(inv *Inventory) error {
				removed = inv.RemoveAllOf(ItemType(itemType))
				return nil
			})
		} else if npc, ok := w.npcs[actor.ID]; ok {
			_ = w.MutateNPCInventory(npc.ID, func(inv *Inventory) error {
				removed = inv.RemoveAllOf(ItemType(itemType))
				return nil
			})
		} else {
			removed = actor.Inventory.RemoveAllOf(ItemType(itemType))
		}
		return toWorldStacks(removed)
	}
}

func (w *World) inventoryDrainFunc(actor *actorState) func() []worldpkg.ItemStack {
	if w == nil || actor == nil {
		return nil
	}
	return func() []worldpkg.ItemStack {
		var stacks []ItemStack
		if _, ok := w.players[actor.ID]; ok {
			_ = w.MutateInventory(actor.ID, func(inv *Inventory) error {
				stacks = inv.DrainAll()
				return nil
			})
		} else if npc, ok := w.npcs[actor.ID]; ok {
			_ = w.MutateNPCInventory(npc.ID, func(inv *Inventory) error {
				stacks = inv.DrainAll()
				return nil
			})
		} else {
			stacks = actor.Inventory.DrainAll()
		}
		return toWorldStacks(stacks)
	}
}

func (w *World) equipmentDrainFunc(actor *actorState) func() []worldpkg.ItemStack {
	if w == nil || actor == nil {
		return nil
	}
	return func() []worldpkg.ItemStack {
		var stacks []ItemStack
		if player, ok := w.players[actor.ID]; ok {
			stacks = w.drainEquipment(&player.actorState, &player.version, player.ID, PatchPlayerEquipment, PatchPlayerHealth, &player.stats)
		} else if npc, ok := w.npcs[actor.ID]; ok {
			stacks = w.drainEquipment(&npc.actorState, &npc.version, npc.ID, PatchNPCEquipment, PatchNPCHealth, &npc.stats)
		} else {
			drained := actor.Equipment.DrainAll()
			if len(drained) > 0 {
				stacks = make([]ItemStack, 0, len(drained))
				for _, entry := range drained {
					if entry.Item.Type == "" || entry.Item.Quantity <= 0 {
						continue
					}
					stacks = append(stacks, entry.Item)
				}
			}
		}
		return toWorldStacks(stacks)
	}
}

func (w *World) dropAllGold(actor *actorState, reason string) int {
	remover := w.removeStacksFunc(actor)
	if remover == nil {
		return 0
	}

	return w.invokeGroundDrop(actor, reason, func(d groundDropDelegates) int {
		return worldpkg.DropAllGold(
			d.items,
			d.itemsByTile,
			d.nextID,
			d.actor,
			reason,
			d.cfg,
			d.angleFn,
			d.distanceFn,
			d.ensureKey,
			d.setQuantity,
			d.setPosition,
			d.logDrop,
			remover,
		)
	})
}

func (w *World) dropAllInventory(actor *actorState, reason string) int {
	inventoryDrain := w.inventoryDrainFunc(actor)
	equipmentDrain := w.equipmentDrainFunc(actor)
	if inventoryDrain == nil && equipmentDrain == nil {
		return 0
	}

	return w.invokeGroundDrop(actor, reason, func(d groundDropDelegates) int {
		return worldpkg.DropAllInventory(
			d.items,
			d.itemsByTile,
			d.nextID,
			d.actor,
			reason,
			d.cfg,
			d.angleFn,
			d.distanceFn,
			d.ensureKey,
			d.setQuantity,
			d.setPosition,
			d.logDrop,
			inventoryDrain,
			equipmentDrain,
		)
	})
}

func (w *World) dropAllItemsOfType(actor *actorState, itemType ItemType, reason string) int {
	if itemType == "" {
		return 0
	}

	remover := w.removeStacksFunc(actor)
	if remover == nil {
		return 0
	}

	return w.invokeGroundDrop(actor, reason, func(d groundDropDelegates) int {
		return worldpkg.DropAllItemsOfType(
			d.items,
			d.itemsByTile,
			d.nextID,
			d.actor,
			string(itemType),
			reason,
			d.cfg,
			d.angleFn,
			d.distanceFn,
			d.ensureKey,
			d.setQuantity,
			d.setPosition,
			d.logDrop,
			remover,
		)
	})
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
