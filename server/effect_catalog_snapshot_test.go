package main

import (
	"os"
	"path/filepath"
	"testing"

	effectcatalog "mine-and-die/server/effects/catalog"
	effectcontract "mine-and-die/server/effects/contract"
)

func TestSnapshotEffectCatalogIncludesManagedFlag(t *testing.T) {
	dir := t.TempDir()
	catalogPath := filepath.Join(dir, "definitions.json")
	catalogJSON := `[
  {
    "id": "attack",
    "contractId": "attack",
    "definition": {
      "typeId": "attack",
      "delivery": "area",
      "shape": "rect",
      "motion": "instant",
      "impact": "all-in-path",
      "lifetimeTicks": 1,
      "hooks": {"onSpawn": "swing"},
      "client": {"sendSpawn": true},
      "end": {"kind": 0}
    },
    "jsEffect": "attack/basic"
  }
]`
	if err := os.WriteFile(catalogPath, []byte(catalogJSON), 0o644); err != nil {
		t.Fatalf("failed to write catalog: %v", err)
	}

	reg := effectcontract.Registry{
		{
			ID:     "attack",
			Spawn:  effectcontract.NoPayload,
			Update: effectcontract.NoPayload,
			End:    effectcontract.NoPayload,
			Owner:  effectcontract.LifecycleOwnerClient,
		},
	}

	resolver, err := effectcatalog.Load(reg, catalogPath)
	if err != nil {
		t.Fatalf("failed to load resolver: %v", err)
	}

	snapshot := snapshotEffectCatalog(resolver)
	if len(snapshot) != 1 {
		t.Fatalf("expected snapshot to contain one entry, got %d", len(snapshot))
	}

	entry, ok := snapshot["attack"]
	if !ok {
		t.Fatalf("expected attack entry in snapshot")
	}
	if !entry.ManagedByClient {
		t.Fatalf("expected managedByClient to be propagated")
	}
	if entry.Definition == nil || !entry.Definition.Client.ManagedByClient {
		t.Fatalf("expected definition to retain managedByClient flag")
	}
}
