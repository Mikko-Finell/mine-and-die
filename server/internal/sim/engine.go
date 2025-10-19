package sim

// Engine defines the minimal surface area exposed to non-simulation callers.
type Engine interface {
	Apply([]Command) error
	Step()
	Snapshot() Snapshot
	DrainPatches() []Patch
}
