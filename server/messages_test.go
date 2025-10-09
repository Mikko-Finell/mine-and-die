package main

import (
	"encoding/json"
	"math"
	"testing"
	"time"
)

func TestStateMessage_ContainsSequenceAndTick(t *testing.T) {
	hub := newHub()
	hub.advance(time.Now(), 1.0/float64(tickRate))

	data, _, err := hub.marshalState(nil, nil, nil, nil, nil, true)
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

	seqValue, ok := payload["sequence"]
	if !ok {
		t.Fatalf("expected payload to include sequence field")
	}
	seqNumber, ok := seqValue.(float64)
	if !ok {
		t.Fatalf("expected sequence to decode as number, got %T", seqValue)
	}
	if seqNumber < 0 {
		t.Fatalf("expected non-negative sequence, got %f", seqNumber)
	}
	if math.Mod(seqNumber, 1) != 0 {
		t.Fatalf("expected sequence to be integral, got %f", seqNumber)
	}
}

func TestTickMonotonicity_AcrossBroadcasts(t *testing.T) {
	hub := newHub()
	dt := 1.0 / float64(tickRate)

	ticks := make([]uint64, 0, 3)
	sequences := make([]uint64, 0, 3)
	for i := 0; i < 3; i++ {
		hub.advance(time.Now(), dt)

		data, _, err := hub.marshalState(nil, nil, nil, nil, nil, true)
		if err != nil {
			t.Fatalf("marshalState returned error: %v", err)
		}

		var payload map[string]any
		if err := json.Unmarshal(data, &payload); err != nil {
			t.Fatalf("failed to decode payload: %v", err)
		}

		tickValue, ok := payload["t"]
		if !ok {
			t.Fatalf("payload missing tick field")
		}
		tickNumber, ok := tickValue.(float64)
		if !ok {
			t.Fatalf("expected tick to decode as number, got %T", tickValue)
		}
		if math.Mod(tickNumber, 1) != 0 {
			t.Fatalf("expected tick to be integral, got %f", tickNumber)
		}
		ticks = append(ticks, uint64(tickNumber))

		sequenceValue, ok := payload["sequence"]
		if !ok {
			t.Fatalf("payload missing sequence field")
		}
		sequenceNumber, ok := sequenceValue.(float64)
		if !ok {
			t.Fatalf("expected sequence to decode as number, got %T", sequenceValue)
		}
		if math.Mod(sequenceNumber, 1) != 0 {
			t.Fatalf("expected sequence to be integral, got %f", sequenceNumber)
		}
		sequences = append(sequences, uint64(sequenceNumber))
	}

	if len(ticks) != 3 {
		t.Fatalf("expected to capture 3 ticks, got %d", len(ticks))
	}

	for i := 1; i < len(ticks); i++ {
		if ticks[i] != ticks[i-1]+1 {
			t.Fatalf("expected ticks to increase by 1, got %d then %d", ticks[i-1], ticks[i])
		}
		if sequences[i] != sequences[i-1]+1 {
			t.Fatalf("expected sequences to increase by 1, got %d then %d", sequences[i-1], sequences[i])
		}
	}
}

func TestMarshalStateMarksResyncWhenNeeded(t *testing.T) {
	hub := newHub()

	snapshotData, _, err := hub.marshalState(nil, nil, nil, nil, nil, false)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}
	var snapshotMsg stateMessage
	if err := json.Unmarshal(snapshotData, &snapshotMsg); err != nil {
		t.Fatalf("failed to decode snapshot state: %v", err)
	}
	if !snapshotMsg.Resync {
		t.Fatalf("expected initial snapshot to include resync flag")
	}

	regularData, _, err := hub.marshalState(nil, nil, nil, nil, nil, true)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}
	var regularMsg stateMessage
	if err := json.Unmarshal(regularData, &regularMsg); err != nil {
		t.Fatalf("failed to decode regular state: %v", err)
	}
	if regularMsg.Resync {
		t.Fatalf("expected draining broadcast to omit resync flag")
	}

	cfg := hub.config
	hub.ResetWorld(cfg)

	resetData, _, err := hub.marshalState(nil, nil, nil, nil, nil, true)
	if err != nil {
		t.Fatalf("marshalState returned error after reset: %v", err)
	}
	var resetMsg stateMessage
	if err := json.Unmarshal(resetData, &resetMsg); err != nil {
		t.Fatalf("failed to decode reset state: %v", err)
	}
	if !resetMsg.Resync {
		t.Fatalf("expected reset broadcast to include resync flag")
	}

	followData, _, err := hub.marshalState(nil, nil, nil, nil, nil, true)
	if err != nil {
		t.Fatalf("marshalState returned error for follow-up broadcast: %v", err)
	}
	var followMsg stateMessage
	if err := json.Unmarshal(followData, &followMsg); err != nil {
		t.Fatalf("failed to decode follow-up state: %v", err)
	}
	if followMsg.Resync {
		t.Fatalf("expected resync flag to clear after reset broadcast")
	}
}

func TestStateMessageIncludesEmptyPatchesSlice(t *testing.T) {
	hub := newHub()
	hub.advance(time.Now(), 1.0/float64(tickRate))

	data, _, err := hub.marshalState(nil, nil, nil, nil, nil, true)
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

	if _, ok := rawPatches.([]any); !ok {
		t.Fatalf("expected patches to decode as array, got %T", rawPatches)
	}

	var msg stateMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to decode state message: %v", err)
	}

	for _, patch := range msg.Patches {
		switch patch.Kind {
		case PatchPlayerPos, PatchPlayerFacing, PatchPlayerIntent, PatchPlayerHealth, PatchPlayerInventory:
			t.Fatalf("expected no player patches in empty state, saw kind %q", patch.Kind)
		}
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
		Patches: []Patch{
			{
				Kind:     PatchPlayerPos,
				EntityID: "player-1",
				Payload: PlayerPosPayload{
					X: 12.5,
					Y: 42.75,
				},
			},
			{
				Kind:     PatchPlayerInventory,
				EntityID: "player-1",
				Payload: PlayerInventoryPayload{
					Slots: []InventorySlot{{
						Slot: 0,
						Item: ItemStack{Type: ItemTypeGold, Quantity: 2},
					}},
				},
			},
		},
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

	if len(decoded.Patches) != 2 {
		t.Fatalf("expected 2 patches after round trip, got %d", len(decoded.Patches))
	}
}

func TestMarshalStateSnapshotDoesNotDrainPatches(t *testing.T) {
	hub := newHub()

	hub.mu.Lock()
	hub.world.journal.AppendPatch(Patch{Kind: PatchPlayerPos, EntityID: "player-1"})
	hub.mu.Unlock()

	if _, _, err := hub.marshalState(nil, nil, nil, nil, nil, false); err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	hub.mu.Lock()
	if patches := hub.world.snapshotPatchesLocked(); len(patches) != 1 {
		hub.mu.Unlock()
		t.Fatalf("expected patches to remain after snapshot, got %d", len(patches))
	}
	hub.mu.Unlock()

	if _, _, err := hub.marshalState(nil, nil, nil, nil, nil, true); err != nil {
		t.Fatalf("marshalState returned error when draining: %v", err)
	}

	hub.mu.Lock()
	if patches := hub.world.snapshotPatchesLocked(); len(patches) != 0 {
		hub.mu.Unlock()
		t.Fatalf("expected patches to drain after broadcast, got %d", len(patches))
	}
	hub.mu.Unlock()
}
