package logging

import "time"

type Config struct {
	EnabledSinks     []string
	BufferSize       int
	MinimumSeverity  Severity
	Fields           map[string]any
	JSON             JSONConfig
	Console          ConsoleConfig
	DropWarnInterval time.Duration
}

type JSONConfig struct {
	FilePath      string
	MaxBatch      int
	FlushInterval time.Duration
}

type ConsoleConfig struct {
	UseColor bool
}

func DefaultConfig() Config {
	return Config{
		EnabledSinks:     []string{"console"},
		BufferSize:       512,
		MinimumSeverity:  SeverityInfo,
		DropWarnInterval: 5 * time.Second,
		JSON: JSONConfig{
			MaxBatch:      32,
			FlushInterval: 2 * time.Second,
		},
	}
}

func (c Config) HasSink(name string) bool {
	for _, s := range c.EnabledSinks {
		if s == name {
			return true
		}
	}
	return false
}

func (c Config) CloneFields() map[string]any {
	if len(c.Fields) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(c.Fields))
	for k, v := range c.Fields {
		cloned[k] = v
	}
	return cloned
}
