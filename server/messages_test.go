package server

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
	itemspkg "mine-and-die/server/internal/items"
	"mine-and-die/server/internal/net/proto"
	"mine-and-die/server/internal/sim"
	simutil "mine-and-die/server/internal/simutil"
	worldpkg "mine-and-die/server/internal/world"
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

func TestJoinResponseAdvertisesHashOnly(t *testing.T) {
	hub := newHub()
	join := hub.Join()

	if join.EffectCatalogHash != effectcontract.EffectCatalogHash {
		t.Fatalf("expected join response to include catalog hash %q, got %q", effectcontract.EffectCatalogHash, join.EffectCatalogHash)
	}

	data, err := proto.EncodeJoinResponse(join)
	if err != nil {
		t.Fatalf("failed to encode join response: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("failed to decode join response: %v", err)
	}

	config, ok := payload["config"].(map[string]any)
	if !ok {
		t.Fatalf("expected join config to decode as object, got %T", payload["config"])
	}

	if _, exists := config["effectCatalog"]; exists {
		t.Fatalf("expected join config to omit effectCatalog payload")
	}
}

func TestStateMessageConfigOmitsEffectCatalogOnSnapshotByDefault(t *testing.T) {
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

	if _, exists := config["effectCatalog"]; exists {
		t.Fatalf("expected snapshot payload to omit effectCatalog metadata by default")
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
	drained := hub.world.journal.DrainEffectEvents()
	if len(drained.Spawns) != 1 {
		t.Fatalf("expected drained spawn event to remain staged, got %d", len(drained.Spawns))
	}
	if len(drained.Ends) != 1 {
		t.Fatalf("expected drained end event to remain staged, got %d", len(drained.Ends))
	}
	if len(drained.LastSeqByID) == 0 {
		t.Fatalf("expected drained effect sequence cursors to remain staged")
	}
	if _, ok := drained.LastSeqByID[spawn.Instance.ID]; !ok {
		t.Fatalf("expected drained effect sequence cursor for %s to be restored", spawn.Instance.ID)
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
		case sim.PatchPlayerPos, sim.PatchPlayerFacing, sim.PatchPlayerIntent, sim.PatchPlayerHealth, sim.PatchPlayerInventory:
			t.Fatalf("expected no player patches in empty state, saw kind %q", patch.Kind)
		}
	}
}

func TestBroadcastLoggingRedactsPayload(t *testing.T) {
	if raceEnabled {
		t.Skip("broadcast logging assertions mutate shared buffers under race detector")
	}
	hub := newHub()
	groundItems := []itemspkg.GroundItem{{
		ID:   "ground-fireball",
		Type: "fireball",
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

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if buf.Len() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	logOutput := buf.String()
	if logOutput == "" {
		t.Fatalf("expected broadcast log output")
	}
	if !strings.Contains(logOutput, "fireball") {
		t.Fatalf("expected broadcast log to mention fireball marker, got %q", logOutput)
	}
	if strings.Contains(logOutput, "\"type\":\"fireball\"") {
		t.Fatalf("expected broadcast log to redact payload contents, got %q", logOutput)
	}
}

func TestMarshalStateIncludesSharedGroundItemSchema(t *testing.T) {
	hub := newHub()
	join := hub.Join()

	hub.mu.Lock()
	player := hub.world.players[join.ID]
	if player == nil {
		hub.mu.Unlock()
		t.Fatalf("expected player %s to be present in world", join.ID)
	}
	stack := hub.world.upsertGroundItem(&player.actorState, ItemStack{Type: ItemTypeGold, Quantity: 3}, "test")
	hub.mu.Unlock()
	if stack == nil {
		t.Fatalf("expected ground item stack to be created")
	}

	data, _, err := hub.marshalState(nil, nil, nil, nil, true, true)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	var msg stateMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("failed to decode state payload: %v", err)
	}

	expected := stack.GroundItem
	if len(msg.GroundItems) == 0 {
		t.Fatalf("expected ground items to be included in snapshot")
	}

	var found bool
	for _, item := range msg.GroundItems {
		if item.ID != expected.ID {
			continue
		}
		found = true
		if item.Type != expected.Type {
			t.Fatalf("expected type %q, got %q", expected.Type, item.Type)
		}
		if item.FungibilityKey != expected.FungibilityKey {
			t.Fatalf("expected fungibility key %q, got %q", expected.FungibilityKey, item.FungibilityKey)
		}
		if item.Qty != expected.Qty {
			t.Fatalf("expected quantity %d, got %d", expected.Qty, item.Qty)
		}
		if item.X != expected.X || item.Y != expected.Y {
			t.Fatalf("expected position (%f,%f), got (%f,%f)", expected.X, expected.Y, item.X, item.Y)
		}
	}
	if !found {
		t.Fatalf("expected snapshot to include ground item %q", expected.ID)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("failed to decode raw payload: %v", err)
	}

	rawItems, ok := payload["groundItems"].([]any)
	if !ok {
		t.Fatalf("expected groundItems field to decode as array, got %T", payload["groundItems"])
	}

	var raw map[string]any
	for _, entry := range rawItems {
		candidate, ok := entry.(map[string]any)
		if !ok {
			t.Fatalf("expected ground item entry to decode as object, got %T", entry)
		}
		if id, _ := candidate["id"].(string); id == expected.ID {
			raw = candidate
			break
		}
	}
	if raw == nil {
		t.Fatalf("expected raw payload to include ground item %q", expected.ID)
	}
	if _, ok := raw["fungibility_key"]; !ok {
		t.Fatalf("expected raw payload to expose fungibility_key field")
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
		Patches: []sim.Patch{
			{
				Kind:     sim.PatchPlayerPos,
				EntityID: "player-1",
				Payload: sim.PlayerPosPayload{
					X: 12.5,
					Y: 42.75,
				},
			},
			{
				Kind:     sim.PatchPlayerInventory,
				EntityID: "player-1",
				Payload: itemspkg.SimInventoryPayloadFromSlots[sim.InventorySlot, sim.PlayerInventoryPayload]([]sim.InventorySlot{{
					Slot: 0,
					Item: sim.ItemStack{Type: sim.ItemType(ItemTypeGold), Quantity: 2},
				}}),
			},
		},
		Tick:       1,
		Sequence:   42,
		ServerTime: time.Now().UnixMilli(),
		Config:     sim.WorldConfig{},
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

	hub.ResetWorld(worldpkg.DefaultConfig())

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

func TestMarshalStateMetadataAcrossHandshakeAndDelta(t *testing.T) {
	hub := newHub()
	hub.SetKeyframeInterval(1)
	dt := 1.0 / float64(tickRate)

	now := time.Now()
	hub.advance(now, dt)

	decode := func(raw []byte) stateMessage {
		var msg stateMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatalf("failed to decode state payload: %v", err)
		}
		return msg
	}

	handshakeData, _, err := hub.marshalState(nil, nil, nil, nil, false, true)
	if err != nil {
		t.Fatalf("marshalState returned error during handshake: %v", err)
	}

	handshake := decode(handshakeData)
	if !handshake.Resync {
		t.Fatalf("expected handshake broadcast to request resync")
	}
	if handshake.Sequence == 0 {
		t.Fatalf("expected handshake broadcast to assign sequence")
	}
	if handshake.KeyframeSeq != handshake.Sequence {
		t.Fatalf("expected handshake keyframe sequence to match broadcast sequence, got %d vs %d", handshake.KeyframeSeq, handshake.Sequence)
	}
	if handshake.Tick != hub.tick.Load() {
		t.Fatalf("unexpected handshake tick: got %d want %d", handshake.Tick, hub.tick.Load())
	}

	nextNow := now.Add(2 * time.Millisecond)
	hub.advance(nextNow, dt)

	deltaData, _, err := hub.marshalState(nil, nil, nil, nil, true, false)
	if err != nil {
		t.Fatalf("marshalState returned error during delta broadcast: %v", err)
	}

	delta := decode(deltaData)
	if delta.Resync {
		t.Fatalf("expected steady delta broadcast to omit resync flag")
	}
	if delta.Sequence <= handshake.Sequence {
		t.Fatalf("expected delta sequence to advance beyond handshake, got %d after %d", delta.Sequence, handshake.Sequence)
	}
	if delta.Tick <= handshake.Tick {
		t.Fatalf("expected delta tick to advance beyond handshake, got %d after %d", delta.Tick, handshake.Tick)
	}
	if delta.KeyframeSeq != handshake.Sequence {
		t.Fatalf("expected delta to reference handshake keyframe sequence, got %d want %d", delta.KeyframeSeq, handshake.Sequence)
	}
}

func TestMarshalStateMetadataForcedResync(t *testing.T) {
	hub := newHub()
	hub.SetKeyframeInterval(1)
	dt := 1.0 / float64(tickRate)
	now := time.Now()

	hub.advance(now, dt)

	decode := func(raw []byte) stateMessage {
		var msg stateMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatalf("failed to decode state payload: %v", err)
		}
		return msg
	}

	bootstrapData, _, err := hub.marshalState(nil, nil, nil, nil, false, true)
	if err != nil {
		t.Fatalf("marshalState returned error during bootstrap: %v", err)
	}
	bootstrap := decode(bootstrapData)

	steadyNow := now.Add(2 * time.Millisecond)
	hub.advance(steadyNow, dt)

	steadyData, _, err := hub.marshalState(nil, nil, nil, nil, true, false)
	if err != nil {
		t.Fatalf("marshalState returned error during steady delta: %v", err)
	}
	steady := decode(steadyData)

	hub.resyncNext.Store(true)
	resyncNow := steadyNow.Add(2 * time.Millisecond)
	hub.advance(resyncNow, dt)

	resyncData, _, err := hub.marshalState(nil, nil, nil, nil, true, true)
	if err != nil {
		t.Fatalf("marshalState returned error during forced resync: %v", err)
	}
	resync := decode(resyncData)
	if !resync.Resync {
		t.Fatalf("expected forced resync broadcast to include resync flag")
	}
	if resync.Sequence <= steady.Sequence {
		t.Fatalf("expected forced resync sequence to advance, got %d after %d", resync.Sequence, steady.Sequence)
	}
	if resync.Tick <= steady.Tick {
		t.Fatalf("expected forced resync tick to advance, got %d after %d", resync.Tick, steady.Tick)
	}
	if resync.KeyframeSeq != resync.Sequence {
		t.Fatalf("expected forced resync to record fresh keyframe sequence, got %d want %d", resync.KeyframeSeq, resync.Sequence)
	}

	followNow := resyncNow.Add(2 * time.Millisecond)
	hub.advance(followNow, dt)

	followData, _, err := hub.marshalState(nil, nil, nil, nil, true, false)
	if err != nil {
		t.Fatalf("marshalState returned error during follow-up delta: %v", err)
	}
	follow := decode(followData)
	if follow.Resync {
		t.Fatalf("expected follow-up delta to clear resync flag")
	}
	if follow.Sequence <= resync.Sequence {
		t.Fatalf("expected follow-up sequence to advance after resync, got %d after %d", follow.Sequence, resync.Sequence)
	}
	if follow.Tick <= resync.Tick {
		t.Fatalf("expected follow-up tick to advance after resync, got %d after %d", follow.Tick, resync.Tick)
	}
	if follow.KeyframeSeq != resync.Sequence {
		t.Fatalf("expected follow-up delta to reference last resync keyframe, got %d want %d", follow.KeyframeSeq, resync.Sequence)
	}
	if bootstrap.Sequence == 0 || steady.Sequence == 0 {
		t.Fatalf("expected bootstrap and steady broadcasts to assign sequences")
	}
}

func TestHubSchedulesResyncAfterJournalHint(t *testing.T) {
	event := effectcontract.EffectUpdateEvent{Tick: 1, ID: "effect-x"}

	legacy := newHub()
	legacy.SetKeyframeInterval(5)

	legacy.mu.Lock()
	legacy.world.journal.RecordEffectUpdate(event)
	legacy.mu.Unlock()

	expectedLegacy, ok := legacy.world.journal.ConsumeResyncHint()
	if !ok {
		t.Fatalf("expected legacy journal to produce resync hint")
	}
	expected := resyncSignalFromTyped(typedEffectResyncSignalFromLegacy(expectedLegacy))

	hub := newHub()
	hub.SetKeyframeInterval(5)

	hub.mu.Lock()
	hub.world.journal.RecordEffectUpdate(event)
	hub.mu.Unlock()

	scheduled, signal := hub.scheduleResyncIfNeeded()
	if !scheduled {
		t.Fatalf("expected resync to be scheduled after journal hint")
	}
	if !reflect.DeepEqual(signal, expected) {
		t.Fatalf("unexpected resync signal from engine\nexpected: %#v\nactual:   %#v", expected, signal)
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

func TestHandleKeyframeRequestOmitsCatalogByDefault(t *testing.T) {
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

	data, err = json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("failed to encode keyframe snapshot: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("failed to decode keyframe snapshot: %v", err)
	}

	config, ok := payload["config"].(map[string]any)
	if !ok {
		t.Fatalf("expected keyframe config to decode as object, got %T", payload["config"])
	}

	if _, exists := config["effectCatalog"]; exists {
		t.Fatalf("expected keyframe config to omit effectCatalog metadata")
	}
}

func TestHandleKeyframeRequestClonesGroundItems(t *testing.T) {
	hub := newHub()
	adapter := hub.adapter
	if adapter == nil {
		t.Fatalf("expected hub adapter to be initialized")
	}

	groundItems := []itemspkg.GroundItem{{
		ID:             "ground-400",
		Type:           "relic",
		FungibilityKey: "relic-temporal",
		X:              2.5,
		Y:              -3.25,
		Qty:            2,
	}, {
		ID:             "ground-401",
		Type:           "potion",
		FungibilityKey: "potion-arcane",
		X:              -9.75,
		Y:              6.5,
		Qty:            6,
	}}
	expected := itemspkg.CloneGroundItems(groundItems)

	frame := sim.Keyframe{
		Sequence:    910,
		Tick:        1200,
		GroundItems: groundItems,
	}

	adapter.RecordKeyframe(frame)

	groundItems[0].Qty = 17
	frame.GroundItems[1].Qty = 33

	snapshot, nack, ok := hub.HandleKeyframeRequest("player-1", nil, frame.Sequence)
	if !ok {
		t.Fatalf("expected keyframe request to succeed")
	}
	if nack != nil {
		t.Fatalf("expected ack response, got nack: %+v", nack)
	}

	if !reflect.DeepEqual(expected, snapshot.GroundItems) {
		t.Fatalf("unexpected ground items in keyframe snapshot: got %#v want %#v", snapshot.GroundItems, expected)
	}

	snapshot.GroundItems[0].Qty = 49

	again, nack, ok := hub.HandleKeyframeRequest("player-1", nil, frame.Sequence)
	if !ok {
		t.Fatalf("expected second keyframe request to succeed")
	}
	if nack != nil {
		t.Fatalf("expected ack response on second fetch, got nack: %+v", nack)
	}

	if !reflect.DeepEqual(expected, again.GroundItems) {
		t.Fatalf("expected cloned ground items in second keyframe snapshot, got %#v want %#v", again.GroundItems, expected)
	}

	if &again.GroundItems[0] == &snapshot.GroundItems[0] {
		t.Fatalf("expected keyframe request to clone ground item slices on each call")
	}

	recorded, ok := hub.world.journal.KeyframeBySequence(frame.Sequence)
	if !ok {
		t.Fatalf("expected journal to contain keyframe %d", frame.Sequence)
	}
	recordedGround, ok := recorded.GroundItems.([]itemspkg.GroundItem)
	if !ok {
		t.Fatalf("expected recorded ground items to be []itemspkg.GroundItem, got %T", recorded.GroundItems)
	}

	if !reflect.DeepEqual(expected, recordedGround) {
		t.Fatalf("expected journal ground items to remain unchanged, got %#v want %#v", recordedGround, expected)
	}
}

func TestHandleKeyframeRequestClonesActors(t *testing.T) {
	hub := newHub()
	adapter := hub.adapter
	if adapter == nil {
		t.Fatalf("expected hub adapter to be initialized")
	}

	playerInventory := []sim.InventorySlot{{
		Slot: 0,
		Item: sim.ItemStack{Type: sim.ItemType("potion"), FungibilityKey: "healing", Quantity: 3},
	}, {
		Slot: 1,
		Item: sim.ItemStack{Type: sim.ItemType("arrow"), FungibilityKey: "iron", Quantity: 12},
	}}
	playerEquipment := []sim.EquippedItem{{
		Slot: sim.EquipSlotMainHand,
		Item: sim.ItemStack{Type: sim.ItemType("sword"), FungibilityKey: "steel", Quantity: 1},
	}, {
		Slot: sim.EquipSlotHead,
		Item: sim.ItemStack{Type: sim.ItemType("helm"), FungibilityKey: "iron", Quantity: 1},
	}}

	npcInventory := []sim.InventorySlot{{
		Slot: 0,
		Item: sim.ItemStack{Type: sim.ItemType("coin"), FungibilityKey: "gold", Quantity: 25},
	}}
	npcEquipment := []sim.EquippedItem{{
		Slot: sim.EquipSlotMainHand,
		Item: sim.ItemStack{Type: sim.ItemType("club"), FungibilityKey: "wood", Quantity: 1},
	}}

	players := []sim.Player{{
		Actor: sim.Actor{
			ID:        "player-400",
			X:         7.25,
			Y:         -4.75,
			Facing:    sim.FacingRight,
			Health:    68,
			MaxHealth: 90,
			Inventory: sim.Inventory{Slots: itemspkg.CloneInventorySlots(playerInventory)},
			Equipment: sim.Equipment{Slots: itemspkg.CloneEquippedItems(playerEquipment)},
		},
	}}
	npcs := []sim.NPC{{
		Actor: sim.Actor{
			ID:        "npc-400",
			X:         -3.5,
			Y:         9.25,
			Facing:    sim.FacingLeft,
			Health:    42,
			MaxHealth: 60,
			Inventory: sim.Inventory{Slots: itemspkg.CloneInventorySlots(npcInventory)},
			Equipment: sim.Equipment{Slots: itemspkg.CloneEquippedItems(npcEquipment)},
		},
		Type:             sim.NPCTypeGoblin,
		AIControlled:     true,
		ExperienceReward: 17,
	}}

	expectedPlayers := simutil.ClonePlayers(players)
	expectedNPCs := simutil.CloneNPCs(npcs)

	frame := sim.Keyframe{
		Sequence:  911,
		Tick:      1337,
		Players:   players,
		NPCs:      npcs,
		Obstacles: []sim.Obstacle{{ID: "obstacle-400", X: 1.5, Y: -2.25, Width: 2, Height: 1.5}},
	}

	adapter.RecordKeyframe(frame)

	players[0].Health = 12
	players[0].Inventory.Slots[0].Item.Quantity = 99
	frame.Players[0].Equipment.Slots[0].Item.Quantity = 44
	npcs[0].Inventory.Slots[0].Item.Quantity = 81
	frame.NPCs[0].Equipment.Slots[0].Item.Quantity = 13

	snapshot, nack, ok := hub.HandleKeyframeRequest("player-1", nil, frame.Sequence)
	if !ok {
		t.Fatalf("expected keyframe request to succeed")
	}
	if nack != nil {
		t.Fatalf("expected ack response, got nack: %+v", nack)
	}

	if len(snapshot.Players) != len(expectedPlayers) {
		t.Fatalf("expected %d players in keyframe snapshot, got %d", len(expectedPlayers), len(snapshot.Players))
	}
	if len(snapshot.NPCs) != len(expectedNPCs) {
		t.Fatalf("expected %d NPCs in keyframe snapshot, got %d", len(expectedNPCs), len(snapshot.NPCs))
	}

	if !reflect.DeepEqual(expectedPlayers, snapshot.Players) {
		t.Fatalf("unexpected keyframe players: got %#v want %#v", snapshot.Players, expectedPlayers)
	}
	if !reflect.DeepEqual(expectedNPCs, snapshot.NPCs) {
		t.Fatalf("unexpected keyframe NPCs: got %#v want %#v", snapshot.NPCs, expectedNPCs)
	}

	playerSlice := &snapshot.Players[0]
	npcSlice := &snapshot.NPCs[0]

	snapshot.Players[0].Inventory.Slots[0].Item.Quantity = 200
	snapshot.Players[0].Equipment.Slots[0].Item.Quantity = 7
	snapshot.NPCs[0].Inventory.Slots[0].Item.Quantity = 201
	snapshot.NPCs[0].Equipment.Slots[0].Item.Quantity = 8

	snapshot.Players[0].Health = 0
	snapshot.NPCs[0].Health = 0

	again, nack, ok := hub.HandleKeyframeRequest("player-1", nil, frame.Sequence)
	if !ok {
		t.Fatalf("expected second keyframe request to succeed")
	}
	if nack != nil {
		t.Fatalf("expected ack response on second fetch, got nack: %+v", nack)
	}

	if !reflect.DeepEqual(expectedPlayers, again.Players) {
		t.Fatalf("expected cloned players in second keyframe snapshot, got %#v want %#v", again.Players, expectedPlayers)
	}
	if !reflect.DeepEqual(expectedNPCs, again.NPCs) {
		t.Fatalf("expected cloned NPCs in second keyframe snapshot, got %#v want %#v", again.NPCs, expectedNPCs)
	}

	if &again.Players[0] == playerSlice {
		t.Fatalf("expected keyframe request to clone player slices on each call")
	}
	if &again.NPCs[0] == npcSlice {
		t.Fatalf("expected keyframe request to clone NPC slices on each call")
	}

	recorded, ok := hub.world.journal.KeyframeBySequence(frame.Sequence)
	if !ok {
		t.Fatalf("expected journal to contain keyframe %d", frame.Sequence)
	}

	recordedPlayers, ok := recorded.Players.([]Player)
	if !ok {
		t.Fatalf("expected recorded players to be []Player, got %T", recorded.Players)
	}
	recordedNPCs, ok := recorded.NPCs.([]NPC)
	if !ok {
		t.Fatalf("expected recorded NPCs to be []NPC, got %T", recorded.NPCs)
	}

	expectedLegacyPlayers := legacyPlayersFromSim(expectedPlayers)
	expectedLegacyNPCs := legacyNPCsFromSim(expectedNPCs)

	if !reflect.DeepEqual(expectedLegacyPlayers, recordedPlayers) {
		t.Fatalf("expected journal players to remain unchanged, got %#v want %#v", recordedPlayers, expectedLegacyPlayers)
	}
	if !reflect.DeepEqual(expectedLegacyNPCs, recordedNPCs) {
		t.Fatalf("expected journal NPCs to remain unchanged, got %#v want %#v", recordedNPCs, expectedLegacyNPCs)
	}
}

func TestHandleKeyframeRequestClonesObstacles(t *testing.T) {
	hub := newHub()
	adapter := hub.adapter
	if adapter == nil {
		t.Fatalf("expected hub adapter to be initialized")
	}

	obstacles := []sim.Obstacle{{
		ID:     "obstacle-500",
		Type:   "rock",
		X:      -4.5,
		Y:      3.25,
		Width:  2.5,
		Height: 1.25,
	}, {
		ID:     "obstacle-501",
		Type:   "tree",
		X:      6.75,
		Y:      -5.5,
		Width:  1.75,
		Height: 3.0,
	}}
	expected := simutil.CloneObstacles(obstacles)

	frame := sim.Keyframe{
		Sequence:  913,
		Tick:      3333,
		Obstacles: obstacles,
	}

	adapter.RecordKeyframe(frame)

	obstacles[0].Width = 9.5
	frame.Obstacles[1].Height = 6.5

	snapshot, nack, ok := hub.HandleKeyframeRequest("player-2", nil, frame.Sequence)
	if !ok {
		t.Fatalf("expected keyframe request to succeed")
	}
	if nack != nil {
		t.Fatalf("expected ack response, got nack: %+v", nack)
	}

	if !reflect.DeepEqual(expected, snapshot.Obstacles) {
		t.Fatalf("unexpected keyframe obstacles: got %#v want %#v", snapshot.Obstacles, expected)
	}

	first := &snapshot.Obstacles[0]
	snapshot.Obstacles[0].Width = 12.25
	snapshot.Obstacles[1].Height = 7.25

	again, nack, ok := hub.HandleKeyframeRequest("player-2", nil, frame.Sequence)
	if !ok {
		t.Fatalf("expected second keyframe request to succeed")
	}
	if nack != nil {
		t.Fatalf("expected ack response on second fetch, got nack: %+v", nack)
	}

	if !reflect.DeepEqual(expected, again.Obstacles) {
		t.Fatalf("expected cloned obstacles in second keyframe snapshot, got %#v want %#v", again.Obstacles, expected)
	}

	if &again.Obstacles[0] == first {
		t.Fatalf("expected keyframe request to clone obstacle slices on each call")
	}

	recorded, ok := hub.world.journal.KeyframeBySequence(frame.Sequence)
	if !ok {
		t.Fatalf("expected journal to contain keyframe %d", frame.Sequence)
	}

	recordedObstacles, ok := recorded.Obstacles.([]Obstacle)
	if !ok {
		t.Fatalf("expected recorded obstacles to be []Obstacle, got %T", recorded.Obstacles)
	}

	expectedLegacyObstacles := legacyObstaclesFromSim(expected)
	if !reflect.DeepEqual(expectedLegacyObstacles, recordedObstacles) {
		t.Fatalf("expected journal obstacles to remain unchanged, got %#v want %#v", recordedObstacles, expectedLegacyObstacles)
	}
}

func TestHubKeyframeCopiesConfig(t *testing.T) {
	hub := newHub()
	adapter := hub.adapter
	if adapter == nil {
		t.Fatalf("expected hub adapter to be initialized")
	}

	config := sim.WorldConfig{
		Obstacles:      true,
		ObstaclesCount: 4,
		GoldMines:      true,
		GoldMineCount:  2,
		NPCs:           true,
		GoblinCount:    3,
		RatCount:       6,
		NPCCount:       9,
		Lava:           true,
		LavaCount:      5,
		Seed:           "volcano",
		Width:          256,
		Height:         128,
	}
	expected := config

	frame := sim.Keyframe{
		Sequence: 381,
		Tick:     2701,
		Config:   config,
	}

	adapter.RecordKeyframe(frame)

	config.GoldMineCount = 77
	frame.Config.Seed = "mutated"

	snapshot, ok := hub.Keyframe(frame.Sequence)
	if !ok {
		t.Fatalf("expected hub to return keyframe %d", frame.Sequence)
	}

	if snapshot.Config != expected {
		t.Fatalf("unexpected keyframe config: got %#v want %#v", snapshot.Config, expected)
	}

	snapshot.Config.Height = 999
	snapshot.Config.LavaCount = 42

	again, ok := hub.Keyframe(frame.Sequence)
	if !ok {
		t.Fatalf("expected hub to return keyframe %d on second lookup", frame.Sequence)
	}

	if again.Config != expected {
		t.Fatalf("expected keyframe config to remain unchanged, got %#v want %#v", again.Config, expected)
	}

	recorded, ok := hub.world.journal.KeyframeBySequence(frame.Sequence)
	if !ok {
		t.Fatalf("expected journal to contain keyframe %d", frame.Sequence)
	}

	recordedConfig, ok := recorded.Config.(worldConfig)
	if !ok {
		t.Fatalf("expected recorded config to be worldConfig, got %T", recorded.Config)
	}

	expectedLegacy := legacyWorldConfigFromSim(expected)
	if !reflect.DeepEqual(expectedLegacy, recordedConfig) {
		t.Fatalf("expected journal config to remain unchanged, got %#v want %#v", recordedConfig, expectedLegacy)
	}
}

func TestHandleKeyframeRequestCopiesConfig(t *testing.T) {
	hub := newHub()
	adapter := hub.adapter
	if adapter == nil {
		t.Fatalf("expected hub adapter to be initialized")
	}

	config := sim.WorldConfig{
		Obstacles:      true,
		ObstaclesCount: 8,
		GoldMines:      true,
		GoldMineCount:  3,
		NPCs:           true,
		GoblinCount:    5,
		RatCount:       7,
		NPCCount:       12,
		Lava:           true,
		LavaCount:      4,
		Seed:           "volcano",
		Width:          256,
		Height:         144,
	}
	expected := config

	frame := sim.Keyframe{
		Sequence: 916,
		Tick:     5001,
		Config:   config,
	}

	adapter.RecordKeyframe(frame)

	config.GoblinCount = 99
	frame.Config.ObstaclesCount = 15
	frame.Config.Seed = "mutated"

	snapshot, nack, ok := hub.HandleKeyframeRequest("player-3", nil, frame.Sequence)
	if !ok {
		t.Fatalf("expected keyframe request to succeed")
	}
	if nack != nil {
		t.Fatalf("expected ack response, got nack: %+v", nack)
	}

	if snapshot.Config != expected {
		t.Fatalf("unexpected keyframe config: got %#v want %#v", snapshot.Config, expected)
	}

	snapshot.Config.GoldMineCount = 77
	snapshot.Config.Width = 999

	again, nack, ok := hub.HandleKeyframeRequest("player-3", nil, frame.Sequence)
	if !ok {
		t.Fatalf("expected second keyframe request to succeed")
	}
	if nack != nil {
		t.Fatalf("expected ack response on second fetch, got nack: %+v", nack)
	}

	if again.Config != expected {
		t.Fatalf("expected keyframe config to remain unchanged, got %#v want %#v", again.Config, expected)
	}

	recorded, ok := hub.world.journal.KeyframeBySequence(frame.Sequence)
	if !ok {
		t.Fatalf("expected journal to contain keyframe %d", frame.Sequence)
	}

	recordedConfig, ok := recorded.Config.(worldConfig)
	if !ok {
		t.Fatalf("expected recorded config to be worldConfig, got %T", recorded.Config)
	}

	expectedLegacy := legacyWorldConfigFromSim(expected)
	if !reflect.DeepEqual(expectedLegacy, recordedConfig) {
		t.Fatalf("expected journal config to remain unchanged, got %#v want %#v", recordedConfig, expectedLegacy)
	}
}
