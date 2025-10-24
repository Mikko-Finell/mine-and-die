package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	server "mine-and-die/server"
	servernet "mine-and-die/server/internal/net"
	"mine-and-die/server/internal/observability"
	"mine-and-die/server/internal/telemetry"
	"mine-and-die/server/logging"
	loggingSinks "mine-and-die/server/logging/sinks"
)

type Config struct {
	Logger        telemetry.Logger
	Observability observability.Config
}

func Run(ctx context.Context, cfg Config) error {
	telemetryLogger := cfg.Logger
	if telemetryLogger == nil {
		telemetryLogger = telemetry.WrapLogger(log.Default())
	}

	fallbackLogger := log.Default()
	if provider, ok := telemetryLogger.(interface{ StandardLogger() *log.Logger }); ok {
		if candidate := provider.StandardLogger(); candidate != nil {
			fallbackLogger = candidate
		}
	}

	logConfig := logging.DefaultConfig()
	sinks := map[string]logging.Sink{
		"console": loggingSinks.NewConsole(os.Stdout),
	}

	router, err := logging.NewRouter(logConfig, logging.SystemClock{}, fallbackLogger, sinks)
	if err != nil {
		return fmt.Errorf("failed to construct logging router: %w", err)
	}
	defer func() {
		if cerr := router.Close(ctx); cerr != nil {
			telemetryLogger.Printf("failed to close logging router: %v", cerr)
		}
	}()

	hubCfg := server.DefaultHubConfig()
	if raw := os.Getenv("KEYFRAME_INTERVAL_TICKS"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			hubCfg.KeyframeInterval = value
		} else {
			telemetryLogger.Printf("invalid KEYFRAME_INTERVAL_TICKS=%q: %v", raw, err)
		}
	}

	hubCfg.Logger = telemetryLogger

	observabilityCfg := cfg.Observability
	if raw := os.Getenv("ENABLE_PPROF_TRACE"); raw != "" {
		if value, err := strconv.ParseBool(raw); err == nil {
			observabilityCfg.EnablePprofTrace = value
		} else {
			telemetryLogger.Printf("invalid ENABLE_PPROF_TRACE=%q: %v", raw, err)
		}
	}

	hub := server.NewHubWithConfig(hubCfg, router)
	stop := make(chan struct{})
	go hub.RunSimulation(stop)
	defer close(stop)

	clientDir := filepath.Clean(filepath.Join("..", "client"))
	handler := servernet.NewHTTPHandler(hub, servernet.HTTPHandlerConfig{
		ClientDir:     clientDir,
		Logger:        telemetryLogger,
		Observability: observabilityCfg,
	})

	srv := &http.Server{Addr: ":8080", Handler: handler}
	telemetryLogger.Printf("server listening on %s", srv.Addr)

	if err := srv.ListenAndServe(); err != nil {
		return fmt.Errorf("server failed: %w", err)
	}
	return nil
}
