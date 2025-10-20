package proto

import (
	"testing"

	"mine-and-die/server/internal/sim"
)

func TestClientCommand(t *testing.T) {
	t.Run("move command", func(t *testing.T) {
		cmd, ok := ClientCommand(ClientMessage{
			Type:   TypeInput,
			DX:     1.5,
			DY:     -0.25,
			Facing: "left",
		})
		if !ok {
			t.Fatalf("expected move command to be recognized")
		}
		if cmd.Type != sim.CommandMove {
			t.Fatalf("expected move command type, got %q", cmd.Type)
		}
		if cmd.Move == nil {
			t.Fatalf("expected move payload")
		}
		if cmd.Move.DX != 1.5 || cmd.Move.DY != -0.25 {
			t.Fatalf("unexpected move vector: %+v", cmd.Move)
		}
		if cmd.Move.Facing != sim.FacingLeft {
			t.Fatalf("unexpected facing: %q", cmd.Move.Facing)
		}
	})

	t.Run("move command with invalid facing", func(t *testing.T) {
		cmd, ok := ClientCommand(ClientMessage{
			Type:   TypeInput,
			DX:     0.1,
			DY:     0.2,
			Facing: "bad",
		})
		if !ok {
			t.Fatalf("expected move command to be recognized")
		}
		if cmd.Move == nil {
			t.Fatalf("expected move payload")
		}
		if cmd.Move.Facing != "" {
			t.Fatalf("expected empty facing, got %q", cmd.Move.Facing)
		}
	})

	t.Run("path command", func(t *testing.T) {
		cmd, ok := ClientCommand(ClientMessage{
			Type: TypePath,
			X:    12.5,
			Y:    -4,
		})
		if !ok {
			t.Fatalf("expected path command to be recognized")
		}
		if cmd.Type != sim.CommandSetPath {
			t.Fatalf("expected set-path type, got %q", cmd.Type)
		}
		if cmd.Path == nil {
			t.Fatalf("expected path payload")
		}
		if cmd.Path.TargetX != 12.5 || cmd.Path.TargetY != -4 {
			t.Fatalf("unexpected path payload: %+v", cmd.Path)
		}
	})

	t.Run("cancel path command", func(t *testing.T) {
		cmd, ok := ClientCommand(ClientMessage{Type: TypeCancelPath})
		if !ok {
			t.Fatalf("expected cancel path command to be recognized")
		}
		if cmd.Type != sim.CommandClearPath {
			t.Fatalf("expected clear-path type, got %q", cmd.Type)
		}
		if cmd.Path != nil || cmd.Move != nil || cmd.Action != nil {
			t.Fatalf("expected no payloads, got %+v", cmd)
		}
	})

	t.Run("action command", func(t *testing.T) {
		cmd, ok := ClientCommand(ClientMessage{Type: TypeAction, Action: "attack"})
		if !ok {
			t.Fatalf("expected action command to be recognized")
		}
		if cmd.Type != sim.CommandAction {
			t.Fatalf("expected action type, got %q", cmd.Type)
		}
		if cmd.Action == nil || cmd.Action.Name != "attack" {
			t.Fatalf("unexpected action payload: %+v", cmd.Action)
		}
	})

	t.Run("action command requires name", func(t *testing.T) {
		if _, ok := ClientCommand(ClientMessage{Type: TypeAction}); ok {
			t.Fatalf("expected empty action to be rejected")
		}
	})

	t.Run("non command payload", func(t *testing.T) {
		if _, ok := ClientCommand(ClientMessage{Type: TypeHeartbeat}); ok {
			t.Fatalf("expected heartbeat to be ignored")
		}
	})
}
