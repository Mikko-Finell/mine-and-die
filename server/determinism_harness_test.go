package server

import (
	"reflect"
	"testing"

	internalruntime "mine-and-die/server/internal/effects/runtime"
	worldpkg "mine-and-die/server/internal/world"
	"mine-and-die/server/logging"
)

const (
	expectedDeterminismHarnessSeed               = "idiom-phase-0-harness"
	expectedDeterminismHarnessTickCount          = 6
	expectedDeterminismHarnessPatchChecksum      = "540fb03fcb225bb93999fd5cf6b39d4be2b8a1177ee21a00f0362be6d4d7c85c"
	expectedDeterminismHarnessJournalChecksum    = "2c977c21fc8e7253cd5a477e83f2af0224dd409ff9c432bb8916485bedf193e2"
	expectedDeterminismHarnessTotalPatches       = 13
	expectedDeterminismHarnessTotalJournalEvents = 0
)

var expectedDeterminismHarnessRecord = DeterminismHarnessRecord{
	PatchChecksum:      expectedDeterminismHarnessPatchChecksum,
	JournalChecksum:    expectedDeterminismHarnessJournalChecksum,
	TotalPatches:       expectedDeterminismHarnessTotalPatches,
	TotalJournalEvents: expectedDeterminismHarnessTotalJournalEvents,
}

func TestDeterminismHarnessGolden(t *testing.T) {
	record := runDeterminismHarnessRecord(t)
	assertDeterminismHarnessBaseline(t, record)
}

func TestDeterminismHarnessGoldenWithKeyframes(t *testing.T) {
	record := runDeterminismHarnessRecordWithOptions(t, DeterminismHarnessOptions{RecordKeyframes: true})
	assertDeterminismHarnessBaseline(t, record)
}

func assertDeterminismBaselineField[T comparable](t *testing.T, field string, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("determinism harness drift: %s mismatch: expected %v, got %v", field, want, got)
	}
}

func assertDeterminismHarnessBaseline(t *testing.T, record DeterminismHarnessRecord) {
	t.Helper()

	assertDeterminismBaselineField(t, "seed", DeterminismHarnessSeed, expectedDeterminismHarnessSeed)
	assertDeterminismBaselineField(t, "ticks", DeterminismHarnessTickCount, expectedDeterminismHarnessTickCount)
	assertDeterminismBaselineField(t, "patch checksum", record.PatchChecksum, expectedDeterminismHarnessRecord.PatchChecksum)
	assertDeterminismBaselineField(t, "journal checksum", record.JournalChecksum, expectedDeterminismHarnessRecord.JournalChecksum)
	assertDeterminismBaselineField(t, "total patches", record.TotalPatches, expectedDeterminismHarnessRecord.TotalPatches)
	assertDeterminismBaselineField(t, "total journal events", record.TotalJournalEvents, expectedDeterminismHarnessRecord.TotalJournalEvents)

	t.Logf("determinism harness baseline: seed=%s patch=%s journal=%s patches=%d journal_events=%d", DeterminismHarnessSeed, record.PatchChecksum, record.JournalChecksum, record.TotalPatches, record.TotalJournalEvents)
}

func runDeterminismHarnessRecord(t *testing.T) DeterminismHarnessRecord {
	return runDeterminismHarnessRecordWithOptions(t, DeterminismHarnessOptions{})
}

func runDeterminismHarnessRecordWithOptions(t *testing.T, opts DeterminismHarnessOptions) DeterminismHarnessRecord {
	t.Helper()

	var harness constructorHarnessPair
	restore := interceptDeterminismConstructors(t, &harness)
	t.Cleanup(restore)

	harness = constructorHarnessPair{}

	hubRecord, _ := RunDeterminismHarness(t, opts)

	assertDeterminismConstructorHarnessParity(t, harness)

	return hubRecord
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
