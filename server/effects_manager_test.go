package main

import (
	"testing"
	"time"

	"mine-and-die/server/logging"
)

func TestRunTickWithoutEmitterStillAdvancesEffects(t *testing.T) {
	world := newWorld(defaultWorldConfig(), logging.NopPublisher{})
	manager := world.effectManager
	if manager == nil {
		t.Fatal("expected effect manager to be initialized")
	}

	const hookID = "test.offline.hook"
	tickCount := 0
	manager.hooks[hookID] = effectHookSet{
		OnTick: func(m *EffectManager, instance *EffectInstance, tick Tick, now time.Time) {
			tickCount++
			if instance.BehaviorState.Extra == nil {
				instance.BehaviorState.Extra = make(map[string]int)
			}
			instance.BehaviorState.Extra["ticks"]++
		},
	}

	definition := &EffectDefinition{
		TypeID:        "offline-effect",
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
		End:   EndPolicy{Kind: EndDuration},
		Hooks: EffectHooks{OnTick: hookID},
	}
	manager.definitions[definition.TypeID] = definition

	manager.EnqueueIntent(EffectIntent{TypeID: definition.TypeID})

	now := time.Unix(0, 0)
	manager.RunTick(1, now, nil)

	if tickCount != 1 {
		t.Fatalf("expected on tick hook to run once, got %d", tickCount)
	}

	var instance *EffectInstance
	for _, inst := range manager.instances {
		if inst != nil {
			instance = inst
			break
		}
	}
	if instance == nil {
		t.Fatal("expected instance to remain active after first tick")
	}
	if instance.BehaviorState.TicksRemaining != 1 {
		t.Fatalf("expected ticks remaining to decrement to 1, got %d", instance.BehaviorState.TicksRemaining)
	}
	if value := instance.BehaviorState.Extra["ticks"]; value != 1 {
		t.Fatalf("expected behavior state extra ticks to be 1, got %d", value)
	}

	manager.RunTick(2, now.Add(time.Millisecond), nil)

	if tickCount != 2 {
		t.Fatalf("expected on tick hook to run twice, got %d", tickCount)
	}
	if len(manager.instances) != 0 {
		t.Fatalf("expected instance to end after second tick, found %d active", len(manager.instances))
	}
}
