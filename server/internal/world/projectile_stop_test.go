package world

import (
	"testing"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

func TestProjectileStopAdapterBindsCallbacks(t *testing.T) {
	effect := &struct{ ID string }{ID: "effect-stop"}
	now := time.UnixMilli(1_700_000_000)

	var (
		allocatedIDs []string
		registered   []any
		spawnRecords []struct {
			effectType string
			category   string
		}
		remainingValues []float64
		recordedReasons []string
	)

	adapter := NewProjectileStopAdapter(ProjectileStopAdapterConfig{
		AllocateID: func() string {
			id := "effect-bound"
			allocatedIDs = append(allocatedIDs, id)
			return id
		},
		RegisterEffect: func(state any) bool {
			registered = append(registered, state)
			return true
		},
		RecordEffectSpawn: func(effectType, category string) {
			spawnRecords = append(spawnRecords, struct {
				effectType string
				category   string
			}{effectType: effectType, category: category})
		},
		CurrentTick: func() effectcontract.Tick { return effectcontract.Tick(42) },
		SetRemainingRange: func(state any, remaining float64) {
			remainingValues = append(remainingValues, remaining)
			if state != effect {
				t.Fatalf("expected remaining range to bind original effect")
			}
		},
		RecordEffectEnd: func(state any, reason string) {
			recordedReasons = append(recordedReasons, reason)
			if state != effect {
				t.Fatalf("expected effect end hook to bind original effect")
			}
		},
	})

	cfg := adapter.StopConfig(effect, now)

	if cfg.Effect != effect {
		t.Fatalf("expected effect pointer to be forwarded")
	}
	if cfg.Now != now {
		t.Fatalf("expected timestamp %v, got %v", now, cfg.Now)
	}

	spawn := cfg.AreaEffectSpawn
	if spawn.Source != effect {
		t.Fatalf("expected spawn source to match effect")
	}
	if spawn.Now != now {
		t.Fatalf("expected spawn timestamp %v, got %v", now, spawn.Now)
	}
	if spawn.CurrentTick != effectcontract.Tick(42) {
		t.Fatalf("expected current tick 42, got %d", spawn.CurrentTick)
	}

	if spawn.AllocateID == nil {
		t.Fatalf("expected AllocateID callback to be bound")
	}
	if spawn.Register == nil {
		t.Fatalf("expected Register callback to be bound")
	}
	if spawn.RecordSpawn == nil {
		t.Fatalf("expected RecordSpawn callback to be bound")
	}

	id := spawn.AllocateID()
	if id != "effect-bound" {
		t.Fatalf("expected allocated ID 'effect-bound', got %q", id)
	}
	if len(allocatedIDs) != 1 {
		t.Fatalf("expected one allocation, got %d", len(allocatedIDs))
	}
	if !spawn.Register(effect) {
		t.Fatalf("expected register callback to return true")
	}
	if len(registered) != 1 || registered[0] != effect {
		t.Fatalf("expected register callback to receive effect")
	}
	spawn.RecordSpawn("explosion", "impact")
	if len(spawnRecords) != 1 {
		t.Fatalf("expected one spawn record, got %d", len(spawnRecords))
	}
	if spawnRecords[0].effectType != "explosion" || spawnRecords[0].category != "impact" {
		t.Fatalf("unexpected spawn record %#v", spawnRecords[0])
	}

	if cfg.SetRemainingRange == nil {
		t.Fatalf("expected SetRemainingRange to be bound")
	}
	cfg.SetRemainingRange(3.5)
	if len(remainingValues) != 1 || remainingValues[0] != 3.5 {
		t.Fatalf("remaining range callback not invoked correctly")
	}

	if cfg.RecordEffectEnd == nil {
		t.Fatalf("expected RecordEffectEnd to be bound")
	}
	cfg.RecordEffectEnd("expiry")
	if len(recordedReasons) != 1 || recordedReasons[0] != "expiry" {
		t.Fatalf("record effect end callback not invoked correctly")
	}
}

func TestProjectileStopAdapterHandlesNilCallbacks(t *testing.T) {
	adapter := NewProjectileStopAdapter(ProjectileStopAdapterConfig{})
	effect := &struct{ ID string }{ID: "effect-nil"}
	now := time.UnixMilli(1234)

	cfg := adapter.StopConfig(effect, now)
	if cfg.Effect != effect {
		t.Fatalf("expected effect pointer to be forwarded")
	}
	if cfg.Now != now {
		t.Fatalf("expected timestamp %v, got %v", now, cfg.Now)
	}

	if cfg.SetRemainingRange != nil {
		t.Fatalf("expected SetRemainingRange to be nil when not configured")
	}
	if cfg.RecordEffectEnd != nil {
		t.Fatalf("expected RecordEffectEnd to be nil when not configured")
	}

	if cfg.AreaEffectSpawn.Source != effect {
		t.Fatalf("expected spawn source to be forwarded")
	}
	if cfg.AreaEffectSpawn.Now != now {
		t.Fatalf("expected spawn timestamp %v, got %v", now, cfg.AreaEffectSpawn.Now)
	}
	if cfg.AreaEffectSpawn.AllocateID != nil {
		t.Fatalf("expected AllocateID to be nil when not configured")
	}
	if cfg.AreaEffectSpawn.Register != nil {
		t.Fatalf("expected Register to be nil when not configured")
	}
	if cfg.AreaEffectSpawn.RecordSpawn != nil {
		t.Fatalf("expected RecordSpawn to be nil when not configured")
	}
}

func TestProjectileStopAdapterInvokesCurrentTickCallback(t *testing.T) {
	tick := 0
	adapter := NewProjectileStopAdapter(ProjectileStopAdapterConfig{
		CurrentTick: func() effectcontract.Tick {
			tick++
			return effectcontract.Tick(tick)
		},
	})

	effect := &struct{}{}
	first := adapter.StopConfig(effect, time.UnixMilli(0))
	second := adapter.StopConfig(effect, time.UnixMilli(1))

	if first.AreaEffectSpawn.CurrentTick != effectcontract.Tick(1) {
		t.Fatalf("expected first tick 1, got %d", first.AreaEffectSpawn.CurrentTick)
	}
	if second.AreaEffectSpawn.CurrentTick != effectcontract.Tick(2) {
		t.Fatalf("expected second tick 2, got %d", second.AreaEffectSpawn.CurrentTick)
	}
}
