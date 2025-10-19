package main

import (
	"reflect"
	"testing"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
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

func TestSimEffectEventBatchConversionRoundTrip(t *testing.T) {
	legacyBatch := EffectEventBatch{
		Spawns: []effectcontract.EffectSpawnEvent{{
			Tick: 22,
			Seq:  3,
			Instance: effectcontract.EffectInstance{
				ID:           "effect-1",
				EntryID:      "entry-1",
				DefinitionID: "fireball",
				Definition: &effectcontract.EffectDefinition{
					TypeID:        "fireball",
					Delivery:      effectcontract.DeliveryKindArea,
					Shape:         effectcontract.GeometryShapeCircle,
					Motion:        effectcontract.MotionKindLinear,
					Impact:        effectcontract.ImpactPolicyAllInPath,
					LifetimeTicks: 40,
					PierceCount:   2,
					Params:        map[string]int{"speed": 5},
					Hooks:         effectcontract.EffectHooks{OnSpawn: "ignite"},
					Client: effectcontract.ReplicationSpec{
						SendSpawn:    true,
						SendUpdates:  true,
						SendEnd:      true,
						UpdateFields: map[string]bool{"position": true},
					},
					End: effectcontract.EndPolicy{Kind: effectcontract.EndDuration},
				},
				StartTick:     19,
				Params:        map[string]int{"power": 10},
				Colors:        []string{"scarlet", "amber"},
				FollowActorID: "player-1",
				OwnerActorID:  "player-1",
				DeliveryState: effectcontract.EffectDeliveryState{
					Geometry: effectcontract.EffectGeometry{
						Shape:    effectcontract.GeometryShapeArc,
						Width:    12,
						Height:   4,
						Variants: map[string]int{"alternate": 2},
					},
					Motion: effectcontract.EffectMotionState{
						PositionX:      1,
						PositionY:      2,
						VelocityX:      3,
						VelocityY:      4,
						RangeRemaining: 9,
					},
					AttachedActorID: "player-2",
					Follow:          effectcontract.FollowTarget,
				},
				BehaviorState: effectcontract.EffectBehaviorState{
					TicksRemaining:    12,
					CooldownTicks:     2,
					TickCadence:       3,
					AccumulatedDamage: 5,
					Stacks:            map[string]int{"charge": 3},
					Extra:             map[string]int{"slow": 1},
				},
				Replication: effectcontract.ReplicationSpec{
					SendSpawn:    true,
					SendUpdates:  true,
					SendEnd:      true,
					UpdateFields: map[string]bool{"motion": true},
				},
				End: effectcontract.EndPolicy{
					Kind: effectcontract.EndInstant,
					Conditions: effectcontract.EndConditions{
						OnExplicitCancel: true,
					},
				},
			},
		}},
		Updates: []effectcontract.EffectUpdateEvent{{
			Tick: 23,
			Seq:  4,
			ID:   "effect-1",
			DeliveryState: &effectcontract.EffectDeliveryState{
				Geometry: effectcontract.EffectGeometry{
					Shape:    effectcontract.GeometryShapeRect,
					Width:    18,
					Height:   7,
					Variants: map[string]int{"focused": 1},
				},
				Motion: effectcontract.EffectMotionState{
					PositionX: 6,
					PositionY: 8,
					VelocityX: 2,
					VelocityY: -1,
				},
				AttachedActorID: "player-3",
				Follow:          effectcontract.FollowOwner,
			},
			BehaviorState: &effectcontract.EffectBehaviorState{
				TicksRemaining:    8,
				CooldownTicks:     1,
				TickCadence:       2,
				AccumulatedDamage: 7,
				Stacks:            map[string]int{"charge": 3},
				Extra:             map[string]int{"slow": 1},
			},
			Params: map[string]int{"damage": 7},
		}},
		Ends: []effectcontract.EffectEndEvent{{
			Tick:   24,
			Seq:    5,
			ID:     "effect-1",
			Reason: effectcontract.EndReasonCancelled,
		}},
		LastSeqByID: map[string]effectcontract.Seq{
			"effect-1": 5,
			"effect-2": 9,
		},
	}

	simBatch := simEffectEventBatchFromLegacy(legacyBatch)
	roundTrip := legacyEffectEventBatchFromSim(simBatch)

	if !reflect.DeepEqual(legacyBatch, roundTrip) {
		t.Fatalf("effect batch round trip mismatch\nlegacy: %#v\nround-trip: %#v", legacyBatch, roundTrip)
	}

	simBatch.Spawns[0].Instance.Params["power"] = 99
	if legacyBatch.Spawns[0].Instance.Params["power"] != 10 {
		t.Fatalf("expected spawn params to remain unchanged after sim mutation, got %v", legacyBatch.Spawns[0].Instance.Params["power"])
	}
	simBatch.Spawns[0].Instance.Colors[0] = "violet"
	if legacyBatch.Spawns[0].Instance.Colors[0] != "scarlet" {
		t.Fatalf("expected spawn colors to remain unchanged after sim mutation, got %v", legacyBatch.Spawns[0].Instance.Colors)
	}
	simBatch.Updates[0].Params["damage"] = 11
	if legacyBatch.Updates[0].Params["damage"] != 7 {
		t.Fatalf("expected update params to remain unchanged after sim mutation, got %v", legacyBatch.Updates[0].Params["damage"])
	}
	simBatch.Updates[0].DeliveryState.Geometry.Width = 99
	if legacyBatch.Updates[0].DeliveryState.Geometry.Width != 18 {
		t.Fatalf("expected update geometry to remain unchanged after sim mutation, got %v", legacyBatch.Updates[0].DeliveryState.Geometry.Width)
	}
	simBatch.Updates[0].BehaviorState.Stacks["charge"] = 6
	if legacyBatch.Updates[0].BehaviorState.Stacks["charge"] != 3 {
		t.Fatalf("expected update stacks to remain unchanged after sim mutation, got %v", legacyBatch.Updates[0].BehaviorState.Stacks["charge"])
	}
	simBatch.LastSeqByID["effect-1"] = 12
	if legacyBatch.LastSeqByID["effect-1"] != 5 {
		t.Fatalf("expected seq cursor to remain unchanged after sim mutation, got %v", legacyBatch.LastSeqByID["effect-1"])
	}

	roundTrip.Spawns[0].Instance.Definition.Params["speed"] = 42
	if simBatch.Spawns[0].Instance.Definition.Params["speed"] != 5 {
		t.Fatalf("expected definition params to remain unchanged after legacy mutation, got %v", simBatch.Spawns[0].Instance.Definition.Params["speed"])
	}
	roundTrip.Updates[0].BehaviorState.Extra["slow"] = 5
	if simBatch.Updates[0].BehaviorState.Extra["slow"] != 1 {
		t.Fatalf("expected update extra map to remain unchanged after legacy mutation, got %v", simBatch.Updates[0].BehaviorState.Extra["slow"])
	}
	roundTrip.LastSeqByID["effect-2"] = 20
	if simBatch.LastSeqByID["effect-2"] != 9 {
		t.Fatalf("expected seq cursor map to remain unchanged after legacy mutation, got %v", simBatch.LastSeqByID["effect-2"])
	}
}

func TestSimEffectResyncSignalConversionRoundTrip(t *testing.T) {
	legacySignal := resyncSignal{
		LostSpawns:  3,
		TotalEvents: 120,
		Reasons: []resyncReason{
			{Kind: "lost_spawn", EffectID: "effect-1"},
			{Kind: "stale_update", EffectID: "effect-2"},
		},
	}

	simSignal := simEffectResyncSignalFromLegacy(legacySignal)
	roundTrip := legacyEffectResyncSignalFromSim(simSignal)

	if !reflect.DeepEqual(legacySignal, roundTrip) {
		t.Fatalf("effect resync signal round trip mismatch\nlegacy: %#v\nround: %#v", legacySignal, roundTrip)
	}
	if len(roundTrip.Reasons) == 0 {
		t.Fatalf("expected round-trip signal to include reasons")
	}
	if &roundTrip.Reasons[0] == &legacySignal.Reasons[0] {
		t.Fatalf("expected reasons slice to be copied, got shared element")
	}
	roundTrip.Reasons[0].Kind = "mutated"
	if legacySignal.Reasons[0].Kind != "lost_spawn" {
		t.Fatalf("expected legacy signal to remain unchanged after mutation")
	}
}
