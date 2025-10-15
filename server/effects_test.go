package main

import (
	"testing"
	"time"

	"mine-and-die/server/logging"
	stats "mine-and-die/server/stats"
)

func TestNPCMiningEmitsInventoryPatch(t *testing.T) {
	w := newWorld(defaultWorldConfig(), logging.NopPublisher{})
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

func TestSpawnContractBloodDecalRespectsInstanceParams(t *testing.T) {
	world := newWorld(defaultWorldConfig(), logging.NopPublisher{})
	now := time.Unix(0, 0)

	instance := &EffectInstance{
		ID:           "contract-blood-test",
		DefinitionID: effectTypeBloodSplatter,
		DeliveryState: EffectDeliveryState{
			Geometry: EffectGeometry{
				Shape:  GeometryShapeRect,
				Width:  quantizeWorldCoord(playerHalf * 2),
				Height: quantizeWorldCoord(playerHalf * 2),
			},
		},
		BehaviorState: EffectBehaviorState{
			TicksRemaining: durationToTicks(bloodSplatterDuration),
			Extra: map[string]int{
				"centerX": quantizeWorldCoord(128),
				"centerY": quantizeWorldCoord(96),
			},
		},
		Params: map[string]int{
			"minStainRadius": 9,
			"maxStainRadius": 17,
		},
		OwnerActorID: "player-blood-owner",
	}

	effect := world.spawnContractBloodDecalFromInstance(instance, now)
	if effect == nil {
		t.Fatal("expected contract blood decal effect to spawn")
	}

	if effect.Params == nil {
		t.Fatal("expected effect params to be initialised")
	}

	if got := effect.Params["minStainRadius"]; got != 9 {
		t.Fatalf("expected minStainRadius 9, got %.2f", got)
	}
	if got := effect.Params["maxStainRadius"]; got != 17 {
		t.Fatalf("expected maxStainRadius 17, got %.2f", got)
	}

	if got := effect.Params["dropletRadius"]; got != 3 {
		t.Fatalf("expected dropletRadius to remain at default 3, got %.2f", got)
	}
}
