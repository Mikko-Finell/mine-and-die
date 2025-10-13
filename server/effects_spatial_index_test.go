package main

import "testing"

func TestEffectSpatialIndexInsertAndRemove(t *testing.T) {
	idx := newEffectSpatialIndex(effectSpatialCellSize, 4)
	eff := &effectState{Effect: Effect{ID: "effect-1", X: 10, Y: 10, Width: 20, Height: 20}}
	if !idx.Upsert(eff) {
		t.Fatalf("expected initial insert to succeed")
	}
	entry := idx.entries[eff.ID]
	if entry == nil || len(entry.cells) == 0 {
		t.Fatalf("expected entry to track occupied cells")
	}

	eff.Effect.X = effectSpatialCellSize * 1.5
	eff.Effect.Y = effectSpatialCellSize * 0.5
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

func TestEffectSpatialIndexCapacity(t *testing.T) {
	idx := newEffectSpatialIndex(effectSpatialCellSize, 1)
	first := &effectState{Effect: Effect{ID: "first", X: 5, Y: 5, Width: 10, Height: 10}}
	if !idx.Upsert(first) {
		t.Fatalf("expected first insert to succeed")
	}
	second := &effectState{Effect: Effect{ID: "second", X: 6, Y: 6, Width: 8, Height: 8}}
	if idx.Upsert(second) {
		t.Fatalf("expected second insert to fail due to capacity")
	}
}
