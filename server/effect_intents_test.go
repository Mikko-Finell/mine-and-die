package server

import (
	"math"
	"testing"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	logging "mine-and-die/server/logging"
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

	intent, ok := newMeleeIntent(owner)
	if !ok {
		t.Fatal("expected melee intent to be constructed")
	}
	if intent.EntryID != effectTypeAttack {
		t.Fatalf("unexpected EntryID: %q", intent.EntryID)
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
	if geom.Shape != effectcontract.GeometryShapeRect {
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

	if intent.EntryID != tpl.Type {
		t.Fatalf("expected EntryID %q, got %q", tpl.Type, intent.EntryID)
	}
	if intent.TypeID != tpl.Type {
		t.Fatalf("expected TypeID %q, got %q", tpl.Type, intent.TypeID)
	}

	if intent.SourceActorID != owner.ID {
		t.Fatalf("expected SourceActorID %q, got %q", owner.ID, intent.SourceActorID)
	}

	expectedRadius := quantizeWorldCoord(math.Max(tpl.SpawnRadius, 1))
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

func TestNewStatusVisualIntent(t *testing.T) {
	target := &actorState{Actor: Actor{ID: "player-3", X: 120, Y: 180}}
	lifetime := 1500 * time.Millisecond

	intent, ok := NewStatusVisualIntent(target, "lava-1", effectTypeBurningVisual, lifetime)
	if !ok {
		t.Fatal("expected status visual intent to be constructed")
	}

	if intent.EntryID != effectTypeBurningVisual {
		t.Fatalf("expected EntryID %q, got %q", effectTypeBurningVisual, intent.EntryID)
	}
	if intent.TypeID != effectTypeBurningVisual {
		t.Fatalf("expected TypeID %q, got %q", effectTypeBurningVisual, intent.TypeID)
	}
	if intent.SourceActorID != "lava-1" {
		t.Fatalf("expected SourceActorID 'lava-1', got %q", intent.SourceActorID)
	}
	if intent.TargetActorID != target.ID {
		t.Fatalf("expected TargetActorID %q, got %q", target.ID, intent.TargetActorID)
	}
	if intent.Delivery != effectcontract.DeliveryKindTarget {
		t.Fatalf("expected DeliveryKindTarget, got %q", intent.Delivery)
	}

	expectedDuration := durationToTicks(lifetime)
	if intent.DurationTicks != expectedDuration {
		t.Fatalf("expected DurationTicks %d, got %d", expectedDuration, intent.DurationTicks)
	}

	expectedFootprint := quantizeWorldCoord(playerHalf * 2)
	if intent.Geometry.Width != expectedFootprint || intent.Geometry.Height != expectedFootprint {
		t.Fatalf("expected footprint %d, got width=%d height=%d", expectedFootprint, intent.Geometry.Width, intent.Geometry.Height)
	}

	fallbackIntent, ok := NewStatusVisualIntent(target, "", effectTypeBurningVisual, lifetime)
	if !ok {
		t.Fatal("expected fallback intent to be constructed")
	}
	if fallbackIntent.SourceActorID != target.ID {
		t.Fatalf("expected SourceActorID to fall back to %q, got %q", target.ID, fallbackIntent.SourceActorID)
	}
}

func TestApplyStatusEffectQueuesIntent(t *testing.T) {
	world := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	if world.effectManager == nil {
		t.Fatal("expected effect manager to be initialised")
	}
	world.effectManager.ResetPendingIntents()

	actor := &actorState{Actor: Actor{ID: "player-status", X: 140, Y: 160}}
	now := time.Unix(0, 0)

	if applied := world.applyStatusEffect(actor, StatusEffectBurning, "lava-bridge", now); !applied {
		t.Fatal("expected status effect to be applied")
	}

	if world.effectManager.PendingIntentCount() != 2 {
		t.Fatalf("expected 2 intents enqueued (visual + tick), got %d", world.effectManager.PendingIntentCount())
	}

	intents := world.effectManager.PendingIntents()
	visual := intents[0]
	if visual.TypeID != effectTypeBurningVisual {
		t.Fatalf("expected first intent TypeID %q, got %q", effectTypeBurningVisual, visual.TypeID)
	}
	if visual.TargetActorID != actor.ID {
		t.Fatalf("expected visual TargetActorID %q, got %q", actor.ID, visual.TargetActorID)
	}
	if visual.SourceActorID != "lava-bridge" {
		t.Fatalf("expected visual SourceActorID 'lava-bridge', got %q", visual.SourceActorID)
	}

	tick := intents[1]
	if tick.TypeID != effectTypeBurningTick {
		t.Fatalf("expected second intent TypeID %q, got %q", effectTypeBurningTick, tick.TypeID)
	}
	if tick.TargetActorID != actor.ID {
		t.Fatalf("expected tick TargetActorID %q, got %q", actor.ID, tick.TargetActorID)
	}
}

func TestMaybeSpawnBloodSplatterQueuesIntent(t *testing.T) {
	world := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	if world.effectManager == nil {
		t.Fatal("expected effect manager to be initialised")
	}
	world.effectManager.ResetPendingIntents()

	eff := &effectState{Type: effectTypeAttack, Owner: "player-attacker"}
	target := &npcState{actorState: actorState{Actor: Actor{ID: "npc-target", X: 220, Y: 300}}, Type: NPCTypeGoblin}
	now := time.Unix(0, 0)

	world.maybeSpawnBloodSplatter(eff, target, now)

	if world.effectManager.PendingIntentCount() != 1 {
		t.Fatalf("expected 1 intent enqueued, got %d", world.effectManager.PendingIntentCount())
	}

	intent := world.effectManager.PendingIntents()[0]
	if intent.TypeID != effectTypeBloodSplatter {
		t.Fatalf("expected TypeID %q, got %q", effectTypeBloodSplatter, intent.TypeID)
	}
	if intent.SourceActorID != "player-attacker" {
		t.Fatalf("expected SourceActorID 'player-attacker', got %q", intent.SourceActorID)
	}
	if intent.Params["centerX"] != quantizeWorldCoord(target.X) {
		t.Fatalf("expected centerX %d, got %d", quantizeWorldCoord(target.X), intent.Params["centerX"])
	}
}
