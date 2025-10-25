package world_test

import (
	"reflect"
	"testing"
	"time"
	"unsafe"

	_ "mine-and-die/server"
	effectcontract "mine-and-die/server/effects/contract"
	internalruntime "mine-and-die/server/internal/effects/runtime"
	journalpkg "mine-and-die/server/internal/journal"
	worldpkg "mine-and-die/server/internal/world"
	"mine-and-die/server/logging"
)

func TestConstructLegacyRehydratesInternalState(t *testing.T) {
	cfg := worldpkg.Config{Seed: "constructor-harness", Width: 96, Height: 64}.Normalized()

	var constructed *worldpkg.World
	deps := worldpkg.Deps{
		Publisher: logging.NopPublisher{},
		JournalRetention: func() (int, time.Duration) {
			return 4, 500 * time.Millisecond
		},
		OnConstructed: func(w *worldpkg.World) {
			constructed = w
		},
	}

	legacy := worldpkg.ConstructLegacy(cfg, logging.NopPublisher{}, deps)
	if constructed == nil {
		t.Fatalf("expected OnConstructed to capture internal world")
	}
	if legacy == nil {
		t.Fatalf("ConstructLegacy returned nil legacy world")
	}

	internalHarness := constructed.ConstructorHarness()
	legacyHarness := requireLegacyHarness(t, legacy)

	if internalHarness.Config != legacyHarness.Config {
		t.Fatalf("expected legacy world config to match internal: %+v vs %+v", legacyHarness.Config, internalHarness.Config)
	}
	if internalHarness.Seed != legacyHarness.Seed {
		t.Fatalf("expected legacy seed %q, got %q", internalHarness.Seed, legacyHarness.Seed)
	}
	if internalHarness.NextEffectID != legacyHarness.NextEffectID {
		t.Fatalf("expected legacy next effect id %d, got %d", internalHarness.NextEffectID, legacyHarness.NextEffectID)
	}
	if uintptr(unsafe.Pointer(internalHarness.RNG)) != uintptr(unsafe.Pointer(legacyHarness.RNG)) {
		t.Fatalf("expected legacy RNG to reuse internal RNG instance")
	}

	assertSharedMap(t, "players", internalHarness.Players, legacyHarness.Players)
	assertSharedMap(t, "npcs", internalHarness.NPCs, legacyHarness.NPCs)
	assertSharedSlice(t, "effects", internalHarness.Effects, legacyHarness.Effects)
	assertSharedMap(t, "effectsByID", internalHarness.EffectsByID, legacyHarness.EffectsByID)
	assertSharedIndex(t, internalHarness.EffectsIndex, legacyHarness.EffectsIndex)
	assertSharedMap(t, "groundItems", internalHarness.GroundItems, legacyHarness.GroundItems)
	assertSharedMap(t, "groundItemsByTile", internalHarness.GroundItemsByTile, legacyHarness.GroundItemsByTile)

	assertSharedRegistry(t, internalHarness.EffectsRegistry, legacyHarness.EffectsRegistry)
}

func TestConstructorsJournalParity(t *testing.T) {
	cfg := worldpkg.Config{Seed: "constructor-parity", Width: 80, Height: 80}.Normalized()

	internalDeps := worldpkg.Deps{
		Publisher: logging.NopPublisher{},
		JournalRetention: func() (int, time.Duration) {
			return 8, time.Second
		},
	}

	internalWorld, err := worldpkg.New(cfg, internalDeps)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	legacy := worldpkg.ConstructLegacy(cfg, logging.NopPublisher{}, internalDeps)
	journalWorld := requireLegacyJournalWorld(t, legacy)

	patch := journalpkg.Patch{
		Kind:     journalpkg.PatchPlayerPos,
		EntityID: "player-1",
		Payload:  journalpkg.PlayerPosPayload{X: 4.5, Y: -2.25},
	}

	internalWorld.AppendPatch(patch)
	journalWorld.AppendPatch(patch)

	if snapshot := internalWorld.SnapshotPatches(); !reflect.DeepEqual(snapshot, journalWorld.SnapshotPatches()) {
		t.Fatalf("snapshot patches diverged: %+v vs %+v", snapshot, journalWorld.SnapshotPatches())
	}

	internalDrained := internalWorld.DrainPatches()
	legacyDrained := journalWorld.DrainPatches()
	if !reflect.DeepEqual(internalDrained, legacyDrained) {
		t.Fatalf("drained patches diverged: %+v vs %+v", internalDrained, legacyDrained)
	}
	if len(internalWorld.DrainPatches()) != 0 || len(journalWorld.DrainPatches()) != 0 {
		t.Fatalf("expected subsequent drain to be empty")
	}

	internalWorld.RestorePatches(internalDrained)
	journalWorld.RestorePatches(legacyDrained)
	if restored := internalWorld.DrainPatches(); !reflect.DeepEqual(restored, journalWorld.DrainPatches()) {
		t.Fatalf("restored patches diverged: %+v vs %+v", restored, journalWorld.DrainPatches())
	}

	internalWorld.RestorePatches(internalDrained)
	journalWorld.RestorePatches(legacyDrained)
	internalWorld.PurgeEntity("player-1")
	journalWorld.PurgeEntity("player-1")
	if len(internalWorld.DrainPatches()) != 0 || len(journalWorld.DrainPatches()) != 0 {
		t.Fatalf("expected purged patches to drain empty")
	}

	spawn := effectcontract.EffectSpawnEvent{
		Tick: 1,
		Instance: effectcontract.EffectInstance{
			ID:           "effect-1",
			DefinitionID: "effect.definition",
			StartTick:    1,
			DeliveryState: effectcontract.EffectDeliveryState{
				Geometry: effectcontract.EffectGeometry{Shape: effectcontract.GeometryShapeCircle},
			},
			BehaviorState: effectcontract.EffectBehaviorState{},
			Replication:   effectcontract.ReplicationSpec{SendSpawn: true, SendUpdates: true, SendEnd: true},
			End:           effectcontract.EndPolicy{Kind: effectcontract.EndDuration},
		},
	}

	internalSpawn := internalWorld.RecordEffectSpawn(spawn)
	legacySpawn := journalWorld.RecordEffectSpawn(spawn)
	if !reflect.DeepEqual(internalSpawn, legacySpawn) {
		t.Fatalf("spawn event diverged: %+v vs %+v", internalSpawn, legacySpawn)
	}

	update := effectcontract.EffectUpdateEvent{Tick: 2, ID: "effect-1", Params: map[string]int{"remaining": 42}}
	internalUpdate := internalWorld.RecordEffectUpdate(update)
	legacyUpdate := journalWorld.RecordEffectUpdate(update)
	if !reflect.DeepEqual(internalUpdate, legacyUpdate) {
		t.Fatalf("update event diverged: %+v vs %+v", internalUpdate, legacyUpdate)
	}

	end := effectcontract.EffectEndEvent{Tick: 3, ID: "effect-1", Reason: effectcontract.EndReasonExpired}
	internalEnd := internalWorld.RecordEffectEnd(end)
	legacyEnd := journalWorld.RecordEffectEnd(end)
	if !reflect.DeepEqual(internalEnd, legacyEnd) {
		t.Fatalf("end event diverged: %+v vs %+v", internalEnd, legacyEnd)
	}

	internalBatch := internalWorld.DrainEffectEvents()
	legacyBatch := journalWorld.DrainEffectEvents()
	if !reflect.DeepEqual(internalBatch, legacyBatch) {
		t.Fatalf("effect event batch diverged: %+v vs %+v", internalBatch, legacyBatch)
	}

	internalWorld.RestoreEffectEvents(internalBatch)
	journalWorld.RestoreEffectEvents(legacyBatch)
	if snapshot := internalWorld.SnapshotEffectEvents(); !reflect.DeepEqual(snapshot, journalWorld.SnapshotEffectEvents()) {
		t.Fatalf("snapshot batch diverged: %+v vs %+v", snapshot, journalWorld.SnapshotEffectEvents())
	}

	if restored := internalWorld.DrainEffectEvents(); !reflect.DeepEqual(restored, journalWorld.DrainEffectEvents()) {
		t.Fatalf("restored batch diverged: %+v vs %+v", restored, journalWorld.DrainEffectEvents())
	}

	resyncInternal, okInternal := internalWorld.ConsumeResyncHint()
	resyncLegacy, okLegacy := journalWorld.ConsumeResyncHint()
	if okInternal != okLegacy || !reflect.DeepEqual(resyncInternal, resyncLegacy) {
		t.Fatalf("resync hint diverged: %+v/%v vs %+v/%v", resyncInternal, okInternal, resyncLegacy, okLegacy)
	}
}

func requireLegacyHarness(t *testing.T, legacy worldpkg.LegacyWorld) worldpkg.ConstructorHarness {
	t.Helper()

	provider, ok := legacy.(interface {
		ConstructorHarness() worldpkg.ConstructorHarness
	})
	if !ok {
		t.Fatalf("legacy world does not expose ConstructorHarness")
	}
	return provider.ConstructorHarness()
}

func requireLegacyJournalWorld(t *testing.T, legacy worldpkg.LegacyWorld) legacyJournalWorld {
	t.Helper()

	world, ok := legacy.(legacyJournalWorld)
	if !ok {
		t.Fatalf("legacy world does not expose journal adapters")
	}
	return world
}

type legacyJournalWorld interface {
	AppendPatch(journalpkg.Patch)
	PurgeEntity(string)
	DrainPatches() []journalpkg.Patch
	SnapshotPatches() []journalpkg.Patch
	RestorePatches([]journalpkg.Patch)
	RecordEffectSpawn(effectcontract.EffectSpawnEvent) effectcontract.EffectSpawnEvent
	RecordEffectUpdate(effectcontract.EffectUpdateEvent) effectcontract.EffectUpdateEvent
	RecordEffectEnd(effectcontract.EffectEndEvent) effectcontract.EffectEndEvent
	DrainEffectEvents() journalpkg.EffectEventBatch
	SnapshotEffectEvents() journalpkg.EffectEventBatch
	RestoreEffectEvents(journalpkg.EffectEventBatch)
	ConsumeResyncHint() (journalpkg.ResyncSignal, bool)
}

func assertSharedMap[K comparable, V any](t *testing.T, name string, internal, legacy map[K]V) {
	t.Helper()
	if pointerOfMap(internal) != pointerOfMap(legacy) {
		t.Fatalf("expected %s map to be reused by legacy constructor", name)
	}
}

func assertSharedSlice[T any](t *testing.T, name string, internal, legacy []T) {
	t.Helper()
	if pointerOfSlice(internal) != pointerOfSlice(legacy) {
		t.Fatalf("expected %s slice to be reused by legacy constructor", name)
	}
}

func assertSharedIndex(t *testing.T, internal, legacy *internalruntime.SpatialIndex) {
	t.Helper()
	if internal != legacy {
		t.Fatalf("expected spatial index pointer to be reused")
	}
}

func assertSharedRegistry(t *testing.T, internal, legacy internalruntime.Registry) {
	t.Helper()

	if pointerOfSlicePtr(internal.Effects) != pointerOfSlicePtr(legacy.Effects) {
		t.Fatalf("expected registry effects slice to be reused")
	}
	if pointerOfMapPtr(internal.ByID) != pointerOfMapPtr(legacy.ByID) {
		t.Fatalf("expected registry effect map to be reused")
	}
	if internal.Index != legacy.Index {
		t.Fatalf("expected registry spatial index to be reused")
	}
}

func pointerOfMap[K comparable, V any](m map[K]V) uintptr {
	if m == nil {
		return 0
	}
	return reflect.ValueOf(m).Pointer()
}

func pointerOfSlice[T any](s []T) uintptr {
	if s == nil {
		return 0
	}
	return reflect.ValueOf(s).Pointer()
}

func pointerOfSlicePtr[T any](ptr *[]T) uintptr {
	if ptr == nil {
		return 0
	}
	return pointerOfSlice(*ptr)
}

func pointerOfMapPtr[K comparable, V any](ptr *map[K]V) uintptr {
	if ptr == nil {
		return 0
	}
	return pointerOfMap(*ptr)
}
