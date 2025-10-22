package effects

import (
	"testing"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

type spawnRecord struct {
	effectType string
	category   string
}

func TestSpawnAreaEffectRegistersAndRecords(t *testing.T) {
	now := time.Unix(1700, 0)
	source := &State{
		Owner:  "owner-1",
		X:      10,
		Y:      20,
		Width:  4,
		Height: 8,
	}
	spec := &ExplosionSpec{
		EffectType: "fireball",
		Radius:     3,
		Duration:   2 * time.Second,
		Params: map[string]float64{
			"base": 7,
		},
	}

	var allocated []string
	var registered *State
	var records []spawnRecord

	effect := SpawnAreaEffect(AreaEffectSpawnConfig{
		Source:      source,
		Spec:        spec,
		Now:         now,
		CurrentTick: effectcontract.Tick(42),
		AllocateID: func() string {
			allocated = append(allocated, "effect-7")
			return "effect-7"
		},
		Register: func(effect *State) bool {
			registered = effect
			return true
		},
		RecordSpawn: func(effectType, category string) {
			records = append(records, spawnRecord{effectType: effectType, category: category})
		},
	})

	if effect == nil {
		t.Fatalf("expected explosion effect to be created")
	}
	if effect != registered {
		t.Fatalf("expected returned effect to be the registered value")
	}
	if len(allocated) != 1 || allocated[0] != "effect-7" {
		t.Fatalf("expected allocator to be used once, got %v", allocated)
	}
	if len(records) != 1 || records[0].effectType != "fireball" || records[0].category != "explosion" {
		t.Fatalf("unexpected spawn record: %#v", records)
	}

	if effect.ID != "effect-7" {
		t.Fatalf("expected effect ID to match allocator, got %q", effect.ID)
	}
	if effect.Type != "fireball" {
		t.Fatalf("expected effect type fireball, got %q", effect.Type)
	}
	if effect.Owner != "owner-1" {
		t.Fatalf("expected owner owner-1, got %q", effect.Owner)
	}
	if effect.Start != now.UnixMilli() {
		t.Fatalf("expected start %d, got %d", now.UnixMilli(), effect.Start)
	}
	if effect.Duration != spec.Duration.Milliseconds() {
		t.Fatalf("expected duration %d, got %d", spec.Duration.Milliseconds(), effect.Duration)
	}

	expectedSize := spec.Radius * 2
	if effect.Width != expectedSize || effect.Height != expectedSize {
		t.Fatalf("expected size %f, got width %f height %f", expectedSize, effect.Width, effect.Height)
	}
	expectedX := (source.X + source.Width/2) - expectedSize/2
	expectedY := (source.Y + source.Height/2) - expectedSize/2
	if effect.X != expectedX || effect.Y != expectedY {
		t.Fatalf("unexpected position: got (%f,%f) want (%f,%f)", effect.X, effect.Y, expectedX, expectedY)
	}

	if got := effect.Params["base"]; got != 7 {
		t.Fatalf("expected base param 7, got %f", got)
	}
	if got := effect.Params["radius"]; got != 3 {
		t.Fatalf("expected radius param 3, got %f", got)
	}
	if got := effect.Params["duration_ms"]; got != 2000 {
		t.Fatalf("expected duration_ms 2000, got %f", got)
	}

	if effect.Instance.ID != "effect-7" {
		t.Fatalf("expected instance ID effect-7, got %q", effect.Instance.ID)
	}
	if effect.Instance.DefinitionID != "fireball" {
		t.Fatalf("expected definition fireball, got %q", effect.Instance.DefinitionID)
	}
	if effect.Instance.OwnerActorID != "owner-1" {
		t.Fatalf("expected instance owner owner-1, got %q", effect.Instance.OwnerActorID)
	}
	if effect.Instance.StartTick != effectcontract.Tick(42) {
		t.Fatalf("expected start tick 42, got %d", effect.Instance.StartTick)
	}
	if effect.TelemetrySpawnTick != effectcontract.Tick(42) {
		t.Fatalf("expected telemetry spawn tick 42, got %d", effect.TelemetrySpawnTick)
	}

	expectedExpiry := now.Add(spec.Duration)
	if !effect.ExpiresAt.Equal(expectedExpiry) {
		t.Fatalf("expected expiry %v, got %v", expectedExpiry, effect.ExpiresAt)
	}
}

func TestSpawnAreaEffectFallbackSizeAndNoDurationParam(t *testing.T) {
	source := &State{Owner: "owner", X: 100, Y: 200}
	spec := &ExplosionSpec{EffectType: "shockwave"}

	var registered *State
	effect := SpawnAreaEffect(AreaEffectSpawnConfig{
		Source: source,
		Spec:   spec,
		Now:    time.Unix(2000, 0),
		AllocateID: func() string {
			return "effect-8"
		},
		Register: func(effect *State) bool {
			registered = effect
			return true
		},
	})

	if effect == nil {
		t.Fatalf("expected explosion effect to be created")
	}
	if effect.Width != 1 || effect.Height != 1 {
		t.Fatalf("expected fallback size 1, got width %f height %f", effect.Width, effect.Height)
	}
	if effect.Params["radius"] != 0 {
		t.Fatalf("expected radius param 0, got %f", effect.Params["radius"])
	}
	if _, ok := effect.Params["duration_ms"]; ok {
		t.Fatalf("expected duration_ms to be omitted when no duration configured")
	}
	if effect != registered {
		t.Fatalf("expected registered effect to be returned")
	}
}

func TestSpawnAreaEffectStopsOnRegisterFailure(t *testing.T) {
	spec := &ExplosionSpec{EffectType: "fire", Radius: 2}
	source := &State{}

	var recordCalled bool
	effect := SpawnAreaEffect(AreaEffectSpawnConfig{
		Source: source,
		Spec:   spec,
		Now:    time.Unix(2500, 0),
		AllocateID: func() string {
			return "effect-9"
		},
		Register: func(*State) bool {
			return false
		},
		RecordSpawn: func(string, string) {
			recordCalled = true
		},
	})

	if effect != nil {
		t.Fatalf("expected spawn to abort when registration fails")
	}
	if recordCalled {
		t.Fatalf("expected record callback to be skipped on registration failure")
	}
}

func TestSpawnAreaEffectRequiresID(t *testing.T) {
	spec := &ExplosionSpec{EffectType: "fire"}
	source := &State{}

	var registerCalled bool
	effect := SpawnAreaEffect(AreaEffectSpawnConfig{
		Source: source,
		Spec:   spec,
		Now:    time.Unix(2600, 0),
		Register: func(*State) bool {
			registerCalled = true
			return true
		},
	})

	if effect != nil {
		t.Fatalf("expected spawn to fail without an allocated ID")
	}
	if registerCalled {
		t.Fatalf("expected register callback to be skipped when no ID is allocated")
	}
}
