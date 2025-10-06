package main

import "strings"

const defaultWorldSeed = "prototype"

// worldConfig captures the toggles used when generating a world.
type worldConfig struct {
	Obstacles bool   `json:"obstacles"`
	NPCs      bool   `json:"npcs"`
	Lava      bool   `json:"lava"`
	Seed      string `json:"seed"`
}

// normalized returns a config with defaults applied.
func (cfg worldConfig) normalized() worldConfig {
	normalized := cfg
	normalized.Seed = strings.TrimSpace(normalized.Seed)
	if normalized.Seed == "" {
		normalized.Seed = defaultWorldSeed
	}
	return normalized
}

// defaultWorldConfig enables every world feature and the default seed.
func defaultWorldConfig() worldConfig {
	return worldConfig{
		Obstacles: true,
		NPCs:      true,
		Lava:      true,
		Seed:      defaultWorldSeed,
	}
}
