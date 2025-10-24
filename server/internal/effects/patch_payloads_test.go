package effects

import (
	"reflect"
	"testing"

	"mine-and-die/server/internal/sim"
	simpatches "mine-and-die/server/internal/sim/patches/typed"
)

func TestEffectParamsPayloadConversions(t *testing.T) {
	typedPayload := simpatches.EffectParamsPayload{Params: map[string]float64{"radius": 3}}
	simPayload := SimEffectParamsPayloadFromTyped(typedPayload)
	if simPayload.Params["radius"] != 3 {
		t.Fatalf("expected radius to be copied, got %v", simPayload.Params["radius"])
	}
	simPayload.Params["radius"] = 9
	if typedPayload.Params["radius"] != 3 {
		t.Fatalf("expected legacy params to remain unchanged after mutation")
	}

	roundTrip := TypedEffectParamsPayloadFromSim(simPayload)
	if !reflect.DeepEqual(roundTrip.Params, map[string]float64{"radius": 9}) {
		t.Fatalf("expected round-trip params to match mutated sim payload, got %#v", roundTrip.Params)
	}
	roundTrip.Params["radius"] = 12
	if simPayload.Params["radius"] != 9 {
		t.Fatalf("expected sim payload to remain unchanged after legacy mutation")
	}

	if converted := SimEffectParamsPayloadFromTypedPtr(nil); converted != nil {
		t.Fatalf("expected nil pointer conversion to return nil, got %#v", converted)
	}
	if converted := TypedEffectParamsPayloadFromSimPtr(nil); converted != nil {
		t.Fatalf("expected nil pointer conversion to return nil, got %#v", converted)
	}

	typedPtr := simpatches.EffectParamsPayload{Params: map[string]float64{"speed": 2}}
	simPtr := SimEffectParamsPayloadFromTypedPtr(&typedPtr)
	if simPtr == nil || simPtr.Params["speed"] != 2 {
		t.Fatalf("expected pointer conversion to copy params, got %#v", simPtr)
	}
	simPtr.Params["speed"] = 5
	if typedPtr.Params["speed"] != 2 {
		t.Fatalf("expected legacy pointer to remain unchanged after sim mutation")
	}

	simPayloadPtr := sim.EffectParamsPayload{Params: map[string]float64{"scale": 4}}
	typedConverted := TypedEffectParamsPayloadFromSimPtr(&simPayloadPtr)
	if typedConverted == nil || typedConverted.Params["scale"] != 4 {
		t.Fatalf("expected legacy pointer conversion to copy params, got %#v", typedConverted)
	}
	typedConverted.Params["scale"] = 6
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
