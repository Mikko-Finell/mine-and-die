package server

import (
	"testing"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

func TestEffectManagerRunTickWithoutEmitterProcessesHooks(t *testing.T) {
	manager := newEffectManager(nil)

	const effectType = "effect.test"
	const hookID = "hook.test.tick"

	manager.definitions[effectType] = &effectcontract.EffectDefinition{
		TypeID:        effectType,
		LifetimeTicks: 2,
		Hooks: effectcontract.EffectHooks{
			OnTick: hookID,
		},
		Client: effectcontract.ReplicationSpec{SendSpawn: true, SendUpdates: true, SendEnd: true},
		End:    effectcontract.EndPolicy{Kind: effectcontract.EndDuration},
	}

	tickCount := 0
	manager.hooks[hookID] = effectHookSet{
		OnTick: func(m *EffectManager, instance *effectcontract.EffectInstance, tick effectcontract.Tick, now time.Time) {
			tickCount++
		},
	}

	manager.EnqueueIntent(effectcontract.EffectIntent{
		EntryID:       effectType,
		TypeID:        effectType,
		DurationTicks: 2,
		SourceActorID: "owner-1",
	})

	now := time.Now()
	manager.RunTick(effectcontract.Tick(1), now, nil)
	if tickCount != 1 {
		t.Fatalf("expected hook to be invoked once on first tick, got %d", tickCount)
	}

	if len(manager.instances) != 1 {
		t.Fatalf("expected one active instance after first tick, got %d", len(manager.instances))
	}
	for _, inst := range manager.instances {
		if inst.EntryID != effectType {
			t.Fatalf("expected instance entry id %q, got %q", effectType, inst.EntryID)
		}
	}

	manager.RunTick(effectcontract.Tick(2), now.Add(time.Millisecond), nil)
	if tickCount != 2 {
		t.Fatalf("expected hook to continue firing without emitter, got %d invocations", tickCount)
	}

	if len(manager.instances) != 0 {
		t.Fatalf("expected effect instance to end after duration without emitter, found %d instances", len(manager.instances))
	}
}
