package effects

import (
	"reflect"
	"testing"

	"mine-and-die/server/internal/sim"
)

func TestCloneEffectTriggersReturnsCopy(t *testing.T) {
	original := []sim.EffectTrigger{{
		ID:       "fx1",
		Type:     "spark",
		Start:    10,
		Duration: 20,
		X:        1.5,
		Y:        2.5,
		Width:    3.5,
		Height:   4.5,
		Params: map[string]float64{
			"radius": 3,
		},
		Colors: []string{"red", "green"},
	}}

	cloned := CloneEffectTriggers(original)
	if !reflect.DeepEqual(cloned, original) {
		t.Fatalf("cloned triggers differ: %+v vs %+v", cloned, original)
	}
	if &cloned[0] == &original[0] {
		t.Fatal("cloned slice shares underlying trigger entry")
	}
	cloned[0].Params["radius"] = 10
	if original[0].Params["radius"] != 3 {
		t.Fatalf("expected original radius to remain 3, got %v", original[0].Params["radius"])
	}
	cloned[0].Colors[0] = "blue"
	if original[0].Colors[0] != "red" {
		t.Fatalf("expected original color to remain red, got %q", original[0].Colors[0])
	}
}

func TestCloneEffectTriggersHandlesEmpty(t *testing.T) {
	if clone := CloneEffectTriggers(nil); clone != nil {
		t.Fatalf("expected nil clone for nil input, got %+v", clone)
	}

	empty := []sim.EffectTrigger{}
	if clone := CloneEffectTriggers(empty); clone != nil {
		t.Fatalf("expected nil clone for empty input, got %+v", clone)
	}
}

func TestCloneEffectTriggerDropsEmptyCollections(t *testing.T) {
	trigger := sim.EffectTrigger{
		Params: map[string]float64{},
		Colors: make([]string, 0, 1),
	}

	cloned := CloneEffectTrigger(trigger)
	if cloned.Params != nil {
		t.Fatalf("expected nil params clone, got %+v", cloned.Params)
	}
	if cloned.Colors != nil {
		t.Fatalf("expected nil colors clone, got %+v", cloned.Colors)
	}
}
