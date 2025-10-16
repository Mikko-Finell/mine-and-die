package contract

import "testing"

type sampleSpawn struct {
	ContractPayload
	Value int
}

type sampleUpdate struct {
	ContractPayload
	Name string
}

type sampleEnd struct {
	ContractPayload
}

func TestRegistryValidate_AllowsValidDefinitions(t *testing.T) {
	registry := Registry{
		{
			ID:     "example",
			Spawn:  (*sampleSpawn)(nil),
			Update: (*sampleUpdate)(nil),
			End:    (*sampleEnd)(nil),
		},
		{
			ID:     "nopayload",
			Spawn:  NoPayload,
			Update: NoPayload,
			End:    NoPayload,
		},
	}

	if err := registry.Validate(); err != nil {
		t.Fatalf("expected registry to validate, got error: %v", err)
	}
}

func TestRegistryValidate_DetectsDuplicateIDs(t *testing.T) {
	registry := Registry{
		{ID: "dup", Spawn: (*sampleSpawn)(nil), Update: NoPayload, End: NoPayload},
		{ID: "dup", Spawn: NoPayload, Update: NoPayload, End: NoPayload},
	}

	if err := registry.Validate(); err == nil {
		t.Fatal("expected duplicate ID to fail validation")
	}
}

func TestRegistryValidate_DetectsNilPayload(t *testing.T) {
	registry := Registry{{ID: "oops", Spawn: nil, Update: NoPayload, End: NoPayload}}
	err := registry.Validate()
	if err == nil {
		t.Fatal("expected nil payload to fail validation")
	}
}

type invalidValuePayload struct{}

func (invalidValuePayload) payloadMarker() {}

type invalidPointerPayload int

func (*invalidPointerPayload) payloadMarker() {}

func TestRegistryValidate_RequiresPointerStructs(t *testing.T) {
	registry := Registry{{ID: "bad", Spawn: invalidValuePayload{}, Update: NoPayload, End: NoPayload}}
	if err := registry.Validate(); err == nil {
		t.Fatal("expected non-pointer payload to fail validation")
	}

	registry = Registry{{ID: "bad2", Spawn: (*invalidPointerPayload)(nil), Update: NoPayload, End: NoPayload}}
	if err := registry.Validate(); err == nil {
		t.Fatal("expected pointer-to-non-struct payload to fail validation")
	}
}

func TestRegistryIndex_BuildsMap(t *testing.T) {
	registry := Registry{{ID: "ok", Spawn: NoPayload, Update: NoPayload, End: NoPayload}}
	index, err := registry.Index()
	if err != nil {
		t.Fatalf("expected index creation to succeed, got %v", err)
	}
	if _, ok := index["ok"]; !ok {
		t.Fatalf("expected entry for id 'ok'")
	}
}
