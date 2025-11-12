package server

import (
	"testing"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	internaleffects "mine-and-die/server/internal/effects"
	worldpkg "mine-and-die/server/internal/world"
)

func TestEffectManagerRunTickWithoutEmitterProcessesHooks(t *testing.T) {
	manager := newEffectManager(nil)

	const effectType = "effect.test"
	const hookID = "hook.test.tick"

	manager.Definitions()[effectType] = &effectcontract.EffectDefinition{
		TypeID:        effectType,
		LifetimeTicks: 2,
		Hooks: effectcontract.EffectHooks{
			OnTick: hookID,
		},
		Client: effectcontract.ReplicationSpec{SendSpawn: true, SendUpdates: true, SendEnd: true},
		End:    effectcontract.EndPolicy{Kind: effectcontract.EndDuration},
	}

	tickCount := 0
	manager.Hooks()[hookID] = internaleffects.HookSet{
		OnTick: func(rt internaleffects.Runtime, instance *effectcontract.EffectInstance, tick effectcontract.Tick, now time.Time) {
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

	if len(manager.Instances()) != 1 {
		t.Fatalf("expected one active instance after first tick, got %d", len(manager.Instances()))
	}
	for _, inst := range manager.Instances() {
		if inst.EntryID != effectType {
			t.Fatalf("expected instance entry id %q, got %q", effectType, inst.EntryID)
		}
	}

	manager.RunTick(effectcontract.Tick(2), now.Add(time.Millisecond), nil)
	if tickCount != 2 {
		t.Fatalf("expected hook to continue firing without emitter, got %d invocations", tickCount)
	}

	if len(manager.Instances()) != 0 {
		t.Fatalf("expected effect instance to end after duration without emitter, found %d instances", len(manager.Instances()))
	}
}

func TestEffectManagerWorldEffectLoadsFromRegistry(t *testing.T) {
	world := &World{
		effectsByID: make(map[string]*effectState),
	}

	constructed, err := worldpkg.New(worldpkg.Config{}, worldpkg.Deps{})
	if err != nil {
		t.Fatalf("failed to construct internal world: %v", err)
	}
	bindAbilityGatesForTest(t, world, constructed)
	world.effectsRegistry = internaleffects.Registry{
		Effects: &world.effects,
		ByID:    &world.effectsByID,
	}

	effect := &effectState{ID: "effect.registry.lookup"}
	if !internaleffects.RegisterEffect(world.effectRegistry(), effect) {
		t.Fatalf("expected register effect to succeed")
	}

	manager := newEffectManager(world)
	if got := manager.WorldEffect(effect.ID); got != effect {
		t.Fatalf("expected world effect lookup to return registered instance, got %#v", got)
	}
}

func TestEffectManagerProjectileLifecycleUpdatesRegistry(t *testing.T) {
	ownerID := "owner-1"
	world := &World{
		players: map[string]*playerState{
			ownerID: {
				ActorState: actorState{Actor: Actor{ID: ownerID, X: 10, Y: 15, Facing: FacingRight}},
				Cooldowns:  make(map[string]time.Time),
			},
		},
		effectsByID:         make(map[string]*effectState),
		projectileTemplates: newProjectileTemplates(),
	}

	constructed, err := worldpkg.New(worldpkg.Config{}, worldpkg.Deps{})
	if err != nil {
		t.Fatalf("failed to construct internal world: %v", err)
	}
	constructed.Players()[ownerID] = world.players[ownerID]
	bindAbilityGatesForTest(t, world, constructed)
	world.effectsIndex = internaleffects.NewSpatialIndex(0, 0)
	world.effectsRegistry = internaleffects.Registry{
		Effects: &world.effects,
		ByID:    &world.effectsByID,
		Index:   world.effectsIndex,
	}

	manager := newEffectManager(world)

	manager.EnqueueIntent(effectcontract.EffectIntent{
		EntryID:       effectTypeFireball,
		TypeID:        effectTypeFireball,
		DurationTicks: 2,
		SourceActorID: ownerID,
	})

	now := time.Unix(0, 0)
	manager.RunTick(effectcontract.Tick(1), now, nil)

	instances := manager.Instances()
	if len(instances) != 1 {
		t.Fatalf("expected one active instance after spawn, got %d", len(instances))
	}
	var instanceID string
	for id := range instances {
		instanceID = id
	}
	if instanceID == "" {
		t.Fatalf("expected spawned instance id to be populated")
	}
	if _, ok := world.effectsByID[instanceID]; !ok {
		t.Fatalf("expected contract-managed effect to be registered in world registry")
	}

	manager.RunTick(effectcontract.Tick(2), now.Add(time.Second), nil)

	if _, ok := world.effectsByID[instanceID]; ok {
		t.Fatalf("expected projectile effect to be unregistered after teardown")
	}
}

func bindAbilityGatesForTest(t *testing.T, world *World, constructed *worldpkg.World) {
	t.Helper()
	if world == nil || constructed == nil {
		return
	}

	world.internalWorld = constructed
	world.configureAbilityOwnerAdapters()

	gates, ok := constructed.AbilityGates()
	if !ok {
		return
	}

	world.meleeAbilityGate = gates.Melee
	world.projectileAbilityGate = gates.Projectile
}
