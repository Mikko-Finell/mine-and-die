package world

import (
	itemspkg "mine-and-die/server/internal/items"
	journalpkg "mine-and-die/server/internal/journal"
	simpkg "mine-and-die/server/internal/sim"
	state "mine-and-die/server/internal/world/state"
	stats "mine-and-die/server/stats"
)

// PatchEmitter emits journal patches for actor mutations.
type PatchEmitter func(kind journalpkg.PatchKind, entityID string, payload any)

// ApplyActorHealth updates an actor's health/max-health and emits the resulting patch via the provided emitter.
func ApplyActorHealth(actor *state.ActorState, version *uint64, entityID string, kind journalpkg.PatchKind, computedMax float64, health float64, emit PatchEmitter) bool {
	if actor == nil || version == nil || entityID == "" || emit == nil {
		return false
	}

	state := HealthState{Health: actor.Health, MaxHealth: actor.MaxHealth}
	if !SetActorHealth(&state, computedMax, health) {
		return false
	}

	actor.Health = state.Health
	actor.MaxHealth = state.MaxHealth
	incrementVersion(version)

	emit(kind, entityID, journalpkg.HealthPayload{Health: state.Health, MaxHealth: state.MaxHealth})
	return true
}

// SetPlayerHealth updates a player's health, bumps the version, and records a patch.
func (w *World) SetPlayerHealth(playerID string, health float64) {
	if w == nil || playerID == "" {
		return
	}

	player, ok := w.players[playerID]
	if !ok || player == nil {
		return
	}

	player.Stats.Resolve(w.currentTick())
	max := player.Stats.GetDerived(stats.DerivedMaxHealth)
	_ = ApplyActorHealth(&player.ActorState, &player.Version, playerID, journalpkg.PatchPlayerHealth, max, health, func(kind journalpkg.PatchKind, entityID string, payload any) {
		w.AppendPatch(journalpkg.Patch{Kind: kind, EntityID: entityID, Payload: payload})
	})
}

// SetNPCHealth updates an NPC's health, bumps the version, and records a patch.
func (w *World) SetNPCHealth(npcID string, health float64) {
	if w == nil || npcID == "" {
		return
	}

	npc, ok := w.npcs[npcID]
	if !ok || npc == nil {
		return
	}

	npc.Stats.Resolve(w.currentTick())
	max := npc.Stats.GetDerived(stats.DerivedMaxHealth)
	_ = ApplyActorHealth(&npc.ActorState, &npc.Version, npcID, journalpkg.PatchNPCHealth, max, health, func(kind journalpkg.PatchKind, entityID string, payload any) {
		w.AppendPatch(journalpkg.Patch{Kind: kind, EntityID: entityID, Payload: payload})
	})
}

// MutateInventory applies the provided mutation to a player's inventory.
func (w *World) MutateInventory(playerID string, mutate func(inv *state.Inventory) error) error {
	if w == nil || playerID == "" || mutate == nil {
		return nil
	}

	player, ok := w.players[playerID]
	if !ok || player == nil {
		return nil
	}

	return MutateActorInventory(&player.ActorState, &player.Version, playerID, journalpkg.PatchPlayerInventory, mutate, func(kind journalpkg.PatchKind, entityID string, payload any) {
		w.AppendPatch(journalpkg.Patch{Kind: kind, EntityID: entityID, Payload: payload})
	})
}

// MutateNPCInventory applies the provided mutation to an NPC inventory.
func (w *World) MutateNPCInventory(npcID string, mutate func(inv *state.Inventory) error) error {
	if w == nil || npcID == "" || mutate == nil {
		return nil
	}

	npc, ok := w.npcs[npcID]
	if !ok || npc == nil {
		return nil
	}

	return MutateActorInventory(&npc.ActorState, &npc.Version, npcID, journalpkg.PatchNPCInventory, mutate, func(kind journalpkg.PatchKind, entityID string, payload any) {
		w.AppendPatch(journalpkg.Patch{Kind: kind, EntityID: entityID, Payload: payload})
	})
}

// MutateEquipment applies the provided mutation to an actor's equipment while preserving patches.
func (w *World) MutateEquipment(entityID string, mutate func(eq *state.Equipment) error) error {
	if w == nil || entityID == "" || mutate == nil {
		return nil
	}

	if player, ok := w.players[entityID]; ok && player != nil {
		return MutateActorEquipment(&player.ActorState, &player.Version, entityID, journalpkg.PatchPlayerEquipment, mutate, func(kind journalpkg.PatchKind, entityID string, payload any) {
			w.AppendPatch(journalpkg.Patch{Kind: kind, EntityID: entityID, Payload: payload})
		})
	}

	if npc, ok := w.npcs[entityID]; ok && npc != nil {
		return MutateActorEquipment(&npc.ActorState, &npc.Version, entityID, journalpkg.PatchNPCEquipment, mutate, func(kind journalpkg.PatchKind, entityID string, payload any) {
			w.AppendPatch(journalpkg.Patch{Kind: kind, EntityID: entityID, Payload: payload})
		})
	}

	return nil
}

func incrementVersion(version *uint64) {
	if version == nil {
		return
	}
	*version = *version + 1
}

func inventoriesEqual(a, b state.Inventory) bool {
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
		if as.Item.FungibilityKey != bs.Item.FungibilityKey {
			return false
		}
		if as.Item.Quantity != bs.Item.Quantity {
			return false
		}
	}
	return true
}

func cloneInventorySlots(slots []state.InventorySlot) []state.InventorySlot {
	if len(slots) == 0 {
		return nil
	}
	cloned := make([]state.InventorySlot, len(slots))
	copy(cloned, slots)
	return cloned
}

func (w *World) drainEquipment(actor *state.ActorState, version *uint64, entityID string, equipPatchKind, healthPatchKind journalpkg.PatchKind, comp *stats.Component) []state.ItemStack {
	if w == nil || actor == nil || version == nil || entityID == "" || comp == nil {
		return nil
	}

	var drained []state.EquippedItem
	_ = MutateActorEquipment(actor, version, entityID, equipPatchKind, func(eq *state.Equipment) error {
		drained = eq.DrainAll()
		return nil
	}, func(kind journalpkg.PatchKind, entityID string, payload any) {
		w.AppendPatch(journalpkg.Patch{Kind: kind, EntityID: entityID, Payload: payload})
	})

	if len(drained) == 0 {
		return nil
	}

	for _, entry := range drained {
		slotKey := stats.SourceKey{Kind: stats.SourceKindEquipment, ID: string(entry.Slot)}
		comp.Apply(stats.CommandStatChange{Layer: stats.LayerEquipment, Source: slotKey, Remove: true})
	}

	comp.Resolve(w.currentTick())
	_ = ApplyActorHealth(actor, version, entityID, healthPatchKind, comp.GetDerived(stats.DerivedMaxHealth), actor.Health, func(kind journalpkg.PatchKind, entityID string, payload any) {
		w.AppendPatch(journalpkg.Patch{Kind: kind, EntityID: entityID, Payload: payload})
	})

	stacks := make([]state.ItemStack, 0, len(drained))
	for _, entry := range drained {
		stacks = append(stacks, entry.Item)
	}
	return stacks
}

// MutateActorInventory mutates the provided actor inventory, updates versioning, and emits a patch through the emitter.
func MutateActorInventory(actor *state.ActorState, version *uint64, entityID string, kind journalpkg.PatchKind, mutate func(inv *state.Inventory) error, emit PatchEmitter) error {
	if actor == nil || version == nil || entityID == "" || mutate == nil || emit == nil {
		return nil
	}

	return itemspkg.MutateActorInventory(
		&actor.Inventory,
		version,
		mutate,
		func(inv state.Inventory) state.Inventory { return inv.Clone() },
		inventoriesEqual,
		func(inv state.Inventory) {
			cloned := cloneInventorySlots(inv.Slots)
			slots := itemspkg.SimInventorySlotsFromAny(cloned)
			payload := itemspkg.SimInventoryPayloadFromSlots[simpkg.InventorySlot, journalpkg.InventoryPayload](slots)
			emit(kind, entityID, payload)
		},
	)
}

// MutateActorEquipment mutates the provided actor equipment and emits the resulting patch.
func MutateActorEquipment(actor *state.ActorState, version *uint64, entityID string, kind journalpkg.PatchKind, mutate func(eq *state.Equipment) error, emit PatchEmitter) error {
	if actor == nil || version == nil || entityID == "" || mutate == nil || emit == nil {
		return nil
	}

	return itemspkg.MutateActorEquipment(
		&actor.Equipment,
		version,
		mutate,
		func(eq state.Equipment) state.Equipment { return eq.Clone() },
		state.EquipmentsEqual,
		func(eq state.Equipment) {
			cloned := state.CloneEquipmentSlots(eq.Slots)
			slots := itemspkg.SimEquippedItemsFromAny(cloned)
			payload := itemspkg.SimEquipmentPayloadFromSlots[simpkg.EquippedItem, journalpkg.EquipmentPayload](slots)
			emit(kind, entityID, payload)
		},
	)
}
