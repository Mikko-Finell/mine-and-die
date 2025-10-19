package main

import (
	"context"
	"io"
	stdlog "log"
	"testing"

	"mine-and-die/server/logging"
)

func TestNewHubWithConfigInjectsSimDeps(t *testing.T) {
	cfg := logging.DefaultConfig()
	cfg.EnabledSinks = nil
	cfg.BufferSize = 1

	router, err := logging.NewRouter(cfg, logging.SystemClock{}, stdlog.New(io.Discard, "", 0), nil)
	if err != nil {
		t.Fatalf("failed to construct router: %v", err)
	}
	t.Cleanup(func() {
		if cerr := router.Close(context.Background()); cerr != nil {
			t.Fatalf("failed to close router: %v", cerr)
		}
	})

	hub := newHubWithConfig(defaultHubConfig(), router)
	if hub.engine == nil {
		t.Fatalf("expected engine to be configured")
	}

	deps := hub.engine.Deps()

	if deps.Logger != stdlog.Default() {
		t.Errorf("expected engine deps logger to use stdlog")
	}

	if deps.Metrics != router.Metrics() {
		t.Errorf("expected engine deps metrics to point at router metrics")
	}

	if deps.Clock != router.Clock() {
		t.Errorf("expected engine deps clock to mirror router clock")
	}

	if deps.RNG != hub.world.rng {
		t.Errorf("expected engine deps RNG to mirror world RNG")
	}
}
