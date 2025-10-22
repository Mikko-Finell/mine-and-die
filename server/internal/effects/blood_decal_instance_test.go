package effects

import (
	"testing"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

type fakeRuntime struct {
	state    map[string]any
	registry Registry
}

func newFakeRuntime(reg Registry) *fakeRuntime {
	return &fakeRuntime{state: make(map[string]any), registry: reg}
}

func (f *fakeRuntime) InstanceState(id string) any {
	if f == nil || id == "" {
		return nil
	}
	return f.state[id]
}

func (f *fakeRuntime) SetInstanceState(id string, value any) {
	if f == nil || id == "" {
		return
	}
	f.state[id] = value
}

func (f *fakeRuntime) ClearInstanceState(id string) {
	if f == nil || id == "" {
		return
	}
	delete(f.state, id)
}

func (f *fakeRuntime) Registry() Registry {
	if f == nil {
		return Registry{}
	}
	return f.registry
}

func TestEnsureBloodDecalInstanceSpawnsRegistersAndSyncs(t *testing.T) {
	tileSize := 40.0
	now := time.UnixMilli(1_024)
	lifetime := 45

	var effects []*State
	byID := make(map[string]*State)
	registry := Registry{Effects: &effects, ByID: &byID}
	rt := newFakeRuntime(registry)

	var pruneCalls int
	var pruneAt time.Time
	prune := func(at time.Time) {
		pruneCalls++
		pruneAt = at
	}

	var recorded []struct {
		effectType string
		producer   string
	}
	recordSpawn := func(effectType, producer string) {
		recorded = append(recorded, struct {
			effectType string
			producer   string
		}{effectType: effectType, producer: producer})
	}

	instance := &effectcontract.EffectInstance{
		ID:           "blood-1",
		DefinitionID: effectcontract.EffectIDBloodSplatter,
		OwnerActorID: "npc-1",
		StartTick:    12,
		DeliveryState: effectcontract.EffectDeliveryState{
			Geometry: effectcontract.EffectGeometry{
				Width:  QuantizeWorldCoord(32, tileSize),
				Height: QuantizeWorldCoord(28, tileSize),
			},
		},
		BehaviorState: effectcontract.EffectBehaviorState{
			TicksRemaining: lifetime,
			Extra: map[string]int{
				"centerX": QuantizeWorldCoord(120, tileSize),
				"centerY": QuantizeWorldCoord(160, tileSize),
			},
		},
	}

	params := map[string]float64{"drag": 0.9}
	colors := []string{"#111", "#222"}

	effect := EnsureBloodDecalInstance(BloodDecalInstanceConfig{
		Runtime:         rt,
		Instance:        instance,
		Now:             now,
		TileSize:        tileSize,
		TickRate:        15,
		DefaultSize:     30,
		DefaultDuration: 1200 * time.Millisecond,
		Params:          params,
		Colors:          colors,
		PruneExpired:    prune,
		RecordSpawn:     recordSpawn,
	})

	if effect == nil {
		t.Fatal("expected blood decal effect")
	}
	if pruneCalls != 1 || pruneAt != now {
		t.Fatalf("expected prune to run once at %v, got %d calls at %v", now, pruneCalls, pruneAt)
	}
	if len(recorded) != 1 {
		t.Fatalf("expected spawn telemetry to fire once, got %d", len(recorded))
	}
	if recorded[0].effectType != effect.Type || recorded[0].producer != "blood-decal" {
		t.Fatalf("unexpected spawn telemetry payload: %+v", recorded[0])
	}
	if stored, ok := rt.state[instance.ID]; !ok || stored != effect {
		t.Fatalf("expected runtime state to store effect for %q", instance.ID)
	}
	if len(effects) != 1 || effects[0] != effect {
		t.Fatalf("expected effect slice to contain registered effect, got %d", len(effects))
	}
	if got := byID[instance.ID]; got != effect {
		t.Fatalf("expected registry to index effect by id, got %v", got)
	}
	if instance.BehaviorState.TicksRemaining != lifetime {
		t.Fatalf("expected ticks remaining to stay %d, got %d", lifetime, instance.BehaviorState.TicksRemaining)
	}
	expectedCenterX := QuantizeWorldCoord(effect.X+effect.Width/2, tileSize)
	expectedCenterY := QuantizeWorldCoord(effect.Y+effect.Height/2, tileSize)
	if instance.Params["centerX"] != expectedCenterX || instance.Params["centerY"] != expectedCenterY {
		t.Fatalf("expected params to mirror effect center, got (%d, %d)", instance.Params["centerX"], instance.Params["centerY"])
	}
	if instance.Colors[0] != colors[0] || instance.Colors[1] != colors[1] {
		t.Fatalf("expected colors to sync to instance, got %v", instance.Colors)
	}
}

func TestEnsureBloodDecalInstanceSyncsExistingEffect(t *testing.T) {
	tileSize := 40.0
	now := time.UnixMilli(2_048)

	existing := &State{
		ID:     "effect-existing",
		Type:   effectcontract.EffectIDBloodSplatter,
		Owner:  "npc-2",
		X:      50,
		Y:      75,
		Width:  32,
		Height: 26,
		Colors: []string{"#111"},
	}
	var effects = []*State{existing}
	byID := map[string]*State{existing.ID: existing}
	registry := Registry{Effects: &effects, ByID: &byID}
	rt := newFakeRuntime(registry)
	rt.state[existing.ID] = existing

	var pruneCalls int
	prune := func(time.Time) { pruneCalls++ }
	var recorded int
	record := func(string, string) { recorded++ }

	instance := &effectcontract.EffectInstance{
		ID:           existing.ID,
		DefinitionID: existing.Type,
		OwnerActorID: existing.Owner,
		BehaviorState: effectcontract.EffectBehaviorState{
			Extra: make(map[string]int),
		},
	}

	effect := EnsureBloodDecalInstance(BloodDecalInstanceConfig{
		Runtime:      rt,
		Instance:     instance,
		Now:          now,
		TileSize:     tileSize,
		DefaultSize:  30,
		Colors:       []string{"#222"},
		PruneExpired: prune,
		RecordSpawn:  record,
	})

	if effect != existing {
		t.Fatalf("expected existing effect to be returned, got %+v", effect)
	}
	if pruneCalls != 0 {
		t.Fatalf("expected prune to be skipped when effect exists, got %d calls", pruneCalls)
	}
	if recorded != 0 {
		t.Fatalf("expected no spawn telemetry when effect exists, got %d calls", recorded)
	}
	expectedWidth := QuantizeWorldCoord(existing.Width, tileSize)
	expectedHeight := QuantizeWorldCoord(existing.Height, tileSize)
	if instance.DeliveryState.Geometry.Width != expectedWidth || instance.DeliveryState.Geometry.Height != expectedHeight {
		t.Fatalf("unexpected geometry sync: %+v", instance.DeliveryState.Geometry)
	}
}

func TestEnsureBloodDecalInstanceRegisterFailureClearsTicks(t *testing.T) {
	rt := newFakeRuntime(Registry{})
	instance := &effectcontract.EffectInstance{
		ID: "missing-registry",
		BehaviorState: effectcontract.EffectBehaviorState{
			TicksRemaining: 99,
			Extra: map[string]int{
				"centerX": 0,
				"centerY": 0,
			},
		},
	}

	effect := EnsureBloodDecalInstance(BloodDecalInstanceConfig{
		Runtime:         rt,
		Instance:        instance,
		Now:             time.UnixMilli(3_072),
		TileSize:        40,
		TickRate:        15,
		DefaultSize:     20,
		DefaultDuration: time.Second,
		PruneExpired:    func(time.Time) {},
	})

	if effect != nil {
		t.Fatalf("expected ensure to fail without registry, got effect %+v", effect)
	}
	if instance.BehaviorState.TicksRemaining != 0 {
		t.Fatalf("expected ticks remaining to be cleared, got %d", instance.BehaviorState.TicksRemaining)
	}
	if len(rt.state) != 0 {
		t.Fatalf("expected runtime state to stay empty, got %v", rt.state)
	}
}

func TestEnsureBloodDecalInstanceSpawnFailure(t *testing.T) {
	var pruneCalls int
	prune := func(time.Time) { pruneCalls++ }

	rt := newFakeRuntime(Registry{})
	instance := &effectcontract.EffectInstance{ID: "spawnless"}

	effect := EnsureBloodDecalInstance(BloodDecalInstanceConfig{
		Runtime:      rt,
		Instance:     instance,
		Now:          time.UnixMilli(4_096),
		TileSize:     40,
		DefaultSize:  30,
		PruneExpired: prune,
	})

	if effect != nil {
		t.Fatalf("expected spawn to fail without center coordinates, got %+v", effect)
	}
	if pruneCalls != 1 {
		t.Fatalf("expected prune to run before spawn failure, got %d calls", pruneCalls)
	}
}
