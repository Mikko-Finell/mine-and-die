package main

import (
	"testing"

	"mine-and-die/server/internal/sim"
)

func TestEnqueueCommandEnforcesPerActorLimit(t *testing.T) {
	hub := newHub()

	actorA := "player-a"
	actorB := "player-b"

	for i := 0; i < commandQueuePerActorLimit; i++ {
		hub.enqueueCommand(sim.Command{ActorID: actorA, Type: sim.CommandMove, Move: &sim.MoveCommand{DX: 1}})
	}

	hub.enqueueCommand(sim.Command{ActorID: actorB, Type: sim.CommandMove, Move: &sim.MoveCommand{DY: 1}})

	hub.enqueueCommand(sim.Command{ActorID: actorA, Type: sim.CommandMove, Move: &sim.MoveCommand{DX: -1}})

	commands := hub.drainCommands()

	var countA, countB int
	for _, cmd := range commands {
		switch cmd.ActorID {
		case actorA:
			countA++
		case actorB:
			countB++
		default:
			t.Fatalf("unexpected actor %q in queue", cmd.ActorID)
		}
	}

	if countA != commandQueuePerActorLimit {
		t.Fatalf("expected %d commands for actor %s, got %d", commandQueuePerActorLimit, actorA, countA)
	}
	if countB != 1 {
		t.Fatalf("expected 1 command for actor %s, got %d", actorB, countB)
	}
	if len(commands) != commandQueuePerActorLimit+1 {
		t.Fatalf("expected %d total commands, got %d", commandQueuePerActorLimit+1, len(commands))
	}

	snapshot := hub.TelemetrySnapshot()
	dropsByReason := snapshot.CommandDrops["limit_exceeded"]
	if dropsByReason == nil {
		t.Fatalf("expected command drops to be recorded")
	}
	if dropsByReason[string(sim.CommandMove)] != 1 {
		t.Fatalf("expected 1 drop recorded for move commands, got %d", dropsByReason[string(sim.CommandMove)])
	}
}
