package server

import (
	"testing"

	journal "mine-and-die/server/internal/journal"
)

func TestResyncPolicySchedulesOnLostSpawnRatio(t *testing.T) {
	policy := journal.NewPolicy()
	for i := 0; i < 20000; i++ {
		policy.NoteEvent()
	}
	policy.NoteLostSpawn("unknown", "effect-1")
	if signal, ok := policy.Consume(); ok {
		t.Fatalf("unexpected pending signal before threshold, got %+v", signal)
	}

	policy.NoteLostSpawn("unknown", "effect-1")
	signal, ok := policy.Consume()
	if !ok {
		t.Fatalf("expected resync hint after exceeding threshold")
	}
	if signal.LostSpawns != 2 {
		t.Fatalf("expected lost spawns 2, got %d", signal.LostSpawns)
	}
	if signal.TotalEvents != 20000 {
		t.Fatalf("expected total events 20000, got %d", signal.TotalEvents)
	}
}

func TestResyncPolicyResetAfterConsume(t *testing.T) {
	policy := journal.NewPolicy()
	policy.NoteEvent()
	policy.NoteLostSpawn("unknown", "effect-2")
	if _, ok := policy.Consume(); !ok {
		t.Fatalf("expected resync signal after lost spawn")
	}
	if signal, ok := policy.Consume(); ok {
		t.Fatalf("expected no signal after reset, got %+v", signal)
	}
	policy.NoteEvent()
	policy.NoteEvent()
	policy.NoteLostSpawn("unknown", "effect-3")
	if _, ok := policy.Consume(); !ok {
		t.Fatalf("expected policy to trigger again after reset")
	}
}
