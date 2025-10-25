package server

import (
	"testing"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	internaleffects "mine-and-die/server/internal/effects"
	"mine-and-die/server/logging"
)

type lifecycleCollector struct {
	spawns  []effectcontract.EffectSpawnEvent
	updates []effectcontract.EffectUpdateEvent
	ends    []effectcontract.EffectEndEvent
}

func (c *lifecycleCollector) collect(evt effectcontract.EffectLifecycleEvent) {
	if c == nil {
		return
	}
	switch e := evt.(type) {
	case effectcontract.EffectSpawnEvent:
		c.spawns = append(c.spawns, e)
	case effectcontract.EffectUpdateEvent:
		c.updates = append(c.updates, e)
	case effectcontract.EffectEndEvent:
		c.ends = append(c.ends, e)
	default:
		panic("unexpected lifecycle event type")
	}
}

func TestContractLifecycleSequencesByDeliveryKind(t *testing.T) {
	cases := []struct {
		name       string
		definition *effectcontract.EffectDefinition
		intent     effectcontract.EffectIntent
		ticks      int
		expect     struct {
			spawns      int
			updates     int
			ends        int
			follow      effectcontract.FollowMode
			endReason   effectcontract.EndReason
			attachCheck bool
		}
	}{
		{
			name: "area delivery emits spawn-update-end",
			definition: &effectcontract.EffectDefinition{
				TypeID:        "contract-case-area",
				Delivery:      effectcontract.DeliveryKindArea,
				Shape:         effectcontract.GeometryShapeCircle,
				Motion:        effectcontract.MotionKindInstant,
				Impact:        effectcontract.ImpactPolicyFirstHit,
				LifetimeTicks: 2,
				Client: effectcontract.ReplicationSpec{
					SendSpawn:   true,
					SendUpdates: true,
					SendEnd:     true,
				},
				End: effectcontract.EndPolicy{Kind: effectcontract.EndDuration},
			},
			intent: effectcontract.EffectIntent{
				EntryID:  "contract-case-area",
				Delivery: effectcontract.DeliveryKindArea,
				Geometry: effectcontract.EffectGeometry{Shape: effectcontract.GeometryShapeCircle},
			},
			ticks: 2,
			expect: struct {
				spawns      int
				updates     int
				ends        int
				follow      effectcontract.FollowMode
				endReason   effectcontract.EndReason
				attachCheck bool
			}{spawns: 1, updates: 2, ends: 1, follow: effectcontract.FollowNone, endReason: effectcontract.EndReasonExpired},
		},
		{
			name: "target delivery follows actor and updates each tick",
			definition: &effectcontract.EffectDefinition{
				TypeID:        "contract-case-target",
				Delivery:      effectcontract.DeliveryKindTarget,
				Shape:         effectcontract.GeometryShapeCircle,
				Motion:        effectcontract.MotionKindFollow,
				Impact:        effectcontract.ImpactPolicyFirstHit,
				LifetimeTicks: 3,
				Client: effectcontract.ReplicationSpec{
					SendSpawn:   true,
					SendUpdates: true,
					SendEnd:     true,
				},
				End: effectcontract.EndPolicy{Kind: effectcontract.EndDuration},
			},
			intent: effectcontract.EffectIntent{
				EntryID:       "contract-case-target",
				Delivery:      effectcontract.DeliveryKindTarget,
				SourceActorID: "target-owner",
				TargetActorID: "attached-target",
				Geometry:      effectcontract.EffectGeometry{Shape: effectcontract.GeometryShapeCircle},
			},
			ticks: 3,
			expect: struct {
				spawns      int
				updates     int
				ends        int
				follow      effectcontract.FollowMode
				endReason   effectcontract.EndReason
				attachCheck bool
			}{spawns: 1, updates: 3, ends: 1, follow: effectcontract.FollowTarget, endReason: effectcontract.EndReasonExpired, attachCheck: true},
		},
		{
			name: "visual delivery omits updates but still ends",
			definition: &effectcontract.EffectDefinition{
				TypeID:        "contract-case-visual",
				Delivery:      effectcontract.DeliveryKindVisual,
				Shape:         effectcontract.GeometryShapeRect,
				Motion:        effectcontract.MotionKindNone,
				Impact:        effectcontract.ImpactPolicyFirstHit,
				LifetimeTicks: 2,
				Client: effectcontract.ReplicationSpec{
					SendSpawn:   true,
					SendUpdates: false,
					SendEnd:     true,
				},
				End: effectcontract.EndPolicy{Kind: effectcontract.EndDuration},
			},
			intent: effectcontract.EffectIntent{
				EntryID:  "contract-case-visual",
				Delivery: effectcontract.DeliveryKindVisual,
				Geometry: effectcontract.EffectGeometry{Shape: effectcontract.GeometryShapeRect},
			},
			ticks: 2,
			expect: struct {
				spawns      int
				updates     int
				ends        int
				follow      effectcontract.FollowMode
				endReason   effectcontract.EndReason
				attachCheck bool
			}{spawns: 1, updates: 0, ends: 1, follow: effectcontract.FollowNone, endReason: effectcontract.EndReasonExpired},
		},
	}

	dt := 1.0 / float64(tickRate)

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			world := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
			world.effectManager.Definitions()[tc.definition.TypeID] = tc.definition

			collector := &lifecycleCollector{}
			world.effectManager.EnqueueIntent(tc.intent)

			start := time.Now()
			for tick := 1; tick <= tc.ticks; tick++ {
				world.Step(uint64(tick), start.Add(time.Duration(tick-1)*time.Millisecond), dt, nil, collector.collect)
			}

			if len(collector.spawns) != tc.expect.spawns {
				t.Fatalf("expected %d spawn events, got %d", tc.expect.spawns, len(collector.spawns))
			}
			if len(collector.updates) != tc.expect.updates {
				t.Fatalf("expected %d update events, got %d", tc.expect.updates, len(collector.updates))
			}
			if len(collector.ends) != tc.expect.ends {
				t.Fatalf("expected %d end events, got %d", tc.expect.ends, len(collector.ends))
			}

			spawn := collector.spawns[0]
			if spawn.Instance.EntryID != tc.intent.EntryID {
				t.Fatalf("expected instance entry id %q, got %q", tc.intent.EntryID, spawn.Instance.EntryID)
			}
			if spawn.Instance.Definition == nil {
				t.Fatalf("expected spawn to include definition metadata")
			}
			if spawn.Instance.Definition.Delivery != tc.definition.Delivery {
				t.Fatalf("expected definition delivery %q, got %q", tc.definition.Delivery, spawn.Instance.Definition.Delivery)
			}
			if spawn.Instance.DeliveryState.Follow != tc.expect.follow {
				t.Fatalf("expected follow mode %q, got %q", tc.expect.follow, spawn.Instance.DeliveryState.Follow)
			}
			if tc.expect.attachCheck && spawn.Instance.DeliveryState.AttachedActorID != tc.intent.TargetActorID {
				t.Fatalf("expected attached actor %q, got %q", tc.intent.TargetActorID, spawn.Instance.DeliveryState.AttachedActorID)
			}

			lifecycleSeqs := []effectcontract.Seq{spawn.Seq}
			instanceID := spawn.Instance.ID

			for _, update := range collector.updates {
				if update.ID != instanceID {
					t.Fatalf("expected update id %q, got %q", instanceID, update.ID)
				}
				lifecycleSeqs = append(lifecycleSeqs, update.Seq)
			}

			if tc.expect.ends > 0 {
				end := collector.ends[0]
				if end.ID != instanceID {
					t.Fatalf("expected end id %q, got %q", instanceID, end.ID)
				}
				if end.Reason != tc.expect.endReason {
					t.Fatalf("expected end reason %q, got %q", tc.expect.endReason, end.Reason)
				}
				lifecycleSeqs = append(lifecycleSeqs, end.Seq)
			}

			for i := 1; i < len(lifecycleSeqs); i++ {
				if lifecycleSeqs[i] <= lifecycleSeqs[i-1] {
					t.Fatalf("expected strictly increasing lifecycle sequence, got %v", lifecycleSeqs)
				}
			}
		})
	}
}

func TestContractBurningVisualUpdatesAttachedEffectLifetime(t *testing.T) {
	world := newTestWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
	if world.effectManager == nil {
		t.Fatal("expected effect manager to be initialized")
	}

	target := newTestPlayerState("contract-burning-lifetime")
	target.X = 200
	target.Y = 200
	world.AddPlayer(target)

	actor := &world.players[target.ID].ActorState
	now := time.Unix(123, 0)

	if applied := world.applyStatusEffect(actor, StatusEffectBurning, "lava-source", now); !applied {
		t.Fatalf("expected burning status effect to apply")
	}

	world.effectManager.RunTick(effectcontract.Tick(1), now, nil)

	inst := actor.StatusEffects[StatusEffectBurning]
	if inst == nil {
		t.Fatalf("expected burning status effect instance to persist")
	}
	effect, ok := inst.AttachedEffect().(*effectState)
	if !ok || effect == nil {
		t.Fatalf("expected burning visual to attach to status effect")
	}

	inst.ExpiresAt = inst.ExpiresAt.Add(2 * burningTickInterval)

	later := now.Add(50 * time.Millisecond)
	world.effectManager.RunTick(effectcontract.Tick(2), later, nil)

	if !effect.ExpiresAt.Equal(inst.ExpiresAt) {
		t.Fatalf("expected effect expiry %v to match status instance %v", effect.ExpiresAt, inst.ExpiresAt)
	}

	expectedDuration := inst.ExpiresAt.Sub(time.UnixMilli(effect.Start))
	if expectedDuration < 0 {
		expectedDuration = 0
	}
	if effect.Duration != expectedDuration.Milliseconds() {
		t.Fatalf("expected effect duration %d, got %d", expectedDuration.Milliseconds(), effect.Duration)
	}

	delete(actor.StatusEffects, StatusEffectBurning)

	expireNow := later.Add(75 * time.Millisecond)
	world.effectManager.RunTick(effectcontract.Tick(3), expireNow, nil)

	if !effect.ExpiresAt.Equal(expireNow) {
		t.Fatalf("expected effect expiry to clamp to %v, got %v", expireNow, effect.ExpiresAt)
	}

	expectedDuration = expireNow.Sub(time.UnixMilli(effect.Start))
	if expectedDuration < 0 {
		expectedDuration = 0
	}
	if effect.Duration != expectedDuration.Milliseconds() {
		t.Fatalf("expected clamped duration %d, got %d", expectedDuration.Milliseconds(), effect.Duration)
	}
}

func TestEffectManagerRespectsTickCadence(t *testing.T) {
	manager := newEffectManager(nil)
	if manager == nil {
		t.Fatalf("expected effect manager instance")
	}

	invocationTicks := make([]effectcontract.Tick, 0)
	hookID := "contract.test.tickCadence"
	manager.Hooks()[hookID] = internaleffects.HookSet{
		OnTick: func(rt internaleffects.Runtime, instance *effectcontract.EffectInstance, tick effectcontract.Tick, now time.Time) {
			invocationTicks = append(invocationTicks, tick)
		},
	}

	definition := &effectcontract.EffectDefinition{
		TypeID:        "contract-tick-cadence",
		Delivery:      effectcontract.DeliveryKindArea,
		Shape:         effectcontract.GeometryShapeCircle,
		Motion:        effectcontract.MotionKindInstant,
		Impact:        effectcontract.ImpactPolicyNone,
		LifetimeTicks: 10,
		Hooks: effectcontract.EffectHooks{
			OnTick: hookID,
		},
		Client: effectcontract.ReplicationSpec{
			SendSpawn:   true,
			SendUpdates: true,
			SendEnd:     true,
		},
		End: effectcontract.EndPolicy{Kind: effectcontract.EndDuration},
	}

	manager.Definitions()[definition.TypeID] = definition

	manager.EnqueueIntent(effectcontract.EffectIntent{
		EntryID:     definition.TypeID,
		TypeID:      definition.TypeID,
		Geometry:    effectcontract.EffectGeometry{Shape: effectcontract.GeometryShapeCircle},
		TickCadence: 3,
	})

	collector := &lifecycleCollector{}
	start := time.Now()
	for tick := 1; tick <= 7; tick++ {
		manager.RunTick(effectcontract.Tick(tick), start.Add(time.Duration(tick-1)*time.Millisecond), collector.collect)
	}

	if len(collector.spawns) != 1 {
		t.Fatalf("expected 1 spawn event, got %d", len(collector.spawns))
	}

	instanceID := collector.spawns[0].Instance.ID

	if got := len(invocationTicks); got != 2 {
		t.Fatalf("expected 2 OnTick invocations, got %d", got)
	}

	expectedTicks := []effectcontract.Tick{3, 6}
	for idx, expected := range expectedTicks {
		if invocationTicks[idx] != expected {
			t.Fatalf("expected OnTick at tick %d, got %d", expected, invocationTicks[idx])
		}
	}

	if len(collector.updates) != len(expectedTicks) {
		t.Fatalf("expected %d update events, got %d", len(expectedTicks), len(collector.updates))
	}

	for idx, update := range collector.updates {
		if update.ID != instanceID {
			t.Fatalf("expected update id %q, got %q", instanceID, update.ID)
		}
		if update.Tick != expectedTicks[idx] {
			t.Fatalf("expected update tick %d, got %d", expectedTicks[idx], update.Tick)
		}
	}
}
