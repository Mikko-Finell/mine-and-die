package telemetry

import (
	"log"

	"mine-and-die/server/logging"
)

// Logger exposes the logging capabilities required by server components.
type Logger interface {
	Printf(format string, args ...any)
}

// LoggerFunc adapts functions into the Logger interface.
type LoggerFunc func(format string, args ...any)

// Printf implements Logger for LoggerFunc.
func (f LoggerFunc) Printf(format string, args ...any) {
	if f == nil {
		return
	}
	f(format, args...)
}

// WrapLogger adapts a standard library logger to the Logger interface.
func WrapLogger(logger *log.Logger) Logger {
	return &loggerAdapter{logger: logger}
}

type loggerAdapter struct {
	logger *log.Logger
}

func (l *loggerAdapter) Printf(format string, args ...any) {
	if l == nil || l.logger == nil {
		return
	}
	l.logger.Printf(format, args...)
}

// Metrics exposes the telemetry methods required by server components.
type Metrics interface {
	Add(key string, delta uint64)
	Store(key string, value uint64)
}

// WrapMetrics adapts the logging router metrics into the Metrics interface.
func WrapMetrics(metrics *logging.Metrics) Metrics {
	return &metricsAdapter{metrics: metrics}
}

type metricsAdapter struct {
	metrics *logging.Metrics
}

func (m *metricsAdapter) Add(key string, delta uint64) {
	if m == nil || m.metrics == nil {
		return
	}
	m.metrics.TelemetryAdd(key, delta)
}

func (m *metricsAdapter) Store(key string, value uint64) {
	if m == nil || m.metrics == nil {
		return
	}
	m.metrics.TelemetryStore(key, value)
}
