package server

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"
)

func TestMarshalItemDefinitionsStable(t *testing.T) {
	defs := ItemDefinitions()
	if len(defs) == 0 {
		t.Fatalf("expected item definitions to be populated")
	}
	data1, err := MarshalItemDefinitions(defs)
	if err != nil {
		t.Fatalf("marshal definitions failed: %v", err)
	}

	reversed := make([]ItemDefinition, len(defs))
	copy(reversed, defs)
	for i := 0; i < len(reversed)/2; i++ {
		j := len(reversed) - 1 - i
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	data2, err := MarshalItemDefinitions(reversed)
	if err != nil {
		t.Fatalf("marshal definitions failed: %v", err)
	}
	if !bytes.Equal(data1, data2) {
		t.Fatalf("expected deterministic marshal output, got %q vs %q", string(data1), string(data2))
	}
}

func TestComposeFungibilityKeySortsTags(t *testing.T) {
	key := ComposeFungibilityKey(ItemTypeGold, 2, "beta", "alpha")
	if key != "gold:2:alpha,beta" {
		t.Fatalf("expected sorted key, got %q", key)
	}
}

func TestNewItemDefinitionRejectsInvalidValues(t *testing.T) {
	if _, err := NewItemDefinition(ItemDefinitionParams{ID: "bad", Class: "unknown", Tier: 1}); err == nil {
		t.Fatalf("expected invalid class to error")
	}
	if _, err := NewItemDefinition(ItemDefinitionParams{ID: "dagger", Class: ItemClassWeapon, Tier: 1}); err == nil {
		t.Fatalf("expected missing equip slot for weapon to error")
	}
	if _, err := NewItemDefinition(ItemDefinitionParams{ID: "bad_actions", Class: ItemClassConsumable, Tier: 1, Stackable: true, Actions: []ItemAction{"spin"}}); err == nil {
		t.Fatalf("expected invalid action to error")
	}
}

func TestItemDefinitionRoundTripJSON(t *testing.T) {
	def, ok := ItemDefinitionFor(ItemTypeVenomCoating)
	if !ok {
		t.Fatalf("missing definition for venom coating")
	}
	data, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var decoded ItemDefinition
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !reflect.DeepEqual(decoded, def) {
		t.Fatalf("round trip mutated definition: %+v vs %+v", decoded, def)
	}
}
