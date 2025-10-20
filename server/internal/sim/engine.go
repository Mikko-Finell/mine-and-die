package sim

import "time"

// Engine exposes the simulation façade, combining the core engine behaviour with
// the command queue and fixed-timestep loop orchestration.
type Engine interface {
	EngineCore
	// Enqueue stages a command for the next simulation step. It returns false
	// when the command cannot be accepted and includes a short reason string for
	// telemetry and caller feedback.
	Enqueue(Command) (bool, string)
	// Pending reports the number of staged commands waiting to be processed.
	Pending() int
	// DrainCommands clears the staged command queue and returns the drained
	// commands in submission order.
	DrainCommands() []Command
	// Advance applies the queued commands and executes a single simulation step
	// using the provided tick timing context. Callers outside the fixed loop use
	// this helper to drive deterministic test scenarios.
	Advance(LoopTickContext) LoopStepResult
	// Run drives the fixed-timestep loop until the stop channel closes.
	Run(stop <-chan struct{})
}

// EngineCore defines the behaviour provided by the underlying simulation
// engine. Implementations embed the legacy world while the façade coordinates
// queueing and loop orchestration.
type EngineCore interface {
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

// LoopTickContext carries timing metadata for a simulation step.
type LoopTickContext struct {
	Tick  uint64
	Now   time.Time
	Delta float64
}

// LoopStepResult captures the outcome of a simulation step orchestrated by the
// loop helper.
type LoopStepResult struct {
	Tick           uint64
	Now            time.Time
	Delta          float64
	Duration       time.Duration
	Budget         time.Duration
	ClampedDelta   bool
	MaxDelta       float64
	Snapshot       Snapshot
	Commands       []Command
	RemovedPlayers []string
}

// LoopHooks customises queue and loop behaviour for the embedding caller.
type LoopHooks struct {
	// NextTick returns the next simulation tick identifier. The hook is invoked
	// once per loop iteration to keep the caller's tick counter authoritative.
	NextTick func() uint64
	// Prepare executes immediately before Apply/Step, giving callers a chance to
	// populate adapter state (tick, dt, now, emit hooks).
	Prepare func(LoopTickContext)
	// AfterStep consumes the loop result after the simulation step completes. It
	// typically fans out snapshots and records telemetry.
	AfterStep func(LoopStepResult)
	// OnCommandDrop reports rejected commands so callers can record telemetry.
	OnCommandDrop func(reason string, cmd Command)
	// OnQueueWarning fires when the staged command count crosses warning
	// thresholds. Callers can use this to emit backpressure logs.
	OnQueueWarning func(length int)
}
