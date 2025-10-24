package proto

import (
	"encoding/json"
	"testing"

	itemspkg "mine-and-die/server/internal/items"
	"mine-and-die/server/internal/sim"
	simpatches "mine-and-die/server/internal/sim/patches/typed"
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

func TestEncodeStateSnapshotV1SetsVersionAndType(t *testing.T) {
	snapshot := StateSnapshotV1{
		Type: TypeState,
		Players: []sim.Player{{
			Actor: sim.Actor{ID: "player-1"},
		}},
		NPCs: []sim.NPC{{
			Actor: sim.Actor{ID: "npc-1"},
		}},
		Obstacles: []sim.Obstacle{{
			ID:     "rock",
			Width:  2,
			Height: 3,
		}},
		GroundItems: []itemspkg.GroundItem{{
			ID:   "ground-1",
			Type: "gold",
			Qty:  5,
		}},
		Patches: []simpatches.Patch{{
			Kind:     simpatches.PatchPlayerPos,
			EntityID: "player-1",
			Payload: simpatches.PlayerPosPayload{
				X: 10,
				Y: -5,
			},
		}},
		Tick:        42,
		Sequence:    7,
		KeyframeSeq: 3,
		ServerTime:  1234,
		Config: sim.WorldConfig{
			Seed:  "abc",
			Width: 128,
		},
	}

	encoded, err := EncodeStateSnapshotV1(snapshot)
	if err != nil {
		t.Fatalf("encode state snapshot v1: %v", err)
	}

	if snapshot.Ver != 0 {
		t.Fatalf("expected encode to operate on a copy, got version %d", snapshot.Ver)
	}

	var decoded struct {
		Ver      int         `json:"ver"`
		Type     string      `json:"type"`
		Sequence uint64      `json:"sequence"`
		Tick     uint64      `json:"t"`
		Patches  []sim.Patch `json:"patches"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal encoded snapshot: %v", err)
	}
	if decoded.Ver != Version {
		t.Fatalf("expected version %d, got %d", Version, decoded.Ver)
	}
	if decoded.Type != TypeState {
		t.Fatalf("expected type %q, got %q", TypeState, decoded.Type)
	}
	if decoded.Sequence != snapshot.Sequence {
		t.Fatalf("expected sequence %d, got %d", snapshot.Sequence, decoded.Sequence)
	}
	if decoded.Tick != snapshot.Tick {
		t.Fatalf("expected tick %d, got %d", snapshot.Tick, decoded.Tick)
	}
	if len(decoded.Patches) != 1 {
		t.Fatalf("expected one patch, got %d", len(decoded.Patches))
	}
	if decoded.Patches[0].Kind != sim.PatchPlayerPos {
		t.Fatalf("expected patch kind %q, got %q", sim.PatchPlayerPos, decoded.Patches[0].Kind)
	}

	viaInterface, err := EncodeStateSnapshot(&snapshot)
	if err != nil {
		t.Fatalf("encode state snapshot via interface: %v", err)
	}
	if string(viaInterface) != string(encoded) {
		t.Fatalf("expected interface encoder to match direct encoding\nwant: %s\ngot:  %s", string(encoded), string(viaInterface))
	}
}

func TestEncodeJoinResponseV1SetsVersion(t *testing.T) {
	resp := JoinResponseV1{
		ID: "player-1",
		Players: []sim.Player{{
			Actor: sim.Actor{ID: "player-1"},
		}},
		NPCs: []sim.NPC{{
			Actor: sim.Actor{ID: "npc-1"},
		}},
		Obstacles: []sim.Obstacle{{
			ID: "rock",
		}},
		Config: sim.WorldConfig{
			Seed: "seed",
		},
		Resync:            true,
		EffectCatalogHash: "catalog",
	}

	encoded, err := EncodeJoinResponseV1(resp)
	if err != nil {
		t.Fatalf("encode join response v1: %v", err)
	}

	var decoded struct {
		Ver int `json:"ver"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal join response: %v", err)
	}
	if decoded.Ver != Version {
		t.Fatalf("expected version %d, got %d", Version, decoded.Ver)
	}

	viaInterface, err := EncodeJoinResponse(&resp)
	if err != nil {
		t.Fatalf("encode join response via interface: %v", err)
	}
	if string(viaInterface) != string(encoded) {
		t.Fatalf("expected interface encoder to match direct encoding\nwant: %s\ngot:  %s", string(encoded), string(viaInterface))
	}
}

func TestEncodeKeyframeSnapshotV1SetsVersionAndType(t *testing.T) {
	frame := KeyframeSnapshotV1{
		Type:     TypeKeyframe,
		Sequence: 9,
		Tick:     99,
		Players: []sim.Player{{
			Actor: sim.Actor{ID: "p1"},
		}},
		NPCs: []sim.NPC{{
			Actor: sim.Actor{ID: "n1"},
		}},
		Obstacles: []sim.Obstacle{{
			ID: "wall",
		}},
		GroundItems: []itemspkg.GroundItem{{
			ID:   "g1",
			Type: "gold",
			Qty:  1,
		}},
		Config: sim.WorldConfig{Seed: "seed"},
	}

	encoded, err := EncodeKeyframeSnapshotV1(frame)
	if err != nil {
		t.Fatalf("encode keyframe snapshot v1: %v", err)
	}

	var decoded struct {
		Ver  int    `json:"ver"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal keyframe snapshot: %v", err)
	}
	if decoded.Ver != Version {
		t.Fatalf("expected version %d, got %d", Version, decoded.Ver)
	}
	if decoded.Type != TypeKeyframe {
		t.Fatalf("expected type %q, got %q", TypeKeyframe, decoded.Type)
	}

	viaInterface, err := EncodeKeyframeSnapshot(&frame)
	if err != nil {
		t.Fatalf("encode keyframe snapshot via interface: %v", err)
	}
	if string(viaInterface) != string(encoded) {
		t.Fatalf("expected interface encoder to match direct encoding\nwant: %s\ngot:  %s", string(encoded), string(viaInterface))
	}
}
