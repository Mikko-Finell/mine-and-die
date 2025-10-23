package ai

import "testing"

func TestBootstrapNPC_UsesLibraryDefaults(t *testing.T) {
	cfg := GlobalLibrary.ConfigForType("goblin")
	if cfg == nil {
		t.Fatalf("missing goblin config")
	}

	var bb Blackboard
	bb.WaypointIndex = 5

	var configID uint16
	var state uint8

	BootstrapNPC(SpawnBootstrapConfig{
		Library:       GlobalLibrary,
		Type:          "goblin",
		ConfigID:      &configID,
		State:         &state,
		Blackboard:    &bb,
		WaypointCount: 4,
	})

	if configID != cfg.ID() {
		t.Fatalf("expected config ID %d, got %d", cfg.ID(), configID)
	}
	if state != cfg.InitialState() {
		t.Fatalf("expected initial state %d, got %d", cfg.InitialState(), state)
	}
	if bb.ArriveRadius != 16 {
		t.Fatalf("expected arrive radius 16, got %f", bb.ArriveRadius)
	}
	if bb.PauseTicks != 30 {
		t.Fatalf("expected pause ticks 30, got %d", bb.PauseTicks)
	}
	if bb.StuckEpsilon != 0.5 {
		t.Fatalf("expected stuck epsilon 0.5, got %f", bb.StuckEpsilon)
	}
	if bb.WaypointIndex != 0 {
		t.Fatalf("expected waypoint index 0, got %d", bb.WaypointIndex)
	}
	if bb.LastWaypointIndex != -1 {
		t.Fatalf("expected last waypoint index -1, got %d", bb.LastWaypointIndex)
	}
	if bb.NextDecisionAt != 0 {
		t.Fatalf("expected next decision at 0, got %d", bb.NextDecisionAt)
	}
}

func TestBootstrapNPC_WithoutLibraryUsesFallbacks(t *testing.T) {
	bb := Blackboard{WaypointIndex: -1}
	var configID uint16 = 123
	var state uint8 = 42

	BootstrapNPC(SpawnBootstrapConfig{
		Library:       nil,
		Type:          "goblin",
		ConfigID:      &configID,
		State:         &state,
		Blackboard:    &bb,
		WaypointCount: 3,
	})

	if configID != 123 {
		t.Fatalf("expected config ID unchanged, got %d", configID)
	}
	if state != 42 {
		t.Fatalf("expected state unchanged, got %d", state)
	}
	if bb.ArriveRadius != 16 {
		t.Fatalf("expected fallback arrive radius 16, got %f", bb.ArriveRadius)
	}
	if bb.PauseTicks != 30 {
		t.Fatalf("expected fallback pause ticks 30, got %d", bb.PauseTicks)
	}
	if bb.StuckEpsilon != 0.5 {
		t.Fatalf("expected fallback stuck epsilon 0.5, got %f", bb.StuckEpsilon)
	}
	if bb.WaypointIndex != 0 {
		t.Fatalf("expected waypoint index clamped to 0, got %d", bb.WaypointIndex)
	}
	if bb.LastWaypointIndex != -1 {
		t.Fatalf("expected last waypoint index -1, got %d", bb.LastWaypointIndex)
	}
	if bb.NextDecisionAt != 0 {
		t.Fatalf("expected next decision at 0, got %d", bb.NextDecisionAt)
	}
}

func TestBootstrapNPC_RatUsesConfigDefaults(t *testing.T) {
	cfg := GlobalLibrary.ConfigForType("rat")
	if cfg == nil {
		t.Fatalf("missing rat config")
	}

	bb := Blackboard{WaypointIndex: 7}

	var configID uint16
	var state uint8

	BootstrapNPC(SpawnBootstrapConfig{
		Library:       GlobalLibrary,
		Type:          "rat",
		ConfigID:      &configID,
		State:         &state,
		Blackboard:    &bb,
		WaypointCount: 0,
	})

	if configID != cfg.ID() {
		t.Fatalf("expected config ID %d, got %d", cfg.ID(), configID)
	}
	if state != cfg.InitialState() {
		t.Fatalf("expected initial state %d, got %d", cfg.InitialState(), state)
	}
	if bb.ArriveRadius != 10 {
		t.Fatalf("expected arrive radius 10, got %f", bb.ArriveRadius)
	}
	if bb.PauseTicks != 20 {
		t.Fatalf("expected pause ticks 20, got %d", bb.PauseTicks)
	}
	if bb.StuckEpsilon != 0.4 {
		t.Fatalf("expected stuck epsilon 0.4, got %f", bb.StuckEpsilon)
	}
	if bb.WaypointIndex != 0 {
		t.Fatalf("expected waypoint index forced to 0, got %d", bb.WaypointIndex)
	}
	if bb.LastWaypointIndex != -1 {
		t.Fatalf("expected last waypoint index -1, got %d", bb.LastWaypointIndex)
	}
	if bb.NextDecisionAt != 0 {
		t.Fatalf("expected next decision at 0, got %d", bb.NextDecisionAt)
	}
}
