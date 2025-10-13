package main

import (
	"math"
	"testing"
	"time"
)

func TestTelemetryEffectParitySnapshot(t *testing.T) {
	counters := newTelemetryCounters()
	for i := 0; i < 10; i++ {
		counters.RecordTickDuration(16 * time.Millisecond)
	}

	counters.RecordEffectParity(effectParitySummary{
		EffectType:    "attack",
		Source:        telemetrySourceLegacy,
		Hits:          3,
		UniqueVictims: 2,
		TotalDamage:   45,
		SpawnTick:     Tick(10),
		FirstHitTick:  Tick(12),
	})

	counters.RecordEffectParity(effectParitySummary{
		EffectType:    "attack",
		Source:        telemetrySourceContract,
		Hits:          0,
		UniqueVictims: 0,
		TotalDamage:   0,
		SpawnTick:     Tick(20),
		FirstHitTick:  0,
	})

	snapshot := counters.Snapshot()
	parity := snapshot.EffectParity
	if parity.TotalTicks != 10 {
		t.Fatalf("expected total ticks to be 10, got %d", parity.TotalTicks)
	}

	attackEntries, ok := parity.Entries["attack"]
	if !ok {
		t.Fatalf("expected attack entries to exist")
	}

	legacy, ok := attackEntries[telemetrySourceLegacy]
	if !ok {
		t.Fatalf("expected legacy entry to exist")
	}
	if legacy.Hits != 3 {
		t.Fatalf("expected 3 hits, got %d", legacy.Hits)
	}
	expectedHitRate := float64(legacy.Hits) * 1000.0 / 10.0
	if math.Abs(legacy.HitsPer1kTicks-expectedHitRate) > 1e-6 {
		t.Fatalf("unexpected hit rate: got %.3f want %.3f", legacy.HitsPer1kTicks, expectedHitRate)
	}
	if math.Abs(legacy.Damage-45) > 1e-9 {
		t.Fatalf("expected damage 45, got %.3f", legacy.Damage)
	}
	expectedDamageRate := legacy.Damage * 100.0
	if math.Abs(legacy.DamagePer1kTicks-expectedDamageRate) > 1e-6 {
		t.Fatalf("unexpected damage per 1k ticks: got %.3f want %.3f", legacy.DamagePer1kTicks, expectedDamageRate)
	}
	if legacy.FirstHitLatencyTicks != 2 {
		t.Fatalf("expected first hit latency 2 ticks, got %.3f", legacy.FirstHitLatencyTicks)
	}
	expectedLatencyMs := float64(legacy.FirstHitLatencyTicks) * 1000.0 / float64(tickRate)
	if math.Abs(legacy.FirstHitLatencyMs-expectedLatencyMs) > 1e-6 {
		t.Fatalf("unexpected latency ms: got %.3f want %.3f", legacy.FirstHitLatencyMs, expectedLatencyMs)
	}
	if count := legacy.VictimBuckets["2"]; count != 1 {
		t.Fatalf("expected bucket '2' to equal 1, got %d", count)
	}

	contract, ok := attackEntries[telemetrySourceContract]
	if !ok {
		t.Fatalf("expected contract entry to exist")
	}
	if contract.Misses != 1 {
		t.Fatalf("expected one miss, got %d", contract.Misses)
	}
	if contract.Hits != 0 {
		t.Fatalf("expected zero hits for contract miss entry, got %d", contract.Hits)
	}
}

func TestTelemetryTickBudgetOverrunMetrics(t *testing.T) {
	counters := newTelemetryCounters()
	budget := time.Second / time.Duration(tickRate)

	if streak := counters.RecordTickBudgetOverrun(budget+budget/2, budget); streak != 1 {
		t.Fatalf("expected first streak to be 1, got %d", streak)
	}
	overrunDuration := 3 * budget
	if streak := counters.RecordTickBudgetOverrun(overrunDuration, budget); streak != 2 {
		t.Fatalf("expected second streak to be 2, got %d", streak)
	}

	snapshot := counters.Snapshot()
	tickBudget := snapshot.TickBudget
	if tickBudget.BudgetMillis != budget.Milliseconds() {
		t.Fatalf("unexpected budget millis: got %d want %d", tickBudget.BudgetMillis, budget.Milliseconds())
	}
	if tickBudget.CurrentStreak != 2 {
		t.Fatalf("expected current streak to be 2, got %d", tickBudget.CurrentStreak)
	}
	if tickBudget.MaxStreak != 2 {
		t.Fatalf("expected max streak to be 2, got %d", tickBudget.MaxStreak)
	}
	if tickBudget.LastOverrunMillis != overrunDuration.Milliseconds() {
		t.Fatalf("expected last overrun millis %d, got %d", overrunDuration.Milliseconds(), tickBudget.LastOverrunMillis)
	}
	if count := tickBudget.Overruns["over_1_5x"]; count != 1 {
		t.Fatalf("expected over_1_5x bucket to be 1, got %d", count)
	}
	if count := tickBudget.Overruns["over_gt3x"]; count != 1 {
		t.Fatalf("expected over_gt3x bucket to be 1, got %d", count)
	}
	if tickBudget.AlarmCount != 0 {
		t.Fatalf("expected alarm count to start at 0, got %d", tickBudget.AlarmCount)
	}

	counters.RecordTickBudgetAlarm(42, 2.75)
	snapshot = counters.Snapshot()
	tickBudget = snapshot.TickBudget
	if tickBudget.AlarmCount != 1 {
		t.Fatalf("expected alarm count to be 1, got %d", tickBudget.AlarmCount)
	}
	if tickBudget.LastAlarmTick != 42 {
		t.Fatalf("expected last alarm tick to be 42, got %d", tickBudget.LastAlarmTick)
	}
	if math.Abs(tickBudget.LastAlarmRatio-2.75) > 1e-9 {
		t.Fatalf("expected last alarm ratio 2.75, got %.4f", tickBudget.LastAlarmRatio)
	}

	counters.ResetTickBudgetOverrunStreak()
	snapshot = counters.Snapshot()
	tickBudget = snapshot.TickBudget
	if tickBudget.CurrentStreak != 0 {
		t.Fatalf("expected current streak to reset to 0, got %d", tickBudget.CurrentStreak)
	}
	if tickBudget.MaxStreak != 2 {
		t.Fatalf("expected max streak to remain 2, got %d", tickBudget.MaxStreak)
	}
	if tickBudget.LastOverrunMillis != overrunDuration.Milliseconds() {
		t.Fatalf("expected last overrun millis to persist at %d, got %d", overrunDuration.Milliseconds(), tickBudget.LastOverrunMillis)
	}
	if tickBudget.AlarmCount != 1 {
		t.Fatalf("expected alarm count to persist at 1, got %d", tickBudget.AlarmCount)
	}
	if tickBudget.LastAlarmTick != 42 {
		t.Fatalf("expected last alarm tick to persist at 42, got %d", tickBudget.LastAlarmTick)
	}
	if math.Abs(tickBudget.LastAlarmRatio-2.75) > 1e-9 {
		t.Fatalf("expected last alarm ratio to persist at 2.75, got %.4f", tickBudget.LastAlarmRatio)
	}
}
