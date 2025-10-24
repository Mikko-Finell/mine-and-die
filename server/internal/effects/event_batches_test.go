package effects

import (
	"reflect"
	"testing"

	effectcontract "mine-and-die/server/effects/contract"
	simpatches "mine-and-die/server/internal/sim/patches/typed"
)

func TestSimEffectEventBatchFromTypedClones(t *testing.T) {
	typed := simpatches.EffectEventBatch{
		Spawns: []effectcontract.EffectSpawnEvent{{
			Instance: effectcontract.EffectInstance{
				ID:           "effect-1",
				DefinitionID: "fireball",
				Params:       map[string]int{"power": 3},
				Colors:       []string{"scarlet", "amber"},
			},
			Tick: 5,
			Seq:  1,
		}},
		Updates: []effectcontract.EffectUpdateEvent{{
			ID:  "effect-1",
			Seq: 2,
			Params: map[string]int{
				"radius": 7,
			},
		}},
		Ends: []effectcontract.EffectEndEvent{{
			ID:   "effect-1",
			Seq:  3,
			Tick: effectcontract.Tick(8),
		}},
		LastSeqByID: map[string]effectcontract.Seq{
			"effect-1": 3,
		},
	}

	converted := SimEffectEventBatchFromTyped(typed)
	if !reflect.DeepEqual(typed, TypedEffectEventBatchFromSim(converted)) {
		t.Fatalf("round-trip conversion mismatch\ntyped: %#v\nsim->typed: %#v", typed, TypedEffectEventBatchFromSim(converted))
	}

	converted.LastSeqByID["effect-1"] = 9
	if typed.LastSeqByID["effect-1"] != 3 {
		t.Fatalf("expected legacy seq map to remain unchanged after mutation")
	}
	converted.Spawns[0].Instance.Params["power"] = 11
	if typed.Spawns[0].Instance.Params["power"] != 3 {
		t.Fatalf("expected legacy spawn params to remain unchanged after mutation")
	}
	converted.Spawns[0].Instance.Colors[0] = "violet"
	if typed.Spawns[0].Instance.Colors[0] != "scarlet" {
		t.Fatalf("expected legacy spawn colors to remain unchanged after mutation")
	}
	converted.Updates[0].Params["radius"] = 99
	if typed.Updates[0].Params["radius"] != 7 {
		t.Fatalf("expected legacy update params to remain unchanged after mutation")
	}
}

func TestSimEffectResyncSignalFromTypedClones(t *testing.T) {
	typed := simpatches.EffectResyncSignal{
		LostSpawns:  2,
		TotalEvents: 40,
		Reasons: []simpatches.EffectResyncReason{
			{Kind: "lost_spawn", EffectID: "effect-1"},
			{Kind: "stale_update", EffectID: "effect-2"},
		},
	}

	converted := SimEffectResyncSignalFromTyped(typed)
	if !reflect.DeepEqual(typed, TypedEffectResyncSignalFromSim(converted)) {
		t.Fatalf("round-trip conversion mismatch\ntyped: %#v\nsim->typed: %#v", typed, TypedEffectResyncSignalFromSim(converted))
	}

	converted.Reasons[0].Kind = "mutated"
	if typed.Reasons[0].Kind != "lost_spawn" {
		t.Fatalf("expected legacy reasons to remain unchanged after mutation")
	}
	if &converted.Reasons[0] == &converted.Reasons[1] {
		t.Fatalf("expected distinct reasons elements")
	}
	converted.Reasons = append(converted.Reasons, converted.Reasons[0])
	if len(typed.Reasons) != 2 {
		t.Fatalf("expected legacy reasons length to remain unchanged")
	}
}
