package combat

import (
	"math"
	"testing"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

func TestNewMeleeIntentConstructsIntent(t *testing.T) {
	tileSize := 40.0
	tickRate := 15.0

	quantize := func(value float64) int {
		return int(math.Round(value * effectcontract.CoordScale))
	}
	durationToTicks := func(duration time.Duration) int {
		if duration <= 0 {
			return 0
		}
		ticks := int(math.Ceil(duration.Seconds() * tickRate))
		if ticks < 1 {
			ticks = 1
		}
		return ticks
	}

	cfg := MeleeIntentConfig{
		Geometry: MeleeAttackGeometryConfig{
			PlayerHalf:    20,
			Reach:         MeleeAttackReach,
			Width:         MeleeAttackWidth,
			DefaultFacing: "down",
		},
		TileSize:        tileSize,
		Damage:          MeleeAttackDamage,
		Duration:        MeleeAttackDuration,
		QuantizeCoord:   quantize,
		DurationToTicks: durationToTicks,
	}

	owner := MeleeIntentOwner{ID: "player-1", X: 200, Y: 180, Facing: "down"}

	intent, ok := NewMeleeIntent(cfg, owner)
	if !ok {
		t.Fatalf("expected melee intent to be constructed")
	}
	if intent.EntryID != EffectTypeAttack || intent.TypeID != EffectTypeAttack {
		t.Fatalf("expected attack effect type, got entry=%q type=%q", intent.EntryID, intent.TypeID)
	}
	if intent.SourceActorID != owner.ID {
		t.Fatalf("expected source %q, got %q", owner.ID, intent.SourceActorID)
	}

	quantizeWorld := func(value float64) int {
		return quantize(value / tileSize)
	}

	expectedWidth := quantizeWorld(MeleeAttackWidth)
	expectedHeight := quantizeWorld(MeleeAttackReach)
	if intent.Geometry.Width != expectedWidth {
		t.Fatalf("expected width %d, got %d", expectedWidth, intent.Geometry.Width)
	}
	if intent.Geometry.Height != expectedHeight {
		t.Fatalf("expected height %d, got %d", expectedHeight, intent.Geometry.Height)
	}

	expectedOffsetY := quantizeWorld(cfg.Geometry.PlayerHalf + cfg.Geometry.Reach/2)
	if intent.Geometry.OffsetY != expectedOffsetY {
		t.Fatalf("expected offsetY %d, got %d", expectedOffsetY, intent.Geometry.OffsetY)
	}
	if intent.Geometry.OffsetX != 0 {
		t.Fatalf("expected zero offsetX, got %d", intent.Geometry.OffsetX)
	}

	if intent.DurationTicks != durationToTicks(MeleeAttackDuration) {
		t.Fatalf("unexpected duration ticks %d", intent.DurationTicks)
	}

	if intent.Params["healthDelta"] != int(math.Round(-MeleeAttackDamage)) {
		t.Fatalf("unexpected health delta %d", intent.Params["healthDelta"])
	}
	if intent.Params["reach"] != int(math.Round(MeleeAttackReach)) {
		t.Fatalf("unexpected reach %d", intent.Params["reach"])
	}
	if intent.Params["width"] != int(math.Round(MeleeAttackWidth)) {
		t.Fatalf("unexpected width %d", intent.Params["width"])
	}
}

func TestMeleeAttackRectangleFacing(t *testing.T) {
	cfg := MeleeAttackGeometryConfig{PlayerHalf: 20, Reach: 56, Width: 40, DefaultFacing: "down"}
	x := 200.0
	y := 180.0

	cases := []struct {
		name         string
		facing       string
		wantX, wantY float64
		wantW, wantH float64
	}{
		{name: "Down", facing: "down", wantX: 180, wantY: 200, wantW: 40, wantH: 56},
		{name: "Up", facing: "up", wantX: 180, wantY: 104, wantW: 40, wantH: 56},
		{name: "Left", facing: "left", wantX: 124, wantY: 160, wantW: 56, wantH: 40},
		{name: "Right", facing: "right", wantX: 220, wantY: 160, wantW: 56, wantH: 40},
		{name: "DefaultEmpty", facing: "", wantX: 180, wantY: 200, wantW: 40, wantH: 56},
		{name: "DefaultUnknown", facing: "weird", wantX: 180, wantY: 200, wantW: 40, wantH: 56},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotX, gotY, gotW, gotH := MeleeAttackRectangle(cfg, x, y, tc.facing)
			if gotX != tc.wantX || gotY != tc.wantY || gotW != tc.wantW || gotH != tc.wantH {
				t.Fatalf("unexpected rectangle: got (%v,%v,%v,%v) want (%v,%v,%v,%v)", gotX, gotY, gotW, gotH, tc.wantX, tc.wantY, tc.wantW, tc.wantH)
			}
		})
	}
}
