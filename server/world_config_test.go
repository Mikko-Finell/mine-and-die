package server

import "testing"

func TestWorldConfigNormalizedPreservesAggregateNPCCount(t *testing.T) {
	cfg := worldConfig{
		NPCs:        true,
		NPCCount:    7,
		GoblinCount: 0,
		RatCount:    0,
	}

	normalized := cfg.normalized()

	if normalized.NPCCount != 7 {
		t.Fatalf("expected NPCCount to remain 7, got %d", normalized.NPCCount)
	}
}

func TestWorldConfigNormalizedTotalsSpeciesCountsWhenProvided(t *testing.T) {
	cfg := worldConfig{
		NPCs:        true,
		NPCCount:    99,
		GoblinCount: 3,
		RatCount:    4,
	}

	normalized := cfg.normalized()

	if normalized.NPCCount != 7 {
		t.Fatalf("expected NPCCount to match species sum (7), got %d", normalized.NPCCount)
	}
}
