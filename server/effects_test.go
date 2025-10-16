package main

import (
	"testing"
	"time"

	"mine-and-die/server/logging"
	stats "mine-and-die/server/stats"
)

func TestNPCMiningEmitsInventoryPatch(t *testing.T) {
	w := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	w.players = make(map[string]*playerState)
	w.npcs = make(map[string]*npcState)
	w.obstacles = []Obstacle{{
		ID:     "ore-test-1",
		Type:   obstacleTypeGoldOre,
		X:      0,
		Y:      0,
		Width:  64,
		Height: 64,
	}}

	npc := &npcState{
		actorState: actorState{Actor: Actor{
			ID:        "npc-miner",
			X:         2,
			Y:         2,
			Facing:    defaultFacing,
			Health:    baselinePlayerMaxHealth,
			MaxHealth: baselinePlayerMaxHealth,
			Inventory: NewInventory(),
			Equipment: NewEquipment(),
		}},
		stats: stats.DefaultComponent(stats.ArchetypeGoblin),
	}
	w.npcs[npc.ID] = npc

	effect := &effectState{Effect: Effect{Type: effectTypeAttack}}
	area := Obstacle{X: 0, Y: 0, Width: 64, Height: 64}

	w.resolveMeleeImpact(effect, &npc.actorState, npc.ID, 1, time.Now(), area)

	if qty := npc.Inventory.QuantityOf(ItemTypeGold); qty != 1 {
		t.Fatalf("expected npc to receive 1 gold, got %d", qty)
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}

	patch := patches[0]
	if patch.Kind != PatchNPCInventory {
		t.Fatalf("expected patch kind %q, got %q", PatchNPCInventory, patch.Kind)
	}

	payload, ok := patch.Payload.(NPCInventoryPayload)
	if !ok {
		t.Fatalf("expected payload to be NPCInventoryPayload, got %T", patch.Payload)
	}
	if len(payload.Slots) != 1 {
		t.Fatalf("expected payload to contain 1 slot, got %d", len(payload.Slots))
	}
	if payload.Slots[0].Item.Type != ItemTypeGold {
		t.Fatalf("expected slot item type %q, got %q", ItemTypeGold, payload.Slots[0].Item.Type)
	}
	if payload.Slots[0].Item.Quantity != 1 {
		t.Fatalf("expected slot quantity 1, got %d", payload.Slots[0].Item.Quantity)
	}
}
