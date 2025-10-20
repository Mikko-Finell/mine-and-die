package server

import (
	"math"

	effectcontract "mine-and-die/server/effects/contract"
)

// FixedVec represents a 2D vector using the fixed-point coordinate system
// defined by COORD_SCALE. All values are stored as integers to guarantee
// deterministic results across platforms.
type FixedVec struct {
	X int
	Y int
}

// FixedRect describes an axis-aligned bounding box in fixed-point units.
type FixedRect struct {
	MinX int
	MinY int
	MaxX int
	MaxY int
}

// FixedCircle represents a circle with integer radius in fixed-point space.
type FixedCircle struct {
	Center FixedVec
	Radius int
}

// FixedSegment captures a line segment with start/end points expressed in
// fixed-point units.
type FixedSegment struct {
	A FixedVec
	B FixedVec
}

// FixedCapsule models a thick segment (segment + radius) commonly used for
// projectile collision in the deterministic effect system.
type FixedCapsule struct {
	Segment FixedSegment
	Radius  int
}

// QuantizeCoord converts a floating point coordinate into the shared
// fixed-point representation. Values are rounded to the nearest sub-unit to
// keep round-trips deterministic.
func QuantizeCoord(value float64) int {
	return int(math.Round(value * effectcontract.CoordScale))
}

// DequantizeCoord converts a fixed-point coordinate back into floating point
// space. The helper is primarily intended for debugging and diagnostics code.
func DequantizeCoord(value int) float64 {
	return float64(value) / effectcontract.CoordScale
}

// QuantizeVelocity converts a world-units-per-second velocity into the
// fixed-point per-tick representation used by EffectMotionState.
func QuantizeVelocity(unitsPerSecond float64) int {
	perTick := unitsPerSecond / float64(tickRate)
	return QuantizeCoord(perTick)
}

// AdvancePosition applies an integer velocity for the provided number of ticks
// and returns the updated position. All arithmetic uses 64-bit intermediates to
// prevent overflow when working with long projectiles or fast-moving effects.
func AdvancePosition(position FixedVec, velocity FixedVec, ticks int) FixedVec {
	if ticks <= 0 {
		return position
	}
	dx := int(int64(velocity.X) * int64(ticks))
	dy := int(int64(velocity.Y) * int64(ticks))
	return FixedVec{X: position.X + dx, Y: position.Y + dy}
}

// NewFixedRect constructs a rectangle from its minimum corner and size (in
// fixed-point units).
func NewFixedRect(minX, minY, width, height int) FixedRect {
	return FixedRect{
		MinX: minX,
		MinY: minY,
		MaxX: minX + width,
		MaxY: minY + height,
	}
}

// RectIntersectsRect reports whether two axis-aligned rectangles overlap.
// Edges are treated as inclusive so touching volumes still count as a hit,
// matching the deterministic collision policy required by the effect contract.
func RectIntersectsRect(a, b FixedRect) bool {
	return a.MinX <= b.MaxX && a.MaxX >= b.MinX &&
		a.MinY <= b.MaxY && a.MaxY >= b.MinY
}

// CircleIntersectsRect reports whether a circle overlaps an axis-aligned
// rectangle using integer math.
func CircleIntersectsRect(circle FixedCircle, rect FixedRect) bool {
	closestX := clampInt(circle.Center.X, rect.MinX, rect.MaxX)
	closestY := clampInt(circle.Center.Y, rect.MinY, rect.MaxY)
	dx := circle.Center.X - closestX
	dy := circle.Center.Y - closestY
	return int64(dx)*int64(dx)+int64(dy)*int64(dy) <= int64(circle.Radius)*int64(circle.Radius)
}

// SegmentIntersectsCircle reports whether a line segment intersects a circle.
func SegmentIntersectsCircle(segment FixedSegment, circle FixedCircle) bool {
	distSq := segmentDistanceSquaredToPoint(segment, circle.Center)
	rSq := int64(circle.Radius) * int64(circle.Radius)
	return distSq <= rSq
}

// CapsuleIntersectsRect reports whether a capsule (thick segment) overlaps an
// axis-aligned rectangle. The check expands the rectangle by the capsule radius
// and performs a segment/rectangle intersection in integer space.
func CapsuleIntersectsRect(capsule FixedCapsule, rect FixedRect) bool {
	expanded := FixedRect{
		MinX: rect.MinX - capsule.Radius,
		MinY: rect.MinY - capsule.Radius,
		MaxX: rect.MaxX + capsule.Radius,
		MaxY: rect.MaxY + capsule.Radius,
	}
	return segmentIntersectsRect(capsule.Segment, expanded)
}

func clampInt(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func segmentDistanceSquaredToPoint(segment FixedSegment, point FixedVec) int64 {
	ax := int64(segment.A.X)
	ay := int64(segment.A.Y)
	bx := int64(segment.B.X)
	by := int64(segment.B.Y)
	px := int64(point.X)
	py := int64(point.Y)

	vx := bx - ax
	vy := by - ay
	wx := px - ax
	wy := py - ay

	if vx == 0 && vy == 0 {
		return (wx * wx) + (wy * wy)
	}

	proj := wx*vx + wy*vy
	segLenSq := vx*vx + vy*vy

	if proj <= 0 {
		return wx*wx + wy*wy
	}
	if proj >= segLenSq {
		dx := px - bx
		dy := py - by
		return dx*dx + dy*dy
	}

	projX := ax + divRoundNearest(vx*proj, segLenSq)
	projY := ay + divRoundNearest(vy*proj, segLenSq)
	dx := px - projX
	dy := py - projY
	return dx*dx + dy*dy
}

func divRoundNearest(num, denom int64) int64 {
	if denom < 0 {
		num = -num
		denom = -denom
	}
	if num >= 0 {
		return (num + denom/2) / denom
	}
	return -((-num + denom/2) / denom)
}

func segmentIntersectsRect(segment FixedSegment, rect FixedRect) bool {
	if rectContainsPoint(rect, segment.A) || rectContainsPoint(rect, segment.B) {
		return true
	}

	edges := [4]FixedSegment{
		{A: FixedVec{X: rect.MinX, Y: rect.MinY}, B: FixedVec{X: rect.MaxX, Y: rect.MinY}},
		{A: FixedVec{X: rect.MaxX, Y: rect.MinY}, B: FixedVec{X: rect.MaxX, Y: rect.MaxY}},
		{A: FixedVec{X: rect.MaxX, Y: rect.MaxY}, B: FixedVec{X: rect.MinX, Y: rect.MaxY}},
		{A: FixedVec{X: rect.MinX, Y: rect.MaxY}, B: FixedVec{X: rect.MinX, Y: rect.MinY}},
	}

	for _, edge := range edges {
		if segmentsIntersect(segment, edge) {
			return true
		}
	}
	return false
}

func rectContainsPoint(rect FixedRect, point FixedVec) bool {
	return point.X >= rect.MinX && point.X <= rect.MaxX &&
		point.Y >= rect.MinY && point.Y <= rect.MaxY
}

func segmentsIntersect(a, b FixedSegment) bool {
	o1 := orientation(a.A, a.B, b.A)
	o2 := orientation(a.A, a.B, b.B)
	o3 := orientation(b.A, b.B, a.A)
	o4 := orientation(b.A, b.B, a.B)

	if o1 != o2 && o3 != o4 {
		return true
	}

	if o1 == 0 && onSegment(a.A, b.A, a.B) {
		return true
	}
	if o2 == 0 && onSegment(a.A, b.B, a.B) {
		return true
	}
	if o3 == 0 && onSegment(b.A, a.A, b.B) {
		return true
	}
	if o4 == 0 && onSegment(b.A, a.B, b.B) {
		return true
	}
	return false
}

func orientation(a, b, c FixedVec) int {
	val := int64(b.Y-a.Y)*int64(c.X-b.X) - int64(b.X-a.X)*int64(c.Y-b.Y)
	if val == 0 {
		return 0
	}
	if val > 0 {
		return 1
	}
	return -1
}

func onSegment(a, b, c FixedVec) bool {
	return b.X >= minInt(a.X, c.X) && b.X <= maxInt(a.X, c.X) &&
		b.Y >= minInt(a.Y, c.Y) && b.Y <= maxInt(a.Y, c.Y)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
