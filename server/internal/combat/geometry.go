package combat

// Rectangle represents an axis-aligned rectangle used by combat helpers.
type Rectangle struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// CircleRectOverlap reports whether a circle intersects the rectangle.
func CircleRectOverlap(cx, cy, radius float64, rect Rectangle) bool {
	closestX := clamp(cx, rect.X, rect.X+rect.Width)
	closestY := clamp(cy, rect.Y, rect.Y+rect.Height)
	dx := cx - closestX
	dy := cy - closestY
	return dx*dx+dy*dy < radius*radius
}

func clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
