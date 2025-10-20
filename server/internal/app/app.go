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
	"mine-and-die/server/logging"
	loggingSinks "mine-and-die/server/logging/sinks"
)

func Run(ctx context.Context) error {
	logger := log.Default()

	logConfig := logging.DefaultConfig()
	sinks := map[string]logging.Sink{
		"console": loggingSinks.NewConsole(os.Stdout),
	}

	router, err := logging.NewRouter(logConfig, logging.SystemClock{}, logger, sinks)
	if err != nil {
		return fmt.Errorf("failed to construct logging router: %w", err)
	}
	defer func() {
		if cerr := router.Close(ctx); cerr != nil {
			logger.Printf("failed to close logging router: %v", cerr)
		}
	}()

	hubCfg := server.DefaultHubConfig()
	if raw := os.Getenv("KEYFRAME_INTERVAL_TICKS"); raw != "" {
		if value, err := strconv.Atoi(raw); err == nil {
			hubCfg.KeyframeInterval = value
		} else {
			logger.Printf("invalid KEYFRAME_INTERVAL_TICKS=%q: %v", raw, err)
		}
	}

	hub := server.NewHubWithConfig(hubCfg, router)
	stop := make(chan struct{})
	go hub.RunSimulation(stop)
	defer close(stop)

	clientDir := filepath.Clean(filepath.Join("..", "client"))
	handler := server.NewHTTPHandler(hub, server.HTTPHandlerConfig{
		ClientDir: clientDir,
		Logger:    logger,
	})

	srv := &http.Server{Addr: ":8080", Handler: handler}
	logger.Printf("server listening on %s", srv.Addr)

	if err := srv.ListenAndServe(); err != nil {
		return fmt.Errorf("server failed: %w", err)
	}
	return nil
}
