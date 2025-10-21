package world

import (
	"testing"
	"time"
)

type statusEffectInstanceStub struct {
	attached any
	typ      string
}

func (s *statusEffectInstanceStub) AttachEffect(value any) {
	s.attached = value
}

func (s *statusEffectInstanceStub) DefinitionType() string {
	return s.typ
}

type statusEffectVisualStub struct {
	state  any
	status string
}

func (s *statusEffectVisualStub) SetStatusEffect(value string) {
	s.status = value
}

func (s *statusEffectVisualStub) EffectState() any {
	return s.state
}

func TestAttachStatusEffectVisualUsesInstanceType(t *testing.T) {
	instance := &statusEffectInstanceStub{typ: "poison"}
	state := struct{}{}
	visual := &statusEffectVisualStub{state: &state}

	AttachStatusEffectVisual(AttachStatusEffectVisualConfig{
		Instance:    instance,
		Effect:      visual,
		DefaultType: "burning",
	})

	if instance.attached != visual.state {
		t.Fatalf("expected instance to attach provided effect state")
	}
	if visual.status != "poison" {
		t.Fatalf("expected visual status to match instance type, got %q", visual.status)
	}
}

func TestAttachStatusEffectVisualFallsBackToDefault(t *testing.T) {
	instance := &statusEffectInstanceStub{}
	state := struct{}{}
	visual := &statusEffectVisualStub{state: &state}

	AttachStatusEffectVisual(AttachStatusEffectVisualConfig{
		Instance:    instance,
		Effect:      visual,
		DefaultType: "burning",
	})

	if visual.status != "burning" {
		t.Fatalf("expected fallback status effect 'burning', got %q", visual.status)
	}
}

func TestAttachStatusEffectVisualNoopWhenStateMissing(t *testing.T) {
	instance := &statusEffectInstanceStub{typ: "burning"}
	visual := &statusEffectVisualStub{}

	AttachStatusEffectVisual(AttachStatusEffectVisualConfig{
		Instance:    instance,
		Effect:      visual,
		DefaultType: "burning",
	})

	if instance.attached != nil {
		t.Fatalf("expected instance attachment to remain nil when state missing")
	}
	if visual.status != "" {
		t.Fatalf("expected visual status to remain empty when state missing")
	}
}

func TestAttachStatusEffectVisualNoopWhenInstanceMissing(t *testing.T) {
	state := struct{}{}
	visual := &statusEffectVisualStub{state: &state}

	AttachStatusEffectVisual(AttachStatusEffectVisualConfig{
		Effect:      visual,
		DefaultType: "burning",
	})

	if visual.status != "" {
		t.Fatalf("expected visual status to remain empty when instance missing")
	}
}

func TestExtendStatusEffectLifetimeUpdatesExpiryAndDuration(t *testing.T) {
	expires := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	start := expires.Add(-500 * time.Millisecond)
	duration := int64(0)
	expectedExpires := expires.Add(500 * time.Millisecond)

	ExtendStatusEffectLifetime(StatusEffectLifetimeFields{
		ExpiresAt:      &expires,
		StartMillis:    start.UnixMilli(),
		DurationMillis: &duration,
	}, expires.Add(500*time.Millisecond))

	if !expires.Equal(expectedExpires) {
		t.Fatalf("expected expiry updated to %v, got %v", expectedExpires, expires)
	}

	expectedDuration := time.Second.Milliseconds()
	if duration != expectedDuration {
		t.Fatalf("expected duration %d, got %d", expectedDuration, duration)
	}
}

func TestExtendStatusEffectLifetimeRespectsEarlierExpiry(t *testing.T) {
	expires := time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)
	start := expires.Add(-time.Second)
	duration := int64(123)

	ExtendStatusEffectLifetime(StatusEffectLifetimeFields{
		ExpiresAt:      &expires,
		StartMillis:    start.UnixMilli(),
		DurationMillis: &duration,
	}, expires.Add(-time.Second))

	if !expires.Equal(time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected expiry unchanged when new value is earlier")
	}
	if duration != 123 {
		t.Fatalf("expected duration unchanged, got %d", duration)
	}
}

func TestExtendStatusEffectLifetimeFallsBackToExpiryWhenStartMissing(t *testing.T) {
	expires := time.Date(2024, 3, 1, 12, 0, 0, 0, time.UTC)
	duration := int64(999)

	ExtendStatusEffectLifetime(StatusEffectLifetimeFields{
		ExpiresAt:      &expires,
		StartMillis:    0,
		DurationMillis: &duration,
	}, expires.Add(2*time.Second))

	if duration != 0 {
		t.Fatalf("expected duration to clamp to zero when start missing, got %d", duration)
	}
}

func TestExpireStatusEffectLifetimeClampsExpiryAndDuration(t *testing.T) {
	expires := time.Date(2024, 4, 5, 6, 7, 8, 0, time.UTC)
	start := expires.Add(-1500 * time.Millisecond)
	duration := int64(0)

	now := expires.Add(-500 * time.Millisecond)
	ExpireStatusEffectLifetime(StatusEffectLifetimeFields{
		ExpiresAt:      &expires,
		StartMillis:    start.UnixMilli(),
		DurationMillis: &duration,
	}, now)

	if !expires.Equal(now) {
		t.Fatalf("expected expiry clamped to now %v, got %v", now, expires)
	}
	expectedDuration := time.Second.Milliseconds()
	if duration != expectedDuration {
		t.Fatalf("expected duration %d, got %d", expectedDuration, duration)
	}
}

func TestExpireStatusEffectLifetimeHandlesFutureStart(t *testing.T) {
	expires := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	duration := int64(42)
	now := expires.Add(-500 * time.Millisecond)

	ExpireStatusEffectLifetime(StatusEffectLifetimeFields{
		ExpiresAt:      &expires,
		StartMillis:    now.Add(2 * time.Second).UnixMilli(),
		DurationMillis: &duration,
	}, now)

	if duration != 0 {
		t.Fatalf("expected duration to clamp to zero, got %d", duration)
	}
}
