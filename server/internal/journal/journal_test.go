package journal

import (
	"reflect"
	"testing"

	effectcontract "mine-and-die/server/effects/contract"
)

func TestJournalPatchBuffersClone(t *testing.T) {
	j := New(0, 0)

	original := Patch{
		Kind:     PatchPlayerHealth,
		EntityID: "player-1",
		Payload: PlayerHealthPayload{
			Health:    75,
			MaxHealth: 100,
		},
	}
	j.AppendPatch(original)

	snapshot := j.SnapshotPatches()
	if len(snapshot) != 1 {
		t.Fatalf("expected snapshot to contain 1 patch, got %d", len(snapshot))
	}
	snapshot[0].EntityID = "mutated"
	snapshot[0].Kind = PatchPlayerFacing

	drained := j.DrainPatches()
	if len(drained) != 1 {
		t.Fatalf("expected drain to return 1 patch, got %d", len(drained))
	}
	if drained[0].EntityID != original.EntityID {
		t.Fatalf("expected drain to preserve entity id %q, got %q", original.EntityID, drained[0].EntityID)
	}
	if drained[0].Kind != original.Kind {
		t.Fatalf("expected drain to preserve kind %q, got %q", original.Kind, drained[0].Kind)
	}

	drained[0].EntityID = "restored"
	j.RestorePatches(drained)
	drained[0].EntityID = "post-restore-mutation"

	restored := j.SnapshotPatches()
	if len(restored) != 1 {
		t.Fatalf("expected restored snapshot to contain 1 patch, got %d", len(restored))
	}
	if restored[0].EntityID != "restored" {
		t.Fatalf("expected restore to capture entity id %q, got %q", "restored", restored[0].EntityID)
	}

	restored[0].EntityID = "changed"
	secondDrain := j.DrainPatches()
	if len(secondDrain) != 1 {
		t.Fatalf("expected second drain to return 1 patch, got %d", len(secondDrain))
	}
	if secondDrain[0].EntityID != "restored" {
		t.Fatalf("expected second drain to keep entity id %q, got %q", "restored", secondDrain[0].EntityID)
	}

	if cleared := j.DrainPatches(); len(cleared) != 0 {
		t.Fatalf("expected journal to be empty after drain, got %d patches", len(cleared))
	}
}

func TestJournalEffectEventsClone(t *testing.T) {
	j := New(0, 0)

	deliveryVariants := map[string]int{"alt": 2}
	spawnParams := map[string]int{"power": 3}
	behaviorExtra := map[string]int{"damage": 15}
	behaviorStacks := map[string]int{"burn": 1}
	definitionParams := map[string]int{"cooldown": 1}
	updateFields := map[string]bool{"ticksRemaining": true}
	spawnColors := []string{"orange"}

	spawnEvent := effectcontract.EffectSpawnEvent{
		Tick: 10,
		Instance: effectcontract.EffectInstance{
			ID:           "effect-1",
			DefinitionID: "fireball",
			Definition: &effectcontract.EffectDefinition{
				TypeID:        "fireball",
				Delivery:      effectcontract.DeliveryKindArea,
				Shape:         effectcontract.GeometryShapeCircle,
				Motion:        effectcontract.MotionKindLinear,
				Impact:        effectcontract.ImpactPolicyAllInPath,
				LifetimeTicks: 5,
				Params:        definitionParams,
				Hooks:         effectcontract.EffectHooks{},
				Client: effectcontract.ReplicationSpec{
					SendSpawn:    true,
					SendUpdates:  true,
					SendEnd:      true,
					UpdateFields: updateFields,
				},
				End: effectcontract.EndPolicy{Kind: effectcontract.EndDuration},
			},
			DeliveryState: effectcontract.EffectDeliveryState{
				Geometry: effectcontract.EffectGeometry{
					Shape:    effectcontract.GeometryShapeCircle,
					Radius:   4,
					Variants: deliveryVariants,
				},
			},
			BehaviorState: effectcontract.EffectBehaviorState{
				TicksRemaining: 5,
				Extra:          behaviorExtra,
				Stacks:         behaviorStacks,
			},
			Params: spawnParams,
			Colors: spawnColors,
			Replication: effectcontract.ReplicationSpec{
				SendSpawn:    true,
				SendUpdates:  true,
				SendEnd:      true,
				UpdateFields: map[string]bool{"behaviorState": true},
			},
		},
	}

	spawn := j.RecordEffectSpawn(spawnEvent)
	if spawn.Seq != 1 {
		t.Fatalf("expected spawn sequence 1, got %d", spawn.Seq)
	}

	deliveryVariants["alt"] = 5
	spawnParams["power"] = 7
	behaviorExtra["damage"] = 40
	behaviorStacks["burn"] = 9
	definitionParams["cooldown"] = 5
	updateFields["ticksRemaining"] = false
	spawnColors[0] = "changed"

	if spawn.Instance.Params["power"] != 3 {
		t.Fatalf("expected spawn params to be cloned, got %d", spawn.Instance.Params["power"])
	}
	if spawn.Instance.BehaviorState.Extra["damage"] != 15 {
		t.Fatalf("expected spawn extra to be cloned, got %d", spawn.Instance.BehaviorState.Extra["damage"])
	}
	if spawn.Instance.BehaviorState.Stacks["burn"] != 1 {
		t.Fatalf("expected spawn stacks to be cloned, got %d", spawn.Instance.BehaviorState.Stacks["burn"])
	}
	if spawn.Instance.DeliveryState.Geometry.Variants["alt"] != 2 {
		t.Fatalf("expected spawn geometry variants to be cloned, got %d", spawn.Instance.DeliveryState.Geometry.Variants["alt"])
	}
	if spawn.Instance.Colors[0] != "orange" {
		t.Fatalf("expected spawn colors to be cloned, got %s", spawn.Instance.Colors[0])
	}
	if spawn.Instance.Definition.Params["cooldown"] != 1 {
		t.Fatalf("expected definition params to be cloned, got %d", spawn.Instance.Definition.Params["cooldown"])
	}
	if spawn.Instance.Definition.Client.UpdateFields["ticksRemaining"] != true {
		t.Fatalf("expected client update fields to be cloned, got %v", spawn.Instance.Definition.Client.UpdateFields["ticksRemaining"])
	}

	updateParams := map[string]int{"damage": 20}
	updateDeliveryVariants := map[string]int{"variant": 3}
	updateBehaviorExtra := map[string]int{"ticks": 2}

	updateEvent := effectcontract.EffectUpdateEvent{
		Tick: 11,
		ID:   "effect-1",
		DeliveryState: &effectcontract.EffectDeliveryState{
			Geometry: effectcontract.EffectGeometry{
				Shape:    effectcontract.GeometryShapeRect,
				Variants: updateDeliveryVariants,
			},
		},
		BehaviorState: &effectcontract.EffectBehaviorState{
			Extra: updateBehaviorExtra,
		},
		Params: updateParams,
	}

	update := j.RecordEffectUpdate(updateEvent)
	if update.Seq != 2 {
		t.Fatalf("expected update sequence 2, got %d", update.Seq)
	}

	updateParams["damage"] = 50
	updateDeliveryVariants["variant"] = 10
	updateBehaviorExtra["ticks"] = 9

	if update.Params["damage"] != 20 {
		t.Fatalf("expected update params to be cloned, got %d", update.Params["damage"])
	}
	if update.DeliveryState.Geometry.Variants["variant"] != 3 {
		t.Fatalf("expected update delivery variants to be cloned, got %d", update.DeliveryState.Geometry.Variants["variant"])
	}
	if update.BehaviorState.Extra["ticks"] != 2 {
		t.Fatalf("expected update behavior state to be cloned, got %d", update.BehaviorState.Extra["ticks"])
	}

	end := j.RecordEffectEnd(effectcontract.EffectEndEvent{Tick: 12, ID: "effect-1"})
	if end.Seq != 3 {
		t.Fatalf("expected end sequence 3, got %d", end.Seq)
	}

	snapshot := j.SnapshotEffectEvents()
	if len(snapshot.Spawns) != 1 || len(snapshot.Updates) != 1 || len(snapshot.Ends) != 1 {
		t.Fatalf("expected snapshot to contain one of each event, got %+v", snapshot)
	}
	snapshot.Spawns[0].Instance.Params["power"] = 99
	snapshot.Updates[0].Params["damage"] = 99
	snapshot.LastSeqByID["effect-1"] = 42

	drained := j.DrainEffectEvents()
	if drained.Spawns[0].Instance.Params["power"] != 3 {
		t.Fatalf("expected drain spawn params to remain 3, got %d", drained.Spawns[0].Instance.Params["power"])
	}
	if drained.Updates[0].Params["damage"] != 20 {
		t.Fatalf("expected drain update params to remain 20, got %d", drained.Updates[0].Params["damage"])
	}
	if drained.LastSeqByID["effect-1"] != 3 {
		t.Fatalf("expected drain cursor to remain 3, got %d", drained.LastSeqByID["effect-1"])
	}

	drained.Spawns[0].Instance.Params["power"] = 77
	drained.Updates[0].Params["damage"] = 33
	drained.LastSeqByID["effect-1"] = 4

	j.RestoreEffectEvents(drained)

	restored := j.SnapshotEffectEvents()
	if restored.Spawns[0].Instance.Params["power"] != 77 {
		t.Fatalf("expected restored spawn params to be 77, got %d", restored.Spawns[0].Instance.Params["power"])
	}
	if restored.Updates[0].Params["damage"] != 33 {
		t.Fatalf("expected restored update params to be 33, got %d", restored.Updates[0].Params["damage"])
	}
	if restored.LastSeqByID["effect-1"] != 4 {
		t.Fatalf("expected restored cursor to be 4, got %d", restored.LastSeqByID["effect-1"])
	}
}

func TestJournalResyncPolicySignals(t *testing.T) {
	j := New(0, 0)

	if signal, ok := j.ConsumeResyncHint(); ok || signal.LostSpawns != 0 || signal.TotalEvents != 0 || len(signal.Reasons) != 0 {
		t.Fatalf("expected no resync signal before events, got %+v", signal)
	}

	// Unknown update should trigger a lost-spawn resync hint.
	dropped := j.RecordEffectUpdate(effectcontract.EffectUpdateEvent{Tick: 5, ID: "missing"})
	if dropped.ID != "" || dropped.Seq != 0 || dropped.DeliveryState != nil || dropped.BehaviorState != nil || len(dropped.Params) != 0 {
		t.Fatalf("expected unknown update to be dropped, got %+v", dropped)
	}

	signal, ok := j.ConsumeResyncHint()
	if !ok {
		t.Fatalf("expected resync hint after unknown update")
	}
	if signal.LostSpawns != 1 {
		t.Fatalf("expected lost spawns to be 1, got %d", signal.LostSpawns)
	}
	if signal.TotalEvents != 1 {
		t.Fatalf("expected total events to be 1, got %d", signal.TotalEvents)
	}
	if len(signal.Reasons) != 1 {
		t.Fatalf("expected one reason, got %d", len(signal.Reasons))
	}
	if signal.Reasons[0].Kind != metricJournalUnknownIDUpdate {
		t.Fatalf("expected reason kind %q, got %q", metricJournalUnknownIDUpdate, signal.Reasons[0].Kind)
	}
	if signal.Reasons[0].EffectID != "missing" {
		t.Fatalf("expected reason effect id 'missing', got %q", signal.Reasons[0].EffectID)
	}

	if _, ok := j.ConsumeResyncHint(); ok {
		t.Fatalf("expected resync hint to reset after consumption")
	}

	// Subsequent known spawn and update events should accumulate without
	// immediately re-triggering the hint until another lost spawn occurs.
	j.RecordEffectSpawn(effectcontract.EffectSpawnEvent{Tick: 6, Instance: effectcontract.EffectInstance{ID: "effect-1"}})
	j.RecordEffectUpdate(effectcontract.EffectUpdateEvent{Tick: 7, ID: "effect-1"})

	if _, ok := j.ConsumeResyncHint(); ok {
		t.Fatalf("expected no resync hint without a new lost spawn")
	}
}

func TestJournalRecordKeyframeCopiesConfig(t *testing.T) {
	type testWorldConfig struct {
		Obstacles      bool
		ObstaclesCount int
		GoldMines      bool
		GoldMineCount  int
		NPCs           bool
		GoblinCount    int
		RatCount       int
		NPCCount       int
		Lava           bool
		LavaCount      int
		Seed           string
		Width          float64
		Height         float64
	}

	journal := New(4, 0)

	config := testWorldConfig{
		Obstacles:      true,
		ObstaclesCount: 6,
		GoldMines:      true,
		GoldMineCount:  3,
		NPCs:           true,
		GoblinCount:    5,
		RatCount:       7,
		NPCCount:       12,
		Lava:           true,
		LavaCount:      2,
		Seed:           "baseline",
		Width:          160,
		Height:         90,
	}
	expected := config

	frame := Keyframe{
		Tick:     256,
		Sequence: 8001,
		Config:   config,
	}

	journal.RecordKeyframe(frame)

	config.GoblinCount = 99
	frame.Config = testWorldConfig{Seed: "mutated"}

	recorded, ok := journal.KeyframeBySequence(frame.Sequence)
	if !ok {
		t.Fatalf("expected journal to return keyframe %d", frame.Sequence)
	}

	typed, ok := recorded.Config.(testWorldConfig)
	if !ok {
		t.Fatalf("expected recorded config to be testWorldConfig, got %T", recorded.Config)
	}
	if !reflect.DeepEqual(expected, typed) {
		t.Fatalf("unexpected recorded keyframe config: got %#v want %#v", typed, expected)
	}
	if _, ok := recorded.Config.(*testWorldConfig); ok {
		t.Fatalf("expected recorded config to be stored by value")
	}
}

func TestJournalKeyframeBySequenceCopiesConfig(t *testing.T) {
	type testWorldConfig struct {
		Obstacles      bool
		ObstaclesCount int
		GoldMines      bool
		GoldMineCount  int
		NPCs           bool
		GoblinCount    int
		RatCount       int
		NPCCount       int
		Lava           bool
		LavaCount      int
		Seed           string
		Width          float64
		Height         float64
	}

	journal := New(4, 0)

	config := testWorldConfig{
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
		Seed:           "steady",
		Width:          128,
		Height:         96,
	}
	expected := config

	frame := Keyframe{
		Tick:     512,
		Sequence: 9001,
		Config:   config,
	}

	journal.RecordKeyframe(frame)

	config.GoblinCount = 99
	frame.Config = testWorldConfig{Seed: "mutated"}

	fetched, ok := journal.KeyframeBySequence(frame.Sequence)
	if !ok {
		t.Fatalf("expected journal to return keyframe %d", frame.Sequence)
	}

	typed, ok := fetched.Config.(testWorldConfig)
	if !ok {
		t.Fatalf("expected fetched config to be testWorldConfig, got %T", fetched.Config)
	}
	if !reflect.DeepEqual(expected, typed) {
		t.Fatalf("unexpected keyframe config: got %#v want %#v", typed, expected)
	}
	if _, ok := fetched.Config.(*testWorldConfig); ok {
		t.Fatalf("expected keyframe config to be stored by value")
	}

	typed.Width = 999
	typed.LavaCount = 42

	again, ok := journal.KeyframeBySequence(frame.Sequence)
	if !ok {
		t.Fatalf("expected journal to return keyframe %d on second lookup", frame.Sequence)
	}

	typedAgain, ok := again.Config.(testWorldConfig)
	if !ok {
		t.Fatalf("expected second fetched config to be testWorldConfig, got %T", again.Config)
	}
	if !reflect.DeepEqual(expected, typedAgain) {
		t.Fatalf("expected keyframe config to remain unchanged, got %#v want %#v", typedAgain, expected)
	}
}
