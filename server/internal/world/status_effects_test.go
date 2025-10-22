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

func TestExtendStatusEffectAttachmentUpdatesLifetime(t *testing.T) {
	expires := time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)
	start := expires.Add(-750 * time.Millisecond)
	duration := int64(0)

	ExtendStatusEffectAttachment(StatusEffectAttachmentFields{
		StatusEffectLifetimeFields: StatusEffectLifetimeFields{
			ExpiresAt:      &expires,
			StartMillis:    start.UnixMilli(),
			DurationMillis: &duration,
		},
	}, expires.Add(250*time.Millisecond))

	if !expires.Equal(start.Add(time.Second)) {
		t.Fatalf("expected expiry updated to %v, got %v", start.Add(time.Second), expires)
	}
	if duration != time.Second.Milliseconds() {
		t.Fatalf("expected duration %d, got %d", time.Second.Milliseconds(), duration)
	}
}

func TestExpireStatusEffectAttachmentSignalsTelemetryRecording(t *testing.T) {
	expires := time.Date(2024, 8, 1, 0, 0, 0, 0, time.UTC)
	start := expires.Add(-2 * time.Second)
	duration := int64(0)

	shouldRecord := ExpireStatusEffectAttachment(StatusEffectAttachmentFields{
		StatusEffectLifetimeFields: StatusEffectLifetimeFields{
			ExpiresAt:      &expires,
			StartMillis:    start.UnixMilli(),
			DurationMillis: &duration,
		},
		TelemetryEnded: false,
	}, expires.Add(-time.Second))

	if !shouldRecord {
		t.Fatalf("expected telemetry record request when not previously ended")
	}
	if duration != time.Second.Milliseconds() {
		t.Fatalf("expected duration %d, got %d", time.Second.Milliseconds(), duration)
	}
}

func TestExpireStatusEffectAttachmentSkipsTelemetryWhenAlreadyEnded(t *testing.T) {
	expires := time.Date(2024, 9, 1, 0, 0, 0, 0, time.UTC)
	duration := int64(123)

	shouldRecord := ExpireStatusEffectAttachment(StatusEffectAttachmentFields{
		StatusEffectLifetimeFields: StatusEffectLifetimeFields{
			ExpiresAt:      &expires,
			StartMillis:    expires.Add(-time.Second).UnixMilli(),
			DurationMillis: &duration,
		},
		TelemetryEnded: true,
	}, expires.Add(time.Second))

	if shouldRecord {
		t.Fatalf("expected telemetry already recorded flag to suppress new record")
	}
}

type statusEffectAdvanceStub struct {
	valid        bool
	tickInterval time.Duration
	nextTick     time.Time
	lastTick     time.Time
	expiresAt    time.Time
	customExpire bool
	onTick       []time.Time
	expireCalls  []time.Time
	attachment   *statusEffectAttachmentAdvanceStub
}

type statusEffectAttachmentAdvanceStub struct {
	effect       any
	shouldRecord bool
	extended     []time.Time
	expired      []time.Time
	clearCount   int
}

func TestAdvanceActorStatusEffectsRemovesInvalidInstances(t *testing.T) {
	now := time.Date(2024, 10, 1, 0, 0, 0, 0, time.UTC)

	instances := map[string]*statusEffectAdvanceStub{
		"nil":         nil,
		"missing-def": {valid: false},
		"active": {
			valid:     true,
			expiresAt: now.Add(time.Second),
		},
	}

	cfg := AdvanceActorStatusEffectsConfig{
		Now: now,
		ForEachInstance: func(visitor func(key string, instance any)) {
			for key, inst := range instances {
				visitor(key, inst)
			}
		},
		Instance: func(value any) (StatusEffectInstanceConfig, bool) {
			inst, _ := value.(*statusEffectAdvanceStub)
			if inst == nil || !inst.valid {
				return StatusEffectInstanceConfig{}, false
			}
			return StatusEffectInstanceConfig{
				Definition: StatusEffectDefinitionCallbacks{},
				ExpiresAt:  func() time.Time { return inst.expiresAt },
			}, true
		},
		Remove: func(key string) {
			delete(instances, key)
		},
	}

	AdvanceActorStatusEffects(cfg)

	if len(instances) != 1 {
		t.Fatalf("expected only active instance to remain, got %d", len(instances))
	}
	if _, ok := instances["active"]; !ok {
		t.Fatalf("expected active instance to remain after advancement")
	}
}

func TestAdvanceActorStatusEffectsTicksAndExtendsAttachment(t *testing.T) {
	now := time.Date(2024, 11, 1, 0, 0, 0, 0, time.UTC)

	attachment := &statusEffectAttachmentAdvanceStub{effect: struct{}{}, shouldRecord: false}
	inst := &statusEffectAdvanceStub{
		valid:        true,
		tickInterval: 100 * time.Millisecond,
		nextTick:     now,
		expiresAt:    now.Add(300 * time.Millisecond),
		attachment:   attachment,
	}

	instances := map[string]*statusEffectAdvanceStub{"burning": inst}

	cfg := AdvanceActorStatusEffectsConfig{
		Now: now,
		ForEachInstance: func(visitor func(key string, instance any)) {
			for key, inst := range instances {
				visitor(key, inst)
			}
		},
		Instance: func(value any) (StatusEffectInstanceConfig, bool) {
			candidate, _ := value.(*statusEffectAdvanceStub)
			if candidate == nil || !candidate.valid {
				return StatusEffectInstanceConfig{}, false
			}
			cfg := StatusEffectInstanceConfig{
				Definition: StatusEffectDefinitionCallbacks{
					TickInterval: candidate.tickInterval,
					OnTick: func(at time.Time) {
						candidate.onTick = append(candidate.onTick, at)
					},
				},
				NextTick: func() time.Time { return candidate.nextTick },
				SetNextTick: func(value time.Time) {
					candidate.nextTick = value
				},
				LastTick: func() time.Time { return candidate.lastTick },
				SetLastTick: func(value time.Time) {
					candidate.lastTick = value
				},
				ExpiresAt: func() time.Time { return candidate.expiresAt },
			}
			if candidate.attachment != nil {
				cfg.Attachment = &StatusEffectAttachmentConfig{
					Extend: func(at time.Time) {
						candidate.attachment.extended = append(candidate.attachment.extended, at)
					},
					Expire: func(time.Time) (any, bool) {
						candidate.attachment.expired = append(candidate.attachment.expired, now)
						return candidate.attachment.effect, candidate.attachment.shouldRecord
					},
					Clear: func() {
						candidate.attachment.clearCount++
					},
				}
			}
			return cfg, true
		},
		Remove: func(key string) {
			delete(instances, key)
		},
	}

	AdvanceActorStatusEffects(cfg)

	if len(inst.onTick) != 1 {
		t.Fatalf("expected one tick, got %d", len(inst.onTick))
	}
	if inst.lastTick != now {
		t.Fatalf("expected last tick to be %v, got %v", now, inst.lastTick)
	}
	expectedNext := now.Add(100 * time.Millisecond)
	if inst.nextTick != expectedNext {
		t.Fatalf("expected next tick %v, got %v", expectedNext, inst.nextTick)
	}
	if len(attachment.extended) != 1 || !attachment.extended[0].Equal(inst.expiresAt) {
		t.Fatalf("expected attachment extended once to %v, got %#v", inst.expiresAt, attachment.extended)
	}
	if len(instances) != 1 {
		t.Fatalf("expected instance to remain active, map len %d", len(instances))
	}
}

func TestAdvanceActorStatusEffectsExpiresAttachmentAndRecords(t *testing.T) {
	now := time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC)

	attachment := &statusEffectAttachmentAdvanceStub{effect: struct{}{}, shouldRecord: true}
	inst := &statusEffectAdvanceStub{
		valid:      true,
		expiresAt:  now,
		attachment: attachment,
	}

	instances := map[string]*statusEffectAdvanceStub{"burning": inst}
	var recorded []any

	cfg := AdvanceActorStatusEffectsConfig{
		Now: now,
		ForEachInstance: func(visitor func(key string, instance any)) {
			for key, inst := range instances {
				visitor(key, inst)
			}
		},
		Instance: func(value any) (StatusEffectInstanceConfig, bool) {
			candidate, _ := value.(*statusEffectAdvanceStub)
			if candidate == nil || !candidate.valid {
				return StatusEffectInstanceConfig{}, false
			}
			cfg := StatusEffectInstanceConfig{
				Definition: StatusEffectDefinitionCallbacks{},
				ExpiresAt:  func() time.Time { return candidate.expiresAt },
			}
			if candidate.attachment != nil {
				cfg.Attachment = &StatusEffectAttachmentConfig{
					Extend: func(time.Time) {},
					Expire: func(at time.Time) (any, bool) {
						candidate.attachment.expired = append(candidate.attachment.expired, at)
						return candidate.attachment.effect, candidate.attachment.shouldRecord
					},
					Clear: func() {
						candidate.attachment.clearCount++
					},
				}
			}
			return cfg, true
		},
		Remove: func(key string) {
			delete(instances, key)
		},
		RecordEffectEnd: func(effect any) {
			recorded = append(recorded, effect)
		},
	}

	AdvanceActorStatusEffects(cfg)

	if len(instances) != 0 {
		t.Fatalf("expected expired instance removed, map len %d", len(instances))
	}
	if len(attachment.expired) != 1 || !attachment.expired[0].Equal(now) {
		t.Fatalf("expected attachment expired at %v, got %#v", now, attachment.expired)
	}
	if attachment.clearCount != 1 {
		t.Fatalf("expected attachment cleared once, got %d", attachment.clearCount)
	}
	if len(recorded) != 1 {
		t.Fatalf("expected one effect end record, got %d", len(recorded))
	}
}

func TestAdvanceActorStatusEffectsRespectsCustomExpire(t *testing.T) {
	now := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	attachment := &statusEffectAttachmentAdvanceStub{effect: struct{}{}, shouldRecord: true}
	inst := &statusEffectAdvanceStub{
		valid:        true,
		expiresAt:    now,
		customExpire: true,
		attachment:   attachment,
	}

	instances := map[string]*statusEffectAdvanceStub{"burning": inst}
	var recorded []any

	cfg := AdvanceActorStatusEffectsConfig{
		Now: now,
		ForEachInstance: func(visitor func(key string, instance any)) {
			for key, inst := range instances {
				visitor(key, inst)
			}
		},
		Instance: func(value any) (StatusEffectInstanceConfig, bool) {
			candidate, _ := value.(*statusEffectAdvanceStub)
			if candidate == nil || !candidate.valid {
				return StatusEffectInstanceConfig{}, false
			}
			cfg := StatusEffectInstanceConfig{
				Definition: StatusEffectDefinitionCallbacks{},
				ExpiresAt:  func() time.Time { return candidate.expiresAt },
			}
			if candidate.customExpire {
				cfg.Definition.OnExpire = func(at time.Time) {
					candidate.expireCalls = append(candidate.expireCalls, at)
				}
			}
			if candidate.attachment != nil {
				cfg.Attachment = &StatusEffectAttachmentConfig{
					Extend: func(time.Time) {},
					Expire: func(at time.Time) (any, bool) {
						candidate.attachment.expired = append(candidate.attachment.expired, at)
						return candidate.attachment.effect, candidate.attachment.shouldRecord
					},
					Clear: func() {
						candidate.attachment.clearCount++
					},
				}
			}
			return cfg, true
		},
		Remove: func(key string) {
			delete(instances, key)
		},
		RecordEffectEnd: func(effect any) {
			recorded = append(recorded, effect)
		},
	}

	AdvanceActorStatusEffects(cfg)

	if len(instances) != 0 {
		t.Fatalf("expected expired instance removed, map len %d", len(instances))
	}
	if len(inst.expireCalls) != 1 || !inst.expireCalls[0].Equal(now) {
		t.Fatalf("expected custom expire called at %v, got %#v", now, inst.expireCalls)
	}
	if len(attachment.expired) != 0 {
		t.Fatalf("expected default attachment expire skipped, got %#v", attachment.expired)
	}
	if attachment.clearCount != 0 {
		t.Fatalf("expected attachment clear skipped, got %d", attachment.clearCount)
	}
	if len(recorded) != 0 {
		t.Fatalf("expected no effect end recorded, got %d", len(recorded))
	}
}

func TestAdvanceStatusEffectsDelegatesToActorConfigs(t *testing.T) {
	now := time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC)
	calls := 0

	iterator := func(apply func(AdvanceActorStatusEffectsConfig)) {
		cfg := AdvanceActorStatusEffectsConfig{}
		cfg.ForEachInstance = func(visitor func(key string, instance any)) {
			calls++
			if visitor != nil {
				visitor("probe", nil)
			}
		}
		cfg.Instance = func(any) (StatusEffectInstanceConfig, bool) {
			return StatusEffectInstanceConfig{}, false
		}
		cfg.Remove = func(string) {}
		apply(cfg)
	}

	AdvanceStatusEffects(AdvanceStatusEffectsConfig{
		Now:           now,
		ForEachPlayer: iterator,
		ForEachNPC:    iterator,
	})

	if calls != 2 {
		t.Fatalf("expected iterator invoked twice, got %d", calls)
	}
}
