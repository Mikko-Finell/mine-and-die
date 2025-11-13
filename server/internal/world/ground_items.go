package world

import (
	"context"
	"fmt"

	itemspkg "mine-and-die/server/internal/items"
	journalpkg "mine-and-die/server/internal/journal"
	state "mine-and-die/server/internal/world/state"
	loggingeconomy "mine-and-die/server/logging/economy"
)

const (
	groundTileSize           = 40.0
	groundPickupRadius       = groundTileSize
	groundScatterMinDistance = groundTileSize * 0.1
	groundScatterMaxDistance = groundTileSize * 0.35
	groundScatterPadding     = groundTileSize * 0.1
)

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

// DropAllInventory drains the actor's inventory and equipment, spawning items on the ground.
func (w *World) DropAllInventory(actor *state.ActorState, reason string) int {
	if w == nil || actor == nil {
		return 0
	}

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

func (w *World) groundDropConfig(actor *state.ActorState) (itemspkg.GroundDropConfig, bool) {
	if w == nil || actor == nil {
		return itemspkg.GroundDropConfig{}, false
	}

	w.ensureGroundItemStorage()

	cfg := itemspkg.GroundDropConfig{
		Items:          w.groundItems,
		ItemsByTile:    w.groundItemsByTile,
		NextID:         &w.nextGroundItemID,
		Actor:          toWorldActor(actor),
		Scatter:        scatterConfig(),
		RandomAngle:    func() float64 { return w.randomAngle() },
		RandomDistance: func(min, max float64) float64 { return w.randomDistance(min, max) },
		EnsureKey:      ensureItemFungibilityKey,
		AppendPatch:    w.AppendPatch,
		LogDrop: func(_ *itemspkg.Actor, stack itemspkg.ItemStack, dropReason, stackID string) {
			w.logGoldDrop(actor, fromWorldItemStack(stack), dropReason, stackID)
		},
	}

	return cfg, true
}

func (w *World) groundDropActorConfig(actor *state.ActorState) (itemspkg.GroundDropActorConfig, bool) {
	if w == nil || actor == nil {
		return itemspkg.GroundDropActorConfig{}, false
	}

	cfg := itemspkg.GroundDropActorConfig{}

	if _, ok := w.players[actor.ID]; ok {
		cfg.RemovePlayerStacks = func(itemType string) ([]itemspkg.ItemStack, bool) {
			if itemType == "" {
				return nil, true
			}
			player, ok := w.players[actor.ID]
			if !ok || player == nil {
				return nil, false
			}
			var removed []state.ItemStack
			_ = w.MutateInventory(actor.ID, func(inv *state.Inventory) error {
				removed = inv.RemoveAllOf(state.ItemType(itemType))
				return nil
			})
			return toWorldStacks(removed), true
		}

		cfg.RemoveGoldQuantity = func(quantity int) (int, error) {
			var removed int
			err := w.MutateInventory(actor.ID, func(inv *state.Inventory) error {
				var innerErr error
				removed, innerErr = inv.RemoveItemTypeQuantity(state.ItemTypeGold, quantity)
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
			player, ok := w.players[actor.ID]
			if !ok || player == nil {
				return nil, false
			}
			var stacks []state.ItemStack
			_ = w.MutateInventory(actor.ID, func(inv *state.Inventory) error {
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
			stacks := w.drainEquipment(&player.ActorState, &player.Version, player.ID, journalpkg.PatchPlayerEquipment, journalpkg.PatchPlayerHealth, &player.Stats)
			return toWorldStacks(stacks), true
		}
	}

	if npc, ok := w.npcs[actor.ID]; ok && npc != nil {
		npcID := npc.ID
		cfg.RemoveNPCStacks = func(itemType string) ([]itemspkg.ItemStack, bool) {
			if itemType == "" {
				return nil, true
			}
			target := w.npcs[npcID]
			if target == nil {
				return nil, false
			}
			var removed []state.ItemStack
			_ = w.MutateNPCInventory(npcID, func(inv *state.Inventory) error {
				removed = inv.RemoveAllOf(state.ItemType(itemType))
				return nil
			})
			return toWorldStacks(removed), true
		}

		cfg.DrainNPCInventory = func() ([]itemspkg.ItemStack, bool) {
			target := w.npcs[npcID]
			if target == nil {
				return nil, false
			}
			var stacks []state.ItemStack
			_ = w.MutateNPCInventory(npcID, func(inv *state.Inventory) error {
				stacks = inv.DrainAll()
				return nil
			})
			return toWorldStacks(stacks), true
		}

		cfg.DrainNPCEquipment = func() ([]itemspkg.ItemStack, bool) {
			target := w.npcs[npcID]
			if target == nil {
				return nil, false
			}
			stacks := w.drainEquipment(&target.ActorState, &target.Version, npcID, journalpkg.PatchNPCEquipment, journalpkg.PatchNPCHealth, &target.Stats)
			return toWorldStacks(stacks), true
		}
	}

	cfg.RemoveFallbackStacks = func(itemType string) ([]itemspkg.ItemStack, bool) {
		if itemType == "" {
			return nil, true
		}
		removed := actor.Inventory.RemoveAllOf(state.ItemType(itemType))
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
		stacks := make([]state.ItemStack, 0, len(drained))
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

func (w *World) logGoldDrop(actor *state.ActorState, stack state.ItemStack, reason, stackID string) {
	if w == nil || actor == nil {
		return
	}
	if stack.Type != state.ItemTypeGold || stack.Quantity <= 0 {
		return
	}
	loggingeconomy.GoldDropped(
		context.Background(),
		w.publisher,
		w.currentTick(),
		w.entityRef(actor.ID),
		loggingeconomy.GoldDroppedPayload{Quantity: stack.Quantity, Reason: reason},
		map[string]any{"stackId": stackID},
	)
}

func scatterConfig() itemspkg.ScatterConfig {
	return itemspkg.ScatterConfig{
		TileSize:    groundTileSize,
		MinDistance: groundScatterMinDistance,
		MaxDistance: groundScatterMaxDistance,
		Padding:     groundScatterPadding,
	}
}

func ensureItemFungibilityKey(stack *itemspkg.ItemStack) bool {
	if stack == nil || stack.Type == "" {
		return false
	}
	if stack.FungibilityKey != "" {
		return true
	}
	if def, ok := state.ItemDefinitionFor(state.ItemType(stack.Type)); ok {
		stack.FungibilityKey = def.FungibilityKey
		return stack.FungibilityKey != ""
	}
	return false
}

func toWorldActor(actor *state.ActorState) *itemspkg.Actor {
	if actor == nil {
		return nil
	}
	return &itemspkg.Actor{ID: actor.ID, X: actor.X, Y: actor.Y}
}

func toWorldItemStack(stack state.ItemStack) itemspkg.ItemStack {
	return itemspkg.ItemStack{Type: string(stack.Type), FungibilityKey: stack.FungibilityKey, Quantity: stack.Quantity}
}

func fromWorldItemStack(stack itemspkg.ItemStack) state.ItemStack {
	return state.ItemStack{Type: state.ItemType(stack.Type), FungibilityKey: stack.FungibilityKey, Quantity: stack.Quantity}
}

func toWorldStacks(stacks []state.ItemStack) []itemspkg.ItemStack {
	if len(stacks) == 0 {
		return nil
	}
	converted := make([]itemspkg.ItemStack, 0, len(stacks))
	for _, stack := range stacks {
		converted = append(converted, toWorldItemStack(stack))
	}
	return converted
}

func (w *World) randomAngle() float64 {
	return RandomAngle(w.RNG())
}

func (w *World) randomDistance(min, max float64) float64 {
	return RandomDistance(w.RNG(), min, max)
}
