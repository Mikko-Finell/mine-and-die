package main

import (
	"reflect"
	"testing"
	"time"

	"mine-and-die/server/internal/sim"
)

func TestSimCommandConversionRoundTrip(t *testing.T) {
	issuedAt := time.Unix(1700000000, 1234).UTC()
	heartbeatAt := issuedAt.Add(10 * time.Millisecond)

	legacy := []Command{{
		OriginTick: 42,
		ActorID:    "player-1",
		Type:       CommandMove,
		IssuedAt:   issuedAt,
		Move: &MoveCommand{
			DX:     1.5,
			DY:     -2.25,
			Facing: FacingLeft,
		},
		Action: &ActionCommand{Name: "attack"},
		Heartbeat: &HeartbeatCommand{
			ReceivedAt: heartbeatAt,
			ClientSent: heartbeatAt.Add(-5 * time.Millisecond).UnixMilli(),
			RTT:        5 * time.Millisecond,
		},
		Path: &PathCommand{TargetX: 12.5, TargetY: -4.25},
	}}

	converted := simCommandsFromLegacy(legacy)
	roundTrip := legacyCommandsFromSim(converted)

	if !reflect.DeepEqual(legacy, roundTrip) {
		t.Fatalf("command round trip mismatch\nlegacy: %#v\nround-trip: %#v", legacy, roundTrip)
	}
}

func TestSimSnapshotConversionRoundTrip(t *testing.T) {
	legacyPlayers := []Player{{
		Actor: Actor{
			ID:        "player-1",
			X:         10.5,
			Y:         -3.75,
			Facing:    FacingUp,
			Health:    87,
			MaxHealth: 100,
			Inventory: Inventory{Slots: []InventorySlot{{
				Slot: 1,
				Item: ItemStack{Type: ItemTypeGold, FungibilityKey: "gold", Quantity: 30},
			}}},
			Equipment: Equipment{Slots: []EquippedItem{{
				Slot: EquipSlotMainHand,
				Item: ItemStack{Type: ItemType("sword"), FungibilityKey: "", Quantity: 1},
			}}},
		},
	}}
	legacyNPCs := []NPC{{
		Actor: Actor{
			ID:        "npc-1",
			X:         -2,
			Y:         8,
			Facing:    FacingDown,
			Health:    25,
			MaxHealth: 30,
			Inventory: Inventory{},
			Equipment: Equipment{},
		},
		Type:             NPCTypeGoblin,
		AIControlled:     true,
		ExperienceReward: 12,
	}}
	legacyGround := []GroundItem{{
		ID:             "ground-1",
		Type:           ItemType("potion"),
		FungibilityKey: "potion-small",
		X:              1.25,
		Y:              -6.5,
		Qty:            3,
	}}
	legacyEffects := []EffectTrigger{{
		ID:       "effect-1",
		Type:     "fireball",
		Start:    10,
		Duration: 20,
		X:        5.5,
		Y:        6.5,
		Width:    2.5,
		Height:   3.5,
		Params:   map[string]float64{"radius": 3.5},
		Colors:   []string{"#ff0000", "#ffaa00"},
	}}

	snapshot := sim.Snapshot{
		Players:      simPlayersFromLegacy(legacyPlayers),
		NPCs:         simNPCsFromLegacy(legacyNPCs),
		GroundItems:  simGroundItemsFromLegacy(legacyGround),
		EffectEvents: simEffectTriggersFromLegacy(legacyEffects),
	}

	roundPlayers := legacyPlayersFromSim(snapshot.Players)
	roundNPCs := legacyNPCsFromSim(snapshot.NPCs)
	roundGround := legacyGroundItemsFromSim(snapshot.GroundItems)
	roundEffects := legacyEffectTriggersFromSim(snapshot.EffectEvents)

	if !reflect.DeepEqual(legacyPlayers, roundPlayers) {
		t.Fatalf("player snapshot round trip mismatch\nlegacy: %#v\nround: %#v", legacyPlayers, roundPlayers)
	}
	if !reflect.DeepEqual(legacyNPCs, roundNPCs) {
		t.Fatalf("npc snapshot round trip mismatch\nlegacy: %#v\nround: %#v", legacyNPCs, roundNPCs)
	}
	if !reflect.DeepEqual(legacyGround, roundGround) {
		t.Fatalf("ground item snapshot round trip mismatch\nlegacy: %#v\nround: %#v", legacyGround, roundGround)
	}
	if !reflect.DeepEqual(legacyEffects, roundEffects) {
		t.Fatalf("effect trigger snapshot round trip mismatch\nlegacy: %#v\nround: %#v", legacyEffects, roundEffects)
	}

	snapshot.Players[0].Inventory.Slots[0].Item.Quantity = 999
	if legacyQty := legacyPlayers[0].Inventory.Slots[0].Item.Quantity; legacyQty != 30 {
		t.Fatalf("player inventory conversion should deep copy slots, got %v", legacyQty)
	}
	if &legacyEffects[0] == &roundEffects[0] {
		t.Fatalf("effect trigger conversion should copy data")
	}
	if legacyEffects[0].Params["radius"] != 3.5 {
		t.Fatalf("effect trigger params should be copied, got %v", legacyEffects[0].Params)
	}
	if legacyEffects[0].Colors[0] != "#ff0000" {
		t.Fatalf("effect trigger colors should be copied, got %v", legacyEffects[0].Colors)
	}
}

func TestSimPatchConversionRoundTrip(t *testing.T) {
	legacyPatches := []Patch{
		{Kind: PatchPlayerPos, EntityID: "player-1", Payload: PositionPayload{X: 1.5, Y: -2.25}},
		{Kind: PatchPlayerFacing, EntityID: "player-2", Payload: FacingPayload{Facing: FacingRight}},
		{Kind: PatchPlayerIntent, EntityID: "player-3", Payload: PlayerIntentPayload{DX: 0.5, DY: 0.75}},
		{Kind: PatchPlayerHealth, EntityID: "player-4", Payload: HealthPayload{Health: 75, MaxHealth: 100}},
		{Kind: PatchPlayerInventory, EntityID: "player-5", Payload: InventoryPayload{Slots: []InventorySlot{{
			Slot: 3,
			Item: ItemStack{Type: ItemType("arrow"), Quantity: 20},
		}}}},
		{Kind: PatchPlayerEquipment, EntityID: "player-6", Payload: EquipmentPayload{Slots: []EquippedItem{{
			Slot: EquipSlotHead,
			Item: ItemStack{Type: ItemType("helm"), Quantity: 1},
		}}}},
		{Kind: PatchEffectParams, EntityID: "effect-1", Payload: EffectParamsPayload{Params: map[string]float64{"radius": 4}}},
		{Kind: PatchGroundItemQty, EntityID: "ground-1", Payload: GroundItemQtyPayload{Qty: 9}},
	}

	simPatches := simPatchesFromLegacy(legacyPatches)
	roundTrip := legacyPatchesFromSim(simPatches)

	if !reflect.DeepEqual(legacyPatches, roundTrip) {
		t.Fatalf("patch round trip mismatch\nlegacy: %#v\nround: %#v", legacyPatches, roundTrip)
	}

	simPayload := simPatches[6].Payload.(sim.EffectParamsPayload)
	simPayload.Params["radius"] = 8
	if legacyParams := legacyPatches[6].Payload.(EffectParamsPayload).Params["radius"]; legacyParams != 4 {
		t.Fatalf("expected legacy effect params to remain unchanged, got %v", legacyParams)
	}
}
