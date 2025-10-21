package effects

import (
	"reflect"
	"testing"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

func TestSpawnContractStatusVisualFromInstance(t *testing.T) {
	now := time.Date(2024, time.January, 2, 3, 4, 5, 0, time.UTC)
	lifetime := 3 * time.Second
	instance := &effectcontract.EffectInstance{
		ID:           "effect-1",
		DefinitionID: "burning-visual",
		OwnerActorID: "caster",
		StartTick:    123,
	}
	target := &StatusVisualTarget{ID: "target-1", X: 100, Y: 150}
	cfg := StatusVisualSpawnConfig{
		Instance:         instance,
		Target:           target,
		Lifetime:         lifetime,
		Now:              now,
		DefaultFootprint: 32,
		FallbackLifetime: 200 * time.Millisecond,
		StatusEffect:     StatusEffectType("burning"),
	}

	effect := SpawnContractStatusVisualFromInstance(cfg)
	if effect == nil {
		t.Fatal("expected spawn helper to return effect state")
	}

	if effect.ID != instance.ID {
		t.Fatalf("expected effect ID %q, got %q", instance.ID, effect.ID)
	}
	if effect.Type != instance.DefinitionID {
		t.Fatalf("expected effect type %q, got %q", instance.DefinitionID, effect.Type)
	}
	if effect.Owner != target.ID {
		t.Fatalf("expected owner %q, got %q", target.ID, effect.Owner)
	}
	if effect.FollowActorID != target.ID {
		t.Fatalf("expected follow actor %q, got %q", target.ID, effect.FollowActorID)
	}
	if effect.StatusEffect != cfg.StatusEffect {
		t.Fatalf("expected status effect %q, got %q", cfg.StatusEffect, effect.StatusEffect)
	}
	if got := effect.Start; got != now.UnixMilli() {
		t.Fatalf("expected start %d, got %d", now.UnixMilli(), got)
	}
	if got := effect.Duration; got != lifetime.Milliseconds() {
		t.Fatalf("expected duration %d, got %d", lifetime.Milliseconds(), got)
	}
	if effect.Width != cfg.DefaultFootprint || effect.Height != cfg.DefaultFootprint {
		t.Fatalf("expected footprint %.1f, got width %.1f height %.1f", cfg.DefaultFootprint, effect.Width, effect.Height)
	}
	expectedX := target.X - cfg.DefaultFootprint/2
	expectedY := target.Y - cfg.DefaultFootprint/2
	if effect.X != expectedX || effect.Y != expectedY {
		t.Fatalf("expected position (%.1f, %.1f), got (%.1f, %.1f)", expectedX, expectedY, effect.X, effect.Y)
	}
	if !effect.ExpiresAt.Equal(now.Add(lifetime)) {
		t.Fatalf("expected expiry %v, got %v", now.Add(lifetime), effect.ExpiresAt)
	}
	if !effectsEqual(&effect.Instance, instance) {
		t.Fatalf("expected effect instance to equal source instance")
	}
	if !effect.ContractManaged {
		t.Fatal("expected effect to be contract-managed")
	}
	if effect.TelemetrySpawnTick != instance.StartTick {
		t.Fatalf("expected telemetry spawn tick %d, got %d", instance.StartTick, effect.TelemetrySpawnTick)
	}
}

func TestSpawnContractStatusVisualFromInstance_FallbackLifetime(t *testing.T) {
	now := time.UnixMilli(500)
	instance := &effectcontract.EffectInstance{ID: "effect-2", DefinitionID: "burning-visual"}
	target := &StatusVisualTarget{ID: "actor", X: 10, Y: 20}
	fallback := 400 * time.Millisecond
	cfg := StatusVisualSpawnConfig{
		Instance:         instance,
		Target:           target,
		Lifetime:         0,
		Now:              now,
		DefaultFootprint: 16,
		FallbackLifetime: fallback,
	}

	effect := SpawnContractStatusVisualFromInstance(cfg)
	if effect == nil {
		t.Fatal("expected spawn helper to return effect state")
	}

	if effect.Duration != fallback.Milliseconds() {
		t.Fatalf("expected fallback duration %d, got %d", fallback.Milliseconds(), effect.Duration)
	}
	expectedExpiry := now.Add(fallback)
	if !effect.ExpiresAt.Equal(expectedExpiry) {
		t.Fatalf("expected expiry %v, got %v", expectedExpiry, effect.ExpiresAt)
	}
}

func TestSpawnContractStatusVisualFromInstance_InvalidInput(t *testing.T) {
	cfg := StatusVisualSpawnConfig{}
	if effect := SpawnContractStatusVisualFromInstance(cfg); effect != nil {
		t.Fatal("expected nil when instance and target missing")
	}

	cfg.Instance = &effectcontract.EffectInstance{}
	if effect := SpawnContractStatusVisualFromInstance(cfg); effect != nil {
		t.Fatal("expected nil when target missing")
	}
}

func effectsEqual(a, b *effectcontract.EffectInstance) bool {
	return reflect.DeepEqual(a, b)
}
