package main

import (
	"math"
	"testing"
	"time"
)

func TestDurationToTicks(t *testing.T) {
	if got := durationToTicks(0); got != 0 {
		t.Fatalf("durationToTicks(0) = %d, want 0", got)
	}

	short := time.Millisecond * 10
	if got := durationToTicks(short); got != 1 {
		t.Fatalf("durationToTicks(short) = %d, want 1", got)
	}

	duration := time.Second / 2
	want := int(math.Ceil(duration.Seconds() * float64(tickRate)))
	if got := durationToTicks(duration); got != want {
		t.Fatalf("durationToTicks(%v) = %d, want %d", duration, got, want)
	}
}

func TestNewMeleeIntent(t *testing.T) {
	owner := &actorState{Actor: Actor{ID: "player-1", X: 200, Y: 180, Facing: FacingDown}}

	intent, ok := NewMeleeIntent(owner)
	if !ok {
		t.Fatal("expected melee intent to be constructed")
	}
	if intent.TypeID != effectTypeAttack {
		t.Fatalf("unexpected TypeID: %q", intent.TypeID)
	}
	if intent.SourceActorID != owner.ID {
		t.Fatalf("expected SourceActorID %q, got %q", owner.ID, intent.SourceActorID)
	}

	expectedDuration := durationToTicks(meleeAttackDuration)
	if intent.DurationTicks != expectedDuration {
		t.Fatalf("expected DurationTicks %d, got %d", expectedDuration, intent.DurationTicks)
	}

	geom := intent.Geometry
	if geom.Shape != GeometryShapeRect {
		t.Fatalf("expected rect geometry, got %q", geom.Shape)
	}

	expectedWidth := quantizeWorldCoord(meleeAttackWidth)
	expectedHeight := quantizeWorldCoord(meleeAttackReach)
	if geom.Width != expectedWidth {
		t.Fatalf("expected width %d, got %d", expectedWidth, geom.Width)
	}
	if geom.Height != expectedHeight {
		t.Fatalf("expected height %d, got %d", expectedHeight, geom.Height)
	}

	expectedOffsetY := quantizeWorldCoord(playerHalf + meleeAttackReach/2)
	if geom.OffsetY != expectedOffsetY {
		t.Fatalf("expected offsetY %d, got %d", expectedOffsetY, geom.OffsetY)
	}

	if geom.OffsetX != 0 {
		t.Fatalf("expected zero offsetX, got %d", geom.OffsetX)
	}

	if intent.Params["healthDelta"] != int(math.Round(-meleeAttackDamage)) {
		t.Fatalf("expected healthDelta %d, got %d", int(math.Round(-meleeAttackDamage)), intent.Params["healthDelta"])
	}
}

func TestNewProjectileIntent(t *testing.T) {
	owner := &actorState{Actor: Actor{ID: "player-2", X: 160, Y: 160, Facing: FacingRight}}
	tpl := newProjectileTemplates()[effectTypeFireball]

	intent, ok := NewProjectileIntent(owner, tpl)
	if !ok {
		t.Fatal("expected projectile intent to be constructed")
	}

	if intent.TypeID != tpl.Type {
		t.Fatalf("expected TypeID %q, got %q", tpl.Type, intent.TypeID)
	}

	if intent.SourceActorID != owner.ID {
		t.Fatalf("expected SourceActorID %q, got %q", owner.ID, intent.SourceActorID)
	}

	expectedRadius := quantizeWorldCoord(sanitizedSpawnRadius(tpl.SpawnRadius))
	if intent.Geometry.Radius != expectedRadius {
		t.Fatalf("expected radius %d, got %d", expectedRadius, intent.Geometry.Radius)
	}

	expectedOffsetX := quantizeWorldCoord(tpl.SpawnOffset)
	if intent.Geometry.OffsetX != expectedOffsetX {
		t.Fatalf("expected offsetX %d, got %d", expectedOffsetX, intent.Geometry.OffsetX)
	}

	if intent.Params["dx"] != 1 || intent.Params["dy"] != 0 {
		t.Fatalf("expected direction (1,0), got (%d,%d)", intent.Params["dx"], intent.Params["dy"])
	}
}
