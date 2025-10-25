package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"hash"
	"testing"
	"time"

	internaleffects "mine-and-die/server/internal/effects"
	itemspkg "mine-and-die/server/internal/items"
	internalsim "mine-and-die/server/internal/sim"
	simpatches "mine-and-die/server/internal/sim/patches/typed"
	simutil "mine-and-die/server/internal/simutil"
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

// DeterminismHarnessOptions controls optional behaviour of the determinism harness helpers.
type DeterminismHarnessOptions struct {
	RecordKeyframes bool
}

type determinismAccumulator struct {
	patchHasher        hash.Hash
	journalHasher      hash.Hash
	totalPatches       int
	totalJournalEvents int
}

func newDeterminismAccumulator() determinismAccumulator {
	return determinismAccumulator{
		patchHasher:        sha256.New(),
		journalHasher:      sha256.New(),
		totalPatches:       0,
		totalJournalEvents: 0,
	}
}

func (a *determinismAccumulator) recordPatches(t *testing.T, tick int, patches []internalsim.Patch) {
	if a == nil || a.patchHasher == nil {
		return
	}
	envelope := struct {
		Tick    int                 `json:"tick"`
		Patches []internalsim.Patch `json:"patches,omitempty"`
	}{Tick: tick, Patches: patches}
	payload := marshalDeterminismPayload(t, envelope)
	a.patchHasher.Write(payload)
	a.totalPatches += len(patches)
}

func (a *determinismAccumulator) recordJournal(t *testing.T, tick int, batch simpatches.EffectEventBatch) {
	if a == nil || a.journalHasher == nil {
		return
	}
	envelope := struct {
		Tick  int                         `json:"tick"`
		Batch simpatches.EffectEventBatch `json:"batch"`
	}{Tick: tick, Batch: batch}
	payload := marshalDeterminismPayload(t, envelope)
	a.journalHasher.Write(payload)
	a.totalJournalEvents += len(batch.Spawns) + len(batch.Updates) + len(batch.Ends)
}

func (a *determinismAccumulator) record() DeterminismHarnessRecord {
	if a == nil {
		return DeterminismHarnessRecord{}
	}
	return DeterminismHarnessRecord{
		PatchChecksum:      hex.EncodeToString(a.patchHasher.Sum(nil)),
		JournalChecksum:    hex.EncodeToString(a.journalHasher.Sum(nil)),
		TotalPatches:       a.totalPatches,
		TotalJournalEvents: a.totalJournalEvents,
	}
}

// RunDeterminismHarness executes the harness loop with the provided options applied.
func RunDeterminismHarness(t *testing.T, opts DeterminismHarnessOptions) (DeterminismHarnessRecord, DeterminismHarnessRecord) {
	t.Helper()

	hubRecord, engineRecord := runDeterminismHarness(t, opts)
	assertDeterminismHarnessParity(t, hubRecord, engineRecord)
	return hubRecord, engineRecord
}

func runDeterminismHarness(t *testing.T, opts DeterminismHarnessOptions) (DeterminismHarnessRecord, DeterminismHarnessRecord) {
	hub := newHub()
	cfg := worldpkg.DefaultConfig()
	cfg.Seed = determinismHarnessSeedValue
	hub.ResetWorld(cfg)

	baseTime := time.Unix(0, 0).UTC()
	player := hub.seedPlayerState(determinismHarnessPlayerIDValue, baseTime)
	hub.mu.Lock()
	hub.world.AddPlayer(player)
	hub.mu.Unlock()

	engineWorld := NewTestLegacyWorld(cfg, logging.NopPublisher{}, worldpkg.Deps{Publisher: logging.NopPublisher{}})
	if engineWorld == nil {
		t.Fatalf("determinism harness: legacy constructor returned nil world")
	}
	AddTestPlayer(engineWorld, NewTestPlayerState(determinismHarnessPlayerIDValue, baseTime))

	engine, err := internalsim.NewEngine(
		engineWorld,
		internalsim.WithDeps(internalsim.Deps{}),
		internalsim.WithLoopConfig(TestLoopConfig()),
	)
	if err != nil {
		t.Fatalf("determinism harness: failed to construct engine: %v", err)
	}

	_ = engine.DrainPatches()
	_ = engine.DrainEffectEvents()

	script := DeterminismHarnessScript(baseTime)
	tickDuration := time.Second / time.Duration(tickRate)
	dtSeconds := tickDuration.Seconds()
	current := baseTime
	engineTick := uint64(0)

	hubAccumulator := newDeterminismAccumulator()
	engineAccumulator := newDeterminismAccumulator()

	for idx, tick := range script {
		issueAt := current
		for _, template := range tick.Commands {
			hubCmd := CloneDeterminismHarnessCommand(template)
			if hubCmd.ActorID == "" {
				hubCmd.ActorID = determinismHarnessPlayerIDValue
			}
			hubCmd.OriginTick = hub.tick.Load()
			if hubCmd.IssuedAt.IsZero() {
				hubCmd.IssuedAt = issueAt
			}
			if ok, reason := hub.engine.Enqueue(hubCmd); !ok {
				t.Fatalf("failed to enqueue command for tick %d: %s", idx+1, reason)
			}

			engineCmd := CloneDeterminismHarnessCommand(template)
			if engineCmd.ActorID == "" {
				engineCmd.ActorID = determinismHarnessPlayerIDValue
			}
			engineCmd.OriginTick = engineTick
			if engineCmd.IssuedAt.IsZero() {
				engineCmd.IssuedAt = issueAt
			}
			if ok, reason := engine.Enqueue(engineCmd); !ok {
				t.Fatalf("determinism harness: failed to enqueue engine command for tick %d: %s", idx+1, reason)
			}
		}

		current = current.Add(tickDuration)
		engineTick++

		players, npcs, triggers, groundItems, _ := hub.advance(current, dtSeconds)
		result := engine.Advance(internalsim.LoopTickContext{Tick: engineTick, Now: current, Delta: dtSeconds})

		if opts.RecordKeyframes {
			simPlayers := simPlayersFromLegacy(players)
			simNPCs := simNPCsFromLegacy(npcs)
			simTriggers := internaleffects.SimEffectTriggersFromLegacy(triggers)
			clonedGroundItems := itemspkg.CloneGroundItems(groundItems)
			if _, _, err := hub.marshalState(simPlayers, simNPCs, simTriggers, clonedGroundItems, false, true); err != nil {
				t.Fatalf("failed to record keyframe during determinism harness: %v", err)
			}

			snapshot := result.Snapshot
			frame := internalsim.Keyframe{
				Tick:        engineTick,
				Sequence:    engineTick,
				Players:     simutil.ClonePlayers(snapshot.Players),
				NPCs:        simutil.CloneNPCs(snapshot.NPCs),
				Obstacles:   simutil.CloneObstacles(snapshot.Obstacles),
				GroundItems: itemspkg.CloneGroundItems(snapshot.GroundItems),
				Config:      simWorldConfigFromLegacy(engineWorld.config),
				RecordedAt:  current,
			}
			engine.RecordKeyframe(frame)
		}

		hubPatches := hub.engine.DrainPatches()
		hubAccumulator.recordPatches(t, idx+1, hubPatches)

		hubBatch := hub.world.DrainEffectEvents()
		hubAccumulator.recordJournal(t, idx+1, simpatches.EffectEventBatch(hubBatch))

		enginePatches := engine.DrainPatches()
		engineAccumulator.recordPatches(t, idx+1, enginePatches)

		engineBatch := engine.DrainEffectEvents()
		engineAccumulator.recordJournal(t, idx+1, simpatches.EffectEventBatch(engineBatch))
	}

	hubRecord := hubAccumulator.record()
	engineRecord := engineAccumulator.record()

	return hubRecord, engineRecord
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

func assertDeterminismHarnessParity(t *testing.T, hubRecord, engineRecord DeterminismHarnessRecord) {
	if engineRecord.PatchChecksum != hubRecord.PatchChecksum {
		t.Fatalf("determinism harness: sim.NewEngine patch checksum mismatch: legacy=%s new=%s", hubRecord.PatchChecksum, engineRecord.PatchChecksum)
	}
	if engineRecord.JournalChecksum != hubRecord.JournalChecksum {
		t.Fatalf("determinism harness: sim.NewEngine journal checksum mismatch: legacy=%s new=%s", hubRecord.JournalChecksum, engineRecord.JournalChecksum)
	}
	if engineRecord.TotalPatches != hubRecord.TotalPatches {
		t.Fatalf("determinism harness: sim.NewEngine patch count mismatch: legacy=%d new=%d", hubRecord.TotalPatches, engineRecord.TotalPatches)
	}
	if engineRecord.TotalJournalEvents != hubRecord.TotalJournalEvents {
		t.Fatalf("determinism harness: sim.NewEngine journal count mismatch: legacy=%d new=%d", hubRecord.TotalJournalEvents, engineRecord.TotalJournalEvents)
	}
}
