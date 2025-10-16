package main

import (
	"testing"
	"time"

	"mine-and-die/server/logging"
)

type lifecycleCollector struct {
	spawns  []EffectSpawnEvent
	updates []EffectUpdateEvent
	ends    []EffectEndEvent
}

func (c *lifecycleCollector) collect(evt EffectLifecycleEvent) {
	if c == nil {
		return
	}
	switch e := evt.(type) {
	case EffectSpawnEvent:
		c.spawns = append(c.spawns, e)
	case EffectUpdateEvent:
		c.updates = append(c.updates, e)
	case EffectEndEvent:
		c.ends = append(c.ends, e)
	default:
		panic("unexpected lifecycle event type")
	}
}

func TestContractLifecycleSequencesByDeliveryKind(t *testing.T) {
	cases := []struct {
		name       string
		definition *EffectDefinition
		intent     EffectIntent
		ticks      int
		expect     struct {
			spawns      int
			updates     int
			ends        int
			follow      FollowMode
			endReason   EndReason
			attachCheck bool
		}
	}{
		{
			name: "area delivery emits spawn-update-end",
			definition: &EffectDefinition{
				TypeID:        "contract-case-area",
				Delivery:      DeliveryKindArea,
				Shape:         GeometryShapeCircle,
				Motion:        MotionKindInstant,
				Impact:        ImpactPolicyFirstHit,
				LifetimeTicks: 2,
				Client: ReplicationSpec{
					SendSpawn:   true,
					SendUpdates: true,
					SendEnd:     true,
				},
				End: EndPolicy{Kind: EndDuration},
			},
			intent: EffectIntent{
				EntryID:  "contract-case-area",
				Delivery: DeliveryKindArea,
				Geometry: EffectGeometry{Shape: GeometryShapeCircle},
			},
			ticks: 2,
			expect: struct {
				spawns      int
				updates     int
				ends        int
				follow      FollowMode
				endReason   EndReason
				attachCheck bool
			}{spawns: 1, updates: 2, ends: 1, follow: FollowNone, endReason: EndReasonExpired},
		},
		{
			name: "target delivery follows actor and updates each tick",
			definition: &EffectDefinition{
				TypeID:        "contract-case-target",
				Delivery:      DeliveryKindTarget,
				Shape:         GeometryShapeCircle,
				Motion:        MotionKindFollow,
				Impact:        ImpactPolicyFirstHit,
				LifetimeTicks: 3,
				Client: ReplicationSpec{
					SendSpawn:   true,
					SendUpdates: true,
					SendEnd:     true,
				},
				End: EndPolicy{Kind: EndDuration},
			},
			intent: EffectIntent{
				EntryID:       "contract-case-target",
				Delivery:      DeliveryKindTarget,
				SourceActorID: "target-owner",
				TargetActorID: "attached-target",
				Geometry:      EffectGeometry{Shape: GeometryShapeCircle},
			},
			ticks: 3,
			expect: struct {
				spawns      int
				updates     int
				ends        int
				follow      FollowMode
				endReason   EndReason
				attachCheck bool
			}{spawns: 1, updates: 3, ends: 1, follow: FollowTarget, endReason: EndReasonExpired, attachCheck: true},
		},
		{
			name: "visual delivery omits updates but still ends",
			definition: &EffectDefinition{
				TypeID:        "contract-case-visual",
				Delivery:      DeliveryKindVisual,
				Shape:         GeometryShapeRect,
				Motion:        MotionKindNone,
				Impact:        ImpactPolicyFirstHit,
				LifetimeTicks: 2,
				Client: ReplicationSpec{
					SendSpawn:   true,
					SendUpdates: false,
					SendEnd:     true,
				},
				End: EndPolicy{Kind: EndDuration},
			},
			intent: EffectIntent{
				EntryID:  "contract-case-visual",
				Delivery: DeliveryKindVisual,
				Geometry: EffectGeometry{Shape: GeometryShapeRect},
			},
			ticks: 2,
			expect: struct {
				spawns      int
				updates     int
				ends        int
				follow      FollowMode
				endReason   EndReason
				attachCheck bool
			}{spawns: 1, updates: 0, ends: 1, follow: FollowNone, endReason: EndReasonExpired},
		},
	}

	dt := 1.0 / float64(tickRate)

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			world := newWorld(fullyFeaturedTestWorldConfig(), logging.NopPublisher{})
			world.effectManager.definitions[tc.definition.TypeID] = tc.definition

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

			lifecycleSeqs := []Seq{spawn.Seq}
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

func TestEffectManagerRespectsTickCadence(t *testing.T) {
	manager := newEffectManager(nil)
	if manager == nil {
		t.Fatalf("expected effect manager instance")
	}

	invocationTicks := make([]Tick, 0)
	hookID := "contract.test.tickCadence"
	manager.hooks[hookID] = effectHookSet{
		OnTick: func(m *EffectManager, instance *EffectInstance, tick Tick, now time.Time) {
			invocationTicks = append(invocationTicks, tick)
		},
	}

	definition := &EffectDefinition{
		TypeID:        "contract-tick-cadence",
		Delivery:      DeliveryKindArea,
		Shape:         GeometryShapeCircle,
		Motion:        MotionKindInstant,
		Impact:        ImpactPolicyNone,
		LifetimeTicks: 10,
		Hooks: EffectHooks{
			OnTick: hookID,
		},
		Client: ReplicationSpec{
			SendSpawn:   true,
			SendUpdates: true,
			SendEnd:     true,
		},
		End: EndPolicy{Kind: EndDuration},
	}

	manager.definitions[definition.TypeID] = definition

	manager.EnqueueIntent(EffectIntent{
		EntryID:     definition.TypeID,
		TypeID:      definition.TypeID,
		Geometry:    EffectGeometry{Shape: GeometryShapeCircle},
		TickCadence: 3,
	})

	collector := &lifecycleCollector{}
	start := time.Now()
	for tick := 1; tick <= 7; tick++ {
		manager.RunTick(Tick(tick), start.Add(time.Duration(tick-1)*time.Millisecond), collector.collect)
	}

	if len(collector.spawns) != 1 {
		t.Fatalf("expected 1 spawn event, got %d", len(collector.spawns))
	}

	instanceID := collector.spawns[0].Instance.ID

	if got := len(invocationTicks); got != 2 {
		t.Fatalf("expected 2 OnTick invocations, got %d", got)
	}

	expectedTicks := []Tick{3, 6}
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
