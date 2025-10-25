package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	internalsim "mine-and-die/server/internal/sim"
	simpatches "mine-and-die/server/internal/sim/patches/typed"
	state "mine-and-die/server/internal/state"
	worldpkg "mine-and-die/server/internal/world"
	"mine-and-die/server/logging"
)

const (
	determinismHarnessSeedValue      = "idiom-phase-0-harness"
	determinismHarnessPlayerIDValue  = "determinism-player"
	determinismHarnessTickCountValue = 6
)

// NewTestHub exposes the unexported hub constructor for deterministic harness tests.
func NewTestHub(pubs ...logging.Publisher) *Hub {
	return newHub(pubs...)
}

// NewTestLegacyWorld constructs a legacy world instance using the shared constructor.
func NewTestLegacyWorld(cfg worldpkg.Config, publisher logging.Publisher, deps worldpkg.Deps) *World {
	return legacyConstructWorld(cfg, publisher, deps)
}

// NewTestLegacyEngineAdapter exposes the legacy engine adapter for harness parity checks.
func NewTestLegacyEngineAdapter(world *World, deps internalsim.Deps) internalsim.EngineCore {
	return newLegacyEngineAdapter(world, deps)
}

// NewTestPlayerState seeds a player using the legacy helper for deterministic tests.
func NewTestPlayerState(id string, now time.Time) *state.PlayerState {
	hub := &Hub{publisher: logging.NopPublisher{}}
	return (*state.PlayerState)(hub.seedPlayerState(id, now))
}

// AddTestPlayer registers a seeded player state with the provided world instance.
func AddTestPlayer(world *World, player *state.PlayerState) {
	if world == nil || player == nil {
		return
	}
	world.AddPlayer((*playerState)(player))
}

// TestLoopConfig returns the loop configuration used by the production hub.
func TestLoopConfig() internalsim.LoopConfig {
	return internalsim.LoopConfig{
		TickRate:        tickRate,
		CatchupMaxTicks: tickBudgetCatchupMaxTicks,
		CommandCapacity: commandBufferCapacity,
		PerActorLimit:   commandQueuePerActorLimit,
		WarningStep:     commandQueueWarningStep,
	}
}

// DeterminismHarnessTick captures the commands issued during a deterministic harness tick.
type DeterminismHarnessTick struct {
	Commands []internalsim.Command
}

// DeterminismHarnessScript returns the fixed command script exercised by the determinism harness.
func DeterminismHarnessScript(baseTime time.Time) []DeterminismHarnessTick {
	return []DeterminismHarnessTick{
		{Commands: []internalsim.Command{{
			Type: internalsim.CommandMove,
			Move: &internalsim.MoveCommand{DX: 1, DY: 0, Facing: internalsim.FacingRight},
		}}},
		{Commands: []internalsim.Command{{
			Type: internalsim.CommandMove,
			Move: &internalsim.MoveCommand{DX: 0, DY: 1, Facing: internalsim.FacingDown},
		}}},
		{Commands: []internalsim.Command{{
			Type: internalsim.CommandMove,
			Move: &internalsim.MoveCommand{DX: -1, DY: 0, Facing: internalsim.FacingLeft},
		}}},
		{Commands: []internalsim.Command{{
			Type: internalsim.CommandMove,
			Move: &internalsim.MoveCommand{DX: 0, DY: -1, Facing: internalsim.FacingUp},
		}}},
		{Commands: []internalsim.Command{{
			Type: internalsim.CommandMove,
			Move: &internalsim.MoveCommand{DX: 0, DY: 0, Facing: internalsim.FacingUp},
		}}},
		{Commands: []internalsim.Command{{
			Type: internalsim.CommandHeartbeat,
			Heartbeat: &internalsim.HeartbeatCommand{
				ClientSent: baseTime.UnixMilli(),
			},
		}}},
	}
}

// CloneDeterminismHarnessCommand returns a deep copy of the provided harness command template.
func CloneDeterminismHarnessCommand(cmd internalsim.Command) internalsim.Command {
	cloned := cmd
	if cmd.Move != nil {
		move := *cmd.Move
		cloned.Move = &move
	}
	if cmd.Action != nil {
		action := *cmd.Action
		cloned.Action = &action
	}
	if cmd.Heartbeat != nil {
		heartbeat := *cmd.Heartbeat
		cloned.Heartbeat = &heartbeat
	}
	if cmd.Path != nil {
		path := *cmd.Path
		cloned.Path = &path
	}
	return cloned
}

// DeterminismHarnessRecord captures the checksum and totals produced by the hub harness.
type DeterminismHarnessRecord struct {
	PatchChecksum      string
	JournalChecksum    string
	TotalPatches       int
	TotalJournalEvents int
}

// RunDeterminismHarnessBaseline executes the hub-driven determinism harness and returns the summary record.
func RunDeterminismHarnessBaseline(t *testing.T) DeterminismHarnessRecord {
	t.Helper()

	hub := newHub()
	cfg := worldpkg.DefaultConfig()
	cfg.Seed = determinismHarnessSeedValue
	hub.ResetWorld(cfg)

	baseTime := time.Unix(0, 0).UTC()
	player := hub.seedPlayerState(determinismHarnessPlayerIDValue, baseTime)
	hub.mu.Lock()
	hub.world.AddPlayer(player)
	hub.mu.Unlock()

	script := DeterminismHarnessScript(baseTime)
	tickDuration := time.Second / time.Duration(tickRate)
	dtSeconds := tickDuration.Seconds()
	current := baseTime

	patchHasher := sha256.New()
	journalHasher := sha256.New()
	totalPatches := 0
	totalJournalEvents := 0

	for idx, tick := range script {
		issueAt := current
		for _, template := range tick.Commands {
			cmd := CloneDeterminismHarnessCommand(template)
			if cmd.ActorID == "" {
				cmd.ActorID = determinismHarnessPlayerIDValue
			}
			cmd.OriginTick = hub.tick.Load()
			if cmd.IssuedAt.IsZero() {
				cmd.IssuedAt = issueAt
			}
			if ok, reason := hub.engine.Enqueue(cmd); !ok {
				t.Fatalf("failed to enqueue command for tick %d: %s", idx+1, reason)
			}
		}

		current = current.Add(tickDuration)
		hub.advance(current, dtSeconds)

		patches := hub.engine.DrainPatches()
		patchEnvelope := struct {
			Tick    int                 `json:"tick"`
			Patches []internalsim.Patch `json:"patches,omitempty"`
		}{Tick: idx + 1, Patches: patches}
		patchBytes := marshalDeterminismPayload(t, patchEnvelope)
		patchHasher.Write(patchBytes)
		totalPatches += len(patches)

		journalBatch := hub.world.DrainEffectEvents()
		effectBatch := simpatches.EffectEventBatch(journalBatch)
		journalEnvelope := struct {
			Tick  int                         `json:"tick"`
			Batch simpatches.EffectEventBatch `json:"batch"`
		}{Tick: idx + 1, Batch: effectBatch}
		journalBytes := marshalDeterminismPayload(t, journalEnvelope)
		journalHasher.Write(journalBytes)
		totalJournalEvents += len(effectBatch.Spawns) + len(effectBatch.Updates) + len(effectBatch.Ends)
	}

	return DeterminismHarnessRecord{
		PatchChecksum:      hex.EncodeToString(patchHasher.Sum(nil)),
		JournalChecksum:    hex.EncodeToString(journalHasher.Sum(nil)),
		TotalPatches:       totalPatches,
		TotalJournalEvents: totalJournalEvents,
	}
}

func marshalDeterminismPayload(t *testing.T, payload any) []byte {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal harness payload: %v", err)
	}
	return data
}

// DeterminismHarnessSeed exposes the fixed seed used by the determinism harness.
const DeterminismHarnessSeed = determinismHarnessSeedValue

// DeterminismHarnessPlayerID exposes the fixed player identifier used by the determinism harness.
const DeterminismHarnessPlayerID = determinismHarnessPlayerIDValue

// DeterminismHarnessTickCount exposes the fixed tick count used by the determinism harness.
const DeterminismHarnessTickCount = determinismHarnessTickCountValue
