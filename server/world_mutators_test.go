package main

import (
	"testing"

	"mine-and-die/server/logging"
)

func TestSetPositionNoopDoesNotEmitPatch(t *testing.T) {
	w := newWorld(defaultWorldConfig(), logging.NopPublisher{})
	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-1", X: 10, Y: 20}}}
	w.AddPlayer(player)

	w.SetPosition("player-1", 10, 20)

	if player.version != 0 {
		t.Fatalf("expected version to remain 0, got %d", player.version)
	}

	if patches := w.snapshotPatchesLocked(); len(patches) != 0 {
		t.Fatalf("expected no patches, got %d", len(patches))
	}
}

func TestSetPositionRecordsPatch(t *testing.T) {
	w := newWorld(defaultWorldConfig(), logging.NopPublisher{})
	player := &playerState{actorState: actorState{Actor: Actor{ID: "player-2", X: 5, Y: 6}}}
	w.AddPlayer(player)

	w.SetPosition("player-2", 15, 25)

	if player.version != 1 {
		t.Fatalf("expected version to increment to 1, got %d", player.version)
	}

	patches := w.snapshotPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}

	patch := patches[0]
	if patch.Kind != PatchPlayerPos {
		t.Fatalf("expected patch kind %q, got %q", PatchPlayerPos, patch.Kind)
	}
	if patch.EntityID != "player-2" {
		t.Fatalf("expected entity id player-2, got %q", patch.EntityID)
	}

	payload, ok := patch.Payload.(PlayerPosPayload)
	if !ok {
		t.Fatalf("expected payload to be PlayerPosPayload, got %T", patch.Payload)
	}
	if payload.X != 15 || payload.Y != 25 {
		t.Fatalf("expected payload coords (15,25), got (%.2f, %.2f)", payload.X, payload.Y)
	}

	w.SetPosition("player-2", 30, 35)
	if player.version != 2 {
		t.Fatalf("expected version to increment to 2, got %d", player.version)
	}
}
