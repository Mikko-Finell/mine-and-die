package logging

import "time"

// Config captures the runtime configuration for the logging router.
type Config struct {
	EnabledSinks []string
	BufferSize   int
	MinSeverity  Severity
	Categories   []Category

	JSON struct {
		MaxBatch      int
		FlushInterval time.Duration
		FilePath      string
	}

	Metadata map[string]string
}

// DefaultConfig returns a configuration mirroring the legacy stdout logging behaviour.
func DefaultConfig() Config {
	cfg := Config{
		EnabledSinks: []string{"console"},
		BufferSize:   1024,
		MinSeverity:  SeverityDebug,
		Categories:   nil,
		Metadata:     make(map[string]string),
	}
	cfg.JSON.MaxBatch = 1
	cfg.JSON.FlushInterval = 0
	return cfg
}

// Clock describes the time source used by the router.
type Clock interface {
	Now() time.Time
}

// SystemClock uses time.Now.
type SystemClock struct{}

// Now returns the current time.
func (SystemClock) Now() time.Time { return time.Now() }
