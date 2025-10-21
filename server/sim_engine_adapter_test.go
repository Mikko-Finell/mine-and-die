package server

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
		GroundItems:    simGroundItemsFromLegacy(legacyGround),
		EffectEvents:   simEffectTriggersFromLegacy(legacyEffects),
		Obstacles:      simObstaclesFromLegacy(legacyObstacles),
		AliveEffectIDs: append([]string(nil), aliveEffects...),
	}

	roundPlayers := legacyPlayersFromSim(snapshot.Players)
	roundNPCs := legacyNPCsFromSim(snapshot.NPCs)
	roundGround := legacyGroundItemsFromSim(snapshot.GroundItems)
	roundEffects := legacyEffectTriggersFromSim(snapshot.EffectEvents)
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

func TestSimAliveEffectIDsFromLegacy(t *testing.T) {
	if ids := simAliveEffectIDsFromLegacy(nil); ids != nil {
		t.Fatalf("expected nil slice for nil effects, got %#v", ids)
	}

	effects := []*effectState{
		{ID: "effect-1"},
		nil,
		{ID: ""},
		{ID: "effect-2"},
	}

	ids := simAliveEffectIDsFromLegacy(effects)
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
		GroundItems: []GroundItem{{
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
	if simFrame.Config.Seed != legacy.Config.Seed {
		t.Fatalf("expected seed %q, got %q", legacy.Config.Seed, simFrame.Config.Seed)
	}

	simFrame.Config.Seed = "mutated-seed"
	if legacy.Config.Seed != "deterministic-seed" {
		t.Fatalf("legacy seed mutated unexpectedly: %q", legacy.Config.Seed)
	}

	roundTrip := legacyKeyframeFromSim(simKeyframeFromLegacy(legacy))
	if !reflect.DeepEqual(legacy, roundTrip) {
		t.Fatalf("keyframe round trip mismatch\nlegacy: %#v\nround-trip: %#v", legacy, roundTrip)
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

	simBatch := simEffectEventBatchFromLegacy(legacy)
	if !reflect.DeepEqual(legacy.LastSeqByID, simBatch.LastSeqByID) {
		t.Fatalf("expected seq map %#v, got %#v", legacy.LastSeqByID, simBatch.LastSeqByID)
	}

	simBatch.LastSeqByID["effect-1"] = 99
	if legacy.LastSeqByID["effect-1"] != 3 {
		t.Fatalf("legacy sequence mutated unexpectedly: %d", legacy.LastSeqByID["effect-1"])
	}

	roundTrip := legacyEffectEventBatchFromSim(simEffectEventBatchFromLegacy(legacy))
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
		GroundItems: []GroundItem{{
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

	simFrame.Players[0].Health = 10
	if legacyFrame.Players[0].Health != 80 {
		t.Fatalf("expected player conversion to deep copy actors, got %v", legacyFrame.Players[0].Health)
	}

	simFrame.Obstacles[0].Width = 99
	if legacyFrame.Obstacles[0].Width != 3 {
		t.Fatalf("expected obstacle conversion to deep copy values, got %v", legacyFrame.Obstacles[0].Width)
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
