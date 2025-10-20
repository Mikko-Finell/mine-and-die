package sim

import (
	"sync"
	"time"

	"mine-and-die/server/internal/telemetry"
	"mine-and-die/server/logging"
)

const (
	// CommandRejectQueueLimit indicates a command was dropped due to per-actor
	// queue throttling.
	CommandRejectQueueLimit = "queue_limit"
	// CommandRejectQueueFull indicates the global command buffer is saturated.
	CommandRejectQueueFull = "queue_full"
)

// LoopConfig tunes the command buffer and tick loop orchestration.
type LoopConfig struct {
	TickRate        int
	CatchupMaxTicks int
	CommandCapacity int
	PerActorLimit   int
	WarningStep     int
}

// Loop coordinates command ingestion and the fixed-timestep simulation runner.
type Loop struct {
	core    EngineCore
	buffer  *CommandBuffer
	hooks   LoopHooks
	config  LoopConfig
	logger  telemetry.Logger
	metrics telemetry.Metrics

	queueMu       sync.Mutex
	perActorCount map[string]int
	dropCounts    map[string]uint64
}

// NewLoop wraps the provided engine core with a ring-buffer queue and loop.
func NewLoop(core EngineCore, cfg LoopConfig, hooks LoopHooks) *Loop {
	if core == nil {
		return nil
	}
	deps := core.Deps()
	buffer := NewCommandBuffer(cfg.CommandCapacity, deps.Metrics)
	loop := &Loop{
		core:          core,
		buffer:        buffer,
		hooks:         hooks,
		config:        cfg,
		logger:        deps.Logger,
		metrics:       deps.Metrics,
		perActorCount: make(map[string]int),
		dropCounts:    make(map[string]uint64),
	}
	return loop
}

// Deps returns the injected dependencies for the underlying engine.
func (l *Loop) Deps() Deps {
	if l == nil {
		return Deps{}
	}
	return l.core.Deps()
}

// Apply delegates to the underlying engine.
func (l *Loop) Apply(cmds []Command) error {
	if l == nil {
		return nil
	}
	return l.core.Apply(cmds)
}

// Step delegates to the underlying engine.
func (l *Loop) Step() {
	if l == nil {
		return
	}
	l.core.Step()
}

// Snapshot delegates to the underlying engine.
func (l *Loop) Snapshot() Snapshot {
	if l == nil {
		return Snapshot{}
	}
	return l.core.Snapshot()
}

// DrainPatches delegates to the underlying engine.
func (l *Loop) DrainPatches() []Patch {
	if l == nil {
		return nil
	}
	return l.core.DrainPatches()
}

// SnapshotPatches delegates to the underlying engine.
func (l *Loop) SnapshotPatches() []Patch {
	if l == nil {
		return nil
	}
	return l.core.SnapshotPatches()
}

// RestorePatches delegates to the underlying engine.
func (l *Loop) RestorePatches(patches []Patch) {
	if l == nil {
		return
	}
	l.core.RestorePatches(patches)
}

// DrainEffectEvents delegates to the underlying engine.
func (l *Loop) DrainEffectEvents() EffectEventBatch {
	if l == nil {
		return EffectEventBatch{}
	}
	return l.core.DrainEffectEvents()
}

// SnapshotEffectEvents delegates to the underlying engine.
func (l *Loop) SnapshotEffectEvents() EffectEventBatch {
	if l == nil {
		return EffectEventBatch{}
	}
	return l.core.SnapshotEffectEvents()
}

// RestoreEffectEvents delegates to the underlying engine.
func (l *Loop) RestoreEffectEvents(batch EffectEventBatch) {
	if l == nil {
		return
	}
	l.core.RestoreEffectEvents(batch)
}

// ConsumeEffectResyncHint delegates to the underlying engine.
func (l *Loop) ConsumeEffectResyncHint() (EffectResyncSignal, bool) {
	if l == nil {
		return EffectResyncSignal{}, false
	}
	return l.core.ConsumeEffectResyncHint()
}

// RecordKeyframe delegates to the underlying engine.
func (l *Loop) RecordKeyframe(frame Keyframe) KeyframeRecordResult {
	if l == nil {
		return KeyframeRecordResult{}
	}
	return l.core.RecordKeyframe(frame)
}

// KeyframeBySequence delegates to the underlying engine.
func (l *Loop) KeyframeBySequence(sequence uint64) (Keyframe, bool) {
	if l == nil {
		return Keyframe{}, false
	}
	return l.core.KeyframeBySequence(sequence)
}

// KeyframeWindow delegates to the underlying engine.
func (l *Loop) KeyframeWindow() (int, uint64, uint64) {
	if l == nil {
		return 0, 0, 0
	}
	return l.core.KeyframeWindow()
}

// Pending reports the number of staged commands.
func (l *Loop) Pending() int {
	if l == nil {
		return 0
	}
	return l.buffer.Len()
}

// DrainCommands clears the staged command queue without advancing the engine.
func (l *Loop) DrainCommands() []Command {
	if l == nil {
		return nil
	}
	return l.drainCommands()
}

// Enqueue stages a command, enforcing per-actor throttling and capacity limits.
func (l *Loop) Enqueue(cmd Command) (bool, string) {
	if l == nil {
		return false, CommandRejectQueueFull
	}
	reason := ""
	var dropCount uint64
	l.queueMu.Lock()
	if l.config.PerActorLimit > 0 && cmd.ActorID != "" {
		count := l.perActorCount[cmd.ActorID]
		if count >= l.config.PerActorLimit {
			reason = CommandRejectQueueLimit
			dropCount = l.incrementDropLocked(cmd.ActorID)
		} else {
			l.perActorCount[cmd.ActorID] = count + 1
		}
	}
	if reason == "" {
		if !l.buffer.Push(cmd) {
			reason = CommandRejectQueueLimit
			dropCount = l.incrementDropLocked(cmd.ActorID)
		} else if l.config.WarningStep > 0 {
			length := l.buffer.Len()
			if length >= l.config.WarningStep && length%l.config.WarningStep == 0 {
				l.queueMu.Unlock()
				l.warnQueue(length)
				return true, ""
			}
		}
	}
	l.queueMu.Unlock()
	if reason != "" {
		l.reportDrop(reason, cmd, dropCount)
		return false, reason
	}
	return true, ""
}

// Advance executes a single simulation step using the staged commands.
func (l *Loop) Advance(ctx LoopTickContext) LoopStepResult {
	if l == nil {
		return LoopStepResult{}
	}
	commands := l.drainCommands()
	if l.hooks.Prepare != nil {
		l.hooks.Prepare(ctx)
	}
	_ = l.core.Apply(commands)
	l.core.Step()
	result := LoopStepResult{
		Tick:           ctx.Tick,
		Now:            ctx.Now,
		Delta:          ctx.Delta,
		Snapshot:       l.core.Snapshot(),
		Commands:       commands,
		RemovedPlayers: l.removedPlayers(),
	}
	return result
}

// Run drives the fixed-timestep loop until the stop channel closes.
func (l *Loop) Run(stop <-chan struct{}) {
	if l == nil {
		return
	}
	tickRate := l.config.TickRate
	if tickRate <= 0 {
		tickRate = 15
	}
	ticker := time.NewTicker(time.Second / time.Duration(tickRate))
	defer ticker.Stop()

	deps := l.core.Deps()
	clock := deps.Clock
	if clock == nil {
		clock = logging.SystemClock{}
	}
	last := clock.Now()
	budgetSeconds := 1.0 / float64(tickRate)
	maxDt := budgetSeconds
	if l.config.CatchupMaxTicks > 1 {
		maxDt = budgetSeconds * float64(l.config.CatchupMaxTicks)
	}
	budgetDuration := time.Second / time.Duration(tickRate)

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			now := clock.Now()
			dt := now.Sub(last).Seconds()
			clamped := false
			if dt <= 0 {
				dt = budgetSeconds
			} else if dt > maxDt {
				dt = maxDt
				clamped = true
			}
			last = now

			var tick uint64
			if l.hooks.NextTick != nil {
				tick = l.hooks.NextTick()
			} else {
				tick++
			}

			start := clock.Now()
			result := l.Advance(LoopTickContext{Tick: tick, Now: now, Delta: dt})
			result.Duration = clock.Now().Sub(start)
			result.Budget = budgetDuration
			result.ClampedDelta = clamped
			result.MaxDelta = maxDt

			if l.hooks.AfterStep != nil {
				l.hooks.AfterStep(result)
			}
		}
	}
}

func (l *Loop) drainCommands() []Command {
	l.queueMu.Lock()
	defer l.queueMu.Unlock()
	commands := l.buffer.Drain()
	if len(l.perActorCount) > 0 {
		l.perActorCount = make(map[string]int)
	}
	return commands
}

func (l *Loop) removedPlayers() []string {
	if reporter, ok := l.core.(interface{ RemovedPlayers() []string }); ok {
		removed := reporter.RemovedPlayers()
		if len(removed) > 0 {
			copied := make([]string, len(removed))
			copy(copied, removed)
			return copied
		}
	}
	return nil
}

func (l *Loop) incrementDropLocked(actorID string) uint64 {
	if actorID == "" {
		return 0
	}
	count := l.dropCounts[actorID] + 1
	l.dropCounts[actorID] = count
	return count
}

func (l *Loop) warnQueue(length int) {
	if l.hooks.OnQueueWarning != nil {
		l.hooks.OnQueueWarning(length)
	}
}

func (l *Loop) reportDrop(reason string, cmd Command, count uint64) {
	if l.hooks.OnCommandDrop != nil {
		l.hooks.OnCommandDrop(reason, cmd)
	}
	if reason == CommandRejectQueueLimit && count > 0 && count&(count-1) == 0 {
		if l.logger != nil {
			l.logger.Printf(
				"[backpressure] dropping command actor=%s type=%s count=%d limit=%d",
				cmd.ActorID,
				cmd.Type,
				count,
				l.config.PerActorLimit,
			)
		}
	}
}

// Ensure Loop implements Engine.
var _ Engine = (*Loop)(nil)

// Ensure we depend on telemetry interfaces only for metric plumbing.
var _ telemetryMetrics = (telemetry.Metrics)(nil)
