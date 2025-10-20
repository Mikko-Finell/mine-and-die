package telemetry

import (
	"bytes"
	"log"
	"testing"

	"mine-and-die/server/logging"
)

func TestWrapLogger(t *testing.T) {
	t.Run("nil logger", func(t *testing.T) {
		logger := WrapLogger(nil)
		logger.Printf("ignored %d", 42)
	})

	t.Run("forwards to logger", func(t *testing.T) {
		var buf bytes.Buffer
		base := log.New(&buf, "", 0)
		logger := WrapLogger(base)
		logger.Printf("hello %s", "world")
		if got := buf.String(); got != "hello world\n" {
			t.Fatalf("unexpected log output: %q", got)
		}
	})
}

func TestWrapMetrics(t *testing.T) {
	metrics := logging.Metrics{}
	adapter := WrapMetrics(&metrics)

	adapter.Add("test_counter", 2)
	adapter.Store("test_counter", 5)
	adapter.Add("test_counter", 3)

	snapshot := metrics.Snapshot()
	if got := snapshot["test_counter"]; got != 8 {
		t.Fatalf("unexpected metric value: %d", got)
	}

	// Ensure nil metrics do not panic.
	var nilAdapter Metrics = WrapMetrics(nil)
	nilAdapter.Add("ignored", 1)
	nilAdapter.Store("ignored", 1)
}
