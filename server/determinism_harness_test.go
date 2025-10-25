package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	internaleffects "mine-and-die/server/internal/effects"
	internalruntime "mine-and-die/server/internal/effects/runtime"
	itemspkg "mine-and-die/server/internal/items"
	"mine-and-die/server/internal/sim"
	simpatches "mine-and-die/server/internal/sim/patches/typed"
	worldpkg "mine-and-die/server/internal/world"
	"mine-and-die/server/logging"
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
	Commands []sim.Command
}

func TestDeterminismHarnessGolden(t *testing.T) {
	baseline := runDeterminismHarness(t)
	assertDeterminismHarnessBaseline(t, baseline)
}

func TestDeterminismHarnessGoldenWithKeyframes(t *testing.T) {
	baseline := runDeterminismHarnessWithOptions(t, determinismHarnessOptions{recordKeyframes: true})
	assertDeterminismHarnessBaseline(t, baseline)
}

func assertDeterminismBaselineField[T comparable](t *testing.T, field string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("determinism harness drift: %s mismatch: expected %v, got %v", field, want, got)
	}
}

func assertDeterminismHarnessBaseline(t *testing.T, baseline determinismBaseline) {
	t.Helper()

	assertDeterminismBaselineField(t, "seed", baseline.Seed, determinismHarnessBaselineRecord.Seed)
	assertDeterminismBaselineField(t, "ticks", baseline.Ticks, determinismHarnessBaselineRecord.Ticks)
	assertDeterminismBaselineField(t, "patch checksum", baseline.PatchChecksum, determinismHarnessBaselineRecord.PatchChecksum)
	assertDeterminismBaselineField(t, "journal checksum", baseline.JournalChecksum, determinismHarnessBaselineRecord.JournalChecksum)
	assertDeterminismBaselineField(t, "total patches", baseline.TotalPatches, determinismHarnessBaselineRecord.TotalPatches)
	assertDeterminismBaselineField(t, "total journal events", baseline.TotalJournalEvents, determinismHarnessBaselineRecord.TotalJournalEvents)

	t.Logf("determinism harness baseline: seed=%s patch=%s journal=%s patches=%d journal_events=%d", baseline.Seed, baseline.PatchChecksum, baseline.JournalChecksum, baseline.TotalPatches, baseline.TotalJournalEvents)
}

type determinismHarnessOptions struct {
	recordKeyframes bool
}

func runDeterminismHarness(t *testing.T) determinismBaseline {
	return runDeterminismHarnessWithOptions(t, determinismHarnessOptions{})
}

func runDeterminismHarnessWithOptions(t *testing.T, opts determinismHarnessOptions) determinismBaseline {
	t.Helper()

	var harness constructorHarnessPair
	restore := interceptDeterminismConstructors(t, &harness)
	t.Cleanup(restore)

	hub := newHub()
	harness = constructorHarnessPair{}

	cfg := worldpkg.DefaultConfig()
	cfg.Seed = determinismHarnessSeed
	hub.ResetWorld(cfg)

	assertDeterminismConstructorHarnessParity(t, harness)

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
			if ok, reason := hub.engine.Enqueue(cmd); !ok {
				t.Fatalf("failed to enqueue command for tick %d: %s", idx+1, reason)
			}
		}

		current = current.Add(tickDuration)
		players, npcs, triggers, groundItems, _ := hub.advance(current, dtSeconds)

		if opts.recordKeyframes {
			simPlayers := simPlayersFromLegacy(players)
			simNPCs := simNPCsFromLegacy(npcs)
			simTriggers := internaleffects.SimEffectTriggersFromLegacy(triggers)
			clonedGroundItems := itemspkg.CloneGroundItems(groundItems)
			if _, _, err := hub.marshalState(simPlayers, simNPCs, simTriggers, clonedGroundItems, false, true); err != nil {
				t.Fatalf("failed to record keyframe during determinism harness: %v", err)
			}
		}

		patches := hub.engine.DrainPatches()
		patchEnvelope := struct {
			Tick    int         `json:"tick"`
			Patches []sim.Patch `json:"patches,omitempty"`
		}{Tick: idx + 1, Patches: patches}
		patchBytes := marshalHarnessPayload(t, patchEnvelope)
		patchHasher.Write(patchBytes)
		totalPatches += len(patches)

		journalBatch := hub.world.DrainEffectEvents()
		effectBatch := simpatches.EffectEventBatch(journalBatch)
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
		harnessTick{Commands: []sim.Command{
			{
				Type: sim.CommandMove,
				Move: &sim.MoveCommand{DX: 1, DY: 0, Facing: toSimFacing(FacingRight)},
			},
		}},
		harnessTick{Commands: []sim.Command{
			{
				Type: sim.CommandMove,
				Move: &sim.MoveCommand{DX: 0, DY: 1, Facing: toSimFacing(FacingDown)},
			},
		}},
		harnessTick{Commands: []sim.Command{
			{
				Type: sim.CommandMove,
				Move: &sim.MoveCommand{DX: -1, DY: 0, Facing: toSimFacing(FacingLeft)},
			},
		}},
		harnessTick{Commands: []sim.Command{
			{
				Type: sim.CommandMove,
				Move: &sim.MoveCommand{DX: 0, DY: -1, Facing: toSimFacing(FacingUp)},
			},
		}},
		harnessTick{Commands: []sim.Command{
			{
				Type: sim.CommandMove,
				Move: &sim.MoveCommand{DX: 0, DY: 0, Facing: toSimFacing(FacingUp)},
			},
		}},
		harnessTick{Commands: []sim.Command{
			{
				Type: sim.CommandHeartbeat,
				Heartbeat: &sim.HeartbeatCommand{
					ClientSent: baseTime.UnixMilli(),
				},
			},
		}},
	)

	return script
}

func cloneHarnessCommand(cmd sim.Command) sim.Command {
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

type constructorHarnessPair struct {
	internal worldpkg.ConstructorHarness
	legacy   worldpkg.ConstructorHarness
}

func interceptDeterminismConstructors(t *testing.T, captured *constructorHarnessPair) func() {
	t.Helper()

	worldpkg.RegisterLegacyConstructor(func(cfg worldpkg.Config, publisher logging.Publisher, deps worldpkg.Deps) worldpkg.LegacyWorld {
		if captured != nil {
			original := deps.OnConstructed
			deps.OnConstructed = func(w *worldpkg.World) {
				if captured != nil {
					captured.internal = w.ConstructorHarness()
				}
				if original != nil {
					original(w)
				}
			}
		}

		legacy := legacyConstructWorld(cfg, publisher, deps)
		if captured != nil {
			captured.legacy = legacy.ConstructorHarness()
		}
		return legacy
	})

	return func() {
		worldpkg.RegisterLegacyConstructor(func(cfg worldpkg.Config, publisher logging.Publisher, deps worldpkg.Deps) worldpkg.LegacyWorld {
			return legacyConstructWorld(cfg, publisher, deps)
		})
	}
}

func assertDeterminismConstructorHarnessParity(t *testing.T, harness constructorHarnessPair) {
	t.Helper()

	if harness.internal.RNG == nil || harness.legacy.RNG == nil {
		t.Fatalf("determinism harness: constructor harness not captured")
	}

	if harness.internal.Config != harness.legacy.Config {
		t.Fatalf("determinism harness: config mismatch: internal=%+v legacy=%+v", harness.internal.Config, harness.legacy.Config)
	}
	if harness.internal.Seed != harness.legacy.Seed {
		t.Fatalf("determinism harness: seed mismatch: internal=%q legacy=%q", harness.internal.Seed, harness.legacy.Seed)
	}
	if harness.internal.NextEffectID != harness.legacy.NextEffectID {
		t.Fatalf("determinism harness: next effect id mismatch: internal=%d legacy=%d", harness.internal.NextEffectID, harness.legacy.NextEffectID)
	}
	if harness.internal.RNG != harness.legacy.RNG {
		t.Fatalf("determinism harness: rng pointer mismatch")
	}

	assertDeterminismSharedMap(t, "players", harness.internal.Players, harness.legacy.Players)
	assertDeterminismSharedMap(t, "npcs", harness.internal.NPCs, harness.legacy.NPCs)
	assertDeterminismSharedSlice(t, "effects", harness.internal.Effects, harness.legacy.Effects)
	assertDeterminismSharedMap(t, "effectsByID", harness.internal.EffectsByID, harness.legacy.EffectsByID)
	assertDeterminismSharedIndex(t, harness.internal.EffectsIndex, harness.legacy.EffectsIndex)
	assertDeterminismSharedMap(t, "groundItems", harness.internal.GroundItems, harness.legacy.GroundItems)
	assertDeterminismSharedMap(t, "groundItemsByTile", harness.internal.GroundItemsByTile, harness.legacy.GroundItemsByTile)

	if harness.internal.Journal == nil || harness.legacy.Journal == nil {
		t.Fatalf("determinism harness: journal not captured")
	}

	assertDeterminismSharedRegistry(t, harness.internal.EffectsRegistry, harness.legacy.EffectsRegistry)
}

func assertDeterminismSharedMap[K comparable, V any](t *testing.T, name string, internal, legacy map[K]V) {
	t.Helper()
	if determinismPointerOfMap(internal) != determinismPointerOfMap(legacy) {
		t.Fatalf("determinism harness: expected %s map to be shared", name)
	}
}

func assertDeterminismSharedSlice[T any](t *testing.T, name string, internal, legacy []T) {
	t.Helper()
	if determinismPointerOfSlice(internal) != determinismPointerOfSlice(legacy) {
		t.Fatalf("determinism harness: expected %s slice to be shared", name)
	}
}

func assertDeterminismSharedIndex(t *testing.T, internal, legacy *internalruntime.SpatialIndex) {
	t.Helper()
	if internal != legacy {
		t.Fatalf("determinism harness: expected spatial index to be shared")
	}
}

func assertDeterminismSharedRegistry(t *testing.T, internal, legacy internalruntime.Registry) {
	t.Helper()

	if determinismPointerOfSlicePtr(internal.Effects) != determinismPointerOfSlicePtr(legacy.Effects) {
		t.Fatalf("determinism harness: expected registry effects slice to be shared")
	}
	if determinismPointerOfMapPtr(internal.ByID) != determinismPointerOfMapPtr(legacy.ByID) {
		t.Fatalf("determinism harness: expected registry map to be shared")
	}
	if internal.Index != legacy.Index {
		t.Fatalf("determinism harness: expected registry index to be shared")
	}
}

func determinismPointerOfMap[K comparable, V any](m map[K]V) uintptr {
	if m == nil {
		return 0
	}
	return reflect.ValueOf(m).Pointer()
}

func determinismPointerOfSlice[T any](s []T) uintptr {
	if s == nil {
		return 0
	}
	return reflect.ValueOf(s).Pointer()
}

func determinismPointerOfSlicePtr[T any](ptr *[]T) uintptr {
	if ptr == nil {
		return 0
	}
	return determinismPointerOfSlice(*ptr)
}

func determinismPointerOfMapPtr[K comparable, V any](ptr *map[K]V) uintptr {
	if ptr == nil {
		return 0
	}
	return determinismPointerOfMap(*ptr)
}
