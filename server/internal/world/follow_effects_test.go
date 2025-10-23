package world

import (
	"testing"
	"time"

	itemspkg "mine-and-die/server/internal/items"
)

type testFollowEffect struct {
	followID string
	width    float64
	height   float64
}

func TestAdvanceLegacyFollowEffectsRepositionsAttachment(t *testing.T) {
	effect := &testFollowEffect{followID: "actor-1", width: 10, height: 12}
	now := time.Unix(10, 0)
	var setX, setY float64
	called := false

	AdvanceLegacyFollowEffects(LegacyFollowEffectAdvanceConfig{
		Now: now,
		ForEachEffect: func(visitor func(effect any)) {
			visitor(effect)
		},
		Inspect: func(effect any) LegacyFollowEffect {
			state, _ := effect.(*testFollowEffect)
			return LegacyFollowEffect{
				FollowActorID: state.followID,
				Width:         state.width,
				Height:        state.height,
			}
		},
		ActorByID: func(id string) *itemspkg.Actor {
			if id != "actor-1" {
				return nil
			}
			return &itemspkg.Actor{ID: id, X: 40, Y: 70}
		},
		SetPosition: func(effect any, x, y float64) {
			called = true
			setX = x
			setY = y
		},
	})

	if !called {
		t.Fatalf("expected SetPosition to be invoked")
	}
	expectedX := 40.0 - effect.width/2
	expectedY := 70.0 - effect.height/2
	if setX != expectedX || setY != expectedY {
		t.Fatalf("expected position (%.2f, %.2f), got (%.2f, %.2f)", expectedX, expectedY, setX, setY)
	}
}

func TestAdvanceLegacyFollowEffectsDefaultsDimensions(t *testing.T) {
	effect := &testFollowEffect{followID: "actor-2"}
	now := time.Unix(11, 0)
	var setX, setY float64
	called := false

	AdvanceLegacyFollowEffects(LegacyFollowEffectAdvanceConfig{
		Now: now,
		ForEachEffect: func(visitor func(effect any)) {
			visitor(effect)
		},
		Inspect: func(effect any) LegacyFollowEffect {
			state, _ := effect.(*testFollowEffect)
			return LegacyFollowEffect{FollowActorID: state.followID}
		},
		ActorByID: func(id string) *itemspkg.Actor {
			if id != "actor-2" {
				return nil
			}
			return &itemspkg.Actor{ID: id, X: 100, Y: 200}
		},
		SetPosition: func(effect any, x, y float64) {
			called = true
			setX = x
			setY = y
		},
	})

	if !called {
		t.Fatalf("expected SetPosition to be invoked")
	}
	expectedOffset := PlayerHalf
	expectedX := 100.0 - expectedOffset
	expectedY := 200.0 - expectedOffset
	if setX != expectedX || setY != expectedY {
		t.Fatalf("expected default position (%.2f, %.2f), got (%.2f, %.2f)", expectedX, expectedY, setX, setY)
	}
}

func TestAdvanceLegacyFollowEffectsExpiresMissingActor(t *testing.T) {
	effect := &testFollowEffect{followID: "actor-3"}
	now := time.Unix(12, 0)
	expired := false
	cleared := false

	AdvanceLegacyFollowEffects(LegacyFollowEffectAdvanceConfig{
		Now: now,
		ForEachEffect: func(visitor func(effect any)) {
			visitor(effect)
		},
		Inspect: func(effect any) LegacyFollowEffect {
			state, _ := effect.(*testFollowEffect)
			return LegacyFollowEffect{FollowActorID: state.followID}
		},
		ActorByID: func(id string) *itemspkg.Actor {
			return nil
		},
		Expire: func(effect any, at time.Time) {
			if at != now {
				t.Fatalf("expected expire time %v, got %v", now, at)
			}
			expired = true
		},
		ClearFollow: func(effect any) {
			state, _ := effect.(*testFollowEffect)
			state.followID = ""
			cleared = true
		},
		SetPosition: func(effect any, x, y float64) {
			t.Fatalf("expected SetPosition not to be invoked when actor is missing")
		},
	})

	if !expired {
		t.Fatalf("expected Expire callback to run")
	}
	if !cleared {
		t.Fatalf("expected ClearFollow callback to run")
	}
	if effect.followID != "" {
		t.Fatalf("expected follow ID to be cleared, got %q", effect.followID)
	}
}

func TestUpdateLegacyFollowEffectSkipsWhenEffectNil(t *testing.T) {
	UpdateLegacyFollowEffect(LegacyFollowEffectUpdateConfig{
		Effect: nil,
		Fields: LegacyFollowEffect{FollowActorID: "actor-4"},
		SetPosition: func(effect any, x, y float64) {
			t.Fatalf("expected SetPosition not to run for nil effect")
		},
	})
}
