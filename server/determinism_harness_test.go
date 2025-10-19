package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"

	"mine-and-die/server/internal/sim"
)

const (
	determinismHarnessSeed      = "idiom-phase-0-harness"
	determinismHarnessPlayerID  = "determinism-player"
	determinismHarnessTickCount = 6

	determinismHarnessPatchChecksum   = "540fb03fcb225bb93999fd5cf6b39d4be2b8a1177ee21a00f0362be6d4d7c85c"
	determinismHarnessJournalChecksum = "2c977c21fc8e7253cd5a477e83f2af0224dd409ff9c432bb8916485bedf193e2"
)

var determinismHarnessBaselineRecord = determinismBaseline{
	Seed:               determinismHarnessSeed,
	Ticks:              determinismHarnessTickCount,
	PatchChecksum:      determinismHarnessPatchChecksum,
	JournalChecksum:    determinismHarnessJournalChecksum,
	TotalPatches:       13,
	TotalJournalEvents: 0,
}

type determinismBaseline struct {
	Seed               string
	Ticks              int
	PatchChecksum      string
	JournalChecksum    string
	TotalPatches       int
	TotalJournalEvents int
}

type harnessTick struct {
	Commands []Command
}

func TestDeterminismHarnessProducesBaseline(t *testing.T) {
	baseline := runDeterminismHarness(t)
	if baseline != determinismHarnessBaselineRecord {
		t.Fatalf("determinism harness drift: expected %+v, got %+v", determinismHarnessBaselineRecord, baseline)
	}
	t.Logf("determinism harness baseline: seed=%s patch=%s journal=%s patches=%d journal_events=%d", baseline.Seed, baseline.PatchChecksum, baseline.JournalChecksum, baseline.TotalPatches, baseline.TotalJournalEvents)
}

func runDeterminismHarness(t *testing.T) determinismBaseline {
	t.Helper()

	hub := newHub()

	cfg := defaultWorldConfig()
	cfg.Seed = determinismHarnessSeed
	hub.ResetWorld(cfg)

	baseTime := time.Unix(0, 0).UTC()

	player := hub.seedPlayerState(determinismHarnessPlayerID, baseTime)
	hub.mu.Lock()
	hub.world.AddPlayer(player)
	hub.mu.Unlock()

	script := buildDeterminismHarnessScript(baseTime)
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
			cmd := cloneHarnessCommand(template)
			if cmd.ActorID == "" {
				cmd.ActorID = determinismHarnessPlayerID
			}
			cmd.OriginTick = hub.tick.Load()
			if cmd.IssuedAt.IsZero() {
				cmd.IssuedAt = issueAt
			}
			if ok, reason := hub.enqueueCommand(cmd); !ok {
				t.Fatalf("failed to enqueue command for tick %d: %s", idx+1, reason)
			}
		}

		current = current.Add(tickDuration)
		_, _, _, _, _ = hub.advance(current, dtSeconds)

		patches := hub.engine.DrainPatches()
		patchEnvelope := struct {
			Tick    int         `json:"tick"`
			Patches []sim.Patch `json:"patches,omitempty"`
		}{Tick: idx + 1, Patches: patches}
		patchBytes := marshalHarnessPayload(t, patchEnvelope)
		patchHasher.Write(patchBytes)
		totalPatches += len(patches)

		effectBatch := hub.world.journal.DrainEffectEvents()
		journalEnvelope := struct {
			Tick  int              `json:"tick"`
			Batch EffectEventBatch `json:"batch"`
		}{Tick: idx + 1, Batch: effectBatch}
		journalBytes := marshalHarnessPayload(t, journalEnvelope)
		journalHasher.Write(journalBytes)
		totalJournalEvents += len(effectBatch.Spawns) + len(effectBatch.Updates) + len(effectBatch.Ends)
	}

	return determinismBaseline{
		Seed:               determinismHarnessSeed,
		Ticks:              len(script),
		PatchChecksum:      hex.EncodeToString(patchHasher.Sum(nil)),
		JournalChecksum:    hex.EncodeToString(journalHasher.Sum(nil)),
		TotalPatches:       totalPatches,
		TotalJournalEvents: totalJournalEvents,
	}
}

func buildDeterminismHarnessScript(baseTime time.Time) []harnessTick {
	script := make([]harnessTick, 0, determinismHarnessTickCount)

	script = append(script,
		harnessTick{Commands: []Command{
			{
				Type: CommandMove,
				Move: &MoveCommand{DX: 1, DY: 0, Facing: FacingRight},
			},
		}},
		harnessTick{Commands: []Command{
			{
				Type: CommandMove,
				Move: &MoveCommand{DX: 0, DY: 1, Facing: FacingDown},
			},
		}},
		harnessTick{Commands: []Command{
			{
				Type: CommandMove,
				Move: &MoveCommand{DX: -1, DY: 0, Facing: FacingLeft},
			},
		}},
		harnessTick{Commands: []Command{
			{
				Type: CommandMove,
				Move: &MoveCommand{DX: 0, DY: -1, Facing: FacingUp},
			},
		}},
		harnessTick{Commands: []Command{
			{
				Type: CommandMove,
				Move: &MoveCommand{DX: 0, DY: 0, Facing: FacingUp},
			},
		}},
		harnessTick{Commands: []Command{
			{
				Type: CommandHeartbeat,
				Heartbeat: &HeartbeatCommand{
					ClientSent: baseTime.UnixMilli(),
				},
			},
		}},
	)

	return script
}

func cloneHarnessCommand(cmd Command) Command {
	cloned := cmd
	if cmd.Move != nil {
		copyMove := *cmd.Move
		cloned.Move = &copyMove
	}
	if cmd.Action != nil {
		copyAction := *cmd.Action
		cloned.Action = &copyAction
	}
	if cmd.Heartbeat != nil {
		copyHeartbeat := *cmd.Heartbeat
		cloned.Heartbeat = &copyHeartbeat
	}
	if cmd.Path != nil {
		copyPath := *cmd.Path
		cloned.Path = &copyPath
	}
	return cloned
}

func marshalHarnessPayload(t *testing.T, payload any) []byte {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal harness payload: %v", err)
	}
	return data
}
