package sim

// Engine defines the minimal surface area exposed to non-simulation callers.
type Engine interface {
	Apply([]Command) error
	Step()
	Snapshot() Snapshot
	DrainPatches() []Patch
	DrainEffectEvents() EffectEventBatch
	SnapshotEffectEvents() EffectEventBatch
	RestoreEffectEvents(EffectEventBatch)
	ConsumeEffectResyncHint() (EffectResyncSignal, bool)
}
