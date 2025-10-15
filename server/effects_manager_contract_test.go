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
				TypeID:   "contract-case-area",
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
				TypeID:        "contract-case-target",
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
				TypeID:   "contract-case-visual",
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
			world := newWorld(defaultWorldConfig(), logging.NopPublisher{})
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

func TestContractMeleeSpawnCarriesOwnerAndOffsets(t *testing.T) {
	world := newWorld(defaultWorldConfig(), logging.NopPublisher{})

	attacker := newTestPlayerState("contract-anchor-owner")
	attacker.X = 200
	attacker.Y = 180
	attacker.Facing = FacingRight
	attacker.cooldowns = make(map[string]time.Time)
	world.players[attacker.ID] = attacker

	intent, ok := NewMeleeIntent(&attacker.actorState)
	if !ok {
		t.Fatalf("expected melee intent to be constructed")
	}
	world.effectManager.EnqueueIntent(intent)

	collector := &lifecycleCollector{}
	dt := 1.0 / float64(tickRate)
	now := time.Unix(0, 0)
	world.Step(1, now, dt, nil, collector.collect)

	if len(collector.spawns) != 1 {
		t.Fatalf("expected 1 spawn, got %d", len(collector.spawns))
	}

	spawn := collector.spawns[0]
	if spawn.Instance.DeliveryState.Motion != nil {
		t.Fatalf("expected motion to be omitted for melee spawn, got %+v", spawn.Instance.DeliveryState.Motion)
	}
	if spawn.Instance.OwnerActorID != attacker.ID {
		t.Fatalf("expected ownerActorId %q, got %q", attacker.ID, spawn.Instance.OwnerActorID)
	}

	geom := spawn.Instance.DeliveryState.Geometry
	if geom.Width == 0 || geom.Height == 0 {
		t.Fatalf("expected spawn geometry dimensions, got width=%d height=%d", geom.Width, geom.Height)
	}
	expectedOffsetX := quantizeWorldCoord(playerHalf + meleeAttackReach/2)
	if geom.OffsetX != expectedOffsetX {
		t.Fatalf("expected offsetX %d, got %d", expectedOffsetX, geom.OffsetX)
	}
	if geom.OffsetY != 0 {
		t.Fatalf("expected offsetY 0, got %d", geom.OffsetY)
	}

	batch := world.journal.DrainEffectEvents()
	if len(batch.Spawns) != 1 {
		t.Fatalf("expected journal to contain 1 spawn, got %d", len(batch.Spawns))
	}
	drained := batch.Spawns[0]
	if drained.Instance.OwnerActorID != attacker.ID {
		t.Fatalf("expected drained ownerActorId %q, got %q", attacker.ID, drained.Instance.OwnerActorID)
	}
	geom = drained.Instance.DeliveryState.Geometry
	if geom.Width == 0 || geom.Height == 0 {
		t.Fatalf("expected drained geometry dimensions, got width=%d height=%d", geom.Width, geom.Height)
	}
	if geom.OffsetX != expectedOffsetX {
		t.Fatalf("expected drained offsetX %d, got %d", expectedOffsetX, geom.OffsetX)
	}
	if geom.OffsetY != 0 {
		t.Fatalf("expected drained offsetY 0, got %d", geom.OffsetY)
	}
}

func TestContractStatusVisualFollowsTargetAnchor(t *testing.T) {
	world := newWorld(defaultWorldConfig(), logging.NopPublisher{})

	target := newTestPlayerState("contract-burning-target")
	target.X = 320
	target.Y = 280
	world.players[target.ID] = target

	lifetime := 2 * time.Second
	intent, ok := NewStatusVisualIntent(&target.actorState, "lava-source", effectTypeBurningVisual, lifetime)
	if !ok {
		t.Fatalf("expected status visual intent to be constructed")
	}
	world.effectManager.EnqueueIntent(intent)

	collector := &lifecycleCollector{}
	dt := 1.0 / float64(tickRate)
	now := time.Unix(0, 0)
	world.Step(1, now, dt, nil, collector.collect)

	if len(collector.spawns) != 1 {
		t.Fatalf("expected 1 spawn, got %d", len(collector.spawns))
	}

	spawn := collector.spawns[0]
	if spawn.Instance.FollowActorID != target.ID {
		t.Fatalf("expected followActorId %q, got %q", target.ID, spawn.Instance.FollowActorID)
	}
	if spawn.Instance.DeliveryState.AttachedActorID != target.ID {
		t.Fatalf("expected attachedActorId %q, got %q", target.ID, spawn.Instance.DeliveryState.AttachedActorID)
	}
	geom := spawn.Instance.DeliveryState.Geometry
	if geom.OffsetX != 0 || geom.OffsetY != 0 {
		t.Fatalf("expected zero offsets, got (%d,%d)", geom.OffsetX, geom.OffsetY)
	}
}

func TestContractPipelineMeleeSpawnAnchorsOwner(t *testing.T) {
	world := newWorld(defaultWorldConfig(), logging.NopPublisher{})

	attacker := newTestPlayerState("contract-pipeline-owner")
	attacker.cooldowns = make(map[string]time.Time)
	world.AddPlayer(attacker)

	collector := &lifecycleCollector{}
	dt := 1.0 / float64(tickRate)
	now := time.Unix(0, 0)
	commands := []Command{{
		ActorID: attacker.ID,
		Type:    CommandAction,
		Action:  &ActionCommand{Name: effectTypeAttack},
	}}

	world.Step(1, now, dt, commands, collector.collect)

	if len(collector.spawns) != 1 {
		t.Fatalf("expected 1 spawn, got %d", len(collector.spawns))
	}

	spawn := collector.spawns[0]
	if spawn.Instance.DeliveryState.Motion != nil {
		t.Fatalf("expected motion to be omitted for static melee, got %+v", spawn.Instance.DeliveryState.Motion)
	}
	if spawn.Instance.OwnerActorID != attacker.ID {
		t.Fatalf("expected ownerActorId %q, got %q", attacker.ID, spawn.Instance.OwnerActorID)
	}
	geom := spawn.Instance.DeliveryState.Geometry
	if geom.OffsetX == 0 && geom.OffsetY == 0 {
		t.Fatalf("expected melee offsets to be non-zero, got (%d,%d)", geom.OffsetX, geom.OffsetY)
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
