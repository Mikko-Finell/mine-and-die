package server

import (
	"encoding/json"
	"testing"
	"time"

	"mine-and-die/server/internal/sim"
)

func TestMeleeAttackCommandPipelineProducesAttackEffect(t *testing.T) {
	hub := newHub()
	hub.SetKeyframeInterval(1)

	attackerID := "attacker"
	attacker := newTestPlayerState(attackerID)
	attacker.Facing = FacingRight
	hub.world.players[attackerID] = attacker

	if _, ok, _ := hub.HandleAction(attackerID, effectTypeAttack); !ok {
		t.Fatalf("expected melee attack command to be accepted")
	}

	if hub.engine.Pending() != 1 {
		t.Fatalf("expected exactly one pending command, got %d", hub.engine.Pending())
	}

	now := time.Now()
	dt := 1.0 / float64(tickRate)
	result := hub.engine.Advance(sim.LoopTickContext{Tick: hub.tick.Add(1), Now: now, Delta: dt})
	if len(result.Commands) != 1 {
		t.Fatalf("expected exactly one command, got %d", len(result.Commands))
	}
	staged := result.Commands[0]
	if staged.Type != sim.CommandAction {
		t.Fatalf("expected pending command type %q, got %q", sim.CommandAction, staged.Type)
	}
	if staged.Action == nil {
		t.Fatalf("expected pending action payload to be populated")
	}
	if staged.Action.Name != effectTypeAttack {
		t.Fatalf("expected action name %q, got %q", effectTypeAttack, staged.Action.Name)
	}
	_, _, _, _, _ = hub.processLoopStep(result)
	if hub.engine.Pending() != 0 {
		t.Fatalf("expected command queue to be drained after advance, got %d", hub.engine.Pending())
	}

	stagedEvents := hub.world.journal.SnapshotEffectEvents()
	if len(stagedEvents.Spawns) != 1 {
		t.Fatalf("expected exactly one staged spawn event, got %d", len(stagedEvents.Spawns))
	}
	spawn := stagedEvents.Spawns[0]
	if spawn.Instance.DefinitionID != effectTypeAttack {
		t.Fatalf("expected staged spawn definition %q, got %q", effectTypeAttack, spawn.Instance.DefinitionID)
	}
	if spawn.Instance.ID == "" {
		t.Fatalf("expected staged spawn to include an instance id")
	}
	if len(stagedEvents.Updates) != 1 {
		t.Fatalf("expected exactly one staged update event, got %d", len(stagedEvents.Updates))
	}
	if stagedEvents.Updates[0].ID != spawn.Instance.ID {
		t.Fatalf("expected staged update id %q to match spawn id", stagedEvents.Updates[0].ID)
	}
	if len(stagedEvents.Ends) != 1 {
		t.Fatalf("expected exactly one staged end event, got %d", len(stagedEvents.Ends))
	}
	if stagedEvents.Ends[0].ID != spawn.Instance.ID {
		t.Fatalf("expected staged end id %q to match spawn id", stagedEvents.Ends[0].ID)
	}

	data, _, err := hub.marshalState(nil, nil, nil, nil, true, true)
	if err != nil {
		t.Fatalf("marshalState returned error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("failed to decode marshalled state: %v", err)
	}

	rawSpawns, ok := payload["effect_spawned"]
	if !ok {
		t.Fatalf("expected marshalled state to include effect_spawned events")
	}
	spawnList, ok := rawSpawns.([]any)
	if !ok {
		t.Fatalf("expected effect_spawned to decode as array, got %T", rawSpawns)
	}
	if len(spawnList) != 1 {
		t.Fatalf("expected marshalled payload to contain exactly one effect spawn, got %d", len(spawnList))
	}
	spawnPayload, ok := spawnList[0].(map[string]any)
	if !ok {
		t.Fatalf("expected spawn payload to decode as object, got %T", spawnList[0])
	}
	instancePayload, ok := spawnPayload["instance"].(map[string]any)
	if !ok {
		t.Fatalf("expected spawn instance payload to decode as object, got %T", spawnPayload["instance"])
	}
	definitionID, ok := instancePayload["definitionId"].(string)
	if !ok {
		t.Fatalf("expected spawn instance definitionId to decode as string, got %T", instancePayload["definitionId"])
	}
	if definitionID != effectTypeAttack {
		t.Fatalf("expected marshalled spawn definition %q, got %q", effectTypeAttack, definitionID)
	}

	rawUpdates, ok := payload["effect_update"]
	if !ok {
		t.Fatalf("expected marshalled state to include effect_update events")
	}
	updateList, ok := rawUpdates.([]any)
	if !ok || len(updateList) == 0 {
		t.Fatalf("expected effect_update to decode as non-empty array, got %T with len %d", rawUpdates, len(updateList))
	}

	rawEnds, ok := payload["effect_ended"]
	if !ok {
		t.Fatalf("expected marshalled state to include effect_ended events")
	}
	endList, ok := rawEnds.([]any)
	if !ok || len(endList) == 0 {
		t.Fatalf("expected effect_ended to decode as non-empty array, got %T with len %d", rawEnds, len(endList))
	}

	rawCursors, ok := payload["effect_seq_cursors"]
	if !ok {
		t.Fatalf("expected marshalled state to include effect_seq_cursors map")
	}
	if _, ok := rawCursors.(map[string]any); !ok {
		t.Fatalf("expected effect_seq_cursors to decode as object, got %T", rawCursors)
	}

	drained := hub.world.journal.SnapshotEffectEvents()
	if len(drained.Spawns) != 0 || len(drained.Updates) != 0 || len(drained.Ends) != 0 {
		t.Fatalf(
			"expected journal to be drained after marshal, got spawns=%d updates=%d ends=%d",
			len(drained.Spawns), len(drained.Updates), len(drained.Ends),
		)
	}
}
