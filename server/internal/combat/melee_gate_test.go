package combat

import (
	"testing"
	"time"
)

func TestReadyCooldownInitialisesRegistryAndRecordsTimestamp(t *testing.T) {
	now := time.Unix(10, 0)
	// Use pointer to nil map to exercise lazy allocation path.
	var registry map[string]time.Time

	if !ReadyCooldown(&registry, "swing", time.Second, now) {
		t.Fatalf("expected cooldown to be ready on first invocation")
	}
	if registry == nil {
		t.Fatalf("expected registry map to be initialised")
	}
	if ts := registry["swing"]; !ts.Equal(now) {
		t.Fatalf("expected timestamp %v, got %v", now, ts)
	}

	if ReadyCooldown(&registry, "swing", time.Second, now.Add(500*time.Millisecond)) {
		t.Fatalf("expected cooldown to block when invoked twice within window")
	}
}

func TestReadyCooldownRespectsCooldownWindow(t *testing.T) {
	now := time.Unix(20, 0)
	registry := map[string]time.Time{"spin": now.Add(-500 * time.Millisecond)}
	if ReadyCooldown(&registry, "spin", time.Second, now) {
		t.Fatalf("expected cooldown to reject triggers within the window")
	}
	if ReadyCooldown(&registry, "spin", time.Second, now.Add(1500*time.Millisecond)) != true {
		t.Fatalf("expected cooldown to allow trigger after duration")
	}
}

func TestNewMeleeAbilityGateUsesLookupAndCooldown(t *testing.T) {
	now := time.Unix(30, 0)
	var recordedOwner string
	cooldowns := make(map[string]time.Time)

	gate := NewMeleeAbilityGate(MeleeAbilityGateConfig{
		AbilityID: "melee",
		Cooldown:  time.Second,
		LookupOwner: func(actorID string) (AbilityOwnerRef, bool) {
			recordedOwner = actorID
			return AbilityOwnerRef{
				ActorID:   actorID,
				Cooldowns: &cooldowns,
				Reference: actorID,
			}, true
		},
	})
	if gate == nil {
		t.Fatalf("expected melee ability gate to be constructed")
	}

	owner, ok := gate("hero", now)
	if !ok {
		t.Fatalf("expected gate to allow first trigger")
	}
	if owner.ActorID != "hero" {
		t.Fatalf("expected owner id 'hero', got %q", owner.ActorID)
	}
	if recordedOwner != "hero" {
		t.Fatalf("expected lookup to be invoked with actor id, got %q", recordedOwner)
	}
	if _, ok := cooldowns["melee"]; !ok {
		t.Fatalf("expected cooldown entry to be recorded")
	}

	if _, ok := gate("hero", now.Add(100*time.Millisecond)); ok {
		t.Fatalf("expected gate to reject triggers during cooldown")
	}
	if _, ok := gate("hero", now.Add(2*time.Second)); !ok {
		t.Fatalf("expected gate to allow trigger after cooldown")
	}
}

func TestNewProjectileAbilityGateUsesLookupAndCooldown(t *testing.T) {
	now := time.Unix(40, 0)
	var recordedOwner string
	cooldowns := make(map[string]time.Time)

	gate := NewProjectileAbilityGate(ProjectileAbilityGateConfig{
		AbilityID: "projectile",
		Cooldown:  500 * time.Millisecond,
		LookupOwner: func(actorID string) (AbilityOwnerRef, bool) {
			recordedOwner = actorID
			return AbilityOwnerRef{
				ActorID:   actorID,
				Cooldowns: &cooldowns,
				Reference: actorID,
			}, true
		},
	})
	if gate == nil {
		t.Fatalf("expected projectile ability gate to be constructed")
	}

	owner, ok := gate("wizard", now)
	if !ok {
		t.Fatalf("expected gate to allow first trigger")
	}
	if owner.ActorID != "wizard" {
		t.Fatalf("expected owner id 'wizard', got %q", owner.ActorID)
	}
	if recordedOwner != "wizard" {
		t.Fatalf("expected lookup to be invoked with actor id, got %q", recordedOwner)
	}
	if _, ok := cooldowns["projectile"]; !ok {
		t.Fatalf("expected cooldown entry to be recorded")
	}

	if _, ok := gate("wizard", now.Add(100*time.Millisecond)); ok {
		t.Fatalf("expected gate to reject triggers during cooldown")
	}
	if _, ok := gate("wizard", now.Add(time.Second)); !ok {
		t.Fatalf("expected gate to allow trigger after cooldown")
	}
}
