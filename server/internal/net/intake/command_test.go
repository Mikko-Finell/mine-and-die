package intake

import (
	"testing"
	"time"

	"mine-and-die/server"
	"mine-and-die/server/internal/net/proto"
	"mine-and-die/server/internal/sim"
)

type fakeEngine struct {
	enqueueOK     bool
	enqueueReason string
	commands      []sim.Command
}

func (f *fakeEngine) Deps() sim.Deps                             { return sim.Deps{} }
func (f *fakeEngine) Apply([]sim.Command) error                  { return nil }
func (f *fakeEngine) Step()                                      {}
func (f *fakeEngine) Snapshot() sim.Snapshot                     { return sim.Snapshot{} }
func (f *fakeEngine) DrainPatches() []sim.Patch                  { return nil }
func (f *fakeEngine) SnapshotPatches() []sim.Patch               { return nil }
func (f *fakeEngine) RestorePatches([]sim.Patch)                 {}
func (f *fakeEngine) DrainEffectEvents() sim.EffectEventBatch    { return sim.EffectEventBatch{} }
func (f *fakeEngine) SnapshotEffectEvents() sim.EffectEventBatch { return sim.EffectEventBatch{} }
func (f *fakeEngine) RestoreEffectEvents(sim.EffectEventBatch)   {}
func (f *fakeEngine) ConsumeEffectResyncHint() (sim.EffectResyncSignal, bool) {
	return sim.EffectResyncSignal{}, false
}
func (f *fakeEngine) RecordKeyframe(sim.Keyframe) sim.KeyframeRecordResult {
	return sim.KeyframeRecordResult{}
}
func (f *fakeEngine) KeyframeBySequence(uint64) (sim.Keyframe, bool) { return sim.Keyframe{}, false }
func (f *fakeEngine) KeyframeWindow() (int, uint64, uint64)          { return 0, 0, 0 }
func (f *fakeEngine) Enqueue(cmd sim.Command) (bool, string) {
	f.commands = append(f.commands, cmd)
	if f.enqueueOK {
		return true, ""
	}
	if f.enqueueReason == "" {
		f.enqueueReason = sim.CommandRejectQueueLimit
	}
	return false, f.enqueueReason
}
func (f *fakeEngine) Pending() int                 { return len(f.commands) }
func (f *fakeEngine) DrainCommands() []sim.Command { return nil }
func (f *fakeEngine) Advance(sim.LoopTickContext) sim.LoopStepResult {
	return sim.LoopStepResult{}
}
func (f *fakeEngine) Run(<-chan struct{}) {}

func TestStageClientCommandAcceptsMove(t *testing.T) {
	engine := &fakeEngine{enqueueOK: true}
	issuedAt := time.Unix(100, 0)
	ctx := CommandContext{
		Engine:    engine,
		HasPlayer: func(id string) bool { return id == "player-1" },
		Tick:      func() uint64 { return 42 },
		Now:       func() time.Time { return issuedAt },
	}

	msg := proto.ClientMessage{Type: proto.TypeInput, DX: 1, DY: 0}
	cmd, ok, reason := StageClientCommand(ctx, "player-1", msg)
	if !ok {
		t.Fatalf("expected command to be accepted, got reason %q", reason)
	}
	if cmd.ActorID != "player-1" {
		t.Fatalf("expected ActorID to be set, got %q", cmd.ActorID)
	}
	if cmd.OriginTick != 42 {
		t.Fatalf("expected OriginTick to be 42, got %d", cmd.OriginTick)
	}
	if !cmd.IssuedAt.Equal(issuedAt) {
		t.Fatalf("expected IssuedAt %v, got %v", issuedAt, cmd.IssuedAt)
	}
	if len(engine.commands) != 1 {
		t.Fatalf("expected engine to record command, got %d", len(engine.commands))
	}
}

func TestStageClientCommandRejectsUnknownPlayer(t *testing.T) {
	engine := &fakeEngine{enqueueOK: true}
	ctx := CommandContext{
		Engine:    engine,
		HasPlayer: func(string) bool { return false },
		Tick:      func() uint64 { return 1 },
		Now:       func() time.Time { return time.Unix(0, 0) },
	}

	msg := proto.ClientMessage{Type: proto.TypeInput, DX: 1, DY: 0}
	_, ok, reason := StageClientCommand(ctx, "missing", msg)
	if ok {
		t.Fatalf("expected rejection for missing player")
	}
	if reason != server.CommandRejectUnknownActor {
		t.Fatalf("expected reason %q, got %q", server.CommandRejectUnknownActor, reason)
	}
}

func TestStageClientCommandRejectsInvalidAction(t *testing.T) {
	engine := &fakeEngine{enqueueOK: true}
	ctx := CommandContext{
		Engine:    engine,
		HasPlayer: func(string) bool { return true },
		Tick:      func() uint64 { return 1 },
		Now:       func() time.Time { return time.Unix(0, 0) },
	}

	msg := proto.ClientMessage{Type: proto.TypeAction, Action: "invalid"}
	_, ok, reason := StageClientCommand(ctx, "player-1", msg)
	if ok {
		t.Fatalf("expected rejection for invalid action")
	}
	if reason != server.CommandRejectInvalidAction {
		t.Fatalf("expected reason %q, got %q", server.CommandRejectInvalidAction, reason)
	}
}

func TestStageClientCommandPropagatesEngineReason(t *testing.T) {
	engine := &fakeEngine{enqueueOK: false, enqueueReason: sim.CommandRejectQueueLimit}
	ctx := CommandContext{
		Engine:    engine,
		HasPlayer: func(string) bool { return true },
		Tick:      func() uint64 { return 1 },
		Now:       func() time.Time { return time.Unix(0, 0) },
	}

	msg := proto.ClientMessage{Type: proto.TypeInput, DX: 1, DY: 0}
	_, ok, reason := StageClientCommand(ctx, "player-1", msg)
	if ok {
		t.Fatalf("expected rejection from engine")
	}
	if reason != sim.CommandRejectQueueLimit {
		t.Fatalf("expected engine reason %q, got %q", sim.CommandRejectQueueLimit, reason)
	}
}

func TestStageClientCommandHandlesNilEngine(t *testing.T) {
	ctx := CommandContext{
		Engine:    nil,
		HasPlayer: func(string) bool { return true },
		Tick:      func() uint64 { return 1 },
		Now:       func() time.Time { return time.Unix(0, 0) },
	}

	msg := proto.ClientMessage{Type: proto.TypeInput, DX: 1, DY: 0}
	_, ok, reason := StageClientCommand(ctx, "player-1", msg)
	if ok {
		t.Fatalf("expected rejection when engine is nil")
	}
	if reason != sim.CommandRejectQueueFull {
		t.Fatalf("expected reason %q, got %q", sim.CommandRejectQueueFull, reason)
	}
}
