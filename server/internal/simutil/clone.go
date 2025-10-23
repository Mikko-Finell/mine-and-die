package simutil

import (
	effectspkg "mine-and-die/server/internal/effects"
	itemspkg "mine-and-die/server/internal/items"
	"mine-and-die/server/internal/sim"
)

// CloneSnapshot returns a deep copy of the provided snapshot, including nested
// player, NPC, ground item, effect trigger, obstacle, and alive effect ID data.
func CloneSnapshot(snapshot sim.Snapshot) sim.Snapshot {
	return sim.Snapshot{
		Players:        ClonePlayers(snapshot.Players),
		NPCs:           CloneNPCs(snapshot.NPCs),
		GroundItems:    itemspkg.CloneGroundItems(snapshot.GroundItems),
		EffectEvents:   effectspkg.CloneEffectTriggers(snapshot.EffectEvents),
		Obstacles:      CloneObstacles(snapshot.Obstacles),
		AliveEffectIDs: CloneAliveEffectIDs(snapshot.AliveEffectIDs),
	}
}

// ClonePlayers returns a deep copy of the provided player slice.
func ClonePlayers(players []sim.Player) []sim.Player {
	if len(players) == 0 {
		return nil
	}
	cloned := make([]sim.Player, len(players))
	for i, player := range players {
		cloned[i] = ClonePlayer(player)
	}
	return cloned
}

// ClonePlayer returns a deep copy of the provided player.
func ClonePlayer(player sim.Player) sim.Player {
	cloned := player
	cloned.Actor = CloneActor(player.Actor)
	return cloned
}

// CloneNPCs returns a deep copy of the provided NPC slice.
func CloneNPCs(npcs []sim.NPC) []sim.NPC {
	if len(npcs) == 0 {
		return nil
	}
	cloned := make([]sim.NPC, len(npcs))
	for i, npc := range npcs {
		cloned[i] = CloneNPC(npc)
	}
	return cloned
}

// CloneNPC returns a deep copy of the provided NPC.
func CloneNPC(npc sim.NPC) sim.NPC {
	cloned := npc
	cloned.Actor = CloneActor(npc.Actor)
	return cloned
}

// CloneActor returns a deep copy of the provided actor.
func CloneActor(actor sim.Actor) sim.Actor {
	cloned := actor
	cloned.Inventory = CloneInventory(actor.Inventory)
	cloned.Equipment = CloneEquipment(actor.Equipment)
	return cloned
}

// CloneInventory returns a deep copy of the provided inventory.
func CloneInventory(inv sim.Inventory) sim.Inventory {
	return sim.Inventory{Slots: CloneInventorySlots(inv.Slots)}
}

// CloneInventorySlots returns a deep copy of the provided inventory slots.
func CloneInventorySlots(slots []sim.InventorySlot) []sim.InventorySlot {
	if len(slots) == 0 {
		return nil
	}
	cloned := make([]sim.InventorySlot, len(slots))
	copy(cloned, slots)
	return cloned
}

// CloneEquipment returns a deep copy of the provided equipment.
func CloneEquipment(eq sim.Equipment) sim.Equipment {
	return sim.Equipment{Slots: CloneEquippedItems(eq.Slots)}
}

// CloneEquippedItems returns a deep copy of the provided equipped item slots.
func CloneEquippedItems(slots []sim.EquippedItem) []sim.EquippedItem {
	if len(slots) == 0 {
		return nil
	}
	cloned := make([]sim.EquippedItem, len(slots))
	copy(cloned, slots)
	return cloned
}

// CloneObstacles returns a deep copy of the provided obstacle slice.
func CloneObstacles(obstacles []sim.Obstacle) []sim.Obstacle {
	if len(obstacles) == 0 {
		return nil
	}
	cloned := make([]sim.Obstacle, len(obstacles))
	copy(cloned, obstacles)
	return cloned
}

// CloneAliveEffectIDs returns a deep copy of the provided effect ID slice,
// filtering out empty identifiers.
func CloneAliveEffectIDs(ids []string) []string {
	if len(ids) == 0 {
		return nil
	}
	filtered := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		filtered = append(filtered, id)
	}
	if len(filtered) == 0 {
		return nil
	}
	return CloneStringSlice(filtered)
}

// ClonePatches returns a deep copy of the provided patch slice.
func ClonePatches(patches []sim.Patch) []sim.Patch {
	if len(patches) == 0 {
		return nil
	}
	cloned := make([]sim.Patch, len(patches))
	for i, patch := range patches {
		cloned[i] = sim.Patch{
			Kind:     patch.Kind,
			EntityID: patch.EntityID,
			Payload:  ClonePatchPayload(patch.Payload),
		}
	}
	return cloned
}

// ClonePatchPayload returns a deep copy of a patch payload.
func ClonePatchPayload(payload any) any {
	switch value := payload.(type) {
	case nil:
		return nil
	case sim.PositionPayload:
		return value
	case *sim.PositionPayload:
		if value == nil {
			return nil
		}
		cloned := *value
		return cloned
	case sim.FacingPayload:
		return value
	case *sim.FacingPayload:
		if value == nil {
			return nil
		}
		cloned := *value
		return cloned
	case sim.PlayerIntentPayload:
		return value
	case *sim.PlayerIntentPayload:
		if value == nil {
			return nil
		}
		cloned := *value
		return cloned
	case sim.HealthPayload:
		return value
	case *sim.HealthPayload:
		if value == nil {
			return nil
		}
		cloned := *value
		return cloned
	case sim.InventoryPayload:
		return sim.InventoryPayload{Slots: CloneInventorySlots(value.Slots)}
	case *sim.InventoryPayload:
		if value == nil {
			return nil
		}
		cloned := sim.InventoryPayload{Slots: CloneInventorySlots(value.Slots)}
		return cloned
	case sim.EquipmentPayload:
		return sim.EquipmentPayload{Slots: CloneEquippedItems(value.Slots)}
	case *sim.EquipmentPayload:
		if value == nil {
			return nil
		}
		cloned := sim.EquipmentPayload{Slots: CloneEquippedItems(value.Slots)}
		return cloned
	case sim.EffectParamsPayload:
		return sim.EffectParamsPayload{Params: CloneFloatMap(value.Params)}
	case *sim.EffectParamsPayload:
		if value == nil {
			return nil
		}
		cloned := sim.EffectParamsPayload{Params: CloneFloatMap(value.Params)}
		return cloned
	case sim.GroundItemQtyPayload:
		return value
	case *sim.GroundItemQtyPayload:
		if value == nil {
			return nil
		}
		cloned := *value
		return cloned
	default:
		return payload
	}
}

// CloneFloatMap returns a deep copy of the provided float map.
func CloneFloatMap(values map[string]float64) map[string]float64 {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]float64, len(values))
	for k, v := range values {
		cloned[k] = v
	}
	return cloned
}

// CloneStringSlice returns a deep copy of the provided string slice.
func CloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}
