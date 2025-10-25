package world

import (
	"math"
	"math/rand"
	"testing"

	state "mine-and-die/server/internal/state"
)

func TestNewNormalizesConfigAndSeedsRNG(t *testing.T) {
	w, err := New(Config{}, Deps{})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if w == nil {
		t.Fatalf("New returned nil world")
	}

	normalized := (Config{}).normalized()
	if got := w.Config(); got != normalized {
		t.Fatalf("Config not normalized: got %+v want %+v", got, normalized)
	}

	if got := w.Seed(); got != normalized.Seed {
		t.Fatalf("Seed mismatch: got %q want %q", got, normalized.Seed)
	}

	rng := w.RNG()
	if rng == nil {
		t.Fatalf("RNG not initialized")
	}

	expected := NewDeterministicRNG(normalized.Seed, "world")
	if diff := math.Abs(rng.Float64() - expected.Float64()); diff > 1e-9 {
		t.Fatalf("world RNG not seeded deterministically: diff=%f", diff)
	}

	sub := w.SubsystemRNG("test")
	wantSub := NewDeterministicRNG(normalized.Seed, "test")
	if diff := math.Abs(sub.Float64() - wantSub.Float64()); diff > 1e-9 {
		t.Fatalf("subsystem RNG mismatch: diff=%f", diff)
	}
}

func TestNewUsesInjectedRNGFactory(t *testing.T) {
	calls := 0
	factory := func(rootSeed, label string) *rand.Rand {
		calls++
		return rand.New(rand.NewSource(123))
	}

	w, err := New(Config{Seed: "custom"}, Deps{RNG: factory})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if calls != 1 {
		t.Fatalf("expected factory to be invoked once for world RNG, got %d", calls)
	}

	_ = w.RNG()
	_ = w.SubsystemRNG("other")

	if calls < 2 {
		t.Fatalf("expected factory to be reused for subsystem RNG, got %d calls", calls)
	}
}

func TestNewInitializesPlayerAndNPCState(t *testing.T) {
	w, err := New(Config{}, Deps{})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if w.players == nil {
		t.Fatalf("players map not initialized")
	}
	if len(w.players) != 0 {
		t.Fatalf("expected no players, got %d", len(w.players))
	}

	if w.npcs == nil {
		t.Fatalf("npcs map not initialized")
	}
	if len(w.npcs) != 0 {
		t.Fatalf("expected no npcs, got %d", len(w.npcs))
	}

	candidate := &state.PlayerState{}
	w.players["player-1"] = candidate
	if w.players["player-1"] != candidate {
		t.Fatalf("players map should store PlayerState values")
	}

	npcCandidate := &state.NPCState{}
	w.npcs["npc-1"] = npcCandidate
	if w.npcs["npc-1"] != npcCandidate {
		t.Fatalf("npcs map should store NPCState values")
	}
}
