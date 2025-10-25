package runtime

import "testing"

func TestSpatialIndexInsertAndRemove(t *testing.T) {
	idx := NewSpatialIndex(DefaultSpatialCellSize, 4)
	eff := &State{ID: "effect-1", X: 10, Y: 10, Width: 20, Height: 20}
	if !idx.Upsert(eff) {
		t.Fatalf("expected initial insert to succeed")
	}
	entry := idx.entries[eff.ID]
	if entry == nil || len(entry.cells) == 0 {
		t.Fatalf("expected entry to track occupied cells")
	}

	eff.X = DefaultSpatialCellSize * 1.5
	eff.Y = DefaultSpatialCellSize * 0.5
	if !idx.Upsert(eff) {
		t.Fatalf("expected update to succeed")
	}
	updated := idx.entries[eff.ID]
	if updated == nil || len(updated.cells) == 0 {
		t.Fatalf("expected updated cells to be recorded")
	}
	if len(idx.cells) == 0 {
		t.Fatalf("expected index to retain cell occupancy after update")
	}

	idx.Remove(eff.ID)
	if idx.entries[eff.ID] != nil {
		t.Fatalf("expected entry to be removed")
	}
}

func TestSpatialIndexCapacity(t *testing.T) {
	idx := NewSpatialIndex(DefaultSpatialCellSize, 1)
	first := &State{ID: "first", X: 5, Y: 5, Width: 10, Height: 10}
	if !idx.Upsert(first) {
		t.Fatalf("expected first insert to succeed")
	}
	second := &State{ID: "second", X: 6, Y: 6, Width: 8, Height: 8}
	if idx.Upsert(second) {
		t.Fatalf("expected second insert to fail due to capacity")
	}
}
