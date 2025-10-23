package effects

import (
	"reflect"
	"testing"

	journal "mine-and-die/server/internal/journal"
	"mine-and-die/server/internal/sim"
)

func TestEffectParamsPayloadConversions(t *testing.T) {
	legacy := journal.EffectParamsPayload{Params: map[string]float64{"radius": 3}}
	simPayload := SimEffectParamsPayloadFromLegacy(legacy)
	if simPayload.Params["radius"] != 3 {
		t.Fatalf("expected radius to be copied, got %v", simPayload.Params["radius"])
	}
	simPayload.Params["radius"] = 9
	if legacy.Params["radius"] != 3 {
		t.Fatalf("expected legacy params to remain unchanged after mutation")
	}

	roundTrip := LegacyEffectParamsPayloadFromSim(simPayload)
	if !reflect.DeepEqual(roundTrip.Params, map[string]float64{"radius": 9}) {
		t.Fatalf("expected round-trip params to match mutated sim payload, got %#v", roundTrip.Params)
	}
	roundTrip.Params["radius"] = 12
	if simPayload.Params["radius"] != 9 {
		t.Fatalf("expected sim payload to remain unchanged after legacy mutation")
	}

	if converted := SimEffectParamsPayloadFromLegacyPtr(nil); converted != nil {
		t.Fatalf("expected nil pointer conversion to return nil, got %#v", converted)
	}
	if converted := LegacyEffectParamsPayloadFromSimPtr(nil); converted != nil {
		t.Fatalf("expected nil pointer conversion to return nil, got %#v", converted)
	}

	legacyPtr := journal.EffectParamsPayload{Params: map[string]float64{"speed": 2}}
	simPtr := SimEffectParamsPayloadFromLegacyPtr(&legacyPtr)
	if simPtr == nil || simPtr.Params["speed"] != 2 {
		t.Fatalf("expected pointer conversion to copy params, got %#v", simPtr)
	}
	simPtr.Params["speed"] = 5
	if legacyPtr.Params["speed"] != 2 {
		t.Fatalf("expected legacy pointer to remain unchanged after sim mutation")
	}

	simPayloadPtr := sim.EffectParamsPayload{Params: map[string]float64{"scale": 4}}
	legacyConverted := LegacyEffectParamsPayloadFromSimPtr(&simPayloadPtr)
	if legacyConverted == nil || legacyConverted.Params["scale"] != 4 {
		t.Fatalf("expected legacy pointer conversion to copy params, got %#v", legacyConverted)
	}
	legacyConverted.Params["scale"] = 6
	if simPayloadPtr.Params["scale"] != 4 {
		t.Fatalf("expected sim payload pointer to remain unchanged after mutation")
	}
}

func TestCloneEffectParams(t *testing.T) {
	params := CloneEffectParams(map[string]float64{"angle": 45})
	if params["angle"] != 45 {
		t.Fatalf("expected cloned params to copy value, got %v", params["angle"])
	}
	params["angle"] = 90
	clonedAgain := CloneEffectParams(params)
	if clonedAgain["angle"] != 90 {
		t.Fatalf("expected clone to copy latest value, got %v", clonedAgain["angle"])
	}
	if CloneEffectParams(nil) != nil {
		t.Fatalf("expected nil input to return nil clone")
	}
	if CloneEffectParams(map[string]float64{}) != nil {
		t.Fatalf("expected empty map to return nil clone")
	}
}
