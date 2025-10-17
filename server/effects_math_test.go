package main

import (
	"math"
	"testing"

	effectcontract "mine-and-die/server/effects/contract"
)

func fixed(value float64) int {
	return int(math.Round(value * effectcontract.CoordScale))
}

func TestQuantizeCoord(t *testing.T) {
	cases := []struct {
		name  string
		input float64
		want  int
	}{
		{name: "zero", input: 0, want: 0},
		{name: "positive", input: 2.5, want: 40},
		{name: "negative", input: -1.125, want: -18},
		{name: "small", input: 1.0 / float64(effectcontract.CoordScale), want: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := QuantizeCoord(tc.input)
			if got != tc.want {
				t.Fatalf("QuantizeCoord(%v) = %d, want %d", tc.input, got, tc.want)
			}
			if de := DequantizeCoord(got); math.Abs(de-tc.input) > 1.0/float64(effectcontract.CoordScale) {
				t.Fatalf("DequantizeCoord(%d) = %f diverges from %f", got, de, tc.input)
			}
		})
	}
}

func TestQuantizeVelocityAndAdvance(t *testing.T) {
	speed := 160.0
	vel := QuantizeVelocity(speed)
	if vel == 0 {
		t.Fatal("QuantizeVelocity returned zero for non-zero speed")
	}

	start := FixedVec{X: fixed(1), Y: fixed(2)}
	position := AdvancePosition(start, FixedVec{X: vel, Y: 0}, 3)
	expectedX := start.X + vel*3
	if position.X != expectedX || position.Y != start.Y {
		t.Fatalf("AdvancePosition mismatch: got (%d,%d), want (%d,%d)", position.X, position.Y, expectedX, start.Y)
	}
}

func TestRectIntersectsRect(t *testing.T) {
	rectA := NewFixedRect(0, 0, fixed(2), fixed(2))
	rectB := NewFixedRect(fixed(1), fixed(1), fixed(2), fixed(2))
	rectC := NewFixedRect(fixed(3), fixed(3), fixed(1), fixed(1))

	if !RectIntersectsRect(rectA, rectB) {
		t.Fatal("expected overlapping rectangles to intersect")
	}
	if RectIntersectsRect(rectA, rectC) {
		t.Fatal("expected separated rectangles to not intersect")
	}
}

func TestCircleIntersectsRect(t *testing.T) {
	rect := NewFixedRect(0, 0, fixed(2), fixed(2))
	circleTouching := FixedCircle{Center: FixedVec{X: -fixed(0.25), Y: fixed(1)}, Radius: fixed(0.5)}
	circleFar := FixedCircle{Center: FixedVec{X: -fixed(2), Y: fixed(1)}, Radius: fixed(0.25)}

	if !CircleIntersectsRect(circleTouching, rect) {
		t.Fatal("expected circle to intersect rectangle")
	}
	if CircleIntersectsRect(circleFar, rect) {
		t.Fatal("expected circle to not intersect rectangle")
	}
}

func TestSegmentIntersectsCircle(t *testing.T) {
	circle := FixedCircle{Center: FixedVec{X: 0, Y: 0}, Radius: fixed(1)}
	segHit := FixedSegment{A: FixedVec{X: -fixed(2), Y: 0}, B: FixedVec{X: fixed(2), Y: 0}}
	segMiss := FixedSegment{A: FixedVec{X: -fixed(2), Y: fixed(2)}, B: FixedVec{X: fixed(2), Y: fixed(2)}}

	if !SegmentIntersectsCircle(segHit, circle) {
		t.Fatal("expected segment to intersect circle")
	}
	if SegmentIntersectsCircle(segMiss, circle) {
		t.Fatal("expected segment to miss circle")
	}
}

func TestCapsuleIntersectsRect(t *testing.T) {
	rect := NewFixedRect(0, 0, fixed(2), fixed(2))
	capsuleHit := FixedCapsule{
		Segment: FixedSegment{A: FixedVec{X: -fixed(1), Y: fixed(1)}, B: FixedVec{X: fixed(3), Y: fixed(1)}},
		Radius:  fixed(0.25),
	}
	capsuleMiss := FixedCapsule{
		Segment: FixedSegment{A: FixedVec{X: -fixed(2), Y: fixed(3)}, B: FixedVec{X: fixed(2), Y: fixed(3)}},
		Radius:  fixed(0.25),
	}

	if !CapsuleIntersectsRect(capsuleHit, rect) {
		t.Fatal("expected capsule to intersect rectangle")
	}
	if CapsuleIntersectsRect(capsuleMiss, rect) {
		t.Fatal("expected capsule to miss rectangle")
	}
}
