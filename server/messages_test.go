package main

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

func TestStateMessage_ContainsTick(t *testing.T) {
	hub := newHub()
	hub.advance(time.Now(), 1.0/float64(tickRate))

	data, _, err := hub.marshalState(nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}

	tickValue, ok := payload["t"]
	if !ok {
		t.Fatalf("expected payload to include tick field")
	}

	tickNumber, ok := tickValue.(float64)
	if !ok {
		t.Fatalf("expected tick to decode as number, got %T", tickValue)
	}
	if tickNumber < 0 {
		t.Fatalf("expected non-negative tick, got %f", tickNumber)
	}
	if math.Mod(tickNumber, 1) != 0 {
		t.Fatalf("expected tick to be integral, got %f", tickNumber)
	}
}

func TestTickMonotonicity_AcrossBroadcasts(t *testing.T) {
	hub := newHub()
	dt := 1.0 / float64(tickRate)

	ticks := make([]uint64, 0, 3)
	for i := 0; i < 3; i++ {
		hub.advance(time.Now(), dt)

		data, _, err := hub.marshalState(nil, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("marshalState returned error: %v", err)
		}

		var payload map[string]any
		if err := json.Unmarshal(data, &payload); err != nil {
			t.Fatalf("failed to decode payload: %v", err)
		}

		value, ok := payload["t"]
		if !ok {
			t.Fatalf("payload missing tick field")
		}
		tickNumber, ok := value.(float64)
		if !ok {
			t.Fatalf("expected tick to decode as number, got %T", value)
		}
		if math.Mod(tickNumber, 1) != 0 {
			t.Fatalf("expected tick to be integral, got %f", tickNumber)
		}
		ticks = append(ticks, uint64(tickNumber))
	}

	if len(ticks) != 3 {
		t.Fatalf("expected to capture 3 ticks, got %d", len(ticks))
	}

	for i := 1; i < len(ticks); i++ {
		if ticks[i] != ticks[i-1]+1 {
			t.Fatalf("expected ticks to increase by 1, got %d then %d", ticks[i-1], ticks[i])
		}
	}
}

func TestStateMessageIncludesEmptyPatchesSlice(t *testing.T) {
	hub := newHub()
	hub.advance(time.Now(), 1.0/float64(tickRate))

	data, _, err := hub.marshalState(nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}

	rawPatches, ok := payload["patches"]
	if !ok {
		t.Fatalf("expected payload to include patches field")
	}

	arr, ok := rawPatches.([]any)
	if !ok {
		t.Fatalf("expected patches to decode as array, got %T", rawPatches)
	}
	if len(arr) != 0 {
		t.Fatalf("expected empty patches array, got %d entries", len(arr))
	}
}

func TestStateMessageWithPatchesRoundTrip(t *testing.T) {
	msg := stateMessage{
		Ver:            ProtocolVersion,
		Type:           "state",
		Players:        nil,
		NPCs:           nil,
		Obstacles:      nil,
		Effects:        nil,
		EffectTriggers: nil,
		GroundItems:    nil,
		Patches: []Patch{{
			Kind:     PatchPlayerPos,
			EntityID: "player-1",
			Payload: PlayerPosPayload{
				X: 10,
				Y: 20,
			},
		}},
		Tick:       1,
		ServerTime: time.Now().UnixMilli(),
		Config:     worldConfig{},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("failed to encode message: %v", err)
	}

	var decoded stateMessage
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to decode message: %v", err)
	}

	if len(decoded.Patches) != 1 {
		t.Fatalf("expected 1 patch after round trip, got %d", len(decoded.Patches))
	}
}

func TestMovementEmitsPlayerPosPatch(t *testing.T) {
	hub := newHub()

	player := &playerState{
		actorState: actorState{Actor: Actor{
			ID:        "player-1",
			X:         120,
			Y:         160,
			Facing:    defaultFacing,
			Health:    playerMaxHealth,
			MaxHealth: playerMaxHealth,
			Inventory: NewInventory(),
		}},
		lastHeartbeat: time.Now(),
		path:          playerPathState{ArriveRadius: defaultPlayerArriveRadius},
	}

	hub.mu.Lock()
	hub.world.AddPlayer(player)
	hub.mu.Unlock()

	hub.enqueueCommand(Command{
		ActorID: player.ID,
		Type:    CommandMove,
		Move: &MoveCommand{
			DX: 1,
			DY: 0,
		},
		IssuedAt: time.Now(),
	})

	hub.advance(time.Now(), 1.0/float64(tickRate))

	data, _, err := hub.marshalState(nil, nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	var msg stateMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}

	if len(msg.Patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(msg.Patches))
	}

	patch := msg.Patches[0]
	if patch.Kind != PatchPlayerPos {
		t.Fatalf("expected patch kind %q, got %q", PatchPlayerPos, patch.Kind)
	}
	if patch.EntityID != player.ID {
		t.Fatalf("expected patch entity %q, got %q", player.ID, patch.EntityID)
	}

	payload, ok := patch.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected payload to decode as map, got %T", patch.Payload)
	}

	xValue, ok := payload["x"].(float64)
	if !ok {
		t.Fatalf("expected payload x to be float, got %T", payload["x"])
	}
	yValue, ok := payload["y"].(float64)
	if !ok {
		t.Fatalf("expected payload y to be float, got %T", payload["y"])
	}

	if math.Abs(xValue-player.X) > 1e-6 {
		t.Fatalf("expected x %.3f, got %.3f", player.X, xValue)
	}
	if math.Abs(yValue-player.Y) > 1e-6 {
		t.Fatalf("expected y %.3f, got %.3f", player.Y, yValue)
	}

	if player.version != 1 {
		t.Fatalf("expected player version to increment, got %d", player.version)
	}

	hub.mu.Lock()
	drained := hub.world.journal.DrainPatches()
	hub.mu.Unlock()
	if len(drained) != 0 {
		t.Fatalf("expected journal to be empty after marshalState, got %d entries", len(drained))
	}
}
