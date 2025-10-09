package main

import "testing"

func TestSetPositionNoChange(t *testing.T) {
	w := &World{
		players: make(map[string]*playerState),
		journal: newJournal(defaultJournalKeyframeCapacity),
	}

	player := &playerState{actorState: actorState{Actor: Actor{
		ID: "player-1",
		X:  48,
		Y:  96,
	}}}
	w.players[player.ID] = player

	w.SetPosition(player.ID, player.X, player.Y)

	if player.version != 0 {
		t.Fatalf("expected version to remain 0, got %d", player.version)
	}

	if patches := w.journal.DrainPatches(); len(patches) != 0 {
		t.Fatalf("expected no patches, got %d", len(patches))
	}
}

func TestSetPositionEmitsPatch(t *testing.T) {
	w := &World{
		players: make(map[string]*playerState),
		journal: newJournal(defaultJournalKeyframeCapacity),
	}

	player := &playerState{actorState: actorState{Actor: Actor{
		ID: "player-1",
		X:  12,
		Y:  24,
	}}}
	w.players[player.ID] = player

	w.SetPosition(player.ID, 60, 72)

	if player.X != 60 || player.Y != 72 {
		t.Fatalf("expected position to update to (60, 72), got (%.2f, %.2f)", player.X, player.Y)
	}
	if player.version != 1 {
		t.Fatalf("expected version to increment to 1, got %d", player.version)
	}

	patches := w.journal.DrainPatches()
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}

	patch := patches[0]
	if patch.Kind != PatchPlayerPos {
		t.Fatalf("expected patch kind %q, got %q", PatchPlayerPos, patch.Kind)
	}
	if patch.EntityID != player.ID {
		t.Fatalf("expected entity %q, got %q", player.ID, patch.EntityID)
	}

	payload, ok := patch.Payload.(PlayerPosPayload)
	if !ok {
		t.Fatalf("expected payload type PlayerPosPayload, got %T", patch.Payload)
	}
	if payload.X != 60 || payload.Y != 72 {
		t.Fatalf("expected payload (60, 72), got (%.2f, %.2f)", payload.X, payload.Y)
	}

	w.SetPosition(player.ID, 60, 72)
	if player.version != 1 {
		t.Fatalf("expected version to remain 1 after no-op, got %d", player.version)
	}
	if len(w.journal.DrainPatches()) != 0 {
		t.Fatalf("expected no patches after no-op call")
	}
}
