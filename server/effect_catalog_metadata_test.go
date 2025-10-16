package main

import (
	"encoding/json"
	"testing"

	effectcatalog "mine-and-die/server/effects/catalog"
)

func TestNewEffectCatalogMetadataClonesEntry(t *testing.T) {
	def := &EffectDefinition{TypeID: "fireball"}
	blocks := map[string]json.RawMessage{
		"jsEffect":   json.RawMessage(`"projectile/fireball"`),
		"parameters": json.RawMessage(`{"speed":320}`),
	}

	entry := effectcatalog.Entry{
		ID:         "fireball",
		ContractID: "fireball",
		Definition: def,
		Blocks:     blocks,
	}

	meta := newEffectCatalogMetadata(entry)

	if meta.Definition == def {
		t.Fatalf("expected definition pointer to be cloned")
	}
	def.TypeID = "mutated"
	if meta.Definition == nil || meta.Definition.TypeID != "fireball" {
		t.Fatalf("expected cloned definition to retain original type id, got %v", meta.Definition)
	}

	blocks["jsEffect"][0] = 'x'
	encoded := string(meta.Blocks["jsEffect"])
	if encoded != "\"projectile/fireball\"" {
		t.Fatalf("expected cloned block to retain original value, got %q", encoded)
	}

	clone := meta.clone()
	if clone.Definition == meta.Definition {
		t.Fatalf("expected clone to copy definition pointer")
	}
	if string(clone.Blocks["jsEffect"]) != encoded {
		t.Fatalf("expected clone to retain metadata value, got %q", string(clone.Blocks["jsEffect"]))
	}
}

func TestEffectCatalogMetadataMarshalJSONFlattensBlocks(t *testing.T) {
	meta := effectCatalogMetadata{
		ContractID: "fireball",
		Definition: &EffectDefinition{
			TypeID:        "fireball",
			Delivery:      DeliveryKindArea,
			Shape:         GeometryShapeCircle,
			Motion:        MotionKindLinear,
			Impact:        ImpactPolicyFirstHit,
			LifetimeTicks: 10,
			Hooks:         EffectHooks{OnSpawn: "spawn"},
			Client:        ReplicationSpec{SendSpawn: true},
			End:           EndPolicy{Kind: EndDuration},
		},
		Blocks: map[string]json.RawMessage{
			"jsEffect":   json.RawMessage(`"projectile/fireball"`),
			"parameters": json.RawMessage(`{"speed":320}`),
		},
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("MarshalJSON returned error: %v", err)
	}

	decoded := make(map[string]json.RawMessage)
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to decode marshalled metadata: %v", err)
	}

	if string(decoded["contractId"]) != "\"fireball\"" {
		t.Fatalf("expected contractId to be encoded, got %s", string(decoded["contractId"]))
	}
	if _, ok := decoded["definition"]; !ok {
		t.Fatalf("expected definition field to be present")
	}
	if string(decoded["jsEffect"]) != "\"projectile/fireball\"" {
		t.Fatalf("expected jsEffect block to be flattened, got %s", string(decoded["jsEffect"]))
	}
	if string(decoded["parameters"]) != "{\"speed\":320}" {
		t.Fatalf("expected parameters block to be flattened, got %s", string(decoded["parameters"]))
	}
}
