package effects

import (
	"testing"

	"mine-and-die/server/internal/sim"
)

func TestSimEffectTriggersFromLegacyClonesCollections(t *testing.T) {
	legacyParams := map[string]float64{"radius": 1}
	legacyColors := []string{"red", "orange"}
	legacy := []Trigger{{
		ID:     "fireball",
		Params: legacyParams,
		Colors: legacyColors,
	}}

	converted := SimEffectTriggersFromLegacy(legacy)
	if len(converted) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(converted))
	}
	legacyParams["radius"] = 2
	legacyColors[0] = "blue"
	if converted[0].Params["radius"] != 1 {
		t.Fatalf("expected params clone to remain 1, got %v", converted[0].Params["radius"])
	}
	if converted[0].Colors[0] != "red" {
		t.Fatalf("expected color clone to remain red, got %q", converted[0].Colors[0])
	}
}

func TestLegacyEffectTriggersFromSimClonesCollections(t *testing.T) {
	simParams := map[string]float64{"radius": 1}
	simColors := []string{"red", "orange"}
	simTriggers := []sim.EffectTrigger{{
		ID:     "fireball",
		Params: simParams,
		Colors: simColors,
	}}

	converted := LegacyEffectTriggersFromSim(simTriggers)
	if len(converted) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(converted))
	}
	simParams["radius"] = 2
	simColors[0] = "blue"
	if converted[0].Params["radius"] != 1 {
		t.Fatalf("expected params clone to remain 1, got %v", converted[0].Params["radius"])
	}
	if converted[0].Colors[0] != "red" {
		t.Fatalf("expected color clone to remain red, got %q", converted[0].Colors[0])
	}
}

func TestEffectTriggerConversionsHandleEmpty(t *testing.T) {
	if SimEffectTriggersFromLegacy(nil) != nil {
		t.Fatal("expected nil legacy slice to return nil")
	}
	if SimEffectTriggersFromLegacy([]Trigger{}) != nil {
		t.Fatal("expected empty legacy slice to return nil")
	}
	if LegacyEffectTriggersFromSim(nil) != nil {
		t.Fatal("expected nil sim slice to return nil")
	}
	if LegacyEffectTriggersFromSim([]sim.EffectTrigger{}) != nil {
		t.Fatal("expected empty sim slice to return nil")
	}
}
