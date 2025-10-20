package world

func Width(cfg Config) float64 {
	if cfg.Width > 0 {
		return cfg.Width
	}
	return DefaultWidth
}

func Height(cfg Config) float64 {
	if cfg.Height > 0 {
		return cfg.Height
	}
	return DefaultHeight
}

func Dimensions(cfg Config) (float64, float64) {
	return Width(cfg), Height(cfg)
}
