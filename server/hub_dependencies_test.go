package server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	stdlog "log"
	"testing"
	"time"

	"mine-and-die/server/internal/sim"
	"mine-and-die/server/internal/telemetry"
	"mine-and-die/server/logging"
)

func TestNewHubWithConfigInjectsSimDeps(t *testing.T) {
	routerCfg := logging.DefaultConfig()
	routerCfg.EnabledSinks = nil
	routerCfg.BufferSize = 1

	router, err := logging.NewRouter(routerCfg, logging.SystemClock{}, stdlog.New(io.Discard, "", 0), nil)
	if err != nil {
		t.Fatalf("failed to construct router: %v", err)
	}
	t.Cleanup(func() {
		if cerr := router.Close(context.Background()); cerr != nil {
			t.Fatalf("failed to close router: %v", cerr)
		}
	})

	var buf bytes.Buffer
	hubCfg := DefaultHubConfig()
	hubCfg.Logger = stdlog.New(&buf, "", 0)

	hub := NewHubWithConfig(hubCfg, router)
	if hub.engine == nil {
		t.Fatalf("expected engine to be configured")
	}

	deps := hub.engine.Deps()

	if deps.Logger == nil {
		t.Fatalf("expected engine deps logger to be configured")
	}

	deps.Logger.Printf("hello %s", "world")
	if got := buf.String(); got != "hello world\n" {
		t.Fatalf("expected injected logger to capture output, got %q", got)
	}

	if deps.Metrics == nil {
		t.Fatalf("expected engine deps metrics to be configured")
	}
	deps.Metrics.Add("test_new_hub_metric", 3)
	if got := router.Metrics().Snapshot()["test_new_hub_metric"]; got != 3 {
		t.Fatalf("expected metrics adapter to forward increments, got %d", got)
	}

	if hub.telemetry == nil {
		t.Fatalf("expected telemetry counters to be configured")
	}

	if hub.telemetry.metrics != deps.Metrics {
		t.Errorf("expected telemetry counters to attach router metrics")
	}

	if deps.Clock != router.Clock() {
		t.Errorf("expected engine deps clock to mirror router clock")
	}

	if deps.RNG != hub.world.rng {
		t.Errorf("expected engine deps RNG to mirror world RNG")
	}
}

func TestNewHubWithConfigUsesConfiguredMetrics(t *testing.T) {
	routerCfg := logging.DefaultConfig()
	routerCfg.EnabledSinks = nil
	routerCfg.BufferSize = 1

	router, err := logging.NewRouter(routerCfg, logging.SystemClock{}, stdlog.New(io.Discard, "", 0), nil)
	if err != nil {
		t.Fatalf("failed to construct router: %v", err)
	}
	t.Cleanup(func() {
		if cerr := router.Close(context.Background()); cerr != nil {
			t.Fatalf("failed to close router: %v", cerr)
		}
	})

	injected := &logging.Metrics{}
	hubCfg := DefaultHubConfig()
	hubCfg.Metrics = telemetry.WrapMetrics(injected)

	hub := NewHubWithConfig(hubCfg, router)
	if hub.telemetry == nil {
		t.Fatalf("expected telemetry counters to be configured")
	}
	if hub.telemetry.metrics != hubCfg.Metrics {
		t.Fatalf("expected telemetry counters to use configured metrics")
	}

	hub.RecordTelemetryBroadcast(16, 2)

	if got := injected.Snapshot()[metricKeyBroadcastTotal]; got != 1 {
		t.Fatalf("expected configured metrics to capture broadcasts, got %d", got)
	}
	if got := router.Metrics().Snapshot()[metricKeyBroadcastTotal]; got != 0 {
		t.Fatalf("expected router metrics to remain untouched, got %d", got)
	}
}

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time { return c.now }

type stubEngine struct {
	deps sim.Deps
}

func (s *stubEngine) Deps() sim.Deps                           { return s.deps }
func (*stubEngine) Apply([]sim.Command) error                  { return nil }
func (*stubEngine) Step()                                      {}
func (*stubEngine) Snapshot() sim.Snapshot                     { return sim.Snapshot{} }
func (*stubEngine) DrainPatches() []sim.Patch                  { return nil }
func (*stubEngine) SnapshotPatches() []sim.Patch               { return nil }
func (*stubEngine) RestorePatches([]sim.Patch)                 {}
func (*stubEngine) DrainEffectEvents() sim.EffectEventBatch    { return sim.EffectEventBatch{} }
func (*stubEngine) SnapshotEffectEvents() sim.EffectEventBatch { return sim.EffectEventBatch{} }
func (*stubEngine) RestoreEffectEvents(sim.EffectEventBatch)   {}
func (*stubEngine) ConsumeEffectResyncHint() (sim.EffectResyncSignal, bool) {
	return sim.EffectResyncSignal{}, false
}
func (*stubEngine) RecordKeyframe(sim.Keyframe) sim.KeyframeRecordResult {
	return sim.KeyframeRecordResult{}
}
func (*stubEngine) KeyframeBySequence(uint64) (sim.Keyframe, bool) { return sim.Keyframe{}, false }
func (*stubEngine) KeyframeWindow() (int, uint64, uint64)          { return 0, 0, 0 }

func TestHubNowUsesEngineClock(t *testing.T) {
	hub := newHub()
	ts := time.Unix(123, 456)
	hub.engine = &stubEngine{deps: sim.Deps{Clock: fixedClock{now: ts}}}

	if got := hub.now(); !got.Equal(ts) {
		t.Fatalf("expected hub.now() to use engine clock, got %v", got)
	}
}

func TestCommandIssuedAtUsesEngineClock(t *testing.T) {
	hub := newHub()
	ts := time.Unix(789, 101112)
	hub.engine = &stubEngine{deps: sim.Deps{Clock: fixedClock{now: ts}}}

	player := hub.seedPlayerState("player-1", ts)
	hub.mu.Lock()
	hub.world.AddPlayer(player)
	hub.mu.Unlock()

	cmd, ok, reason := hub.UpdateIntent("player-1", 1, 0, "")
	if !ok {
		t.Fatalf("expected command to enqueue, got reason %q", reason)
	}
	if !cmd.IssuedAt.Equal(ts) {
		t.Fatalf("expected command IssuedAt to use engine clock, got %v", cmd.IssuedAt)
	}
}

func TestHubLogfUsesEngineLogger(t *testing.T) {
	hub := newHub()
	var buf bytes.Buffer
	logger := telemetry.LoggerFunc(func(format string, args ...any) {
		fmt.Fprintf(&buf, format, args...)
		buf.WriteByte('\n')
	})
	hub.engine = &stubEngine{deps: sim.Deps{Logger: logger}}

	hub.logf("hello %s", "world")

	if got := buf.String(); got != "hello world\n" {
		t.Fatalf("expected log output to use injected logger, got %q", got)
	}
}
