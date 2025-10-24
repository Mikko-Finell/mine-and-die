package server

import (
	"math"
	"testing"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
	"mine-and-die/server/internal/telemetry"
	"mine-and-die/server/logging"
)

func TestTelemetryEffectParitySnapshot(t *testing.T) {
	counters := newTelemetryCounters(nil)
	for i := 0; i < 10; i++ {
		counters.RecordTickDuration(16 * time.Millisecond)
	}

	counters.RecordEffectParity(effectParitySummary{
		EffectType:    "attack",
		Hits:          3,
		UniqueVictims: 2,
		TotalDamage:   45,
		SpawnTick:     effectcontract.Tick(10),
		FirstHitTick:  effectcontract.Tick(12),
	})

	counters.RecordEffectParity(effectParitySummary{
		EffectType:    "attack",
		Hits:          0,
		UniqueVictims: 0,
		TotalDamage:   0,
		SpawnTick:     effectcontract.Tick(20),
		FirstHitTick:  0,
	})

	snapshot := counters.Snapshot()
	parity := snapshot.EffectParity
	if parity.TotalTicks != 10 {
		t.Fatalf("expected total ticks to be 10, got %d", parity.TotalTicks)
	}

	entry, ok := parity.Entries["attack"]
	if !ok {
		t.Fatalf("expected attack entry to exist")
	}
	if entry.Hits != 3 {
		t.Fatalf("expected 3 hits, got %d", entry.Hits)
	}
	expectedHitRate := float64(entry.Hits) * 1000.0 / 10.0
	if math.Abs(entry.HitsPer1kTicks-expectedHitRate) > 1e-6 {
		t.Fatalf("unexpected hit rate: got %.3f want %.3f", entry.HitsPer1kTicks, expectedHitRate)
	}
	if math.Abs(entry.Damage-45) > 1e-9 {
		t.Fatalf("expected damage 45, got %.3f", entry.Damage)
	}
	expectedDamageRate := entry.Damage * 100.0
	if math.Abs(entry.DamagePer1kTicks-expectedDamageRate) > 1e-6 {
		t.Fatalf("unexpected damage per 1k ticks: got %.3f want %.3f", entry.DamagePer1kTicks, expectedDamageRate)
	}
	if entry.FirstHitLatencyTicks != 2 {
		t.Fatalf("expected first hit latency 2 ticks, got %.3f", entry.FirstHitLatencyTicks)
	}
	expectedLatencyMs := float64(entry.FirstHitLatencyTicks) * 1000.0 / float64(tickRate)
	if math.Abs(entry.FirstHitLatencyMs-expectedLatencyMs) > 1e-6 {
		t.Fatalf("unexpected latency ms: got %.3f want %.3f", entry.FirstHitLatencyMs, expectedLatencyMs)
	}
	if count := entry.VictimBuckets["2"]; count != 1 {
		t.Fatalf("expected bucket '2' to equal 1, got %d", count)
	}
	if entry.Misses != 1 {
		t.Fatalf("expected one miss, got %d", entry.Misses)
	}
}

func TestTelemetryTickBudgetOverrunMetrics(t *testing.T) {
	counters := newTelemetryCounters(nil)
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

func TestTelemetrySubscriberQueueSnapshot(t *testing.T) {
	counters := newTelemetryCounters(nil)

	for i := 0; i < tickRate; i++ {
		counters.RecordTickDuration(time.Second / time.Duration(tickRate))
	}

	counters.RecordSubscriberQueueDepth(3)
	counters.RecordSubscriberQueueDepth(7)
	counters.RecordSubscriberQueueDrop(7)
	counters.RecordSubscriberQueueDepth(2)

	snapshot := counters.Snapshot()
	queues := snapshot.SubscriberQueues
	if queues.Depth != 2 {
		t.Fatalf("expected queue depth 2, got %d", queues.Depth)
	}
	if queues.MaxDepth != 7 {
		t.Fatalf("expected max depth 7, got %d", queues.MaxDepth)
	}
	if queues.Drops != 1 {
		t.Fatalf("expected drops 1, got %d", queues.Drops)
	}
	if math.Abs(queues.DropRatePerSecond-1.0) > 1e-9 {
		t.Fatalf("expected drop rate 1.0, got %.6f", queues.DropRatePerSecond)
	}
}

func TestTelemetryMetricsAdapterRecordsMetrics(t *testing.T) {
	metrics := &logging.Metrics{}
	counters := newTelemetryCounters(telemetry.WrapMetrics(metrics))

	counters.RecordBroadcast(64, 3)

	duration := 15 * time.Millisecond
	counters.RecordTickDuration(duration)

	budget := time.Second / time.Duration(tickRate)
	overrun := budget + budget/2
	counters.RecordTickBudgetOverrun(overrun, budget)
	counters.RecordTickBudgetAlarm(42, 2.5)
	counters.ResetTickBudgetOverrunStreak()

	counters.RecordKeyframeJournal(5, 10, 20)
	counters.RecordKeyframeRequest(25*time.Millisecond, true)
	counters.RecordKeyframeRequest(0, false)
	counters.IncrementKeyframeExpired()
	counters.IncrementKeyframeRateLimited()

	counters.RecordEffectsActive(7)
	counters.RecordEffectSpawned("fireball", "wizard")
	counters.RecordEffectUpdated("fireball", "spread")
	counters.RecordEffectEnded("fireball", "timeout")
	counters.RecordEffectSpatialOverflow("fireball")
	counters.RecordEffectTrigger("chain")
	counters.RecordEffectParity(effectParitySummary{
		EffectType:    "fireball",
		Hits:          2,
		UniqueVictims: 3,
		SpawnTick:     effectcontract.Tick(5),
		FirstHitTick:  effectcontract.Tick(7),
	})
	counters.RecordEffectParity(effectParitySummary{
		EffectType:    "fireball",
		Hits:          0,
		UniqueVictims: 0,
	})
	counters.RecordJournalDrop("overflow")
	counters.RecordJournalDrop("")
	counters.RecordCommandDropped("queue_full", "move")
	counters.RecordCommandDropped("", "")
	counters.RecordSubscriberQueueDepth(4)
	counters.RecordSubscriberQueueDrop(4)
	counters.RecordBroadcastQueueDepth(7)
	counters.RecordBroadcastQueueDrop(7)

	snapshot := metrics.Snapshot()
	if got := snapshot[metricKeyBroadcastTotal]; got != 1 {
		t.Fatalf("expected broadcast total 1, got %d", got)
	}
	if got := snapshot[metricKeyBroadcastBytesTotal]; got != 64 {
		t.Fatalf("expected broadcast bytes 64, got %d", got)
	}
	if got := snapshot[metricKeyBroadcastEntitiesTotal]; got != 3 {
		t.Fatalf("expected broadcast entities 3, got %d", got)
	}
	if got := snapshot[metricKeyBroadcastLastBytes]; got != 64 {
		t.Fatalf("expected last broadcast bytes 64, got %d", got)
	}
	if got := snapshot[metricKeyBroadcastLastEntities]; got != 3 {
		t.Fatalf("expected last broadcast entities 3, got %d", got)
	}
	if got := snapshot[metricKeyTickDurationMillis]; got != uint64(duration.Milliseconds()) {
		t.Fatalf("expected tick duration %d, got %d", duration.Milliseconds(), got)
	}
	if got := snapshot[metricKeyTickTotal]; got != 1 {
		t.Fatalf("expected tick total 1, got %d", got)
	}
	if got := snapshot[metricKeyTickBudgetOverrunTotal]; got != 1 {
		t.Fatalf("expected overrun total 1, got %d", got)
	}
	if got := snapshot[metricKeyTickBudgetOverrunLastMs]; got != uint64(overrun.Milliseconds()) {
		t.Fatalf("expected overrun millis %d, got %d", overrun.Milliseconds(), got)
	}
	if got := snapshot[metricKeyTickBudgetBudgetMillis]; got != uint64(budget.Milliseconds()) {
		t.Fatalf("expected budget millis %d, got %d", budget.Milliseconds(), got)
	}
	if got := snapshot[metricKeyTickBudgetStreakCurrent]; got != 0 {
		t.Fatalf("expected current streak reset to 0, got %d", got)
	}
	if got := snapshot[metricKeyTickBudgetStreakMax]; got != 1 {
		t.Fatalf("expected max streak 1, got %d", got)
	}
	expectedRatioBits := math.Float64bits(float64(overrun) / float64(budget))
	if got := snapshot[metricKeyTickBudgetRatioBits]; got != expectedRatioBits {
		t.Fatalf("expected ratio bits %d, got %d", expectedRatioBits, got)
	}
	if got := snapshot[metricKeyTickBudgetAlarmTotal]; got != 1 {
		t.Fatalf("expected alarm total 1, got %d", got)
	}
	if got := snapshot[metricKeyTickBudgetAlarmLastTick]; got != 42 {
		t.Fatalf("expected last alarm tick 42, got %d", got)
	}
	expectedAlarmRatioBits := math.Float64bits(2.5)
	if got := snapshot[metricKeyTickBudgetAlarmRatioBits]; got != expectedAlarmRatioBits {
		t.Fatalf("expected alarm ratio bits %d, got %d", expectedAlarmRatioBits, got)
	}
	if got := snapshot[metricKeyKeyframeJournalSize]; got != 5 {
		t.Fatalf("expected keyframe journal size 5, got %d", got)
	}
	if got := snapshot[metricKeyKeyframeOldestSequence]; got != 10 {
		t.Fatalf("expected oldest keyframe sequence 10, got %d", got)
	}
	if got := snapshot[metricKeyKeyframeNewestSequence]; got != 20 {
		t.Fatalf("expected newest keyframe sequence 20, got %d", got)
	}
	if got := snapshot[metricKeyKeyframeRequestsTotal]; got != 2 {
		t.Fatalf("expected keyframe request total 2, got %d", got)
	}
	if got := snapshot[metricKeyKeyframeRequestLatencyMs]; got != 25 {
		t.Fatalf("expected keyframe latency 25ms, got %d", got)
	}
	if got := snapshot[metricKeyKeyframeNackExpiredTotal]; got != 1 {
		t.Fatalf("expected keyframe expired total 1, got %d", got)
	}
	if got := snapshot[metricKeyKeyframeNackRateLimited]; got != 1 {
		t.Fatalf("expected keyframe rate limited total 1, got %d", got)
	}
	if got := snapshot[metricKeyEffectsActiveGauge]; got != 7 {
		t.Fatalf("expected effects active gauge 7, got %d", got)
	}
	spawnedKey := metricKeyEffectsSpawnedTotalPrefix + "/fireball/wizard"
	if got := snapshot[spawnedKey]; got != 1 {
		t.Fatalf("expected spawned metric 1, got %d", got)
	}
	updatedKey := metricKeyEffectsUpdatedTotalPrefix + "/fireball/spread"
	if got := snapshot[updatedKey]; got != 1 {
		t.Fatalf("expected updated metric 1, got %d", got)
	}
	endedKey := metricKeyEffectsEndedTotalPrefix + "/fireball/timeout"
	if got := snapshot[endedKey]; got != 1 {
		t.Fatalf("expected ended metric 1, got %d", got)
	}
	overflowKey := metricKeyEffectsSpatialOverflow + "/fireball"
	if got := snapshot[overflowKey]; got != 1 {
		t.Fatalf("expected spatial overflow metric 1, got %d", got)
	}
	triggerKey := metricKeyEffectTriggersEnqueued + "/chain"
	if got := snapshot[triggerKey]; got != 1 {
		t.Fatalf("expected trigger metric 1, got %d", got)
	}
	hitsKey := metricKeyEffectParityHitsTotalPrefix + "/fireball"
	if got := snapshot[hitsKey]; got != 2 {
		t.Fatalf("expected effect parity hits metric 2, got %d", got)
	}
	missesKey := metricKeyEffectParityMissesTotalPrefix + "/fireball"
	if got := snapshot[missesKey]; got != 1 {
		t.Fatalf("expected effect parity misses metric 1, got %d", got)
	}
	latencyKey := metricKeyEffectParityFirstHitLatencyPrefix + "/fireball"
	if got := snapshot[latencyKey]; got != 2 {
		t.Fatalf("expected first hit latency total 2 ticks, got %d", got)
	}
	samplesKey := metricKeyEffectParityFirstHitSamplesPrefix + "/fireball"
	if got := snapshot[samplesKey]; got != 1 {
		t.Fatalf("expected first hit samples total 1, got %d", got)
	}
	victimsKey := metricKeyEffectParityVictimsTotalPrefix + "/fireball/3"
	if got := snapshot[victimsKey]; got != 1 {
		t.Fatalf("expected victims bucket metric 1, got %d", got)
	}
	journalOverflowKey := metricKeyJournalDropsTotalPrefix + "/overflow"
	if got := snapshot[journalOverflowKey]; got != 1 {
		t.Fatalf("expected journal overflow metric 1, got %d", got)
	}
	journalUnknownKey := metricKeyJournalDropsTotalPrefix + "/unknown"
	if got := snapshot[journalUnknownKey]; got != 1 {
		t.Fatalf("expected journal unknown metric 1, got %d", got)
	}
	commandDropKey := metricKeyCommandDropsTotalPrefix + "/queue_full/move"
	if got := snapshot[commandDropKey]; got != 1 {
		t.Fatalf("expected command drop metric 1, got %d", got)
	}
	commandUnknownKey := metricKeyCommandDropsTotalPrefix + "/unknown/unknown"
	if got := snapshot[commandUnknownKey]; got != 1 {
		t.Fatalf("expected command unknown metric 1, got %d", got)
	}
	if got := snapshot[metricKeySubscriberQueueDepth]; got != 4 {
		t.Fatalf("expected subscriber queue depth 4, got %d", got)
	}
	if got := snapshot[metricKeySubscriberQueueMaxDepth]; got != 4 {
		t.Fatalf("expected subscriber queue max depth 4, got %d", got)
	}
	if got := snapshot[metricKeySubscriberQueueDropsTotal]; got != 1 {
		t.Fatalf("expected subscriber queue drops 1, got %d", got)
	}
	if got := snapshot[metricKeyBroadcastQueueDepth]; got != 7 {
		t.Fatalf("expected broadcast queue depth 7, got %d", got)
	}
	if got := snapshot[metricKeyBroadcastQueueMaxDepth]; got != 7 {
		t.Fatalf("expected broadcast queue max depth 7, got %d", got)
	}
	if got := snapshot[metricKeyBroadcastQueueDropsTotal]; got != 1 {
		t.Fatalf("expected broadcast queue drops 1, got %d", got)
	}

	snapshotState := counters.Snapshot()
	queues := snapshotState.SubscriberQueues
	if queues.Depth != 4 {
		t.Fatalf("expected subscriber queue depth 4, got %d", queues.Depth)
	}
	if queues.MaxDepth != 4 {
		t.Fatalf("expected subscriber queue max depth 4, got %d", queues.MaxDepth)
	}
	if queues.Drops != 1 {
		t.Fatalf("expected subscriber queue drops 1, got %d", queues.Drops)
	}
	if math.Abs(queues.DropRatePerSecond-float64(tickRate)) > 0.0001 {
		t.Fatalf("expected subscriber drop rate %d, got %.4f", tickRate, queues.DropRatePerSecond)
	}
	broadcast := snapshotState.BroadcastQueue
	if broadcast.Depth != 7 {
		t.Fatalf("expected broadcast queue depth 7, got %d", broadcast.Depth)
	}
	if broadcast.MaxDepth != 7 {
		t.Fatalf("expected broadcast queue max depth 7, got %d", broadcast.MaxDepth)
	}
	if broadcast.Drops != 1 {
		t.Fatalf("expected broadcast queue drops 1, got %d", broadcast.Drops)
	}
	if math.Abs(broadcast.DropRatePerSecond-float64(tickRate)) > 0.0001 {
		t.Fatalf("expected broadcast drop rate %d, got %.4f", tickRate, broadcast.DropRatePerSecond)
	}
	return
}
