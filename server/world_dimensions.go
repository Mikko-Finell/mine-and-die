package server

// width returns the configured world width, falling back to the default when
// unset or when the world is nil.
func (w *World) width() float64 {
	if w == nil {
		return worldWidth
	}
	if w.config.Width > 0 {
		return w.config.Width
	}
	return worldWidth
}

// height returns the configured world height, falling back to the default when
// unset or when the world is nil.
func (w *World) height() float64 {
	if w == nil {
		return worldHeight
	}
	if w.config.Height > 0 {
		return w.config.Height
	}
	return worldHeight
}

// dimensions returns the configured world width and height.
func (w *World) dimensions() (float64, float64) {
	return w.width(), w.height()
}
