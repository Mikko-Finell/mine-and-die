package server

import worldpkg "mine-and-die/server/internal/world"

func (w *World) width() float64 {
	if w == nil {
		return worldpkg.DefaultWidth
	}
	return worldpkg.Width(w.config)
}

func (w *World) height() float64 {
	if w == nil {
		return worldpkg.DefaultHeight
	}
	return worldpkg.Height(w.config)
}

func (w *World) dimensions() (float64, float64) {
	if w == nil {
		return worldpkg.DefaultWidth, worldpkg.DefaultHeight
	}
	return worldpkg.Dimensions(w.config)
}
