package server

import (
	"context"
	"fmt"

	itemspkg "mine-and-die/server/internal/items"
	loggingeconomy "mine-and-die/server/logging/economy"
)

const groundPickupRadius = tileSize

const (
	groundScatterMinDistance = tileSize * 0.1
	groundScatterMaxDistance = tileSize * 0.35
	groundScatterPadding     = tileSize * 0.1
)

func toWorldItemStack(stack ItemStack) itemspkg.ItemStack {
	return itemspkg.ItemStack{Type: string(stack.Type), FungibilityKey: stack.FungibilityKey, Quantity: stack.Quantity}
}

func toWorldStacks(stacks []ItemStack) []itemspkg.ItemStack {
	if len(stacks) == 0 {
		return nil
	}
	converted := make([]itemspkg.ItemStack, 0, len(stacks))
	for _, stack := range stacks {
		converted = append(converted, toWorldItemStack(stack))
	}
	return converted
}

func fromWorldItemStack(stack itemspkg.ItemStack) ItemStack {
	return ItemStack{Type: ItemType(stack.Type), FungibilityKey: stack.FungibilityKey, Quantity: stack.Quantity}
}

func toWorldActor(actor *actorState) *itemspkg.Actor {
	if actor == nil {
		return nil
	}
	return &itemspkg.Actor{ID: actor.ID, X: actor.X, Y: actor.Y}
}

func scatterConfig() itemspkg.ScatterConfig {
	return itemspkg.ScatterConfig{
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
		w.groundItems = make(map[string]*itemspkg.GroundItemState)
	}
	if w.groundItemsByTile == nil {
		w.groundItemsByTile = make(map[itemspkg.GroundTileKey]map[string]*itemspkg.GroundItemState)
	}
}

// GroundItemsSnapshot returns a copy of the ground items for broadcasting.
func (w *World) GroundItemsSnapshot() []itemspkg.GroundItem {
	if w == nil {
		return make([]itemspkg.GroundItem, 0)
	}
	return itemspkg.GroundItemsSnapshot(w.groundItems)
}

func tileForPosition(x, y float64) itemspkg.GroundTileKey {
	return itemspkg.TileForPosition(x, y, tileSize)
}

func tileCenter(key itemspkg.GroundTileKey) (float64, float64) {
	return itemspkg.TileCenter(key, tileSize)
}

func (w *World) upsertGroundItem(actor *actorState, stack ItemStack, reason string) *itemspkg.GroundItemState {
	if w == nil || actor == nil || stack.Quantity <= 0 || stack.Type == "" {
		return nil
	}

	if w.groundItems == nil {
		w.groundItems = make(map[string]*itemspkg.GroundItemState)
	}
	if w.groundItemsByTile == nil {
		w.groundItemsByTile = make(map[itemspkg.GroundTileKey]map[string]*itemspkg.GroundItemState)
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

	return itemspkg.UpsertGroundItem(
		w.groundItems,
		w.groundItemsByTile,
		&w.nextGroundItemID,
		worldActor,
		worldStack,
		reason,
		cfg,
		angleFn,
		distanceFn,
		func(s *itemspkg.ItemStack) bool {
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
		itemspkg.GroundItemQuantityJournalSetter(w.journal.AppendPatch),
		itemspkg.GroundItemPositionJournalSetter(w.journal.AppendPatch),
		func(_ *itemspkg.Actor, stack itemspkg.ItemStack, reason, stackID string) {
			w.logGoldDrop(actor, fromWorldItemStack(stack), reason, stackID)
		},
	)
}

func (w *World) scatterGroundItemPosition(actor *actorState, tile itemspkg.GroundTileKey) (float64, float64) {
	cfg := scatterConfig()
	worldActor := toWorldActor(actor)

	var angleFn func() float64
	var distanceFn func(min, max float64) float64
	if w != nil {
		w.ensureRNG()
		angleFn = func() float64 { return w.randomAngle() }
		distanceFn = func(min, max float64) float64 { return w.randomDistance(min, max) }
	}

	return itemspkg.ScatterGroundItemPosition(worldActor, tile, cfg, angleFn, distanceFn)
}

func (w *World) nearestGroundItem(actor *actorState, itemType ItemType) (*itemspkg.GroundItemState, float64) {
	if w == nil {
		return nil, 0
	}
	return itemspkg.NearestGroundItem(w.groundItems, toWorldActor(actor), string(itemType))
}

func (w *World) pickupNearestGold(actor *actorState) (*itemspkg.PickupResult, *itemspkg.PickupFailure) {
	if w == nil || actor == nil {
		return nil, &itemspkg.PickupFailure{Reason: itemspkg.PickupFailureReasonNotFound}
	}

	worldActor := toWorldActor(actor)
	if worldActor == nil {
		return nil, &itemspkg.PickupFailure{Reason: itemspkg.PickupFailureReasonNotFound}
	}

	return itemspkg.PickupNearestItem(
		w.groundItems,
		w.groundItemsByTile,
		worldActor,
		string(ItemTypeGold),
		groundPickupRadius,
		func(stack itemspkg.ItemStack) error {
			return w.MutateInventory(actor.ID, func(inv *Inventory) error {
				_, addErr := inv.AddStack(ItemStack{
					Type:           ItemType(stack.Type),
					FungibilityKey: stack.FungibilityKey,
					Quantity:       stack.Quantity,
				})
				return addErr
			})
		},
		w.journal.AppendPatch,
	)
}

func (w *World) dropGold(actor *actorState, quantity int, reason string) (*itemspkg.DropResult, *itemspkg.DropFailure) {
	if w == nil || actor == nil {
		return nil, &itemspkg.DropFailure{Reason: itemspkg.DropFailureReasonInventoryError}
	}

	cfg, ok := w.groundDropConfig(actor)
	if !ok {
		return nil, &itemspkg.DropFailure{Reason: itemspkg.DropFailureReasonInventoryError}
	}

	delegates, ok := itemspkg.BuildGroundDropDelegates(cfg)
	if !ok {
		return nil, &itemspkg.DropFailure{Reason: itemspkg.DropFailureReasonInventoryError}
	}

	actorCfg, ok := w.groundDropActorConfig(actor)
	if !ok {
		return nil, &itemspkg.DropFailure{Reason: itemspkg.DropFailureReasonInventoryError}
	}

	removeQuantity := itemspkg.GroundDropRemoveGoldQuantityFunc(actorCfg)
	if removeQuantity == nil {
		return nil, &itemspkg.DropFailure{Reason: itemspkg.DropFailureReasonInventoryError}
	}

	return itemspkg.DropGoldQuantity(
		delegates,
		quantity,
		reason,
		func() int { return actor.Inventory.QuantityOf(ItemTypeGold) },
		removeQuantity,
	)
}

func (w *World) groundDropConfig(actor *actorState) (itemspkg.GroundDropConfig, bool) {
	if w == nil || actor == nil {
		return itemspkg.GroundDropConfig{}, false
	}

	w.ensureGroundItemStorage()

	worldActor := toWorldActor(actor)
	if worldActor == nil {
		return itemspkg.GroundDropConfig{}, false
	}

	cfg := scatterConfig()

	w.ensureRNG()
	angleFn := func() float64 { return w.randomAngle() }
	distanceFn := func(min, max float64) float64 { return w.randomDistance(min, max) }

	ensureKey := func(s *itemspkg.ItemStack) bool {
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

	return itemspkg.GroundDropConfig{
		Items:          w.groundItems,
		ItemsByTile:    w.groundItemsByTile,
		NextID:         &w.nextGroundItemID,
		Actor:          worldActor,
		Scatter:        cfg,
		RandomAngle:    angleFn,
		RandomDistance: distanceFn,
		EnsureKey:      ensureKey,
		AppendPatch:    w.journal.AppendPatch,
		LogDrop: func(_ *itemspkg.Actor, stack itemspkg.ItemStack, dropReason, stackID string) {
			w.logGoldDrop(actor, fromWorldItemStack(stack), dropReason, stackID)
		},
	}, true
}

func (w *World) groundDropActorConfig(actor *actorState) (itemspkg.GroundDropActorConfig, bool) {
	if w == nil || actor == nil {
		return itemspkg.GroundDropActorConfig{}, false
	}

	cfg := itemspkg.GroundDropActorConfig{}

	if _, ok := w.players[actor.ID]; ok {
		cfg.RemovePlayerStacks = func(itemType string) ([]itemspkg.ItemStack, bool) {
			if itemType == "" {
				return nil, true
			}
			if _, ok := w.players[actor.ID]; !ok {
				return nil, false
			}
			var removed []ItemStack
			_ = w.MutateInventory(actor.ID, func(inv *Inventory) error {
				removed = inv.RemoveAllOf(ItemType(itemType))
				return nil
			})
			return toWorldStacks(removed), true
		}

		cfg.RemoveGoldQuantity = func(quantity int) (int, error) {
			var removed int
			err := w.MutateInventory(actor.ID, func(inv *Inventory) error {
				var innerErr error
				removed, innerErr = inv.RemoveItemTypeQuantity(ItemTypeGold, quantity)
				return innerErr
			})
			if err != nil {
				return removed, err
			}
			if removed != quantity {
				return removed, fmt.Errorf("removed %d of requested %d", removed, quantity)
			}
			return removed, nil
		}

		cfg.DrainPlayerInventory = func() ([]itemspkg.ItemStack, bool) {
			if _, ok := w.players[actor.ID]; !ok {
				return nil, false
			}
			var stacks []ItemStack
			_ = w.MutateInventory(actor.ID, func(inv *Inventory) error {
				stacks = inv.DrainAll()
				return nil
			})
			return toWorldStacks(stacks), true
		}

		cfg.DrainPlayerEquipment = func() ([]itemspkg.ItemStack, bool) {
			player, ok := w.players[actor.ID]
			if !ok || player == nil {
				return nil, false
			}
			stacks := w.drainEquipment(&player.ActorState, &player.Version, player.ID, PatchPlayerEquipment, PatchPlayerHealth, &player.Stats)
			return toWorldStacks(stacks), true
		}
	}

	if npc, ok := w.npcs[actor.ID]; ok {
		npcID := npc.ID
		cfg.RemoveNPCStacks = func(itemType string) ([]itemspkg.ItemStack, bool) {
			if itemType == "" {
				return nil, true
			}
			if npc := w.npcs[npcID]; npc != nil {
				var removed []ItemStack
				_ = w.MutateNPCInventory(npcID, func(inv *Inventory) error {
					removed = inv.RemoveAllOf(ItemType(itemType))
					return nil
				})
				return toWorldStacks(removed), true
			}
			return nil, false
		}

		cfg.DrainNPCInventory = func() ([]itemspkg.ItemStack, bool) {
			npc := w.npcs[npcID]
			if npc == nil {
				return nil, false
			}
			var stacks []ItemStack
			_ = w.MutateNPCInventory(npcID, func(inv *Inventory) error {
				stacks = inv.DrainAll()
				return nil
			})
			return toWorldStacks(stacks), true
		}

		cfg.DrainNPCEquipment = func() ([]itemspkg.ItemStack, bool) {
			npc := w.npcs[npcID]
			if npc == nil {
				return nil, false
			}
			stacks := w.drainEquipment(&npc.ActorState, &npc.Version, npcID, PatchNPCEquipment, PatchNPCHealth, &npc.Stats)
			return toWorldStacks(stacks), true
		}
	}

	cfg.RemoveFallbackStacks = func(itemType string) ([]itemspkg.ItemStack, bool) {
		if itemType == "" {
			return nil, true
		}
		removed := actor.Inventory.RemoveAllOf(ItemType(itemType))
		return toWorldStacks(removed), true
	}

	cfg.DrainFallbackInventory = func() ([]itemspkg.ItemStack, bool) {
		stacks := actor.Inventory.DrainAll()
		return toWorldStacks(stacks), true
	}

	cfg.DrainFallbackEquipment = func() ([]itemspkg.ItemStack, bool) {
		drained := actor.Equipment.DrainAll()
		if len(drained) == 0 {
			return nil, true
		}
		stacks := make([]ItemStack, 0, len(drained))
		for _, entry := range drained {
			if entry.Item.Type == "" || entry.Item.Quantity <= 0 {
				continue
			}
			stacks = append(stacks, entry.Item)
		}
		return toWorldStacks(stacks), true
	}

	return cfg, true
}

func (w *World) dropAllGold(actor *actorState, reason string) int {
	actorCfg, ok := w.groundDropActorConfig(actor)
	if !ok {
		return 0
	}

	remover := itemspkg.GroundDropRemoveStacksFunc(actorCfg)
	if remover == nil {
		return 0
	}

	cfg, ok := w.groundDropConfig(actor)
	if !ok {
		return 0
	}

	return itemspkg.InvokeGroundDrop(cfg, func(d itemspkg.GroundDropDelegates) int {
		return itemspkg.DropAllGold(d, reason, remover)
	})
}

func (w *World) dropAllInventory(actor *actorState, reason string) int {
	actorCfg, ok := w.groundDropActorConfig(actor)
	if !ok {
		return 0
	}

	inventoryDrain := itemspkg.GroundDropInventoryDrainFunc(actorCfg)
	equipmentDrain := itemspkg.GroundDropEquipmentDrainFunc(actorCfg)
	if inventoryDrain == nil && equipmentDrain == nil {
		return 0
	}

	cfg, ok := w.groundDropConfig(actor)
	if !ok {
		return 0
	}

	return itemspkg.InvokeGroundDrop(cfg, func(d itemspkg.GroundDropDelegates) int {
		return itemspkg.DropAllInventory(d, reason, inventoryDrain, equipmentDrain)
	})
}

func (w *World) dropAllItemsOfType(actor *actorState, itemType ItemType, reason string) int {
	if itemType == "" {
		return 0
	}

	actorCfg, ok := w.groundDropActorConfig(actor)
	if !ok {
		return 0
	}

	remover := itemspkg.GroundDropRemoveStacksFunc(actorCfg)
	if remover == nil {
		return 0
	}

	cfg, ok := w.groundDropConfig(actor)
	if !ok {
		return 0
	}

	return itemspkg.InvokeGroundDrop(cfg, func(d itemspkg.GroundDropDelegates) int {
		return itemspkg.DropAllItemsOfType(d, string(itemType), reason, remover)
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
