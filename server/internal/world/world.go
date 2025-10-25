package world

import (
	"math/rand"
	"os"
	"strconv"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	itemspkg "mine-and-die/server/internal/items"
	journalpkg "mine-and-die/server/internal/journal"
	state "mine-and-die/server/internal/state"
	"mine-and-die/server/logging"
)

const (
	defaultJournalKeyframeCapacity = 8
	defaultJournalKeyframeMaxAge   = 5 * time.Second
	envJournalCapacity             = "KEYFRAME_JOURNAL_CAPACITY"
	envJournalMaxAgeMS             = "KEYFRAME_JOURNAL_MAX_AGE_MS"
)

// RNGFactory produces deterministic RNG instances for world subsystems.
type RNGFactory func(rootSeed, label string) *rand.Rand

// Deps bundles runtime dependencies required to construct a World instance.
type Deps struct {
	Publisher        logging.Publisher
	RNG              RNGFactory
	JournalRetention func() (int, time.Duration)
	JournalTelemetry journalpkg.Telemetry
}

// World owns the deterministic RNG root and configuration for the simulation.
type World struct {
	config Config
	seed   string

	publisher  logging.Publisher
	rngFactory RNGFactory
	rng        *rand.Rand

	players map[string]*state.PlayerState
	npcs    map[string]*state.NPCState

	groundItems       map[string]*itemspkg.GroundItemState
	groundItemsByTile map[itemspkg.GroundTileKey]map[string]*itemspkg.GroundItemState

	statusEffectDefinitions map[string]ApplyStatusEffectDefinition

	journal journalpkg.Journal
}

// New constructs a world instance with normalized configuration and seeded RNG.
func New(cfg Config, deps Deps) (*World, error) {
	normalized := cfg.normalized()

	factory := deps.RNG
	if factory == nil {
		factory = NewDeterministicRNG
	}

	publisher := deps.Publisher
	if publisher == nil {
		publisher = logging.NopPublisher{}
	}

	seed := normalized.Seed
	if seed == "" {
		seed = DefaultSeed
	}

	capacity, maxAge := journalRetention()
	if deps.JournalRetention != nil {
		capacity, maxAge = normalizeJournalRetention(deps.JournalRetention())
	}

	world := &World{
		config:                  normalized,
		seed:                    seed,
		publisher:               publisher,
		rngFactory:              factory,
		rng:                     factory(seed, "world"),
		players:                 make(map[string]*state.PlayerState),
		npcs:                    make(map[string]*state.NPCState),
		groundItems:             make(map[string]*itemspkg.GroundItemState),
		groundItemsByTile:       make(map[itemspkg.GroundTileKey]map[string]*itemspkg.GroundItemState),
		statusEffectDefinitions: make(map[string]ApplyStatusEffectDefinition),
		journal:                 journalpkg.New(capacity, maxAge),
	}

	if deps.JournalTelemetry != nil {
		world.journal.AttachTelemetry(deps.JournalTelemetry)
	}

	return world, nil
}

// Config returns the normalized configuration captured at construction time.
func (w *World) Config() Config {
	if w == nil {
		return Config{}
	}
	return w.config
}

// Seed reports the deterministic seed applied to the world RNG hierarchy.
func (w *World) Seed() string {
	if w == nil {
		return ""
	}
	return w.seed
}

// RNG exposes the root RNG instance seeded for the world.
func (w *World) RNG() *rand.Rand {
	if w == nil {
		return nil
	}
	if w.rng == nil {
		w.rng = w.ensureFactory()(w.seed, "world")
	}
	return w.rng
}

// SubsystemRNG returns a deterministic RNG derived from the world seed.
func (w *World) SubsystemRNG(label string) *rand.Rand {
	if w == nil {
		return NewDeterministicRNG(DefaultSeed, label)
	}
	seed := w.seed
	if seed == "" {
		seed = DefaultSeed
	}
	return w.ensureFactory()(seed, label)
}

func (w *World) ensureFactory() RNGFactory {
	if w == nil || w.rngFactory == nil {
		return NewDeterministicRNG
	}
	return w.rngFactory
}

// AppendPatch records a patch for the current tick in the world journal.
func (w *World) AppendPatch(p journalpkg.Patch) {
	if w == nil {
		return
	}
	w.journal.AppendPatch(p)
}

// PurgeEntity drops staged patches referencing the provided entity ID.
func (w *World) PurgeEntity(entityID string) {
	if w == nil {
		return
	}
	w.journal.PurgeEntity(entityID)
}

// DrainPatches returns all staged patches from the journal and clears them.
func (w *World) DrainPatches() []journalpkg.Patch {
	if w == nil {
		return nil
	}
	return w.journal.DrainPatches()
}

// SnapshotPatches returns a copy of the staged patches without clearing them.
func (w *World) SnapshotPatches() []journalpkg.Patch {
	if w == nil {
		return nil
	}
	return w.journal.SnapshotPatches()
}

// RestorePatches reinserts drained patches back into the journal.
func (w *World) RestorePatches(patches []journalpkg.Patch) {
	if w == nil || len(patches) == 0 {
		return
	}
	w.journal.RestorePatches(patches)
}

// RecordEffectSpawn journals an effect spawn envelope and returns the stored copy.
func (w *World) RecordEffectSpawn(event effectcontract.EffectSpawnEvent) effectcontract.EffectSpawnEvent {
	if w == nil {
		return effectcontract.EffectSpawnEvent{}
	}
	return w.journal.RecordEffectSpawn(event)
}

// RecordEffectUpdate journals an effect update envelope and returns the stored copy.
func (w *World) RecordEffectUpdate(event effectcontract.EffectUpdateEvent) effectcontract.EffectUpdateEvent {
	if w == nil {
		return effectcontract.EffectUpdateEvent{}
	}
	return w.journal.RecordEffectUpdate(event)
}

// RecordEffectEnd journals an effect end envelope and returns the stored copy.
func (w *World) RecordEffectEnd(event effectcontract.EffectEndEvent) effectcontract.EffectEndEvent {
	if w == nil {
		return effectcontract.EffectEndEvent{}
	}
	return w.journal.RecordEffectEnd(event)
}

// DrainEffectEvents returns the staged effect lifecycle batch and clears it.
func (w *World) DrainEffectEvents() journalpkg.EffectEventBatch {
	if w == nil {
		return journalpkg.EffectEventBatch{}
	}
	return w.journal.DrainEffectEvents()
}

// SnapshotEffectEvents returns a copy of the staged effect lifecycle batch.
func (w *World) SnapshotEffectEvents() journalpkg.EffectEventBatch {
	if w == nil {
		return journalpkg.EffectEventBatch{}
	}
	return w.journal.SnapshotEffectEvents()
}

// RestoreEffectEvents reinserts a drained lifecycle batch back into the journal.
func (w *World) RestoreEffectEvents(batch journalpkg.EffectEventBatch) {
	if w == nil {
		return
	}
	w.journal.RestoreEffectEvents(batch)
}

// ConsumeResyncHint reports whether the journal observed a resync-worthy pattern.
func (w *World) ConsumeResyncHint() (journalpkg.ResyncSignal, bool) {
	if w == nil {
		return journalpkg.ResyncSignal{}, false
	}
	return w.journal.ConsumeResyncHint()
}

// RecordKeyframe stores a keyframe in the journal enforcing retention limits.
func (w *World) RecordKeyframe(frame journalpkg.Keyframe) journalpkg.KeyframeRecordResult {
	if w == nil {
		return journalpkg.KeyframeRecordResult{}
	}
	return w.journal.RecordKeyframe(frame)
}

// Keyframes returns a copy of the stored keyframes.
func (w *World) Keyframes() []journalpkg.Keyframe {
	if w == nil {
		return nil
	}
	return w.journal.Keyframes()
}

// KeyframeBySequence looks up a keyframe by sequence number.
func (w *World) KeyframeBySequence(sequence uint64) (journalpkg.Keyframe, bool) {
	if w == nil {
		return journalpkg.Keyframe{}, false
	}
	return w.journal.KeyframeBySequence(sequence)
}

// KeyframeWindow reports the current keyframe buffer size and bounds.
func (w *World) KeyframeWindow() (int, uint64, uint64) {
	if w == nil {
		return 0, 0, 0
	}
	return w.journal.KeyframeWindow()
}

// AttachJournalTelemetry wires telemetry counters into the journal.
func (w *World) AttachJournalTelemetry(t journalpkg.Telemetry) {
	if w == nil {
		return
	}
	w.journal.AttachTelemetry(t)
}

func journalRetention() (int, time.Duration) {
	capacity := defaultJournalKeyframeCapacity
	if raw := os.Getenv(envJournalCapacity); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			capacity = parsed
		}
	}

	maxAge := defaultJournalKeyframeMaxAge
	if raw := os.Getenv(envJournalMaxAgeMS); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			maxAge = time.Duration(parsed) * time.Millisecond
		}
	}

	return normalizeJournalRetention(capacity, maxAge)
}

func normalizeJournalRetention(capacity int, maxAge time.Duration) (int, time.Duration) {
	if capacity < 0 {
		capacity = 0
	}
	if maxAge < 0 {
		maxAge = 0
	}
	return capacity, maxAge
}
