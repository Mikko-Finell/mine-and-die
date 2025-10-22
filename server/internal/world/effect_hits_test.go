package world

import (
	"testing"
	"time"
)

func TestEffectHitPlayerCallbackInvokesAdapter(t *testing.T) {
	called := false
	callback := EffectHitPlayerCallback(EffectHitPlayerConfig{
		ApplyActorHit: func(effect any, target any, now time.Time) {
			called = true
			if effect != "effect" {
				t.Fatalf("expected effect value passed through")
			}
			if target != "target" {
				t.Fatalf("expected target value passed through")
			}
		},
	})

	if callback == nil {
		t.Fatalf("expected callback to be constructed")
	}

	callback("effect", "target", time.UnixMilli(42))

	if !called {
		t.Fatalf("expected adapter to be invoked")
	}
}

func TestEffectHitPlayerCallbackIgnoresNilTarget(t *testing.T) {
	callback := EffectHitPlayerCallback(EffectHitPlayerConfig{
		ApplyActorHit: func(effect any, target any, now time.Time) {
			t.Fatalf("expected adapter not to be called for nil target")
		},
	})

	if callback == nil {
		t.Fatalf("expected callback to be constructed")
	}

	callback("effect", nil, time.UnixMilli(42))
}

func TestEffectHitPlayerCallbackNilAdapter(t *testing.T) {
	if callback := EffectHitPlayerCallback(EffectHitPlayerConfig{}); callback != nil {
		t.Fatalf("expected nil callback when adapter missing")
	}
}

func TestEffectHitNPCCallbackMirrorsLegacyFlow(t *testing.T) {
	spawnCalled := false
	applyCalled := false
	defeatCalled := false
	alive := true

	callback := EffectHitNPCCallback(EffectHitNPCConfig{
		ApplyActorHit: func(effect any, target any, now time.Time) {
			applyCalled = true
			alive = false
		},
		SpawnBlood: func(effect any, target any, now time.Time) {
			spawnCalled = true
		},
		IsAlive: func(target any) bool {
			return alive
		},
		HandleDefeat: func(target any) {
			defeatCalled = true
		},
	})

	if callback == nil {
		t.Fatalf("expected callback to be constructed")
	}

	callback("effect", "target", time.UnixMilli(42))

	if !spawnCalled {
		t.Fatalf("expected SpawnBlood to be invoked before applying damage")
	}
	if !applyCalled {
		t.Fatalf("expected ApplyActorHit to be invoked")
	}
	if !defeatCalled {
		t.Fatalf("expected HandleDefeat to be invoked after defeat")
	}
}

func TestEffectHitNPCCallbackSkipsDefeatWhenStillAlive(t *testing.T) {
	defeatCalled := false

	callback := EffectHitNPCCallback(EffectHitNPCConfig{
		ApplyActorHit: func(effect any, target any, now time.Time) {},
		IsAlive: func(target any) bool {
			return true
		},
		HandleDefeat: func(target any) {
			defeatCalled = true
		},
	})

	if callback == nil {
		t.Fatalf("expected callback to be constructed")
	}

	callback("effect", "target", time.UnixMilli(42))

	if defeatCalled {
		t.Fatalf("expected defeat handler not to run when actor remains alive")
	}
}

func TestEffectHitNPCCallbackNilAdapter(t *testing.T) {
	if callback := EffectHitNPCCallback(EffectHitNPCConfig{}); callback != nil {
		t.Fatalf("expected nil callback when actor adapter missing")
	}
}

func TestEffectHitNPCCallbackIgnoresNilTarget(t *testing.T) {
	callback := EffectHitNPCCallback(EffectHitNPCConfig{
		ApplyActorHit: func(effect any, target any, now time.Time) {
			t.Fatalf("expected adapter not to be called for nil target")
		},
	})

	if callback == nil {
		t.Fatalf("expected callback to be constructed")
	}

	callback("effect", nil, time.UnixMilli(42))
}
