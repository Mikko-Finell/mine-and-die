package runtime

import (
	"testing"
	"time"
)

func TestRegisterEffectAllocatesStorage(t *testing.T) {
	var effects []*State
	var byID map[string]*State
	reg := Registry{Effects: &effects, ByID: &byID}

	eff := &State{ID: "effect-1", Type: "spark"}
	if !RegisterEffect(reg, eff) {
		t.Fatalf("RegisterEffect returned false")
	}
	if len(effects) != 1 || effects[0] != eff {
		t.Fatalf("expected effect appended, got %#v", effects)
	}
	if byID == nil {
		t.Fatalf("expected map allocation")
	}
	if got := byID[eff.ID]; got != eff {
		t.Fatalf("expected map entry for %q", eff.ID)
	}
}

func TestRegisterEffectRejectsSpatialOverflow(t *testing.T) {
	idx := NewSpatialIndex(DefaultSpatialCellSize, 1)
	effectsSlice := make([]*State, 0)
	byID := make(map[string]*State)
	var overflow []string
	reg := Registry{
		Effects: &effectsSlice,
		ByID:    &byID,
		Index:   idx,
		RecordSpatialOverflow: func(effectType string) {
			overflow = append(overflow, effectType)
		},
	}

	first := &State{ID: "eff-1", Type: "alpha", Width: 40, Height: 40}
	if !RegisterEffect(reg, first) {
		t.Fatalf("expected first registration to succeed")
	}
	second := &State{ID: "eff-2", Type: "beta", Width: 40, Height: 40}
	if RegisterEffect(reg, second) {
		t.Fatalf("expected overflow to fail registration")
	}
	if len(effectsSlice) != 1 {
		t.Fatalf("expected slice to remain unchanged, got %d", len(effectsSlice))
	}
	if _, exists := byID[second.ID]; exists {
		t.Fatalf("unexpected map entry for rejected effect")
	}
	if len(overflow) != 1 || overflow[0] != second.Type {
		t.Fatalf("expected overflow callback for %q, got %#v", second.Type, overflow)
	}
}

func TestUnregisterEffectRemovesIndexEntry(t *testing.T) {
	idx := NewSpatialIndex(DefaultSpatialCellSize, 1)
	effectsSlice := make([]*State, 0)
	byID := make(map[string]*State)
	reg := Registry{Effects: &effectsSlice, ByID: &byID, Index: idx}

	eff := &State{ID: "eff-1", Type: "spark", Width: 40, Height: 40}
	if !RegisterEffect(reg, eff) {
		t.Fatalf("expected registration to succeed")
	}
	UnregisterEffect(reg, eff)
	if _, exists := byID[eff.ID]; exists {
		t.Fatalf("expected map entry cleared")
	}
	if !RegisterEffect(reg, eff) {
		t.Fatalf("expected registration to succeed after unregister")
	}
}

func TestFindByIDReturnsRegisteredEffect(t *testing.T) {
	effectsSlice := make([]*State, 0)
	byID := make(map[string]*State)
	reg := Registry{Effects: &effectsSlice, ByID: &byID}

	eff := &State{ID: "eff-1", Type: "spark"}
	if !RegisterEffect(reg, eff) {
		t.Fatalf("expected registration to succeed")
	}
	if got := FindByID(reg, eff.ID); got != eff {
		t.Fatalf("expected lookup to return registered effect, got %#v", got)
	}
	if missing := FindByID(reg, "missing"); missing != nil {
		t.Fatalf("expected missing lookup to return nil, got %#v", missing)
	}
}

func TestPruneExpiredRemovesExpiredEffects(t *testing.T) {
	idx := NewSpatialIndex(DefaultSpatialCellSize, 4)
	effectsSlice := make([]*State, 0, 2)
	byID := make(map[string]*State)
	reg := Registry{Effects: &effectsSlice, ByID: &byID, Index: idx}

	now := time.UnixMilli(1_700_000_000_000)
	active := &State{ID: "keep", Type: "alpha", Width: 40, Height: 40, ExpiresAt: now.Add(time.Second)}
	expired := &State{ID: "drop", Type: "beta", Width: 40, Height: 40, ExpiresAt: now.Add(-time.Second)}

	if !RegisterEffect(reg, active) {
		t.Fatalf("expected active registration to succeed")
	}
	if !RegisterEffect(reg, expired) {
		t.Fatalf("expected expired registration to succeed")
	}

	removed := PruneExpired(reg, now)
	if len(removed) != 1 || removed[0] != expired {
		t.Fatalf("expected expired effect returned, got %#v", removed)
	}
	if len(effectsSlice) != 1 || effectsSlice[0] != active {
		t.Fatalf("expected only active effect retained, got %#v", effectsSlice)
	}
	if _, exists := byID[expired.ID]; exists {
		t.Fatalf("expected expired effect removed from map")
	}
	if _, exists := idx.entries[expired.ID]; exists {
		t.Fatalf("expected expired effect removed from spatial index")
	}
}

func TestPruneExpiredKeepsAllActiveEffects(t *testing.T) {
	effectsSlice := make([]*State, 0, 1)
	byID := make(map[string]*State)
	reg := Registry{Effects: &effectsSlice, ByID: &byID}

	now := time.UnixMilli(1_700_000_000_000)
	eff := &State{ID: "keep", Type: "spark", ExpiresAt: now.Add(time.Second)}

	if !RegisterEffect(reg, eff) {
		t.Fatalf("expected registration to succeed")
	}

	removed := PruneExpired(reg, now)
	if len(removed) != 0 {
		t.Fatalf("expected no expired effects, got %#v", removed)
	}
	if len(effectsSlice) != 1 || effectsSlice[0] != eff {
		t.Fatalf("expected effect retained, got %#v", effectsSlice)
	}
	if byID[eff.ID] != eff {
		t.Fatalf("expected map entry retained")
	}
}
