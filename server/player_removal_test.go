package main

import (
	"testing"

	"mine-and-die/server/logging"
)

func TestRemovePlayerEmitsRemovalPatch(t *testing.T) {
	world := newWorld(defaultWorldConfig(), logging.NopPublisher{})

	player := newTestPlayerState("player-remove")
	world.AddPlayer(player)
	world.drainPatchesLocked()

	if removed := world.RemovePlayer(player.ID); !removed {
		t.Fatalf("expected player %s to be removed", player.ID)
	}

	patches := world.drainPatchesLocked()
	if len(patches) != 1 {
		t.Fatalf("expected removal patch, got %d entries", len(patches))
	}

	patch := patches[0]
	if patch.Kind != PatchPlayerRemoved {
		t.Fatalf("expected kind %q, got %q", PatchPlayerRemoved, patch.Kind)
	}
	if patch.EntityID != player.ID {
		t.Fatalf("expected entity id %q, got %q", player.ID, patch.EntityID)
	}
}
