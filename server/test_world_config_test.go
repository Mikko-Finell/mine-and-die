package main

func fullyFeaturedTestWorldConfig() worldConfig {
	cfg := defaultWorldConfig()
	cfg.Width = 2400
	cfg.Height = 1800
	cfg.Obstacles = true
	cfg.ObstaclesCount = 2
	cfg.GoldMines = true
	cfg.GoldMineCount = 1
	cfg.NPCs = true
	cfg.GoblinCount = 2
	cfg.RatCount = 1
	cfg.NPCCount = cfg.GoblinCount + cfg.RatCount
	cfg.Lava = true
	cfg.LavaCount = 3
	return cfg
}

func newHubWithFullWorld() *Hub {
	hub := newHub()
	hub.ResetWorld(fullyFeaturedTestWorldConfig())
	return hub
}
