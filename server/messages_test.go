package main

import (
	"bytes"
	"encoding/json"
	"errors"
	stdlog "log"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

type failingPayload struct{}

func (failingPayload) MarshalJSON() ([]byte, error) {
	return nil, errors.New("failing payload")
}

func TestStateMessage_ContainsTick(t *testing.T) {
	hub := newHub()
	hub.SetKeyframeInterval(1)
	hub.advance(time.Now(), 1.0/float64(tickRate))

	data, _, err := hub.marshalState(nil, nil, nil, nil, true, true)
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

	if resyncValue, present := payload["resync"]; present {
		if resyncBool, ok := resyncValue.(bool); !ok || resyncBool {
			t.Fatalf("expected resync flag to be absent or false during steady broadcasts")
		}
	}
}

func TestJoinResponseIncludesEffectCatalog(t *testing.T) {
	hub := newHub()
	join := hub.Join()

	if len(join.EffectCatalog) == 0 {
		t.Fatalf("expected join response to include effect catalog entries")
	}

	entry, ok := join.EffectCatalog["fireball"]
	if !ok {
		t.Fatalf("expected catalog to include fireball entry")
	}
	if entry.ContractID == "" {
		t.Fatalf("expected catalog entry to include contract id")
	}
	if entry.Definition == nil || entry.Definition.TypeID == "" {
		t.Fatalf("expected catalog entry to include definition metadata")
	}
}

func TestStateMessageConfigIncludesEffectCatalogOnSnapshot(t *testing.T) {
	hub := newHub()
	hub.SetKeyframeInterval(1)
	hub.advance(time.Now(), 1.0/float64(tickRate))

	data, _, err := hub.marshalState(nil, nil, nil, nil, true, true)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}

	config, ok := payload["config"].(map[string]any)
	if !ok {
		t.Fatalf("expected config to decode as object, got %T", payload["config"])
	}

	catalog, exists := config["effectCatalog"].(map[string]any)
	if !exists {
		t.Fatalf("expected snapshot payload to include effectCatalog metadata")
	}
	if len(catalog) == 0 {
		t.Fatalf("expected effectCatalog metadata to include entries")
	}
	if _, ok := catalog["fireball"].(map[string]any); !ok {
		t.Fatalf("expected effectCatalog metadata to include fireball entry")
	}
}

func TestStateMessageConfigOmitsEffectCatalogOnDelta(t *testing.T) {
	hub := newHub()
	hub.SetKeyframeInterval(1)
	hub.advance(time.Now(), 1.0/float64(tickRate))

	data, _, err := hub.marshalState(nil, nil, nil, nil, true, false)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}

	config, ok := payload["config"].(map[string]any)
	if !ok {
		t.Fatalf("expected config to decode as object, got %T", payload["config"])
	}

	if _, exists := config["effectCatalog"]; exists {
		t.Fatalf("expected delta payload to omit effectCatalog metadata")
	}
}

func TestTickMonotonicity_AcrossBroadcasts(t *testing.T) {
	hub := newHub()
	hub.SetKeyframeInterval(1)
	dt := 1.0 / float64(tickRate)

	ticks := make([]uint64, 0, 3)
	sequences := make([]uint64, 0, 3)
	for i := 0; i < 3; i++ {
		hub.advance(time.Now(), dt)

		data, _, err := hub.marshalState(nil, nil, nil, nil, true, true)
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

		seqValue, ok := payload["sequence"]
		if !ok {
			t.Fatalf("payload missing sequence field")
		}
		seqNumber, ok := seqValue.(float64)
		if !ok {
			t.Fatalf("expected sequence to decode as number, got %T", seqValue)
		}
		if math.Mod(seqNumber, 1) != 0 {
			t.Fatalf("expected sequence to be integral, got %f", seqNumber)
		}
		sequences = append(sequences, uint64(seqNumber))
	}

	if len(ticks) != 3 {
		t.Fatalf("expected to capture 3 ticks, got %d", len(ticks))
	}

	for i := 1; i < len(ticks); i++ {
		if ticks[i] != ticks[i-1]+1 {
			t.Fatalf("expected ticks to increase by 1, got %d then %d", ticks[i-1], ticks[i])
		}
	}

	if len(sequences) != 3 {
		t.Fatalf("expected to capture 3 sequences, got %d", len(sequences))
	}
	for i := 1; i < len(sequences); i++ {
		if sequences[i] <= sequences[i-1] {
			t.Fatalf("expected sequences to strictly increase, got %d then %d", sequences[i-1], sequences[i])
		}
	}
}

func TestMarshalStateRestoresBuffersOnError(t *testing.T) {
	hub := newHub()
	join := hub.Join()
	if join.ID == "" {
		t.Fatalf("expected join response to include player id")
	}

	hub.mu.Lock()
	hub.world.drainPatchesLocked()
	hub.mu.Unlock()

	hub.mu.Lock()
	hub.world.appendPatch(PatchPlayerPos, join.ID, failingPayload{})

	spawn := effectcontract.EffectSpawnEvent{
		Tick: effectcontract.Tick(hub.tick.Load()),
		Instance: effectcontract.EffectInstance{
			ID:           "effect-1",
			DefinitionID: "dummy",
			Definition: &effectcontract.EffectDefinition{
				TypeID:        "dummy",
				Delivery:      effectcontract.DeliveryKindVisual,
				Motion:        effectcontract.MotionKindNone,
				Impact:        effectcontract.ImpactPolicyNone,
				LifetimeTicks: 1,
				Client:        effectcontract.ReplicationSpec{},
			},
			DeliveryState: effectcontract.EffectDeliveryState{
				Geometry: effectcontract.EffectGeometry{Shape: effectcontract.GeometryShapeCircle},
				Motion:   effectcontract.EffectMotionState{},
			},
			BehaviorState: effectcontract.EffectBehaviorState{TicksRemaining: 1},
			Replication:   effectcontract.ReplicationSpec{SendSpawn: true, SendEnd: true},
		},
	}
	spawn = hub.world.journal.RecordEffectSpawn(spawn)
	if spawn.Instance.ID == "" {
		t.Fatalf("expected spawn event to record effect instance")
	}

	hub.world.journal.RecordEffectEnd(effectcontract.EffectEndEvent{Tick: effectcontract.Tick(hub.tick.Load()), ID: spawn.Instance.ID, Reason: effectcontract.EndReasonExpired})
	hub.mu.Unlock()

	if _, _, err := hub.marshalState(nil, nil, nil, nil, true, true); err == nil {
		t.Fatalf("expected marshalState to fail when payload encoding fails")
	}

	hub.mu.Lock()
	patches := hub.world.journal.SnapshotPatches()
	effects := hub.world.journal.SnapshotEffectEvents()
	_, seqExists := hub.world.journal.effectSeq[spawn.Instance.ID]
	_, recentlyEnded := hub.world.journal.recentlyEnded[spawn.Instance.ID]
	endedCount := 0
	for _, id := range hub.world.journal.endedIDs {
		if id == spawn.Instance.ID {
			endedCount++
		}
	}
	hub.mu.Unlock()

	if len(patches) != 1 {
		t.Fatalf("expected 1 patch after failed marshal, got %d", len(patches))
	}
	if _, ok := patches[0].Payload.(failingPayload); !ok {
		t.Fatalf("expected failing payload to remain staged after error")
	}

	if len(effects.Spawns) != 1 {
		t.Fatalf("expected spawn event to remain staged, got %d", len(effects.Spawns))
	}
	if len(effects.Ends) != 1 {
		t.Fatalf("expected end event to remain staged, got %d", len(effects.Ends))
	}
	if len(effects.LastSeqByID) == 0 {
		t.Fatalf("expected effect sequence cursors to remain staged")
	}
	if _, ok := effects.LastSeqByID[spawn.Instance.ID]; !ok {
		t.Fatalf("expected effect sequence cursor for %s to be restored", spawn.Instance.ID)
	}
	if !seqExists {
		t.Fatalf("expected journal effect sequence map to retain id after restore")
	}
	if endedCount != 1 {
		t.Fatalf("expected journal endedIDs to contain restored id once, got %d", endedCount)
	}
	if !recentlyEnded {
		t.Fatalf("expected journal recentlyEnded to restore tick for ended effect")
	}
}

func TestStateMessageIncludesEmptyPatchesSlice(t *testing.T) {
	hub := newHub()
	hub.SetKeyframeInterval(1)
	hub.advance(time.Now(), 1.0/float64(tickRate))

	data, _, err := hub.marshalState(nil, nil, nil, nil, true, true)
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

func TestBroadcastLoggingRedactsPayload(t *testing.T) {
	hub := newHub()
	groundItems := []GroundItem{{
		ID:   "ground-fireball",
		Type: ItemType("fireball"),
		X:    1,
		Y:    2,
		Qty:  1,
	}}

	var buf bytes.Buffer
	originalOutput := stdlog.Writer()
	originalFlags := stdlog.Flags()
	originalPrefix := stdlog.Prefix()
	stdlog.SetOutput(&buf)
	stdlog.SetFlags(0)
	stdlog.SetPrefix("")
	t.Cleanup(func() {
		stdlog.SetOutput(originalOutput)
		stdlog.SetFlags(originalFlags)
		stdlog.SetPrefix(originalPrefix)
	})

	hub.broadcastState(nil, nil, nil, groundItems)

	logOutput := buf.String()
	if !strings.Contains(logOutput, "fireball") {
		t.Fatalf("expected broadcast log to mention fireball marker, got %q", logOutput)
	}
	if strings.Contains(logOutput, "\"type\":\"fireball\"") {
		t.Fatalf("expected broadcast log to redact payload contents, got %q", logOutput)
	}
}

func TestStateMessageWithPatchesRoundTrip(t *testing.T) {
	msg := stateMessage{
		Ver:            ProtocolVersion,
		Type:           "state",
		Players:        nil,
		NPCs:           nil,
		Obstacles:      nil,
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
		Sequence:   42,
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

func TestStateMessageIncludesEffectEventsWhenEnabled(t *testing.T) {
	hub := newHub()
	hub.SetKeyframeInterval(1)

	if hub.world.effectManager == nil {
		t.Fatalf("expected effect manager to be initialized")
	}

	hub.world.effectManager.EnqueueIntent(effectcontract.EffectIntent{
		EntryID:  effectTypeAttack,
		TypeID:   effectTypeAttack,
		Delivery: effectcontract.DeliveryKindArea,
		Geometry: effectcontract.EffectGeometry{Shape: effectcontract.GeometryShapeRect},
	})

	hub.advance(time.Now(), 1.0/float64(tickRate))

	data, _, err := hub.marshalState(nil, nil, nil, nil, true, true)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}

	if _, present := payload["effects"]; present {
		t.Fatalf("expected legacy effects array to be omitted when transport active")
	}

	rawSpawns, ok := payload["effect_spawned"]
	if !ok {
		t.Fatalf("expected payload to include effect_spawned when manager enabled")
	}
	spawns, ok := rawSpawns.([]any)
	if !ok || len(spawns) == 0 {
		t.Fatalf("expected effect_spawned to decode as non-empty array, got %T with len %d", rawSpawns, len(spawns))
	}

	if rawUpdates, ok := payload["effect_update"]; !ok {
		t.Fatalf("expected payload to include effect_update when manager enabled")
	} else if updates, ok := rawUpdates.([]any); !ok || len(updates) == 0 {
		t.Fatalf("expected effect_update to decode as non-empty array, got %T with len %d", rawUpdates, len(updates))
	}

	if rawEnds, ok := payload["effect_ended"]; !ok {
		t.Fatalf("expected payload to include effect_ended when manager enabled")
	} else if ends, ok := rawEnds.([]any); !ok || len(ends) == 0 {
		t.Fatalf("expected effect_ended to decode as non-empty array, got %T with len %d", rawEnds, len(ends))
	}

	if rawCursors, ok := payload["effect_seq_cursors"]; !ok {
		t.Fatalf("expected payload to include effect_seq_cursors when manager enabled")
	} else if cursors, ok := rawCursors.(map[string]any); !ok || len(cursors) == 0 {
		t.Fatalf("expected effect_seq_cursors to decode as non-empty map, got %T with len %d", rawCursors, len(cursors))
	}

	followUp, _, err := hub.marshalState(nil, nil, nil, nil, true, true)
	if err != nil {
		t.Fatalf("marshalState returned error on follow-up: %v", err)
	}

	payload = make(map[string]any)
	if err := json.Unmarshal(followUp, &payload); err != nil {
		t.Fatalf("failed to decode follow-up payload: %v", err)
	}

	if _, present := payload["effect_spawned"]; present {
		t.Fatalf("expected effect_spawned to be drained after broadcast")
	}
	if _, present := payload["effect_update"]; present {
		t.Fatalf("expected effect_update to be drained after broadcast")
	}
	if _, present := payload["effect_ended"]; present {
		t.Fatalf("expected effect_ended to be drained after broadcast")
	}
	if _, present := payload["effect_seq_cursors"]; present {
		t.Fatalf("expected effect_seq_cursors to be cleared after broadcast")
	}
}

func TestResyncLifecycleAcrossSnapshotsAndResets(t *testing.T) {
	hub := newHub()
	hub.SetKeyframeInterval(1)
	hub.advance(time.Now(), 1.0/float64(tickRate))

	data, _, err := hub.marshalState(nil, nil, nil, nil, false, true)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	assertResyncFlag(t, data, true)

	data, _, err = hub.marshalState(nil, nil, nil, nil, true, true)
	if err != nil {
		t.Fatalf("marshalState returned error for steady broadcast: %v", err)
	}

	assertResyncFlag(t, data, false)

	hub.ResetWorld(defaultWorldConfig())

	data, _, err = hub.marshalState(nil, nil, nil, nil, true, true)
	if err != nil {
		t.Fatalf("marshalState returned error after reset: %v", err)
	}

	assertResyncFlag(t, data, true)

	data, _, err = hub.marshalState(nil, nil, nil, nil, true, true)
	if err != nil {
		t.Fatalf("marshalState returned error on follow-up broadcast: %v", err)
	}

	assertResyncFlag(t, data, false)
}

func TestHubSchedulesResyncAfterJournalHint(t *testing.T) {
	hub := newHub()
	hub.SetKeyframeInterval(5)

	hub.mu.Lock()
	hub.world.journal.RecordEffectUpdate(effectcontract.EffectUpdateEvent{Tick: 1, ID: "effect-x"})
	hub.mu.Unlock()

	scheduled, signal := hub.scheduleResyncIfNeeded()
	if !scheduled {
		t.Fatalf("expected resync to be scheduled after journal hint")
	}
	if signal.LostSpawns != 1 {
		t.Fatalf("expected lost spawn count 1, got %d", signal.LostSpawns)
	}

	includeSnapshot := hub.shouldIncludeSnapshot()
	if !includeSnapshot {
		t.Fatalf("expected forced keyframe after resync hint")
	}

	data, _, err := hub.marshalState(nil, nil, nil, nil, true, includeSnapshot)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}
	assertResyncFlag(t, data, true)
}

func assertResyncFlag(t *testing.T, raw []byte, expected bool) {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}

	value, present := payload["resync"]
	if !present {
		if expected {
			t.Fatalf("expected resync flag to be present")
		}
		return
	}

	resyncBool, ok := value.(bool)
	if !ok {
		t.Fatalf("expected resync flag to be boolean, got %T", value)
	}
	if resyncBool != expected {
		t.Fatalf("unexpected resync flag value: got %v, want %v", resyncBool, expected)
	}
}

func TestMarshalStateSnapshotDoesNotDrainPatches(t *testing.T) {
	hub := newHub()
	hub.SetKeyframeInterval(1)

	hub.mu.Lock()
	hub.world.journal.AppendPatch(Patch{Kind: PatchPlayerPos, EntityID: "player-1"})
	hub.mu.Unlock()

	if _, _, err := hub.marshalState(nil, nil, nil, nil, false, true); err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	hub.mu.Lock()
	if patches := hub.world.snapshotPatchesLocked(); len(patches) != 1 {
		hub.mu.Unlock()
		t.Fatalf("expected patches to remain after snapshot, got %d", len(patches))
	}
	hub.mu.Unlock()

	if _, _, err := hub.marshalState(nil, nil, nil, nil, true, true); err != nil {
		t.Fatalf("marshalState returned error when draining: %v", err)
	}

	hub.mu.Lock()
	if patches := hub.world.snapshotPatchesLocked(); len(patches) != 0 {
		hub.mu.Unlock()
		t.Fatalf("expected patches to drain after broadcast, got %d", len(patches))
	}
	hub.mu.Unlock()
}

func TestMarshalStateRecordsKeyframe(t *testing.T) {
	hub := newHub()
	hub.SetKeyframeInterval(1)

	data, _, err := hub.marshalState(nil, nil, nil, nil, false, true)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	var msg stateMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to decode state payload: %v", err)
	}

	if msg.KeyframeSeq == 0 {
		t.Fatalf("expected keyframe sequence to be populated")
	}
	if msg.KeyframeSeq != msg.Sequence {
		t.Fatalf("expected keyframe sequence %d to match message sequence %d", msg.KeyframeSeq, msg.Sequence)
	}

	snapshot, ok := hub.Keyframe(msg.KeyframeSeq)
	if !ok {
		t.Fatalf("expected hub to return keyframe %d", msg.KeyframeSeq)
	}

	if snapshot.Sequence != msg.KeyframeSeq {
		t.Fatalf("unexpected keyframe sequence: got %d want %d", snapshot.Sequence, msg.KeyframeSeq)
	}
	if snapshot.Tick != msg.Tick {
		t.Fatalf("unexpected keyframe tick: got %d want %d", snapshot.Tick, msg.Tick)
	}
	if len(snapshot.Players) != len(msg.Players) {
		t.Fatalf("expected %d players in keyframe, got %d", len(msg.Players), len(snapshot.Players))
	}
	if len(snapshot.NPCs) != len(msg.NPCs) {
		t.Fatalf("expected %d NPCs in keyframe, got %d", len(msg.NPCs), len(snapshot.NPCs))
	}
	if len(snapshot.GroundItems) != len(msg.GroundItems) {
		t.Fatalf("expected %d ground items in keyframe, got %d", len(msg.GroundItems), len(snapshot.GroundItems))
	}

	if _, ok := hub.Keyframe(msg.KeyframeSeq + 1); ok {
		t.Fatalf("expected lookup for unknown keyframe sequence to fail")
	}
}

func TestHandleKeyframeRequestReturnsSnapshot(t *testing.T) {
	hub := newHub()
	hub.SetKeyframeInterval(1)

	data, _, err := hub.marshalState(nil, nil, nil, nil, true, true)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	var msg stateMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to decode state payload: %v", err)
	}

	snapshot, nack, ok := hub.HandleKeyframeRequest("player-1", nil, msg.Sequence)
	if !ok {
		t.Fatalf("expected handle to succeed")
	}
	if nack != nil {
		t.Fatalf("expected ack response, got nack: %+v", nack)
	}
	if snapshot.Sequence != msg.Sequence {
		t.Fatalf("unexpected snapshot sequence: got %d want %d", snapshot.Sequence, msg.Sequence)
	}
}

func TestHandleKeyframeRequestReturnsCatalogSnapshot(t *testing.T) {
	hub := newHub()
	hub.SetKeyframeInterval(1)

	data, _, err := hub.marshalState(nil, nil, nil, nil, true, true)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	var msg stateMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to decode state payload: %v", err)
	}

	snapshot, nack, ok := hub.HandleKeyframeRequest("player-1", nil, msg.Sequence)
	if !ok {
		t.Fatalf("expected handle to succeed")
	}
	if nack != nil {
		t.Fatalf("expected ack response, got nack: %+v", nack)
	}

	resolver := hub.world.effectManager.catalog
	expected := snapshotEffectCatalog(resolver)
	if len(expected) == 0 {
		t.Fatalf("expected effect catalog snapshot to contain entries")
	}

	if snapshot.Config.EffectCatalog == nil {
		t.Fatalf("expected keyframe config to include effect catalog snapshot")
	}

	if !reflect.DeepEqual(snapshot.Config.EffectCatalog, expected) {
		t.Fatalf("unexpected effect catalog snapshot: got %+v want %+v", snapshot.Config.EffectCatalog, expected)
	}
}

func TestHandleKeyframeRequestClonesCatalogSnapshot(t *testing.T) {
	hub := newHub()
	hub.SetKeyframeInterval(1)

	data, _, err := hub.marshalState(nil, nil, nil, nil, true, true)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	var msg stateMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to decode state payload: %v", err)
	}

	snapshot, nack, ok := hub.HandleKeyframeRequest("player-1", nil, msg.Sequence)
	if !ok {
		t.Fatalf("expected handle to succeed")
	}
	if nack != nil {
		t.Fatalf("expected ack response, got nack: %+v", nack)
	}

	expected := snapshotEffectCatalog(hub.world.effectManager.catalog)
	if !reflect.DeepEqual(snapshot.Config.EffectCatalog, expected) {
		t.Fatalf("unexpected effect catalog snapshot: got %+v want %+v", snapshot.Config.EffectCatalog, expected)
	}

	frame, found := hub.world.journal.KeyframeBySequence(msg.Sequence)
	if !found {
		t.Fatalf("expected journal to retain keyframe %d", msg.Sequence)
	}
	if frame.Config.EffectCatalog == nil {
		t.Fatalf("expected journal keyframe to include effect catalog")
	}
	if snapshot.Config.EffectCatalog == nil {
		t.Fatalf("expected keyframe response to include effect catalog")
	}

	const effectID = "fireball"
	frameMeta, ok := frame.Config.EffectCatalog[effectID]
	if !ok {
		t.Fatalf("expected journal keyframe to include %s metadata", effectID)
	}
	responseMeta, ok := snapshot.Config.EffectCatalog[effectID]
	if !ok {
		t.Fatalf("expected keyframe response to include %s metadata", effectID)
	}
	if frameMeta.Definition == nil || responseMeta.Definition == nil {
		t.Fatalf("expected %s metadata to include definition", effectID)
	}
	if frameMeta.Definition == responseMeta.Definition {
		t.Fatalf("expected keyframe response to clone definition metadata")
	}

	originalFrameMeta := frameMeta
	snapshot.Config.EffectCatalog[effectID] = effectCatalogMetadata{}
	if !reflect.DeepEqual(frame.Config.EffectCatalog[effectID], originalFrameMeta) {
		t.Fatalf("expected journal keyframe metadata to remain unchanged after mutating response")
	}
}

func TestHandleKeyframeRequestExpired(t *testing.T) {
	t.Setenv(envJournalCapacity, "1")
	hub := newHub()
	hub.SetKeyframeInterval(1)

	expectedCatalog := snapshotEffectCatalog(hub.world.effectManager.catalog)
	if len(expectedCatalog) == 0 {
		t.Fatalf("expected effect catalog snapshot to contain entries")
	}

	first, _, err := hub.marshalState(nil, nil, nil, nil, true, true)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}
	var firstMsg stateMessage
	if err := json.Unmarshal(first, &firstMsg); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}

	if _, _, err := hub.marshalState(nil, nil, nil, nil, true, true); err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	_, nack, ok := hub.HandleKeyframeRequest("player-2", nil, firstMsg.Sequence)
	if !ok {
		t.Fatalf("expected handler to respond")
	}
	if nack == nil {
		t.Fatalf("expected nack when sequence expired")
	}
	if nack.Reason != "expired" {
		t.Fatalf("expected expired reason, got %s", nack.Reason)
	}
	if !nack.Resync {
		t.Fatalf("expected nack to request resync")
	}
	if !reflect.DeepEqual(nack.Config.EffectCatalog, expectedCatalog) {
		t.Fatalf("unexpected effect catalog snapshot on nack: got %+v want %+v", nack.Config.EffectCatalog, expectedCatalog)
	}

	data, _, err := hub.marshalState(nil, nil, nil, nil, true, false)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	var msg stateMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}

	if !msg.Resync {
		t.Fatalf("expected follow-up state payload to request resync")
	}
	if msg.Config.EffectCatalog == nil {
		t.Fatalf("expected resync payload to include effect catalog snapshot")
	}
	for id, expected := range expectedCatalog {
		actual, ok := msg.Config.EffectCatalog[id]
		if !ok {
			t.Fatalf("expected resync catalog to include %s", id)
		}
		if actual.ContractID != expected.ContractID {
			t.Fatalf("unexpected contract id for %s: got %s want %s", id, actual.ContractID, expected.ContractID)
		}
		if actual.ManagedByClient != expected.ManagedByClient {
			t.Fatalf("unexpected managed flag for %s: got %t want %t", id, actual.ManagedByClient, expected.ManagedByClient)
		}
		if expected.Definition != nil && actual.Definition == nil {
			t.Fatalf("expected resync catalog %s to include definition", id)
		}
	}
}

func TestHandleKeyframeRequestRateLimited(t *testing.T) {
	hub := newHub()
	sub := &subscriber{limiter: newKeyframeRateLimiter(1, 1)}
	sub.limiter.allow(time.Now())

	_, nack, ok := hub.HandleKeyframeRequest("player-3", sub, 5)
	if !ok {
		t.Fatalf("expected handler to respond to rate limited request")
	}
	if nack == nil {
		t.Fatalf("expected nack when rate limited")
	}
	if nack.Reason != "rate_limited" {
		t.Fatalf("expected rate_limited reason, got %s", nack.Reason)
	}
	if nack.Sequence != 5 {
		t.Fatalf("expected nack sequence 5, got %d", nack.Sequence)
	}
	if !nack.Resync {
		t.Fatalf("expected rate limited nack to request resync")
	}
	expectedCatalog := snapshotEffectCatalog(hub.world.effectManager.catalog)
	if !reflect.DeepEqual(nack.Config.EffectCatalog, expectedCatalog) {
		t.Fatalf("unexpected effect catalog snapshot on rate limit nack: got %+v want %+v", nack.Config.EffectCatalog, expectedCatalog)
	}

	data, _, err := hub.marshalState(nil, nil, nil, nil, true, false)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	var msg stateMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}
	if !msg.Resync {
		t.Fatalf("expected resync broadcast after rate limited nack")
	}
	if msg.Config.EffectCatalog == nil {
		t.Fatalf("expected resync broadcast to include effect catalog snapshot")
	}
	for id, expected := range expectedCatalog {
		actual, ok := msg.Config.EffectCatalog[id]
		if !ok {
			t.Fatalf("expected resync catalog to include %s", id)
		}
		if actual.ContractID != expected.ContractID {
			t.Fatalf("unexpected contract id for %s: got %s want %s", id, actual.ContractID, expected.ContractID)
		}
		if actual.ManagedByClient != expected.ManagedByClient {
			t.Fatalf("unexpected managed flag for %s: got %t want %t", id, actual.ManagedByClient, expected.ManagedByClient)
		}
		if expected.Definition != nil && actual.Definition == nil {
			t.Fatalf("expected resync catalog %s to include definition", id)
		}
	}
}
