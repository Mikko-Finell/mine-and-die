package effects

import "testing"

func TestAliveEffectIDsFromStates(t *testing.T) {
	if ids := AliveEffectIDsFromStates(nil); ids != nil {
		t.Fatalf("expected nil for nil input, got %#v", ids)
	}

	effects := []*State{
		{ID: "effect-1"},
		nil,
		{ID: ""},
		{ID: "effect-2"},
	}

	ids := AliveEffectIDsFromStates(effects)
	expected := []string{"effect-1", "effect-2"}
	if len(ids) != len(expected) {
		t.Fatalf("expected %d ids, got %d", len(expected), len(ids))
	}
	for i, want := range expected {
		if ids[i] != want {
			t.Fatalf("expected ids[%d] = %s, got %s", i, want, ids[i])
		}
	}

	effects[0].ID = "mutated"
	if ids[0] != "effect-1" {
		t.Fatalf("expected returned ids to be independent copy, got %q", ids[0])
	}
}

func TestCloneAliveEffectIDs(t *testing.T) {
	if ids := CloneAliveEffectIDs(nil); ids != nil {
		t.Fatalf("expected nil for nil input, got %#v", ids)
	}

	cloned := CloneAliveEffectIDs([]string{"effect-1", "", "effect-2"})
	expected := []string{"effect-1", "effect-2"}
	if len(cloned) != len(expected) {
		t.Fatalf("expected %d ids, got %d", len(expected), len(cloned))
	}
	for i, want := range expected {
		if cloned[i] != want {
			t.Fatalf("expected ids[%d] = %s, got %s", i, want, cloned[i])
		}
	}

	cloned[0] = "mutated"
	ids := CloneAliveEffectIDs([]string{"effect-1"})
	if len(ids) != 1 || ids[0] != "effect-1" {
		t.Fatalf("expected new allocation to remain unchanged, got %#v", ids)
	}
}
