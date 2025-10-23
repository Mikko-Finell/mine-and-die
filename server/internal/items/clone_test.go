package items

import "testing"

func TestCloneGroundItemsReturnsCopy(t *testing.T) {
	original := []GroundItem{{ID: "ground-1", Qty: 3}, {ID: "ground-2", Qty: 7}}

	cloned := CloneGroundItems(original)

	if len(cloned) != len(original) {
		t.Fatalf("expected clone length %d, got %d", len(original), len(cloned))
	}

	if len(cloned) > 0 && &cloned[0] == &original[0] {
		t.Fatalf("expected clone to allocate new backing array")
	}

	cloned[0].Qty = 99
	if original[0].Qty != 3 {
		t.Fatalf("expected original slice to remain unchanged, got %d", original[0].Qty)
	}
}

func TestCloneGroundItemsHandlesEmpty(t *testing.T) {
	if clone := CloneGroundItems(nil); clone != nil {
		t.Fatalf("expected nil input to remain nil")
	}

	empty := make([]GroundItem, 0)
	if clone := CloneGroundItems(empty); clone != nil {
		t.Fatalf("expected empty slice to return nil clone")
	}
}
