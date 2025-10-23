package simutil

import (
	effectspkg "mine-and-die/server/internal/effects"
	itemspkg "mine-and-die/server/internal/items"
	itemssim "mine-and-die/server/internal/items/simpayloads"
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
		AliveEffectIDs: effectspkg.CloneAliveEffectIDs(snapshot.AliveEffectIDs),
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
	return sim.Inventory{Slots: itemssim.CloneInventorySlots(inv.Slots)}
}

// CloneEquipment returns a deep copy of the provided equipment.
func CloneEquipment(eq sim.Equipment) sim.Equipment {
	return sim.Equipment{Slots: itemssim.CloneEquippedItems(eq.Slots)}
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
		return sim.InventoryPayload{Slots: itemssim.CloneInventorySlots(value.Slots)}
	case *sim.InventoryPayload:
		if value == nil {
			return nil
		}
		cloned := sim.InventoryPayload{Slots: itemssim.CloneInventorySlots(value.Slots)}
		return cloned
	case sim.EquipmentPayload:
		return sim.EquipmentPayload{Slots: itemssim.CloneEquippedItems(value.Slots)}
	case *sim.EquipmentPayload:
		if value == nil {
			return nil
		}
		cloned := sim.EquipmentPayload{Slots: itemssim.CloneEquippedItems(value.Slots)}
		return cloned
	case sim.EffectParamsPayload:
		return sim.EffectParamsPayload{Params: effectspkg.CloneEffectParams(value.Params)}
	case *sim.EffectParamsPayload:
		if value == nil {
			return nil
		}
		cloned := sim.EffectParamsPayload{Params: effectspkg.CloneEffectParams(value.Params)}
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

// CloneStringSlice returns a deep copy of the provided string slice.
func CloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}
