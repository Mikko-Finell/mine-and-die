package effects

import (
	"reflect"
	"testing"

	effectcontract "mine-and-die/server/effects/contract"
	journal "mine-and-die/server/internal/journal"
)

func TestSimEffectEventBatchFromLegacyClones(t *testing.T) {
	legacy := journal.EffectEventBatch{
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

	converted := SimEffectEventBatchFromLegacy(legacy)
	if !reflect.DeepEqual(legacy, LegacyEffectEventBatchFromSim(converted)) {
		t.Fatalf("round-trip conversion mismatch\nlegacy: %#v\nsim->legacy: %#v", legacy, LegacyEffectEventBatchFromSim(converted))
	}

	converted.LastSeqByID["effect-1"] = 9
	if legacy.LastSeqByID["effect-1"] != 3 {
		t.Fatalf("expected legacy seq map to remain unchanged after mutation")
	}
	converted.Spawns[0].Instance.Params["power"] = 11
	if legacy.Spawns[0].Instance.Params["power"] != 3 {
		t.Fatalf("expected legacy spawn params to remain unchanged after mutation")
	}
	converted.Spawns[0].Instance.Colors[0] = "violet"
	if legacy.Spawns[0].Instance.Colors[0] != "scarlet" {
		t.Fatalf("expected legacy spawn colors to remain unchanged after mutation")
	}
	converted.Updates[0].Params["radius"] = 99
	if legacy.Updates[0].Params["radius"] != 7 {
		t.Fatalf("expected legacy update params to remain unchanged after mutation")
	}
}

func TestSimEffectResyncSignalFromLegacyClones(t *testing.T) {
	legacy := journal.ResyncSignal{
		LostSpawns:  2,
		TotalEvents: 40,
		Reasons: []journal.ResyncReason{
			{Kind: "lost_spawn", EffectID: "effect-1"},
			{Kind: "stale_update", EffectID: "effect-2"},
		},
	}

	converted := SimEffectResyncSignalFromLegacy(legacy)
	if !reflect.DeepEqual(legacy, LegacyEffectResyncSignalFromSim(converted)) {
		t.Fatalf("round-trip conversion mismatch\nlegacy: %#v\nsim->legacy: %#v", legacy, LegacyEffectResyncSignalFromSim(converted))
	}

	converted.Reasons[0].Kind = "mutated"
	if legacy.Reasons[0].Kind != "lost_spawn" {
		t.Fatalf("expected legacy reasons to remain unchanged after mutation")
	}
	if &converted.Reasons[0] == &converted.Reasons[1] {
		t.Fatalf("expected distinct reasons elements")
	}
	converted.Reasons = append(converted.Reasons, converted.Reasons[0])
	if len(legacy.Reasons) != 2 {
		t.Fatalf("expected legacy reasons length to remain unchanged")
	}
}
