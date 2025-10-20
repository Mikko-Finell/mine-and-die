package sim

import "testing"

func TestCommandBufferWraparound(t *testing.T) {
	buffer := NewCommandBuffer(3, nil)
	cmds := []Command{
		{ActorID: "a"},
		{ActorID: "b"},
		{ActorID: "c"},
	}
	for _, cmd := range cmds {
		if !buffer.Push(cmd) {
			t.Fatalf("expected push to succeed for %+v", cmd)
		}
	}
	if buffer.Push(Command{ActorID: "overflow"}) {
		t.Fatalf("expected push to fail when buffer full")
	}
	drained := buffer.Drain()
	if len(drained) != len(cmds) {
		t.Fatalf("expected %d commands, got %d", len(cmds), len(drained))
	}
	for i, cmd := range drained {
		if cmd.ActorID != cmds[i].ActorID {
			t.Fatalf("expected drain order %v, got %v", cmds[i].ActorID, cmd.ActorID)
		}
	}
	// Push again to ensure the indices wrap correctly.
	for _, cmd := range []Command{{ActorID: "d"}, {ActorID: "e"}} {
		if !buffer.Push(cmd) {
			t.Fatalf("expected push to succeed after drain for %+v", cmd)
		}
	}
	wrapped := buffer.Drain()
	if len(wrapped) != 2 {
		t.Fatalf("expected 2 commands after wraparound, got %d", len(wrapped))
	}
	if wrapped[0].ActorID != "d" || wrapped[1].ActorID != "e" {
		t.Fatalf("unexpected order after wraparound: %+v", wrapped)
	}
}

func TestCommandBufferOverflow(t *testing.T) {
	buffer := NewCommandBuffer(1, nil)
	if !buffer.Push(Command{ActorID: "one"}) {
		t.Fatalf("expected initial push to succeed")
	}
	if buffer.Push(Command{ActorID: "two"}) {
		t.Fatalf("expected push to fail when capacity exceeded")
	}
	drained := buffer.Drain()
	if len(drained) != 1 || drained[0].ActorID != "one" {
		t.Fatalf("unexpected drained commands: %+v", drained)
	}
}
