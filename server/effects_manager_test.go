package main

import (
	"testing"
	"time"
)

func TestEffectManagerRunTickWithoutEmitterProcessesHooks(t *testing.T) {
	manager := newEffectManager(nil)

	const effectType = "effect.test"
	const hookID = "hook.test.tick"

	manager.definitions[effectType] = &EffectDefinition{
		TypeID:        effectType,
		LifetimeTicks: 2,
		Hooks: EffectHooks{
			OnTick: hookID,
		},
		Client: ReplicationSpec{SendSpawn: true, SendUpdates: true, SendEnd: true},
		End:    EndPolicy{Kind: EndDuration},
	}

	tickCount := 0
	manager.hooks[hookID] = effectHookSet{
		OnTick: func(m *EffectManager, instance *EffectInstance, tick Tick, now time.Time) {
			tickCount++
		},
	}

	manager.EnqueueIntent(EffectIntent{
		TypeID:        effectType,
		DurationTicks: 2,
		SourceActorID: "owner-1",
	})

	now := time.Now()
	manager.RunTick(1, now, nil)
	if tickCount != 1 {
		t.Fatalf("expected hook to be invoked once on first tick, got %d", tickCount)
	}

	manager.RunTick(2, now.Add(time.Millisecond), nil)
	if tickCount != 2 {
		t.Fatalf("expected hook to continue firing without emitter, got %d invocations", tickCount)
	}

	if len(manager.instances) != 0 {
		t.Fatalf("expected effect instance to end after duration without emitter, found %d instances", len(manager.instances))
	}
}
