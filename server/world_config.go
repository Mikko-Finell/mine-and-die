package main

// worldConfig captures the toggles used when generating a world.
type worldConfig struct {
	Obstacles bool `json:"obstacles"`
	NPCs      bool `json:"npcs"`
	Lava      bool `json:"lava"`
}

// defaultWorldConfig enables every world feature.
func defaultWorldConfig() worldConfig {
	return worldConfig{
		Obstacles: true,
		NPCs:      true,
		Lava:      true,
	}
}
