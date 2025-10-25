package world

import (
	"math"
	"math/rand"
	"testing"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	internalruntime "mine-and-die/server/internal/effects/runtime"
	itemspkg "mine-and-die/server/internal/items"
	journalpkg "mine-and-die/server/internal/journal"
       state "mine-and-die/server/internal/world/state"
)

func TestNewNormalizesConfigAndSeedsRNG(t *testing.T) {
	w, err := New(Config{}, Deps{})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if w == nil {
		t.Fatalf("New returned nil world")
	}

	normalized := (Config{}).normalized()
	if got := w.Config(); got != normalized {
		t.Fatalf("Config not normalized: got %+v want %+v", got, normalized)
	}

	if got := w.Seed(); got != normalized.Seed {
		t.Fatalf("Seed mismatch: got %q want %q", got, normalized.Seed)
	}

	rng := w.RNG()
	if rng == nil {
		t.Fatalf("RNG not initialized")
	}

	expected := NewDeterministicRNG(normalized.Seed, "world")
	if diff := math.Abs(rng.Float64() - expected.Float64()); diff > 1e-9 {
		t.Fatalf("world RNG not seeded deterministically: diff=%f", diff)
	}

	sub := w.SubsystemRNG("test")
	wantSub := NewDeterministicRNG(normalized.Seed, "test")
	if diff := math.Abs(sub.Float64() - wantSub.Float64()); diff > 1e-9 {
		t.Fatalf("subsystem RNG mismatch: diff=%f", diff)
	}
}

func TestNewUsesInjectedRNGFactory(t *testing.T) {
	calls := 0
	factory := func(rootSeed, label string) *rand.Rand {
		calls++
		return rand.New(rand.NewSource(123))
	}

	w, err := New(Config{Seed: "custom"}, Deps{RNG: factory})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if calls != 1 {
		t.Fatalf("expected factory to be invoked once for world RNG, got %d", calls)
	}

	_ = w.RNG()
	_ = w.SubsystemRNG("other")

	if calls < 2 {
		t.Fatalf("expected factory to be reused for subsystem RNG, got %d calls", calls)
	}
}

func TestNewInitializesPlayerAndNPCState(t *testing.T) {
	w, err := New(Config{}, Deps{})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if w.players == nil {
		t.Fatalf("players map not initialized")
	}
	if len(w.players) != 0 {
		t.Fatalf("expected no players, got %d", len(w.players))
	}

	if w.npcs == nil {
		t.Fatalf("npcs map not initialized")
	}
	if len(w.npcs) != 0 {
		t.Fatalf("expected no npcs, got %d", len(w.npcs))
	}

	candidate := &state.PlayerState{}
	w.players["player-1"] = candidate
	if w.players["player-1"] != candidate {
		t.Fatalf("players map should store PlayerState values")
	}

	npcCandidate := &state.NPCState{}
	w.npcs["npc-1"] = npcCandidate
	if w.npcs["npc-1"] != npcCandidate {
		t.Fatalf("npcs map should store NPCState values")
	}
}

func TestNewInitializesGroundItems(t *testing.T) {
	w, err := New(Config{}, Deps{})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if w.groundItems == nil {
		t.Fatalf("groundItems map not initialized")
	}
	if len(w.groundItems) != 0 {
		t.Fatalf("expected no ground items, got %d", len(w.groundItems))
	}

	if w.groundItemsByTile == nil {
		t.Fatalf("groundItemsByTile map not initialized")
	}
	if len(w.groundItemsByTile) != 0 {
		t.Fatalf("expected no ground item tiles, got %d", len(w.groundItemsByTile))
	}

	candidate := &itemspkg.GroundItemState{}
	w.groundItems["ground-1"] = candidate
	if w.groundItems["ground-1"] != candidate {
		t.Fatalf("groundItems should store GroundItemState values")
	}

	tile := itemspkg.GroundTileKey{X: 1, Y: 2}
	bucket := map[string]*itemspkg.GroundItemState{"gold": candidate}
	w.groundItemsByTile[tile] = bucket
	if w.groundItemsByTile[tile]["gold"] != candidate {
		t.Fatalf("groundItemsByTile should store bucket entries")
	}
}

func TestNewInitializesStatusEffectDefinitions(t *testing.T) {
	w, err := New(Config{}, Deps{})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if w.statusEffectDefinitions == nil {
		t.Fatalf("statusEffectDefinitions map not initialized")
	}
	if len(w.statusEffectDefinitions) != 0 {
		t.Fatalf("expected no status effect definitions, got %d", len(w.statusEffectDefinitions))
	}

	w.statusEffectDefinitions["burning"] = ApplyStatusEffectDefinition{Duration: 1}
	if def, ok := w.statusEffectDefinitions["burning"]; !ok || def.Duration != 1 {
		t.Fatalf("expected to store status effect definition in map")
	}
}

func TestNewInitializesJournalWithDefaults(t *testing.T) {
	t.Setenv(envJournalCapacity, "")
	t.Setenv(envJournalMaxAgeMS, "")

	w, err := New(Config{}, Deps{})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if size, oldest, newest := w.KeyframeWindow(); size != 0 || oldest != 0 || newest != 0 {
		t.Fatalf("expected empty journal window, got size=%d oldest=%d newest=%d", size, oldest, newest)
	}

	for seq := 1; seq <= defaultJournalKeyframeCapacity+1; seq++ {
		frame := journalpkg.Keyframe{Sequence: uint64(seq), Tick: uint64(seq)}
		result := w.RecordKeyframe(frame)
		if seq == defaultJournalKeyframeCapacity+1 {
			if result.Size != defaultJournalKeyframeCapacity {
				t.Fatalf("expected journal to retain %d frames, got %d", defaultJournalKeyframeCapacity, result.Size)
			}
			if len(result.Evicted) != 1 {
				t.Fatalf("expected single eviction when journal overflows, got %d", len(result.Evicted))
			}
			eviction := result.Evicted[0]
			if eviction.Sequence != 1 || eviction.Reason != "count" {
				t.Fatalf("unexpected eviction: %+v", eviction)
			}
		}
	}
}

func TestNewConfiguresJournalFromOverride(t *testing.T) {
	w, err := New(Config{}, Deps{JournalRetention: func() (int, time.Duration) {
		return 2, time.Millisecond
	}})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	first := w.RecordKeyframe(journalpkg.Keyframe{Sequence: 1, Tick: 1})
	if first.Size != 1 || len(first.Evicted) != 0 {
		t.Fatalf("expected first keyframe to be retained, got size=%d evicted=%d", first.Size, len(first.Evicted))
	}

	time.Sleep(2 * time.Millisecond)

	second := w.RecordKeyframe(journalpkg.Keyframe{Sequence: 2, Tick: 2})
	if len(second.Evicted) != 1 {
		t.Fatalf("expected expired keyframe to be evicted, got %d evictions", len(second.Evicted))
	}
	eviction := second.Evicted[0]
	if eviction.Sequence != 1 || eviction.Reason != "expired" {
		t.Fatalf("unexpected eviction: %+v", eviction)
	}

	third := w.RecordKeyframe(journalpkg.Keyframe{Sequence: 3, Tick: 3})
	if third.Size != 2 {
		t.Fatalf("expected journal size to respect capacity override, got %d", third.Size)
	}
}

func TestWorldJournalPatchAdapters(t *testing.T) {
	w, err := New(Config{}, Deps{})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	patch := journalpkg.Patch{Kind: journalpkg.PatchPlayerPos, EntityID: "player-1"}
	w.AppendPatch(patch)

	snapshot := w.SnapshotPatches()
	if len(snapshot) != 1 || snapshot[0].EntityID != "player-1" {
		t.Fatalf("expected snapshot to include appended patch, got %+v", snapshot)
	}

	drained := w.DrainPatches()
	if len(drained) != 1 || drained[0].EntityID != "player-1" {
		t.Fatalf("expected drained patch to match appended patch, got %+v", drained)
	}

	if again := w.DrainPatches(); len(again) != 0 {
		t.Fatalf("expected journal to be empty after drain, got %d patches", len(again))
	}

	w.RestorePatches(drained)
	restored := w.DrainPatches()
	if len(restored) != 1 || restored[0].EntityID != "player-1" {
		t.Fatalf("expected restored patches to drain, got %+v", restored)
	}

	w.RestorePatches(restored)
	w.PurgeEntity("player-1")
	if purged := w.DrainPatches(); len(purged) != 0 {
		t.Fatalf("expected purge to remove patches, got %d", len(purged))
	}
}

func TestWorldJournalEffectAdapters(t *testing.T) {
	w, err := New(Config{}, Deps{})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	spawn := effectcontract.EffectSpawnEvent{Instance: effectcontract.EffectInstance{ID: "effect-1"}, Tick: 1}
	spawned := w.RecordEffectSpawn(spawn)
	if spawned.Instance.ID != "effect-1" || spawned.Seq == 0 {
		t.Fatalf("expected spawn to be recorded with sequence, got %+v", spawned)
	}

	update := effectcontract.EffectUpdateEvent{ID: "effect-1", Tick: 2}
	updated := w.RecordEffectUpdate(update)
	if updated.ID != "effect-1" || updated.Seq <= spawned.Seq {
		t.Fatalf("expected update sequence to advance, spawn=%d update=%d", spawned.Seq, updated.Seq)
	}

	end := effectcontract.EffectEndEvent{ID: "effect-1", Tick: 3}
	ended := w.RecordEffectEnd(end)
	if ended.ID != "effect-1" || ended.Seq <= updated.Seq {
		t.Fatalf("expected end sequence to advance, update=%d end=%d", updated.Seq, ended.Seq)
	}

	snapshot := w.SnapshotEffectEvents()
	if len(snapshot.Spawns) != 1 || snapshot.Spawns[0].Instance.ID != "effect-1" {
		t.Fatalf("expected snapshot to retain spawn, got %+v", snapshot)
	}

	drained := w.DrainEffectEvents()
	if len(drained.Spawns) != 1 || len(drained.Updates) != 1 || len(drained.Ends) != 1 {
		t.Fatalf("expected lifecycle batch to drain, got %+v", drained)
	}

	if again := w.DrainEffectEvents(); len(again.Spawns) != 0 || len(again.Updates) != 0 || len(again.Ends) != 0 {
		t.Fatalf("expected lifecycle batch to be empty after drain, got %+v", again)
	}

	w.RestoreEffectEvents(drained)
	restored := w.DrainEffectEvents()
	if len(restored.Spawns) != 1 || restored.Spawns[0].Instance.ID != "effect-1" {
		t.Fatalf("expected restored events to drain, got %+v", restored)
	}
}

func TestWorldJournalResyncAndKeyframeAdapters(t *testing.T) {
	w, err := New(Config{}, Deps{})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	// Trigger a lost-spawn hint by recording an update without a prior spawn.
	dropped := w.RecordEffectUpdate(effectcontract.EffectUpdateEvent{ID: "missing", Tick: 10})
	if dropped.ID != "" {
		t.Fatalf("expected missing spawn update to be dropped, got %+v", dropped)
	}

	if signal, ok := w.ConsumeResyncHint(); !ok || signal.LostSpawns == 0 {
		t.Fatalf("expected resync hint after lost spawn, got %+v ok=%v", signal, ok)
	}

	if _, ok := w.ConsumeResyncHint(); ok {
		t.Fatalf("expected hint to clear after consumption")
	}

	frame := journalpkg.Keyframe{Sequence: 42, Tick: 100}
	result := w.RecordKeyframe(frame)
	if result.Size != 1 || len(result.Evicted) != 0 {
		t.Fatalf("expected single keyframe recorded, got %+v", result)
	}

	frames := w.Keyframes()
	if len(frames) != 1 || frames[0].Sequence != 42 {
		t.Fatalf("expected keyframes to include recorded frame, got %+v", frames)
	}

	found, ok := w.KeyframeBySequence(42)
	if !ok || found.Sequence != 42 {
		t.Fatalf("expected to lookup keyframe by sequence, got %+v ok=%v", found, ok)
	}

	if size, oldest, newest := w.KeyframeWindow(); size != 1 || oldest != 42 || newest != 42 {
		t.Fatalf("expected keyframe window to track recorded frame, got size=%d oldest=%d newest=%d", size, oldest, newest)
	}
}

func TestNewAttachesJournalTelemetry(t *testing.T) {
	telemetry := &recordingJournalTelemetry{}
	w, err := New(Config{}, Deps{JournalTelemetry: telemetry})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	dropped := w.RecordEffectUpdate(effectcontract.EffectUpdateEvent{ID: "missing", Tick: 1})
	if dropped.ID != "" {
		t.Fatalf("expected unknown update to be dropped, got %+v", dropped)
	}

	if !telemetry.recorded("journal_unknown_id_update") {
		t.Fatalf("expected telemetry to record journal drop")
	}
}

func TestAttachJournalTelemetry(t *testing.T) {
	w, err := New(Config{}, Deps{})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	telemetry := &recordingJournalTelemetry{}
	w.AttachJournalTelemetry(telemetry)

	dropped := w.RecordEffectUpdate(effectcontract.EffectUpdateEvent{ID: "missing", Tick: 1})
	if dropped.ID != "" {
		t.Fatalf("expected unknown update to be dropped, got %+v", dropped)
	}

	if !telemetry.recorded("journal_unknown_id_update") {
		t.Fatalf("expected telemetry to record journal drop")
	}
}

type recordingJournalTelemetry struct {
	metrics []string
}

func (t *recordingJournalTelemetry) RecordJournalDrop(metric string) {
	t.metrics = append(t.metrics, metric)
}

func (t *recordingJournalTelemetry) recorded(metric string) bool {
	for _, candidate := range t.metrics {
		if candidate == metric {
			return true
		}
	}
	return false
}

func TestEffectRegistryBindsWorldStorage(t *testing.T) {
	w, err := New(Config{}, Deps{})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	registry := w.EffectRegistry()
	if registry.Effects == nil || registry.ByID == nil || registry.Index == nil {
		t.Fatalf("expected registry pointers to be initialised")
	}

	effect := &internalruntime.State{ID: "effect-1"}
	if !internalruntime.RegisterEffect(registry, effect) {
		t.Fatalf("expected effect registration to succeed")
	}

	if len(w.effects) != 1 || w.effects[0] != effect {
		t.Fatalf("expected effect slice to store runtime effect, got %+v", w.effects)
	}

	if got := w.effectsByID[effect.ID]; got != effect {
		t.Fatalf("expected effect map to reference runtime effect, got %+v", got)
	}
}

func TestAbilityOwnerStateLookupPrefersPlayersAndFallsBackToNPCs(t *testing.T) {
	w, err := New(Config{}, Deps{})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	player := &state.PlayerState{
		ActorState: state.ActorState{Actor: state.Actor{ID: "actor-1"}},
		Cooldowns:  map[string]time.Time{"ability": time.Now()},
	}
	npc := &state.NPCState{
		ActorState: state.ActorState{Actor: state.Actor{ID: "actor-1"}},
		Cooldowns:  map[string]time.Time{"ability": time.Now()},
	}
	w.players[player.ID] = player
	w.npcs[npc.ID] = npc

	lookup := w.AbilityOwnerStateLookup()
	if lookup == nil {
		t.Fatalf("expected ability owner state lookup")
	}

	statePtr, cooldowns, ok := lookup("actor-1")
	if !ok || statePtr != &player.ActorState || cooldowns != &player.Cooldowns {
		t.Fatalf("expected lookup to return player state, got ok=%v state=%p cooldowns=%p", ok, statePtr, cooldowns)
	}

	delete(w.players, player.ID)

	statePtr, cooldowns, ok = lookup("actor-1")
	if !ok || statePtr != &npc.ActorState || cooldowns != &npc.Cooldowns {
		t.Fatalf("expected lookup to fall back to npc state, got ok=%v state=%p cooldowns=%p", ok, statePtr, cooldowns)
	}

	if _, _, ok := lookup("missing"); ok {
		t.Fatalf("expected missing actor lookup to fail")
	}
}

func TestProjectileStopAdapterRegistersEffects(t *testing.T) {
	w, err := New(Config{}, Deps{})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	adapter := w.ProjectileStopAdapter(ProjectileStopAdapterOptions{})
	if adapter.allocateID == nil {
		t.Fatalf("expected allocateID to be bound")
	}

	bindings := adapter.StopConfig(&internalruntime.State{}, time.Now())
	if bindings.AreaEffectSpawn.AllocateID == nil {
		t.Fatalf("expected AllocateID to be provided")
	}

	if id := bindings.AreaEffectSpawn.AllocateID(); id != "effect-1" {
		t.Fatalf("unexpected allocated id: %q", id)
	}

	if bindings.AreaEffectSpawn.Register == nil {
		t.Fatalf("expected Register to be provided")
	}

	spawned := &internalruntime.State{ID: "effect-2"}
	if !bindings.AreaEffectSpawn.Register(spawned) {
		t.Fatalf("expected spawned effect registration to succeed")
	}

	if len(w.effects) != 1 || w.effects[0] != spawned {
		t.Fatalf("expected spawned effect to be stored, got %+v", w.effects)
	}
	if ref := w.effectsByID[spawned.ID]; ref != spawned {
		t.Fatalf("expected spawned effect to be indexed, got %+v", ref)
	}
}
