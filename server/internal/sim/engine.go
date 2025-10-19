package sim

// Engine defines the minimal surface area exposed to non-simulation callers.
type Engine interface {
	Deps() Deps
	Apply([]Command) error
	Step()
	Snapshot() Snapshot
	DrainPatches() []Patch
	SnapshotPatches() []Patch
	RestorePatches([]Patch)
	DrainEffectEvents() EffectEventBatch
	SnapshotEffectEvents() EffectEventBatch
	RestoreEffectEvents(EffectEventBatch)
	ConsumeEffectResyncHint() (EffectResyncSignal, bool)
	RecordKeyframe(Keyframe) KeyframeRecordResult
	KeyframeBySequence(uint64) (Keyframe, bool)
	KeyframeWindow() (int, uint64, uint64)
}
