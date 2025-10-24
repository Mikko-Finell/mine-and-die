package server

import (
	"reflect"
	"testing"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	internaleffects "mine-and-die/server/internal/effects"
	itemspkg "mine-and-die/server/internal/items"
	journal "mine-and-die/server/internal/journal"
	"mine-and-die/server/internal/sim"
	simutil "mine-and-die/server/internal/simutil"
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
			Inventory: itemspkg.InventoryValueFromSlots[InventorySlot, Inventory]([]InventorySlot{{
				Slot: 1,
				Item: ItemStack{Type: ItemTypeGold, FungibilityKey: "gold", Quantity: 30},
			}}),
			Equipment: itemspkg.EquipmentValueFromSlots[EquippedItem, Equipment]([]EquippedItem{{
				Slot: EquipSlotMainHand,
				Item: ItemStack{Type: ItemType("sword"), FungibilityKey: "", Quantity: 1},
			}}),
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
	legacyGround := []itemspkg.GroundItem{{
		ID:             "ground-1",
		Type:           "potion",
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
	legacyObstacles := []Obstacle{{
		ID:     "obstacle-1",
		Type:   "rock",
		X:      2,
		Y:      3,
		Width:  1,
		Height: 1,
	}}
	aliveEffects := []string{"effect-1", "effect-2"}

	snapshot := sim.Snapshot{
		Players:        simPlayersFromLegacy(legacyPlayers),
		NPCs:           simNPCsFromLegacy(legacyNPCs),
		GroundItems:    itemspkg.CloneGroundItems(legacyGround),
		EffectEvents:   internaleffects.SimEffectTriggersFromLegacy(legacyEffects),
		Obstacles:      simObstaclesFromLegacy(legacyObstacles),
		AliveEffectIDs: append([]string(nil), aliveEffects...),
	}

	roundPlayers := legacyPlayersFromSim(snapshot.Players)
	roundNPCs := legacyNPCsFromSim(snapshot.NPCs)
	roundGround := itemspkg.CloneGroundItems(snapshot.GroundItems)
	roundEffects := internaleffects.LegacyEffectTriggersFromSim(snapshot.EffectEvents)
	roundObstacles := legacyObstaclesFromSim(snapshot.Obstacles)

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
	if !reflect.DeepEqual(legacyObstacles, roundObstacles) {
		t.Fatalf("obstacle snapshot round trip mismatch\nlegacy: %#v\nround: %#v", legacyObstacles, roundObstacles)
	}
	if !reflect.DeepEqual(aliveEffects, snapshot.AliveEffectIDs) {
		t.Fatalf("alive effect IDs should copy legacy slice, got %#v", snapshot.AliveEffectIDs)
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
	snapshot.AliveEffectIDs[0] = "mutated"
	if aliveEffects[0] != "effect-1" {
		t.Fatalf("alive effect ID conversion should copy data")
	}
}

func TestAliveEffectIDsFromStates(t *testing.T) {
	if ids := internaleffects.AliveEffectIDsFromStates(nil); ids != nil {
		t.Fatalf("expected nil slice for nil effects, got %#v", ids)
	}

	effects := []*effectState{
		{ID: "effect-1"},
		nil,
		{ID: ""},
		{ID: "effect-2"},
	}

	ids := internaleffects.AliveEffectIDsFromStates(effects)
	expected := []string{"effect-1", "effect-2"}
	if !reflect.DeepEqual(expected, ids) {
		t.Fatalf("expected %v, got %v", expected, ids)
	}

	effects[0].ID = "mutated"
	if ids[0] != "effect-1" {
		t.Fatalf("conversion should copy IDs, got %v", ids[0])
	}
}

func TestSimPatchConversionRoundTrip(t *testing.T) {
	legacyPatches := []Patch{
		{Kind: PatchPlayerPos, EntityID: "player-1", Payload: PositionPayload{X: 1.5, Y: -2.25}},
		{Kind: PatchPlayerFacing, EntityID: "player-2", Payload: FacingPayload{Facing: FacingRight}},
		{Kind: PatchPlayerIntent, EntityID: "player-3", Payload: PlayerIntentPayload{DX: 0.5, DY: 0.75}},
		{Kind: PatchPlayerHealth, EntityID: "player-4", Payload: HealthPayload{Health: 75, MaxHealth: 100}},
		{Kind: PatchPlayerInventory, EntityID: "player-5", Payload: itemspkg.InventoryPayloadFromSlots[InventorySlot, InventoryPayload]([]InventorySlot{{
			Slot: 3,
			Item: ItemStack{Type: ItemType("arrow"), Quantity: 20},
		}})},
		{Kind: PatchPlayerEquipment, EntityID: "player-6", Payload: itemspkg.EquipmentPayloadFromSlots[EquippedItem, EquipmentPayload]([]EquippedItem{{
			Slot: EquipSlotHead,
			Item: ItemStack{Type: ItemType("helm"), Quantity: 1},
		}})},
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

func TestConvertPatchPayloadToSimInventoryPointerNil(t *testing.T) {
	if converted := convertPatchPayloadToSim((*InventoryPayload)(nil)); converted != nil {
		t.Fatalf("expected nil result for nil inventory payload pointer, got %T", converted)
	}
}

func TestConvertPatchPayloadToSimInventoryPointerClones(t *testing.T) {
	slots := []InventorySlot{{
		Slot: 4,
		Item: ItemStack{Type: ItemType("arrow"), FungibilityKey: "stack", Quantity: 3},
	}}

	payload := itemspkg.InventoryPayloadFromSlots[InventorySlot, InventoryPayload](slots)
	converted, ok := convertPatchPayloadToSim(&payload).(sim.InventoryPayload)
	if !ok {
		t.Fatalf("expected sim.InventoryPayload, got %T", convertPatchPayloadToSim(payload))
	}
	if len(converted.Slots) != 1 {
		t.Fatalf("expected 1 inventory slot, got %d", len(converted.Slots))
	}
	if converted.Slots[0].Item.Quantity != 3 {
		t.Fatalf("expected quantity 3, got %d", converted.Slots[0].Item.Quantity)
	}

	slots[0].Item.Quantity = 9
	if converted.Slots[0].Item.Quantity != 3 {
		t.Fatalf("expected converted payload to remain unchanged, got %d", converted.Slots[0].Item.Quantity)
	}
}

func TestConvertPatchPayloadToSimEquipmentPointerNil(t *testing.T) {
	if converted := convertPatchPayloadToSim((*EquipmentPayload)(nil)); converted != nil {
		t.Fatalf("expected nil result for nil equipment payload pointer, got %T", converted)
	}
}

func TestConvertPatchPayloadToSimEquipmentPointerClones(t *testing.T) {
	slots := []EquippedItem{{
		Slot: EquipSlotMainHand,
		Item: ItemStack{Type: ItemType("sword"), FungibilityKey: "unique", Quantity: 1},
	}}

	payload := itemspkg.EquipmentPayloadFromSlots[EquippedItem, EquipmentPayload](slots)
	converted, ok := convertPatchPayloadToSim(&payload).(sim.EquipmentPayload)
	if !ok {
		t.Fatalf("expected sim.EquipmentPayload, got %T", convertPatchPayloadToSim(payload))
	}
	if len(converted.Slots) != 1 {
		t.Fatalf("expected 1 equipment slot, got %d", len(converted.Slots))
	}
	if converted.Slots[0].Item.FungibilityKey != "unique" {
		t.Fatalf("expected fungibility key unique, got %q", converted.Slots[0].Item.FungibilityKey)
	}

	slots[0].Item.FungibilityKey = "mutated"
	if converted.Slots[0].Item.FungibilityKey != "unique" {
		t.Fatalf("expected converted payload to remain unchanged, got %q", converted.Slots[0].Item.FungibilityKey)
	}
}

func TestLegacyPatchesFromSimEquipmentUsesItemsAssembler(t *testing.T) {
	simSlots := []sim.EquippedItem{{
		Slot: sim.EquipSlotMainHand,
		Item: sim.ItemStack{Type: sim.ItemType("sword"), FungibilityKey: "unique", Quantity: 1},
	}}

	patches := []sim.Patch{
		{
			Kind:     sim.PatchPlayerEquipment,
			EntityID: "player-1",
			Payload:  sim.EquipmentPayload{Slots: itemspkg.CloneEquippedItems(simSlots)},
		},
		{
			Kind:     sim.PatchNPCEquipment,
			EntityID: "npc-1",
			Payload:  sim.EquipmentPayload{Slots: itemspkg.CloneEquippedItems(simSlots)},
		},
	}

	converted := legacyPatchesFromSim(patches)
	if len(converted) != len(patches) {
		t.Fatalf("expected %d patches, got %d", len(patches), len(converted))
	}

	expectedSlots := []EquippedItem{{
		Slot: EquipSlotMainHand,
		Item: ItemStack{Type: ItemType("sword"), FungibilityKey: "unique", Quantity: 1},
	}}
	expectedPayload := itemspkg.EquipmentPayloadFromSlots[EquippedItem, EquipmentPayload](append([]EquippedItem(nil), expectedSlots...))

	for i, patch := range converted {
		if patch.Kind != legacyPatchKindFromSim(patches[i].Kind) {
			t.Fatalf("expected patch kind %v, got %v", legacyPatchKindFromSim(patches[i].Kind), patch.Kind)
		}

		payload, ok := patch.Payload.(EquipmentPayload)
		if !ok {
			t.Fatalf("expected EquipmentPayload, got %T", patch.Payload)
		}

		slots, ok := payload.Slots.([]EquippedItem)
		if !ok {
			t.Fatalf("expected equipment slots to use []EquippedItem, got %T", payload.Slots)
		}

		if !reflect.DeepEqual(expectedSlots, slots) {
			t.Fatalf("expected slots %#v, got %#v", expectedSlots, slots)
		}

		if !reflect.DeepEqual(expectedPayload, payload) {
			t.Fatalf("expected payload %#v, got %#v", expectedPayload, payload)
		}
	}

	simSlots[0].Item.Quantity = 99
	convertedSlots, ok := converted[0].Payload.(EquipmentPayload).Slots.([]EquippedItem)
	if !ok {
		t.Fatalf("expected EquipmentPayload slots to assert, got %T", converted[0].Payload)
	}
	if convertedSlots[0].Item.Quantity != 1 {
		t.Fatalf("expected converted payload to retain quantity 1, got %d", convertedSlots[0].Item.Quantity)
	}
}

func TestLegacyActorFromSimEquipmentUsesItemsAssembler(t *testing.T) {
	simActor := sim.Actor{
		ID: "npc-1",
		Equipment: sim.Equipment{Slots: []sim.EquippedItem{{
			Slot: sim.EquipSlotHead,
			Item: sim.ItemStack{Type: sim.ItemType("helm"), FungibilityKey: "rare-helm", Quantity: 1},
		}}},
	}

	converted := legacyActorFromSim(simActor)

	expectedSlots := []EquippedItem{{
		Slot: EquipSlotHead,
		Item: ItemStack{Type: ItemType("helm"), FungibilityKey: "rare-helm", Quantity: 1},
	}}
	expectedEquipment := itemspkg.EquipmentValueFromSlots[EquippedItem, Equipment](append([]EquippedItem(nil), expectedSlots...))

	if !reflect.DeepEqual(expectedEquipment, converted.Equipment) {
		t.Fatalf("expected equipment %#v, got %#v", expectedEquipment, converted.Equipment)
	}

	converted.Equipment.Slots[0].Item.Quantity = 7
	if simActor.Equipment.Slots[0].Item.Quantity != 1 {
		t.Fatalf("expected legacy conversion to deep copy slots, got quantity %d", simActor.Equipment.Slots[0].Item.Quantity)
	}
}

func TestSimActorFromLegacyEquipmentUsesItemsAssembler(t *testing.T) {
	legacyActor := Actor{
		ID: "player-1",
		Equipment: itemspkg.EquipmentValueFromSlots[EquippedItem, Equipment]([]EquippedItem{{
			Slot: EquipSlotOffHand,
			Item: ItemStack{Type: ItemType("shield"), FungibilityKey: "tower-shield", Quantity: 1},
		}}),
	}

	converted := simActorFromLegacy(legacyActor)

	expectedSlots := []sim.EquippedItem{{
		Slot: sim.EquipSlotOffHand,
		Item: sim.ItemStack{Type: sim.ItemType("shield"), FungibilityKey: "tower-shield", Quantity: 1},
	}}
	expectedEquipment := itemspkg.EquipmentValueFromSlots[sim.EquippedItem, sim.Equipment](append([]sim.EquippedItem(nil), expectedSlots...))

	if !reflect.DeepEqual(expectedEquipment, converted.Equipment) {
		t.Fatalf("expected equipment %#v, got %#v", expectedEquipment, converted.Equipment)
	}

	converted.Equipment.Slots[0].Item.Quantity = 6
	if legacyActor.Equipment.Slots[0].Item.Quantity != 1 {
		t.Fatalf("expected sim conversion to deep copy slots, got quantity %d", legacyActor.Equipment.Slots[0].Item.Quantity)
	}
}

func TestLegacyKeyframeFromSimEquipmentUsesItemsAssembler(t *testing.T) {
	simPlayerSlots := []sim.EquippedItem{{
		Slot: sim.EquipSlotMainHand,
		Item: sim.ItemStack{Type: sim.ItemType("sword"), FungibilityKey: "unique-sword", Quantity: 1},
	}, {
		Slot: sim.EquipSlotHead,
		Item: sim.ItemStack{Type: sim.ItemType("helm"), FungibilityKey: "iron-helm", Quantity: 1},
	}}
	simNPCSlots := []sim.EquippedItem{{
		Slot: sim.EquipSlotMainHand,
		Item: sim.ItemStack{Type: sim.ItemType("axe"), FungibilityKey: "goblin-axe", Quantity: 1},
	}}

	frame := sim.Keyframe{
		Tick:     88,
		Sequence: 12,
		Players: []sim.Player{{
			Actor: sim.Actor{
				ID:        "player-1",
				Equipment: sim.Equipment{Slots: itemspkg.CloneEquippedItems(simPlayerSlots)},
			},
		}},
		NPCs: []sim.NPC{{
			Actor: sim.Actor{
				ID:        "npc-1",
				Equipment: sim.Equipment{Slots: itemspkg.CloneEquippedItems(simNPCSlots)},
			},
		}},
	}

	legacy := legacyKeyframeFromSim(frame)

	players, ok := legacy.Players.([]Player)
	if !ok {
		t.Fatalf("expected legacy players to assert to []Player, got %T", legacy.Players)
	}
	if len(players) != 1 {
		t.Fatalf("expected 1 legacy player, got %d", len(players))
	}

	npcs, ok := legacy.NPCs.([]NPC)
	if !ok {
		t.Fatalf("expected legacy NPCs to assert to []NPC, got %T", legacy.NPCs)
	}
	if len(npcs) != 1 {
		t.Fatalf("expected 1 legacy NPC, got %d", len(npcs))
	}

	expectedPlayerSlots := []EquippedItem{{
		Slot: EquipSlotMainHand,
		Item: ItemStack{Type: ItemType("sword"), FungibilityKey: "unique-sword", Quantity: 1},
	}, {
		Slot: EquipSlotHead,
		Item: ItemStack{Type: ItemType("helm"), FungibilityKey: "iron-helm", Quantity: 1},
	}}
	expectedNPCSlots := []EquippedItem{{
		Slot: EquipSlotMainHand,
		Item: ItemStack{Type: ItemType("axe"), FungibilityKey: "goblin-axe", Quantity: 1},
	}}

	expectedPlayerEquipment := itemspkg.EquipmentValueFromSlots[EquippedItem, Equipment](append([]EquippedItem(nil), expectedPlayerSlots...))
	expectedNPCEquipment := itemspkg.EquipmentValueFromSlots[EquippedItem, Equipment](append([]EquippedItem(nil), expectedNPCSlots...))

	if !reflect.DeepEqual(expectedPlayerEquipment, players[0].Equipment) {
		t.Fatalf("expected player equipment %#v, got %#v", expectedPlayerEquipment, players[0].Equipment)
	}
	if !reflect.DeepEqual(expectedNPCEquipment, npcs[0].Equipment) {
		t.Fatalf("expected npc equipment %#v, got %#v", expectedNPCEquipment, npcs[0].Equipment)
	}

	frame.Players[0].Equipment.Slots[0].Item.Quantity = 99
	if players[0].Equipment.Slots[0].Item.Quantity != 1 {
		t.Fatalf("expected legacy player equipment to deep copy slots, got %d", players[0].Equipment.Slots[0].Item.Quantity)
	}

	players[0].Equipment.Slots[0].Item.Quantity = 7
	if frame.Players[0].Equipment.Slots[0].Item.Quantity != 99 {
		t.Fatalf("expected legacy conversion to avoid mutating sim frame, got %d", frame.Players[0].Equipment.Slots[0].Item.Quantity)
	}

	frame.NPCs[0].Actor.Equipment.Slots[0].Item.Quantity = 42
	if npcs[0].Equipment.Slots[0].Item.Quantity != 1 {
		t.Fatalf("expected legacy npc equipment to deep copy slots, got %d", npcs[0].Equipment.Slots[0].Item.Quantity)
	}

	npcs[0].Equipment.Slots[0].Item.Quantity = 5
	if frame.NPCs[0].Actor.Equipment.Slots[0].Item.Quantity != 42 {
		t.Fatalf("expected npc conversion to avoid mutating sim frame, got %d", frame.NPCs[0].Actor.Equipment.Slots[0].Item.Quantity)
	}
}

func TestLegacyKeyframeFromSimInventoryUsesItemsAssembler(t *testing.T) {
	simPlayerSlots := []sim.InventorySlot{{
		Slot: 0,
		Item: sim.ItemStack{Type: sim.ItemType("potion"), FungibilityKey: "healing", Quantity: 3},
	}, {
		Slot: 1,
		Item: sim.ItemStack{Type: sim.ItemType("arrow"), FungibilityKey: "iron-arrow", Quantity: 15},
	}}
	simNPCSlots := []sim.InventorySlot{{
		Slot: 0,
		Item: sim.ItemStack{Type: sim.ItemType("coin"), FungibilityKey: "gold", Quantity: 25},
	}}

	frame := sim.Keyframe{
		Players: []sim.Player{{
			Actor: sim.Actor{
				ID:        "player-2",
				Inventory: sim.Inventory{Slots: itemspkg.CloneInventorySlots(simPlayerSlots)},
			},
		}},
		NPCs: []sim.NPC{{
			Actor: sim.Actor{
				ID:        "npc-2",
				Inventory: sim.Inventory{Slots: itemspkg.CloneInventorySlots(simNPCSlots)},
			},
		}},
	}

	legacy := legacyKeyframeFromSim(frame)

	players, ok := legacy.Players.([]Player)
	if !ok {
		t.Fatalf("expected legacy players to assert to []Player, got %T", legacy.Players)
	}
	if len(players) != 1 {
		t.Fatalf("expected 1 legacy player, got %d", len(players))
	}

	npcs, ok := legacy.NPCs.([]NPC)
	if !ok {
		t.Fatalf("expected legacy NPCs to assert to []NPC, got %T", legacy.NPCs)
	}
	if len(npcs) != 1 {
		t.Fatalf("expected 1 legacy NPC, got %d", len(npcs))
	}

	expectedPlayerSlots := []InventorySlot{{
		Slot: 0,
		Item: ItemStack{Type: ItemType("potion"), FungibilityKey: "healing", Quantity: 3},
	}, {
		Slot: 1,
		Item: ItemStack{Type: ItemType("arrow"), FungibilityKey: "iron-arrow", Quantity: 15},
	}}
	expectedNPCSlots := []InventorySlot{{
		Slot: 0,
		Item: ItemStack{Type: ItemType("coin"), FungibilityKey: "gold", Quantity: 25},
	}}

	expectedPlayerInventory := itemspkg.InventoryValueFromSlots[InventorySlot, Inventory](append([]InventorySlot(nil), expectedPlayerSlots...))
	expectedNPCInventory := itemspkg.InventoryValueFromSlots[InventorySlot, Inventory](append([]InventorySlot(nil), expectedNPCSlots...))

	if !reflect.DeepEqual(expectedPlayerInventory, players[0].Inventory) {
		t.Fatalf("expected player inventory %#v, got %#v", expectedPlayerInventory, players[0].Inventory)
	}
	if !reflect.DeepEqual(expectedNPCInventory, npcs[0].Inventory) {
		t.Fatalf("expected npc inventory %#v, got %#v", expectedNPCInventory, npcs[0].Inventory)
	}

	frame.Players[0].Inventory.Slots[0].Item.Quantity = 99
	if players[0].Inventory.Slots[0].Item.Quantity != 3 {
		t.Fatalf("expected legacy player inventory to deep copy slots, got %d", players[0].Inventory.Slots[0].Item.Quantity)
	}

	players[0].Inventory.Slots[0].Item.Quantity = 7
	if frame.Players[0].Inventory.Slots[0].Item.Quantity != 99 {
		t.Fatalf("expected legacy conversion to avoid mutating sim inventory, got %d", frame.Players[0].Inventory.Slots[0].Item.Quantity)
	}

	frame.NPCs[0].Actor.Inventory.Slots[0].Item.Quantity = 50
	if npcs[0].Inventory.Slots[0].Item.Quantity != 25 {
		t.Fatalf("expected legacy npc inventory to deep copy slots, got %d", npcs[0].Inventory.Slots[0].Item.Quantity)
	}

	npcs[0].Inventory.Slots[0].Item.Quantity = 13
	if frame.NPCs[0].Actor.Inventory.Slots[0].Item.Quantity != 50 {
		t.Fatalf("expected npc conversion to avoid mutating sim inventory, got %d", frame.NPCs[0].Actor.Inventory.Slots[0].Item.Quantity)
	}
}

func TestSimKeyframeFromLegacyEquipmentUsesItemsAssembler(t *testing.T) {
	legacyPlayerSlots := []EquippedItem{{
		Slot: EquipSlotMainHand,
		Item: ItemStack{Type: ItemType("staff"), FungibilityKey: "mage-staff", Quantity: 1},
	}}
	legacyNPCSlots := []EquippedItem{{
		Slot: EquipSlotBody,
		Item: ItemStack{Type: ItemType("robe"), FungibilityKey: "cloth-robe", Quantity: 1},
	}}

	frame := keyframe{
		Players: []Player{{
			Actor: Actor{
				ID:        "player-3",
				Equipment: itemspkg.EquipmentValueFromSlots[EquippedItem, Equipment](append([]EquippedItem(nil), legacyPlayerSlots...)),
			},
		}},
		NPCs: []NPC{{
			Actor: Actor{
				ID:        "npc-3",
				Equipment: itemspkg.EquipmentValueFromSlots[EquippedItem, Equipment](append([]EquippedItem(nil), legacyNPCSlots...)),
			},
		}},
	}

	simFrame := simKeyframeFromLegacy(frame)

	if len(simFrame.Players) != 1 {
		t.Fatalf("expected 1 sim player, got %d", len(simFrame.Players))
	}
	if len(simFrame.NPCs) != 1 {
		t.Fatalf("expected 1 sim npc, got %d", len(simFrame.NPCs))
	}

	expectedPlayerSlots := []sim.EquippedItem{{
		Slot: sim.EquipSlotMainHand,
		Item: sim.ItemStack{Type: sim.ItemType("staff"), FungibilityKey: "mage-staff", Quantity: 1},
	}}
	expectedNPCSlots := []sim.EquippedItem{{
		Slot: sim.EquipSlotBody,
		Item: sim.ItemStack{Type: sim.ItemType("robe"), FungibilityKey: "cloth-robe", Quantity: 1},
	}}

	expectedPlayerEquipment := itemspkg.EquipmentValueFromSlots[sim.EquippedItem, sim.Equipment](append([]sim.EquippedItem(nil), expectedPlayerSlots...))
	expectedNPCEquipment := itemspkg.EquipmentValueFromSlots[sim.EquippedItem, sim.Equipment](append([]sim.EquippedItem(nil), expectedNPCSlots...))

	if !reflect.DeepEqual(expectedPlayerEquipment, simFrame.Players[0].Equipment) {
		t.Fatalf("expected sim player equipment %#v, got %#v", expectedPlayerEquipment, simFrame.Players[0].Equipment)
	}
	if !reflect.DeepEqual(expectedNPCEquipment, simFrame.NPCs[0].Equipment) {
		t.Fatalf("expected sim npc equipment %#v, got %#v", expectedNPCEquipment, simFrame.NPCs[0].Equipment)
	}

	legacyPlayers, ok := frame.Players.([]Player)
	if !ok {
		t.Fatalf("expected legacy players to assert, got %T", frame.Players)
	}
	legacyPlayers[0].Equipment.Slots[0].Item.Quantity = 5
	if simFrame.Players[0].Equipment.Slots[0].Item.Quantity != 1 {
		t.Fatalf("expected sim player equipment to deep copy slots, got %d", simFrame.Players[0].Equipment.Slots[0].Item.Quantity)
	}

	simFrame.Players[0].Equipment.Slots[0].Item.Quantity = 9
	if legacyPlayers[0].Equipment.Slots[0].Item.Quantity != 5 {
		t.Fatalf("expected legacy player equipment to remain unchanged, got %d", legacyPlayers[0].Equipment.Slots[0].Item.Quantity)
	}

	legacyNPCs, ok := frame.NPCs.([]NPC)
	if !ok {
		t.Fatalf("expected legacy NPCs to assert, got %T", frame.NPCs)
	}
	legacyNPCs[0].Equipment.Slots[0].Item.Quantity = 7
	if simFrame.NPCs[0].Equipment.Slots[0].Item.Quantity != 1 {
		t.Fatalf("expected sim npc equipment to deep copy slots, got %d", simFrame.NPCs[0].Equipment.Slots[0].Item.Quantity)
	}

	simFrame.NPCs[0].Equipment.Slots[0].Item.Quantity = 11
	if legacyNPCs[0].Equipment.Slots[0].Item.Quantity != 7 {
		t.Fatalf("expected legacy npc equipment to remain unchanged, got %d", legacyNPCs[0].Equipment.Slots[0].Item.Quantity)
	}
}

func TestSimKeyframeFromLegacyInventoryUsesItemsAssembler(t *testing.T) {
	legacyPlayerSlots := []InventorySlot{{
		Slot: 0,
		Item: ItemStack{Type: ItemType("gem"), FungibilityKey: "ruby", Quantity: 2},
	}}
	legacyNPCSlots := []InventorySlot{{
		Slot: 0,
		Item: ItemStack{Type: ItemType("scroll"), FungibilityKey: "fireball", Quantity: 1},
	}}

	frame := keyframe{
		Players: []Player{{
			Actor: Actor{
				ID:        "player-4",
				Inventory: itemspkg.InventoryValueFromSlots[InventorySlot, Inventory](append([]InventorySlot(nil), legacyPlayerSlots...)),
			},
		}},
		NPCs: []NPC{{
			Actor: Actor{
				ID:        "npc-4",
				Inventory: itemspkg.InventoryValueFromSlots[InventorySlot, Inventory](append([]InventorySlot(nil), legacyNPCSlots...)),
			},
		}},
	}

	simFrame := simKeyframeFromLegacy(frame)

	if len(simFrame.Players) != 1 {
		t.Fatalf("expected 1 sim player, got %d", len(simFrame.Players))
	}
	if len(simFrame.NPCs) != 1 {
		t.Fatalf("expected 1 sim npc, got %d", len(simFrame.NPCs))
	}

	expectedPlayerSlots := []sim.InventorySlot{{
		Slot: 0,
		Item: sim.ItemStack{Type: sim.ItemType("gem"), FungibilityKey: "ruby", Quantity: 2},
	}}
	expectedNPCSlots := []sim.InventorySlot{{
		Slot: 0,
		Item: sim.ItemStack{Type: sim.ItemType("scroll"), FungibilityKey: "fireball", Quantity: 1},
	}}

	expectedPlayerInventory := itemspkg.InventoryValueFromSlots[sim.InventorySlot, sim.Inventory](append([]sim.InventorySlot(nil), expectedPlayerSlots...))
	expectedNPCInventory := itemspkg.InventoryValueFromSlots[sim.InventorySlot, sim.Inventory](append([]sim.InventorySlot(nil), expectedNPCSlots...))

	if !reflect.DeepEqual(expectedPlayerInventory, simFrame.Players[0].Inventory) {
		t.Fatalf("expected sim player inventory %#v, got %#v", expectedPlayerInventory, simFrame.Players[0].Inventory)
	}
	if !reflect.DeepEqual(expectedNPCInventory, simFrame.NPCs[0].Inventory) {
		t.Fatalf("expected sim npc inventory %#v, got %#v", expectedNPCInventory, simFrame.NPCs[0].Inventory)
	}

	legacyPlayers, ok := frame.Players.([]Player)
	if !ok {
		t.Fatalf("expected legacy players to assert, got %T", frame.Players)
	}
	legacyPlayers[0].Inventory.Slots[0].Item.Quantity = 5
	if simFrame.Players[0].Inventory.Slots[0].Item.Quantity != 2 {
		t.Fatalf("expected sim player inventory to deep copy slots, got %d", simFrame.Players[0].Inventory.Slots[0].Item.Quantity)
	}

	simFrame.Players[0].Inventory.Slots[0].Item.Quantity = 7
	if legacyPlayers[0].Inventory.Slots[0].Item.Quantity != 5 {
		t.Fatalf("expected legacy player inventory to remain unchanged, got %d", legacyPlayers[0].Inventory.Slots[0].Item.Quantity)
	}

	legacyNPCs, ok := frame.NPCs.([]NPC)
	if !ok {
		t.Fatalf("expected legacy NPCs to assert, got %T", frame.NPCs)
	}
	legacyNPCs[0].Inventory.Slots[0].Item.Quantity = 3
	if simFrame.NPCs[0].Inventory.Slots[0].Item.Quantity != 1 {
		t.Fatalf("expected sim npc inventory to deep copy slots, got %d", simFrame.NPCs[0].Inventory.Slots[0].Item.Quantity)
	}

	simFrame.NPCs[0].Inventory.Slots[0].Item.Quantity = 9
	if legacyNPCs[0].Inventory.Slots[0].Item.Quantity != 3 {
		t.Fatalf("expected legacy npc inventory to remain unchanged, got %d", legacyNPCs[0].Inventory.Slots[0].Item.Quantity)
	}
}

func TestSimKeyframeConversionRoundTripPreservesSequencing(t *testing.T) {
	recorded := time.Unix(1700000000, 0).UTC()
	legacy := keyframe{
		Tick:     123,
		Sequence: 456,
		Players: []Player{{
			Actor: Actor{ID: "player-1", X: 1, Y: 2, Facing: FacingUp},
		}},
		NPCs: []NPC{{
			Actor: Actor{ID: "npc-1", X: 5, Y: -3, Facing: FacingLeft},
			Type:  NPCTypeGoblin,
		}},
		Obstacles: []Obstacle{{
			ID: "obstacle-1", Type: "rock", X: 10, Y: 20, Width: 3, Height: 4,
		}},
		GroundItems: []itemspkg.GroundItem{{
			ID:             "ground-1",
			Type:           "potion",
			FungibilityKey: "potion-small",
			X:              6,
			Y:              7,
			Qty:            2,
		}},
		Config: worldConfig{
			Obstacles:      true,
			ObstaclesCount: 5,
			GoldMines:      true,
			GoldMineCount:  2,
			NPCs:           true,
			GoblinCount:    3,
			RatCount:       4,
			NPCCount:       7,
			Lava:           true,
			LavaCount:      1,
			Seed:           "deterministic-seed",
			Width:          128,
			Height:         256,
		},
		RecordedAt: recorded,
	}

	simFrame := simKeyframeFromLegacy(legacy)
	if simFrame.Tick != legacy.Tick {
		t.Fatalf("expected tick %d, got %d", legacy.Tick, simFrame.Tick)
	}
	if simFrame.Sequence != legacy.Sequence {
		t.Fatalf("expected sequence %d, got %d", legacy.Sequence, simFrame.Sequence)
	}
	legacyConfig, ok := legacy.Config.(worldConfig)
	if !ok {
		t.Fatalf("expected legacy config to be worldConfig, got %T", legacy.Config)
	}
	if simFrame.Config.Seed != legacyConfig.Seed {
		t.Fatalf("expected seed %q, got %q", legacyConfig.Seed, simFrame.Config.Seed)
	}

	simFrame.Config.Seed = "mutated-seed"
	legacyConfig, ok = legacy.Config.(worldConfig)
	if !ok {
		t.Fatalf("expected legacy config to be worldConfig, got %T", legacy.Config)
	}
	if legacyConfig.Seed != "deterministic-seed" {
		t.Fatalf("legacy seed mutated unexpectedly: %q", legacyConfig.Seed)
	}

	roundTrip := legacyKeyframeFromSim(simKeyframeFromLegacy(legacy))
	if !reflect.DeepEqual(legacy, roundTrip) {
		t.Fatalf("keyframe round trip mismatch\nlegacy: %#v\nround-trip: %#v", legacy, roundTrip)
	}
}

func TestSimKeyframeFromLegacyClonesGroundItems(t *testing.T) {
	legacyGround := []itemspkg.GroundItem{{
		ID:             "ground-1",
		Type:           "potion",
		FungibilityKey: "potion-small",
		X:              4.5,
		Y:              -2.25,
		Qty:            3,
	}, {
		ID:             "ground-2",
		Type:           "gold",
		FungibilityKey: "gold",
		X:              -7.5,
		Y:              8.75,
		Qty:            120,
	}}
	expected := itemspkg.CloneGroundItems(legacyGround)

	frame := keyframe{
		Tick:        99,
		Sequence:    1001,
		GroundItems: legacyGround,
	}

	converted := simKeyframeFromLegacy(frame)

	if !reflect.DeepEqual(expected, converted.GroundItems) {
		t.Fatalf("expected ground items %#v, got %#v", expected, converted.GroundItems)
	}

	legacyGround[0].Qty = 9
	if converted.GroundItems[0].Qty != expected[0].Qty {
		t.Fatalf("expected converted ground item qty %d after legacy mutation, got %d", expected[0].Qty, converted.GroundItems[0].Qty)
	}

	converted.GroundItems[1].Qty = 64
	if legacyGround[1].Qty != expected[1].Qty {
		t.Fatalf("expected legacy ground item qty %d after sim mutation, got %d", expected[1].Qty, legacyGround[1].Qty)
	}

	if &converted.GroundItems[0] == &expected[0] {
		t.Fatalf("expected converted ground items to clone values, but addresses match")
	}
}

func TestLegacyKeyframeFromSimClonesGroundItems(t *testing.T) {
	simGround := []itemspkg.GroundItem{{
		ID:             "ground-3",
		Type:           "scroll",
		FungibilityKey: "scroll-fire",
		X:              12.5,
		Y:              0.25,
		Qty:            1,
	}, {
		ID:             "ground-4",
		Type:           "potion",
		FungibilityKey: "potion-large",
		X:              -3.5,
		Y:              -9.5,
		Qty:            2,
	}}
	expected := itemspkg.CloneGroundItems(simGround)

	frame := sim.Keyframe{
		Tick:        200,
		Sequence:    211,
		GroundItems: simGround,
	}

	converted := legacyKeyframeFromSim(frame)

	typed, ok := converted.GroundItems.([]itemspkg.GroundItem)
	if !ok {
		t.Fatalf("expected legacy ground items to be []itemspkg.GroundItem, got %T", converted.GroundItems)
	}

	if !reflect.DeepEqual(expected, typed) {
		t.Fatalf("expected legacy ground items %#v, got %#v", expected, typed)
	}

	frame.GroundItems[0].Qty = 17
	if typed[0].Qty != expected[0].Qty {
		t.Fatalf("expected legacy ground item qty %d after sim mutation, got %d", expected[0].Qty, typed[0].Qty)
	}

	typed[1].Qty = 42
	if frame.GroundItems[1].Qty != expected[1].Qty {
		t.Fatalf("expected sim ground item qty %d after legacy mutation, got %d", expected[1].Qty, frame.GroundItems[1].Qty)
	}

	if &typed[0] == &expected[0] {
		t.Fatalf("expected legacy keyframe conversion to clone ground items, but addresses match")
	}
}

func TestSimKeyframeRecordResultRoundTripPreservesSequences(t *testing.T) {
	legacy := keyframeRecordResult{
		Size:           3,
		OldestSequence: 100,
		NewestSequence: 102,
		Evicted: []journalEviction{{
			Sequence: 90,
			Tick:     450,
			Reason:   "expired",
		}},
	}

	simResult := simKeyframeRecordResultFromLegacy(legacy)
	if simResult.OldestSequence != legacy.OldestSequence || simResult.NewestSequence != legacy.NewestSequence {
		t.Fatalf("expected sequences %d-%d, got %d-%d", legacy.OldestSequence, legacy.NewestSequence, simResult.OldestSequence, simResult.NewestSequence)
	}

	if len(simResult.Evicted) != len(legacy.Evicted) {
		t.Fatalf("expected %d evictions, got %d", len(legacy.Evicted), len(simResult.Evicted))
	}

	simResult.Evicted[0].Reason = "mutated"
	if legacy.Evicted[0].Reason != "expired" {
		t.Fatalf("legacy eviction mutated unexpectedly: %q", legacy.Evicted[0].Reason)
	}

	roundTrip := legacyKeyframeRecordResultFromSim(simKeyframeRecordResultFromLegacy(legacy))
	if !reflect.DeepEqual(legacy, roundTrip) {
		t.Fatalf("keyframe record result round trip mismatch\nlegacy: %#v\nround-trip: %#v", legacy, roundTrip)
	}
}

func TestSimEffectEventBatchConversionRoundTripPreservesSequences(t *testing.T) {
	legacy := EffectEventBatch{
		Spawns: []effectcontract.EffectSpawnEvent{{
			Instance: effectcontract.EffectInstance{ID: "effect-1", DefinitionID: "fireball"},
			Tick:     12,
			Seq:      1,
		}},
		Updates: []effectcontract.EffectUpdateEvent{{
			ID:   "effect-1",
			Seq:  2,
			Tick: effectcontract.Tick(13),
		}},
		Ends: []effectcontract.EffectEndEvent{{
			ID:   "effect-1",
			Seq:  3,
			Tick: effectcontract.Tick(14),
		}},
		LastSeqByID: map[string]effectcontract.Seq{
			"effect-1": 3,
		},
	}

	simBatch := internaleffects.SimEffectEventBatchFromLegacy(journal.EffectEventBatch(legacy))
	if !reflect.DeepEqual(legacy.LastSeqByID, simBatch.LastSeqByID) {
		t.Fatalf("expected seq map %#v, got %#v", legacy.LastSeqByID, simBatch.LastSeqByID)
	}

	simBatch.LastSeqByID["effect-1"] = 99
	if legacy.LastSeqByID["effect-1"] != 3 {
		t.Fatalf("legacy sequence mutated unexpectedly: %d", legacy.LastSeqByID["effect-1"])
	}

	roundTrip := internaleffects.LegacyEffectEventBatchFromSim(internaleffects.SimEffectEventBatchFromLegacy(journal.EffectEventBatch(legacy)))
	if !reflect.DeepEqual(legacy, roundTrip) {
		t.Fatalf("effect batch round trip mismatch\nlegacy: %#v\nround-trip: %#v", legacy, roundTrip)
	}
}

func TestLegacyAdapterRestorePatches(t *testing.T) {
	hub := newHub()

	adapter := hub.adapter
	if adapter == nil {
		t.Fatalf("expected hub adapter to be initialized")
	}

	baseline := Patch{Kind: PatchNPCPos, EntityID: "npc-1", Payload: PositionPayload{X: 3, Y: 4}}
	hub.world.journal.AppendPatch(baseline)

	simPatches := []sim.Patch{
		{
			Kind:     sim.PatchPlayerFacing,
			EntityID: "player-1",
			Payload:  sim.PlayerFacingPayload{Facing: sim.FacingLeft},
		},
		{
			Kind:     sim.PatchEffectParams,
			EntityID: "effect-1",
			Payload:  sim.EffectParamsPayload{Params: map[string]float64{"radius": 5}},
		},
	}

	expectedRestored := legacyPatchesFromSim(simPatches)

	adapter.RestorePatches(simPatches)

	effectPayload := simPatches[1].Payload.(sim.EffectParamsPayload)
	effectPayload.Params["radius"] = 9
	simPatches[1].Payload = effectPayload

	drained := hub.world.journal.DrainPatches()

	if len(drained) != len(expectedRestored)+1 {
		t.Fatalf("unexpected drained patch count: want %d got %d", len(expectedRestored)+1, len(drained))
	}

	for idx := range expectedRestored {
		if !reflect.DeepEqual(expectedRestored[idx], drained[idx]) {
			t.Fatalf("restored patch mismatch at %d\nwant: %#v\ngot: %#v", idx, expectedRestored[idx], drained[idx])
		}
	}

	if !reflect.DeepEqual(drained[len(expectedRestored)], baseline) {
		t.Fatalf("expected baseline patch to remain queued, got %#v", drained[len(expectedRestored)])
	}

	if qty := drained[1].Payload.(EffectParamsPayload).Params["radius"]; qty != 5 {
		t.Fatalf("expected journal restore to deep copy params, got %v", qty)
	}
}

func TestSimKeyframeConversionRoundTrip(t *testing.T) {
	recordedAt := time.Unix(1700000100, 42).UTC()

	legacyFrame := keyframe{
		Tick:     64,
		Sequence: 7,
		Players: []Player{{
			Actor: Actor{ID: "player-1", X: 10, Y: -5, Facing: FacingLeft, Health: 80, MaxHealth: 100},
		}},
		NPCs: []NPC{{
			Actor: Actor{ID: "npc-1", X: -2, Y: 4, Facing: FacingDown, Health: 40, MaxHealth: 50},
			Type:  NPCTypeGoblin,
		}},
		Obstacles: []Obstacle{{
			ID: "obstacle-1", Type: "rock", X: 1.5, Y: 2.5, Width: 3, Height: 4,
		}},
		GroundItems: []itemspkg.GroundItem{{
			ID: "ground-1", Type: "potion", FungibilityKey: "potion", X: 3, Y: 4, Qty: 2,
		}},
		Config: worldConfig{
			Obstacles:      true,
			ObstaclesCount: 5,
			GoldMines:      true,
			GoldMineCount:  2,
			NPCs:           true,
			GoblinCount:    3,
			RatCount:       1,
			NPCCount:       4,
			Lava:           true,
			LavaCount:      2,
			Seed:           "seed",
			Width:          128,
			Height:         256,
		},
		RecordedAt: recordedAt,
	}

	simFrame := simKeyframeFromLegacy(legacyFrame)
	roundTrip := legacyKeyframeFromSim(simFrame)

	if !reflect.DeepEqual(legacyFrame, roundTrip) {
		t.Fatalf("keyframe round trip mismatch\nlegacy: %#v\nround: %#v", legacyFrame, roundTrip)
	}

	legacyPlayers, ok := legacyFrame.Players.([]Player)
	if !ok {
		t.Fatalf("expected legacy players to be []Player, got %T", legacyFrame.Players)
	}
	simFrame.Players[0].Health = 10
	if legacyPlayers[0].Health != 80 {
		t.Fatalf("expected player conversion to deep copy actors, got %v", legacyPlayers[0].Health)
	}

	legacyObstacles, ok := legacyFrame.Obstacles.([]Obstacle)
	if !ok {
		t.Fatalf("expected legacy obstacles to be []Obstacle, got %T", legacyFrame.Obstacles)
	}
	simFrame.Obstacles[0].Width = 99
	if legacyObstacles[0].Width != 3 {
		t.Fatalf("expected obstacle conversion to deep copy values, got %v", legacyObstacles[0].Width)
	}
}

func TestSimKeyframeRecordResultConversionRoundTrip(t *testing.T) {
	legacyResult := keyframeRecordResult{
		Size:           4,
		OldestSequence: 10,
		NewestSequence: 13,
		Evicted: []journalEviction{{
			Sequence: 3,
			Tick:     120,
			Reason:   "expired",
		}, {
			Sequence: 4,
			Tick:     130,
			Reason:   "count",
		}},
	}

	simResult := simKeyframeRecordResultFromLegacy(legacyResult)
	roundTrip := legacyKeyframeRecordResultFromSim(simResult)

	if !reflect.DeepEqual(legacyResult, roundTrip) {
		t.Fatalf("keyframe record result round trip mismatch\nlegacy: %#v\nround: %#v", legacyResult, roundTrip)
	}

	if len(simResult.Evicted) == 0 {
		t.Fatalf("expected converted record result to include evictions")
	}

	simResult.Evicted[0].Reason = "mutated"
	if legacyResult.Evicted[0].Reason != "expired" {
		t.Fatalf("expected keyframe record result conversion to deep copy evictions, got %q", legacyResult.Evicted[0].Reason)
	}
}

func TestHubAdapterKeyframeRecordingMatchesJournal(t *testing.T) {
	t.Setenv(envJournalCapacity, "2")
	t.Setenv(envJournalMaxAgeMS, "5")

	capacity, maxAge := journalConfig()
	expected := newJournal(capacity, maxAge)

	hub := newHub()
	hub.world.journal = newJournal(capacity, maxAge)

	adapter := hub.adapter
	if adapter == nil {
		t.Fatalf("expected hub adapter to be initialized")
	}

	cases := []struct {
		sequence uint64
		tick     uint64
		wait     time.Duration
	}{
		{sequence: 1, tick: 100},
		{sequence: 2, tick: 110},
		{sequence: 3, tick: 120, wait: 15 * time.Millisecond},
		{sequence: 4, tick: 130},
	}

	for _, tc := range cases {
		if tc.wait > 0 {
			time.Sleep(tc.wait)
		}

		want := simKeyframeRecordResultFromLegacy(expected.RecordKeyframe(keyframe{Sequence: tc.sequence, Tick: tc.tick}))
		got := adapter.RecordKeyframe(sim.Keyframe{Sequence: tc.sequence, Tick: tc.tick})

		if got.Size != want.Size {
			t.Fatalf("unexpected journal size for seq %d: want %d got %d", tc.sequence, want.Size, got.Size)
		}
		if got.OldestSequence != want.OldestSequence {
			t.Fatalf("unexpected oldest sequence for seq %d: want %d got %d", tc.sequence, want.OldestSequence, got.OldestSequence)
		}
		if got.NewestSequence != want.NewestSequence {
			t.Fatalf("unexpected newest sequence for seq %d: want %d got %d", tc.sequence, want.NewestSequence, got.NewestSequence)
		}
		if len(got.Evicted) != len(want.Evicted) {
			t.Fatalf("unexpected eviction count for seq %d: want %d got %d", tc.sequence, len(want.Evicted), len(got.Evicted))
		}
		for idx := range want.Evicted {
			expectedEviction := want.Evicted[idx]
			actualEviction := got.Evicted[idx]
			if actualEviction.Sequence != expectedEviction.Sequence || actualEviction.Tick != expectedEviction.Tick || actualEviction.Reason != expectedEviction.Reason {
				t.Fatalf(
					"unexpected eviction at seq %d index %d: want {seq:%d tick:%d reason:%s} got {seq:%d tick:%d reason:%s}",
					tc.sequence,
					idx,
					expectedEviction.Sequence,
					expectedEviction.Tick,
					expectedEviction.Reason,
					actualEviction.Sequence,
					actualEviction.Tick,
					actualEviction.Reason,
				)
			}
		}

		size, oldest, newest := hub.world.journal.KeyframeWindow()
		if size != got.Size || oldest != got.OldestSequence || newest != got.NewestSequence {
			t.Fatalf("unexpected adapter window after seq %d: size=%d oldest=%d newest=%d want size=%d oldest=%d newest=%d", tc.sequence, size, oldest, newest, got.Size, got.OldestSequence, got.NewestSequence)
		}
	}
}

func TestLegacyAdapterRecordKeyframeClonesGroundItems(t *testing.T) {
	hub := newHub()
	adapter := hub.adapter
	if adapter == nil {
		t.Fatalf("expected hub adapter to be initialized")
	}

	groundItems := []itemspkg.GroundItem{{
		ID:             "ground-100",
		Type:           "potion",
		FungibilityKey: "potion-small",
		X:              1.5,
		Y:              -2.75,
		Qty:            3,
	}, {
		ID:             "ground-101",
		Type:           "gold",
		FungibilityKey: "gold",
		X:              -4.25,
		Y:              9.75,
		Qty:            120,
	}}
	expected := itemspkg.CloneGroundItems(groundItems)

	frame := sim.Keyframe{
		Sequence:    410,
		Tick:        905,
		GroundItems: groundItems,
	}

	adapter.RecordKeyframe(frame)

	frame.GroundItems[0].Qty = 99
	groundItems[1].Qty = 77

	recorded, ok := hub.world.journal.KeyframeBySequence(frame.Sequence)
	if !ok {
		t.Fatalf("expected journal to contain keyframe %d", frame.Sequence)
	}

	recordedGround, ok := recorded.GroundItems.([]itemspkg.GroundItem)
	if !ok {
		t.Fatalf("expected recorded ground items to be []itemspkg.GroundItem, got %T", recorded.GroundItems)
	}

	if !reflect.DeepEqual(expected, recordedGround) {
		t.Fatalf("unexpected recorded ground items: got %#v want %#v", recordedGround, expected)
	}

	if recordedGround[0].Qty != expected[0].Qty || recordedGround[1].Qty != expected[1].Qty {
		t.Fatalf("expected recorded ground item quantities %v, got %v", []int{expected[0].Qty, expected[1].Qty}, []int{recordedGround[0].Qty, recordedGround[1].Qty})
	}

	if &recordedGround[0] == &frame.GroundItems[0] {
		t.Fatalf("expected recorded ground items to be cloned from input frame")
	}

	if &recordedGround[0] == &groundItems[0] {
		t.Fatalf("expected recorded ground items to be cloned from original slice")
	}
}

func TestLegacyAdapterRecordKeyframeCopiesConfig(t *testing.T) {
	hub := newHub()
	adapter := hub.adapter
	if adapter == nil {
		t.Fatalf("expected hub adapter to be initialized")
	}

	config := sim.WorldConfig{
		Obstacles:      true,
		ObstaclesCount: 6,
		GoldMines:      true,
		GoldMineCount:  4,
		NPCs:           true,
		GoblinCount:    7,
		RatCount:       8,
		NPCCount:       15,
		Lava:           true,
		LavaCount:      9,
		Seed:           "crystal-caves",
		Width:          160,
		Height:         140,
	}
	expected := config

	frame := sim.Keyframe{
		Sequence: 512,
		Tick:     1331,
		Config:   config,
		Players:  []sim.Player{{Actor: sim.Actor{ID: "player-150"}}},
		GroundItems: []itemspkg.GroundItem{{
			ID:             "ground-150",
			Type:           "gem",
			FungibilityKey: "emerald",
			X:              2.5,
			Y:              -1.25,
			Qty:            5,
		}},
	}

	adapter.RecordKeyframe(frame)

	frame.Config.Seed = "mutated"
	config.GoblinCount = 99

	recorded, ok := hub.world.journal.KeyframeBySequence(frame.Sequence)
	if !ok {
		t.Fatalf("expected journal to contain keyframe %d", frame.Sequence)
	}

	recordedConfig, ok := recorded.Config.(worldConfig)
	if !ok {
		t.Fatalf("expected recorded config to be worldConfig, got %T", recorded.Config)
	}

	expectedLegacy := legacyWorldConfigFromSim(expected)
	if !reflect.DeepEqual(expectedLegacy, recordedConfig) {
		t.Fatalf("expected recorded config to remain unchanged, got %#v want %#v", recordedConfig, expectedLegacy)
	}
}

func TestLegacyAdapterKeyframeBySequenceClonesGroundItems(t *testing.T) {
	hub := newHub()
	adapter := hub.adapter
	if adapter == nil {
		t.Fatalf("expected hub adapter to be initialized")
	}

	groundItems := []itemspkg.GroundItem{{
		ID:             "ground-200",
		Type:           "scroll",
		FungibilityKey: "scroll-fire",
		X:              -7.5,
		Y:              4.5,
		Qty:            2,
	}, {
		ID:             "ground-201",
		Type:           "potion",
		FungibilityKey: "potion-large",
		X:              12.25,
		Y:              -6.75,
		Qty:            5,
	}}
	expected := itemspkg.CloneGroundItems(groundItems)

	frame := sim.Keyframe{
		Sequence:    512,
		Tick:        1337,
		GroundItems: groundItems,
	}

	adapter.RecordKeyframe(frame)

	groundItems[0].Qty = 41
	frame.GroundItems[1].Qty = 19

	fetched, ok := adapter.KeyframeBySequence(frame.Sequence)
	if !ok {
		t.Fatalf("expected adapter to return keyframe %d", frame.Sequence)
	}

	if !reflect.DeepEqual(expected, fetched.GroundItems) {
		t.Fatalf("unexpected fetched ground items: got %#v want %#v", fetched.GroundItems, expected)
	}

	fetched.GroundItems[0].Qty = 73

	again, ok := adapter.KeyframeBySequence(frame.Sequence)
	if !ok {
		t.Fatalf("expected adapter to return keyframe %d on second fetch", frame.Sequence)
	}

	if !reflect.DeepEqual(expected, again.GroundItems) {
		t.Fatalf("expected adapter to clone ground items, got %#v want %#v", again.GroundItems, expected)
	}

	if &again.GroundItems[0] == &fetched.GroundItems[0] {
		t.Fatalf("expected adapter keyframe lookup to clone ground item slices on each call")
	}

	recorded, ok := hub.world.journal.KeyframeBySequence(frame.Sequence)
	if !ok {
		t.Fatalf("expected journal to contain keyframe %d", frame.Sequence)
	}
	recordedGround, ok := recorded.GroundItems.([]itemspkg.GroundItem)
	if !ok {
		t.Fatalf("expected recorded ground items to be []itemspkg.GroundItem, got %T", recorded.GroundItems)
	}

	if !reflect.DeepEqual(expected, recordedGround) {
		t.Fatalf("expected journal to retain original ground items, got %#v want %#v", recordedGround, expected)
	}
}

func TestLegacyAdapterKeyframeBySequenceCopiesConfig(t *testing.T) {
	hub := newHub()
	adapter := hub.adapter
	if adapter == nil {
		t.Fatalf("expected hub adapter to be initialized")
	}

	config := sim.WorldConfig{
		Obstacles:      true,
		ObstaclesCount: 7,
		GoldMines:      true,
		GoldMineCount:  3,
		NPCs:           true,
		GoblinCount:    4,
		RatCount:       6,
		NPCCount:       10,
		Lava:           true,
		LavaCount:      8,
		Seed:           "volcano",
		Width:          192,
		Height:         108,
	}
	expected := config

	frame := sim.Keyframe{
		Sequence:  704,
		Tick:      2112,
		Config:    config,
		Players:   []sim.Player{{Actor: sim.Actor{ID: "player-200"}}},
		Obstacles: []sim.Obstacle{{ID: "obstacle-700"}},
	}

	adapter.RecordKeyframe(frame)

	config.GoldMineCount = 11
	frame.Config.Seed = "mutated"

	fetched, ok := adapter.KeyframeBySequence(frame.Sequence)
	if !ok {
		t.Fatalf("expected adapter to return keyframe %d", frame.Sequence)
	}

	if fetched.Config != expected {
		t.Fatalf("unexpected adapter keyframe config: got %#v want %#v", fetched.Config, expected)
	}

	fetched.Config.Height = 512
	fetched.Config.LavaCount = 2

	again, ok := adapter.KeyframeBySequence(frame.Sequence)
	if !ok {
		t.Fatalf("expected adapter to return keyframe %d on second fetch", frame.Sequence)
	}

	if again.Config != expected {
		t.Fatalf("expected adapter keyframe config to remain unchanged, got %#v want %#v", again.Config, expected)
	}

	recorded, ok := hub.world.journal.KeyframeBySequence(frame.Sequence)
	if !ok {
		t.Fatalf("expected journal to contain keyframe %d", frame.Sequence)
	}

	recordedConfig, ok := recorded.Config.(worldConfig)
	if !ok {
		t.Fatalf("expected recorded config to be worldConfig, got %T", recorded.Config)
	}

	expectedLegacy := legacyWorldConfigFromSim(expected)
	if !reflect.DeepEqual(expectedLegacy, recordedConfig) {
		t.Fatalf("expected journal config to remain unchanged, got %#v want %#v", recordedConfig, expectedLegacy)
	}
}

func TestHubKeyframeClonesGroundItems(t *testing.T) {
	hub := newHub()
	adapter := hub.adapter
	if adapter == nil {
		t.Fatalf("expected hub adapter to be initialized")
	}

	groundItems := []itemspkg.GroundItem{{
		ID:             "ground-300",
		Type:           "relic",
		FungibilityKey: "relic-ancient",
		X:              6.75,
		Y:              -1.5,
		Qty:            1,
	}, {
		ID:             "ground-301",
		Type:           "gold",
		FungibilityKey: "gold",
		X:              -11.25,
		Y:              3.5,
		Qty:            75,
	}}
	expected := itemspkg.CloneGroundItems(groundItems)

	frame := sim.Keyframe{
		Sequence:    640,
		Tick:        2048,
		GroundItems: groundItems,
	}

	adapter.RecordKeyframe(frame)

	groundItems[0].Qty = 5
	frame.GroundItems[1].Qty = 48

	snapshot, ok := hub.Keyframe(frame.Sequence)
	if !ok {
		t.Fatalf("expected hub to return keyframe %d", frame.Sequence)
	}

	if !reflect.DeepEqual(expected, snapshot.GroundItems) {
		t.Fatalf("unexpected hub keyframe ground items: got %#v want %#v", snapshot.GroundItems, expected)
	}

	snapshot.GroundItems[0].Qty = 29

	again, ok := hub.Keyframe(frame.Sequence)
	if !ok {
		t.Fatalf("expected hub to return keyframe %d on second lookup", frame.Sequence)
	}

	if !reflect.DeepEqual(expected, again.GroundItems) {
		t.Fatalf("expected hub keyframe lookup to clone ground items, got %#v want %#v", again.GroundItems, expected)
	}

	if &again.GroundItems[0] == &snapshot.GroundItems[0] {
		t.Fatalf("expected hub keyframe lookup to clone ground item slices on each call")
	}

	recorded, ok := hub.world.journal.KeyframeBySequence(frame.Sequence)
	if !ok {
		t.Fatalf("expected journal to contain keyframe %d", frame.Sequence)
	}
	recordedGround, ok := recorded.GroundItems.([]itemspkg.GroundItem)
	if !ok {
		t.Fatalf("expected recorded ground items to be []itemspkg.GroundItem, got %T", recorded.GroundItems)
	}

	if !reflect.DeepEqual(expected, recordedGround) {
		t.Fatalf("expected journal to retain original ground items, got %#v want %#v", recordedGround, expected)
	}
}

func TestHubKeyframeClonesActors(t *testing.T) {
	hub := newHub()
	adapter := hub.adapter
	if adapter == nil {
		t.Fatalf("expected hub adapter to be initialized")
	}

	playerInventory := []sim.InventorySlot{{
		Slot: 0,
		Item: sim.ItemStack{Type: sim.ItemType("potion"), FungibilityKey: "stamina", Quantity: 4},
	}, {
		Slot: 2,
		Item: sim.ItemStack{Type: sim.ItemType("scroll"), FungibilityKey: "teleport", Quantity: 1},
	}}
	playerEquipment := []sim.EquippedItem{{
		Slot: sim.EquipSlotMainHand,
		Item: sim.ItemStack{Type: sim.ItemType("axe"), FungibilityKey: "double", Quantity: 1},
	}, {
		Slot: sim.EquipSlotBoots,
		Item: sim.ItemStack{Type: sim.ItemType("boots"), FungibilityKey: "leather", Quantity: 1},
	}}

	npcInventory := []sim.InventorySlot{{
		Slot: 1,
		Item: sim.ItemStack{Type: sim.ItemType("gem"), FungibilityKey: "sapphire", Quantity: 2},
	}}
	npcEquipment := []sim.EquippedItem{{
		Slot: sim.EquipSlotHead,
		Item: sim.ItemStack{Type: sim.ItemType("mask"), FungibilityKey: "shadow", Quantity: 1},
	}}

	players := []sim.Player{{
		Actor: sim.Actor{
			ID:        "player-500",
			X:         -6.5,
			Y:         12.75,
			Facing:    sim.FacingUp,
			Health:    95,
			MaxHealth: 110,
			Inventory: sim.Inventory{Slots: itemspkg.CloneInventorySlots(playerInventory)},
			Equipment: sim.Equipment{Slots: itemspkg.CloneEquippedItems(playerEquipment)},
		},
	}}
	npcs := []sim.NPC{{
		Actor: sim.Actor{
			ID:        "npc-500",
			X:         4.0,
			Y:         -7.5,
			Facing:    sim.FacingDown,
			Health:    38,
			MaxHealth: 55,
			Inventory: sim.Inventory{Slots: itemspkg.CloneInventorySlots(npcInventory)},
			Equipment: sim.Equipment{Slots: itemspkg.CloneEquippedItems(npcEquipment)},
		},
		Type:             sim.NPCTypeRat,
		AIControlled:     false,
		ExperienceReward: 9,
	}}

	expectedPlayers := simutil.ClonePlayers(players)
	expectedNPCs := simutil.CloneNPCs(npcs)

	frame := sim.Keyframe{
		Sequence:  912,
		Tick:      2049,
		Players:   players,
		NPCs:      npcs,
		Obstacles: []sim.Obstacle{{ID: "obstacle-500", X: -8, Y: 3.25, Width: 1.5, Height: 1}},
	}

	adapter.RecordKeyframe(frame)

	players[0].Inventory.Slots[0].Item.Quantity = 22
	players[0].Equipment.Slots[0].Item.Quantity = 3
	frame.Players[0].Health = 10
	npcs[0].Inventory.Slots[0].Item.Quantity = 11
	frame.NPCs[0].Equipment.Slots[0].Item.Quantity = 5

	snapshot, ok := hub.Keyframe(frame.Sequence)
	if !ok {
		t.Fatalf("expected hub to return keyframe %d", frame.Sequence)
	}

	if len(snapshot.Players) != len(expectedPlayers) {
		t.Fatalf("expected %d players in keyframe snapshot, got %d", len(expectedPlayers), len(snapshot.Players))
	}
	if len(snapshot.NPCs) != len(expectedNPCs) {
		t.Fatalf("expected %d NPCs in keyframe snapshot, got %d", len(expectedNPCs), len(snapshot.NPCs))
	}

	if !reflect.DeepEqual(expectedPlayers, snapshot.Players) {
		t.Fatalf("unexpected hub keyframe players: got %#v want %#v", snapshot.Players, expectedPlayers)
	}
	if !reflect.DeepEqual(expectedNPCs, snapshot.NPCs) {
		t.Fatalf("unexpected hub keyframe NPCs: got %#v want %#v", snapshot.NPCs, expectedNPCs)
	}

	playerSlice := &snapshot.Players[0]
	npcSlice := &snapshot.NPCs[0]

	snapshot.Players[0].Inventory.Slots[0].Item.Quantity = 77
	snapshot.Players[0].Equipment.Slots[0].Item.Quantity = 6
	snapshot.NPCs[0].Inventory.Slots[0].Item.Quantity = 88
	snapshot.NPCs[0].Equipment.Slots[0].Item.Quantity = 7

	snapshot.Players[0].Health = 0
	snapshot.NPCs[0].Health = 0

	again, ok := hub.Keyframe(frame.Sequence)
	if !ok {
		t.Fatalf("expected hub to return keyframe %d on second lookup", frame.Sequence)
	}

	if !reflect.DeepEqual(expectedPlayers, again.Players) {
		t.Fatalf("expected cloned players on second hub keyframe lookup, got %#v want %#v", again.Players, expectedPlayers)
	}
	if !reflect.DeepEqual(expectedNPCs, again.NPCs) {
		t.Fatalf("expected cloned NPCs on second hub keyframe lookup, got %#v want %#v", again.NPCs, expectedNPCs)
	}

	if &again.Players[0] == playerSlice {
		t.Fatalf("expected hub keyframe lookup to clone player slices on each call")
	}
	if &again.NPCs[0] == npcSlice {
		t.Fatalf("expected hub keyframe lookup to clone NPC slices on each call")
	}

	recorded, ok := hub.world.journal.KeyframeBySequence(frame.Sequence)
	if !ok {
		t.Fatalf("expected journal to contain keyframe %d", frame.Sequence)
	}

	recordedPlayers, ok := recorded.Players.([]Player)
	if !ok {
		t.Fatalf("expected recorded players to be []Player, got %T", recorded.Players)
	}
	recordedNPCs, ok := recorded.NPCs.([]NPC)
	if !ok {
		t.Fatalf("expected recorded NPCs to be []NPC, got %T", recorded.NPCs)
	}

	expectedLegacyPlayers := legacyPlayersFromSim(expectedPlayers)
	expectedLegacyNPCs := legacyNPCsFromSim(expectedNPCs)

	if !reflect.DeepEqual(expectedLegacyPlayers, recordedPlayers) {
		t.Fatalf("expected journal players to remain unchanged, got %#v want %#v", recordedPlayers, expectedLegacyPlayers)
	}
	if !reflect.DeepEqual(expectedLegacyNPCs, recordedNPCs) {
		t.Fatalf("expected journal NPCs to remain unchanged, got %#v want %#v", recordedNPCs, expectedLegacyNPCs)
	}
}

func TestHubKeyframeClonesObstacles(t *testing.T) {
	hub := newHub()
	adapter := hub.adapter
	if adapter == nil {
		t.Fatalf("expected hub adapter to be initialized")
	}

	obstacles := []sim.Obstacle{{
		ID:     "obstacle-600",
		Type:   "pillar",
		X:      2.75,
		Y:      -1.5,
		Width:  1.25,
		Height: 4.5,
	}, {
		ID:     "obstacle-601",
		Type:   "spike",
		X:      -7.0,
		Y:      5.5,
		Width:  2.0,
		Height: 1.75,
	}}
	expected := simutil.CloneObstacles(obstacles)

	frame := sim.Keyframe{
		Sequence:  914,
		Tick:      4097,
		Obstacles: obstacles,
	}

	adapter.RecordKeyframe(frame)

	obstacles[0].Width = 3.75
	frame.Obstacles[1].Height = 6.25

	snapshot, ok := hub.Keyframe(frame.Sequence)
	if !ok {
		t.Fatalf("expected hub to return keyframe %d", frame.Sequence)
	}

	if !reflect.DeepEqual(expected, snapshot.Obstacles) {
		t.Fatalf("unexpected hub keyframe obstacles: got %#v want %#v", snapshot.Obstacles, expected)
	}

	first := &snapshot.Obstacles[0]
	snapshot.Obstacles[0].Width = 5.5
	snapshot.Obstacles[1].Height = 8.0

	again, ok := hub.Keyframe(frame.Sequence)
	if !ok {
		t.Fatalf("expected hub to return keyframe %d on second lookup", frame.Sequence)
	}

	if !reflect.DeepEqual(expected, again.Obstacles) {
		t.Fatalf("expected cloned obstacles on second hub keyframe lookup, got %#v want %#v", again.Obstacles, expected)
	}

	if &again.Obstacles[0] == first {
		t.Fatalf("expected hub keyframe lookup to clone obstacle slices on each call")
	}

	recorded, ok := hub.world.journal.KeyframeBySequence(frame.Sequence)
	if !ok {
		t.Fatalf("expected journal to contain keyframe %d", frame.Sequence)
	}

	recordedObstacles, ok := recorded.Obstacles.([]Obstacle)
	if !ok {
		t.Fatalf("expected recorded obstacles to be []Obstacle, got %T", recorded.Obstacles)
	}

	expectedLegacyObstacles := legacyObstaclesFromSim(expected)
	if !reflect.DeepEqual(expectedLegacyObstacles, recordedObstacles) {
		t.Fatalf("expected journal obstacles to remain unchanged, got %#v want %#v", recordedObstacles, expectedLegacyObstacles)
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

	simBatch := internaleffects.SimEffectEventBatchFromLegacy(journal.EffectEventBatch(legacyBatch))
	roundTrip := internaleffects.LegacyEffectEventBatchFromSim(simBatch)

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

	simSignal := internaleffects.SimEffectResyncSignalFromLegacy(journal.ResyncSignal(legacySignal))
	roundTrip := internaleffects.LegacyEffectResyncSignalFromSim(simSignal)

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
