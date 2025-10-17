package main

import (
	"encoding/json"
	"testing"

	effectcatalog "mine-and-die/server/effects/catalog"
	effectcontract "mine-and-die/server/effects/contract"
)

func TestNewEffectCatalogMetadataClonesEntry(t *testing.T) {
	def := &effectcontract.EffectDefinition{
		TypeID: "fireball",
		Client: effectcontract.ReplicationSpec{ManagedByClient: true},
	}
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

	if !meta.ManagedByClient {
		t.Fatalf("expected managedByClient flag to mirror entry definition")
	}
	def.Client.ManagedByClient = false
	if !meta.ManagedByClient {
		t.Fatalf("expected cloned metadata to retain managedByClient flag after source mutation")
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
	if !clone.ManagedByClient {
		t.Fatalf("expected clone to retain managedByClient flag")
	}
}

func TestEffectCatalogMetadataMarshalJSONFlattensBlocks(t *testing.T) {
	meta := effectCatalogMetadata{
		ContractID: "fireball",
		Definition: &effectcontract.EffectDefinition{
			TypeID:        "fireball",
			Delivery:      effectcontract.DeliveryKindArea,
			Shape:         effectcontract.GeometryShapeCircle,
			Motion:        effectcontract.MotionKindLinear,
			Impact:        effectcontract.ImpactPolicyFirstHit,
			LifetimeTicks: 10,
			Hooks:         effectcontract.EffectHooks{OnSpawn: "spawn"},
			Client:        effectcontract.ReplicationSpec{SendSpawn: true, ManagedByClient: true},
			End:           effectcontract.EndPolicy{Kind: effectcontract.EndDuration},
		},
		Blocks: map[string]json.RawMessage{
			"jsEffect":   json.RawMessage(`"projectile/fireball"`),
			"parameters": json.RawMessage(`{"speed":320}`),
		},
		ManagedByClient: true,
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
	if string(decoded["managedByClient"]) != "true" {
		t.Fatalf("expected managedByClient to be encoded, got %s", string(decoded["managedByClient"]))
	}
	if string(decoded["jsEffect"]) != "\"projectile/fireball\"" {
		t.Fatalf("expected jsEffect block to be flattened, got %s", string(decoded["jsEffect"]))
	}
	if string(decoded["parameters"]) != "{\"speed\":320}" {
		t.Fatalf("expected parameters block to be flattened, got %s", string(decoded["parameters"]))
	}
}
