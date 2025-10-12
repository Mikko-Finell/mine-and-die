package main

import "testing"

func TestResyncPolicySchedulesOnLostSpawnRatio(t *testing.T) {
	policy := newResyncPolicy()
	for i := 0; i < 20000; i++ {
		policy.noteEvent()
	}
	policy.noteLostSpawn("unknown", "effect-1")
	if signal, ok := policy.consume(); ok {
		t.Fatalf("unexpected pending signal before threshold, got %+v", signal)
	}

	policy.noteLostSpawn("unknown", "effect-1")
	signal, ok := policy.consume()
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
	policy := newResyncPolicy()
	policy.noteEvent()
	policy.noteLostSpawn("unknown", "effect-2")
	if _, ok := policy.consume(); !ok {
		t.Fatalf("expected resync signal after lost spawn")
	}
	if signal, ok := policy.consume(); ok {
		t.Fatalf("expected no signal after reset, got %+v", signal)
	}
	policy.noteEvent()
	policy.noteEvent()
	policy.noteLostSpawn("unknown", "effect-3")
	if _, ok := policy.consume(); !ok {
		t.Fatalf("expected policy to trigger again after reset")
	}
}
