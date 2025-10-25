package sim

import (
	"errors"
	"math/rand"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

var (
	// ErrMissingWorld indicates NewEngine was invoked without a world instance.
	ErrMissingWorld = errors.New("sim: world is nil")
	// ErrUnsupportedWorld indicates the provided world cannot produce an engine core.
	ErrUnsupportedWorld = errors.New("sim: world does not provide an engine adapter")
	// ErrMissingEngineCore indicates the adapter factory returned a nil engine core.
	ErrMissingEngineCore = errors.New("sim: engine adapter returned nil")
)

// EngineOption configures NewEngine behaviour.
//
// The option surface is intentionally broad so the follow-up changes can thread
// queue sizing, loop hooks, and journal/keyframe integration without reshaping
// call sites repeatedly while the hub façade is removed.
//
// Options are applied in order; later options override earlier ones.
type EngineOption interface {
	apply(*engineConfig)
}

type engineOptionFunc func(*engineConfig)

func (f engineOptionFunc) apply(cfg *engineConfig) {
	if f != nil {
		f(cfg)
	}
}

// EngineLoopHooks describes the loop orchestration callbacks exposed by
// NewEngine. It mirrors LoopHooks so callers can customise tick sequencing and
// telemetry fan-out when composing the runtime outside the legacy hub.
type EngineLoopHooks struct {
	LoopHooks
}

// EngineJournalHooks exposes callbacks triggered when the engine interacts with
// the underlying journal. Future changes will route telemetry and replication
// bookkeeping through these hooks as the hub wiring migrates to the internal
// entry point.
type EngineJournalHooks struct {
	// OnRecord is invoked after the engine persists a keyframe. The callback
	// receives the recorded frame and the journal response so callers can
	// emit telemetry without peeking into engine internals.
	OnRecord func(Keyframe, KeyframeRecordResult)
}

// EngineConfig captures the aggregated option state after applying all
// EngineOption values.
type engineConfig struct {
	deps         Deps
	loopConfig   LoopConfig
	loopHooks    EngineLoopHooks
	journalHooks EngineJournalHooks
}

// EngineWorld represents the world implementation consumed by NewEngine. The
// placeholder interface keeps the constructor available while the world package
// still imports sim via the legacy adapters. Follow-up work will narrow this to
// the concrete *world.World type once the dependency cycle is resolved.
type EngineWorld interface{}

type engineAdapterProvider interface {
	EngineAdapter(Deps) EngineCore
}

type engineRNGProvider interface {
	EngineRNG() *rand.Rand
}

// WithDeps injects shared infrastructure dependencies used by the engine core
// and loop orchestration.
func WithDeps(deps Deps) EngineOption {
	return engineOptionFunc(func(cfg *engineConfig) {
		cfg.deps = deps
	})
}

// WithLoopConfig overrides the default command queue and tick loop sizing used
// by the engine.
func WithLoopConfig(config LoopConfig) EngineOption {
	return engineOptionFunc(func(cfg *engineConfig) {
		cfg.loopConfig = config
	})
}

// WithLoopHooks supplies custom loop callbacks.
func WithLoopHooks(hooks EngineLoopHooks) EngineOption {
	return engineOptionFunc(func(cfg *engineConfig) {
		cfg.loopHooks = hooks
	})
}

// WithJournalHooks registers callbacks to observe journal activity produced by
// the engine.
func WithJournalHooks(hooks EngineJournalHooks) EngineOption {
	return engineOptionFunc(func(cfg *engineConfig) {
		cfg.journalHooks = hooks
	})
}

// NewEngine constructs an Engine instance backed by the provided world and
// configures the loop and journaling hooks described by the supplied options.
func NewEngine(world EngineWorld, opts ...EngineOption) (Engine, error) {
	if world == nil {
		return nil, ErrMissingWorld
	}

	cfg := engineConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt.apply(&cfg)
		}
	}

	if cfg.deps.RNG == nil {
		if provider, ok := world.(engineRNGProvider); ok {
			cfg.deps.RNG = provider.EngineRNG()
		}
	}

	var core EngineCore
	switch candidate := world.(type) {
	case engineAdapterProvider:
		core = candidate.EngineAdapter(cfg.deps)
	case EngineCore:
		core = candidate
	default:
		return nil, ErrUnsupportedWorld
	}
	if core == nil {
		return nil, ErrMissingEngineCore
	}

	hooks := cfg.loopHooks.LoopHooks
	if preparer, ok := core.(interface {
		PrepareStep(uint64, time.Time, float64, func(effectcontract.EffectLifecycleEvent))
	}); ok {
		userPrepare := hooks.Prepare
		hooks.Prepare = func(ctx LoopTickContext) {
			preparer.PrepareStep(ctx.Tick, ctx.Now, ctx.Delta, nil)
			if userPrepare != nil {
				userPrepare(ctx)
			}
		}
	}

	if cfg.journalHooks.OnRecord != nil {
		core = &journalHookedCore{EngineCore: core, hooks: cfg.journalHooks}
	}

	engine := NewLoop(core, cfg.loopConfig, hooks)
	if engine == nil {
		return nil, ErrMissingEngineCore
	}

	return engine, nil
}

type journalHookedCore struct {
	EngineCore
	hooks EngineJournalHooks
}

func (c *journalHookedCore) RecordKeyframe(frame Keyframe) KeyframeRecordResult {
	if c == nil || c.EngineCore == nil {
		return KeyframeRecordResult{}
	}
	result := c.EngineCore.RecordKeyframe(frame)
	if c.hooks.OnRecord != nil {
		c.hooks.OnRecord(frame, result)
	}
	return result
}
