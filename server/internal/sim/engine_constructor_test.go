package sim_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"mine-and-die/server"
	"mine-and-die/server/internal/sim"
	simpatches "mine-and-die/server/internal/sim/patches/typed"
	worldpkg "mine-and-die/server/internal/world"
	"mine-and-die/server/logging"
)

type harnessResult struct {
	patchChecksum      string
	journalChecksum    string
	totalPatches       int
	totalJournalEvents int
}

func TestEngineConstructorMatchesHubHarness(t *testing.T) {
	baseline := server.RunDeterminismHarnessBaseline(t)
	manual := runManualConstructorHarness(t)

	if manual.patchChecksum != baseline.PatchChecksum {
		t.Fatalf("patch checksum mismatch: want %s got %s", baseline.PatchChecksum, manual.patchChecksum)
	}
	if manual.journalChecksum != baseline.JournalChecksum {
		t.Fatalf("journal checksum mismatch: want %s got %s", baseline.JournalChecksum, manual.journalChecksum)
	}
	if manual.totalPatches != baseline.TotalPatches {
		t.Fatalf("total patch count mismatch: want %d got %d", baseline.TotalPatches, manual.totalPatches)
	}
	if manual.totalJournalEvents != baseline.TotalJournalEvents {
		t.Fatalf("total journal events mismatch: want %d got %d", baseline.TotalJournalEvents, manual.totalJournalEvents)
	}
}

func runManualConstructorHarness(t *testing.T) harnessResult {
	t.Helper()

	cfg := worldpkg.DefaultConfig()
	cfg.Seed = server.DeterminismHarnessSeed
	deps := worldpkg.Deps{Publisher: logging.NopPublisher{}}
	world := server.NewTestLegacyWorld(cfg, logging.NopPublisher{}, deps)
	if world == nil {
		t.Fatalf("legacy constructor returned nil world")
	}

	baseTime := time.Unix(0, 0).UTC()
	playerID := server.DeterminismHarnessPlayerID
	server.AddTestPlayer(world, server.NewTestPlayerState(playerID, baseTime))

	engine, err := sim.NewEngine(
		world,
		sim.WithDeps(sim.Deps{}),
		sim.WithLoopConfig(server.TestLoopConfig()),
	)
	if err != nil {
		t.Fatalf("failed to construct engine: %v", err)
	}

	// Clear any patches or effect events emitted during initialisation.
	_ = engine.DrainPatches()
	_ = engine.DrainEffectEvents()

	script := server.DeterminismHarnessScript(baseTime)
	tickDuration := time.Second / time.Duration(server.TickRate())
	dtSeconds := tickDuration.Seconds()

	current := baseTime
	patchHasher := sha256.New()
	journalHasher := sha256.New()
	totalPatches := 0
	totalJournal := 0

	for idx, tick := range script {
		issueAt := current
		for _, template := range tick.Commands {
			cmd := server.CloneDeterminismHarnessCommand(template)
			if cmd.ActorID == "" {
				cmd.ActorID = playerID
			}
			cmd.OriginTick = uint64(idx)
			if cmd.IssuedAt.IsZero() {
				cmd.IssuedAt = issueAt
			}
			if ok, reason := engine.Enqueue(cmd); !ok {
				t.Fatalf("enqueue failed for tick %d: %s", idx+1, reason)
			}
		}

		current = current.Add(tickDuration)
		engine.Advance(sim.LoopTickContext{Tick: uint64(idx + 1), Now: current, Delta: dtSeconds})

		patches := engine.DrainPatches()
		patchEnvelope := struct {
			Tick    int         `json:"tick"`
			Patches []sim.Patch `json:"patches,omitempty"`
		}{Tick: idx + 1, Patches: patches}
		patchBytes := marshalHarnessPayload(t, patchEnvelope)
		patchHasher.Write(patchBytes)
		totalPatches += len(patches)

		batch := engine.DrainEffectEvents()
		effectBatch := simpatches.EffectEventBatch(batch)
		journalEnvelope := struct {
			Tick  int                         `json:"tick"`
			Batch simpatches.EffectEventBatch `json:"batch"`
		}{Tick: idx + 1, Batch: effectBatch}
		journalBytes := marshalHarnessPayload(t, journalEnvelope)
		journalHasher.Write(journalBytes)
		totalJournal += len(effectBatch.Spawns) + len(effectBatch.Updates) + len(effectBatch.Ends)
	}

	return harnessResult{
		patchChecksum:      hex.EncodeToString(patchHasher.Sum(nil)),
		journalChecksum:    hex.EncodeToString(journalHasher.Sum(nil)),
		totalPatches:       totalPatches,
		totalJournalEvents: totalJournal,
	}
}

func marshalHarnessPayload(t *testing.T, payload any) []byte {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal harness payload: %v", err)
	}
	return data
}
