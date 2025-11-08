package server

import (
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"mine-and-die/server/internal/telemetry"
	worldpkg "mine-and-die/server/internal/world"
)

const (
	metricKeyBroadcastTotal                    = "telemetry_broadcast_total"
	metricKeyBroadcastBytesTotal               = "telemetry_broadcast_bytes_total"
	metricKeyBroadcastEntitiesTotal            = "telemetry_broadcast_entities_total"
	metricKeyBroadcastLastBytes                = "telemetry_broadcast_last_bytes"
	metricKeyBroadcastLastEntities             = "telemetry_broadcast_last_entities"
	metricKeyTickDurationMillis                = "telemetry_tick_duration_millis"
	metricKeyTickTotal                         = "telemetry_tick_total"
	metricKeyTickBudgetOverrunTotal            = "telemetry_tick_budget_overrun_total"
	metricKeyTickBudgetOverrunLastMs           = "telemetry_tick_budget_overrun_last_millis"
	metricKeyTickBudgetBudgetMillis            = "telemetry_tick_budget_overrun_budget_millis"
	metricKeyTickBudgetStreakCurrent           = "telemetry_tick_budget_overrun_current_streak"
	metricKeyTickBudgetStreakMax               = "telemetry_tick_budget_overrun_max_streak"
	metricKeyTickBudgetRatioBits               = "telemetry_tick_budget_overrun_ratio_bits"
	metricKeyTickBudgetAlarmTotal              = "telemetry_tick_budget_alarm_total"
	metricKeyTickBudgetAlarmLastTick           = "telemetry_tick_budget_alarm_last_tick"
	metricKeyTickBudgetAlarmRatioBits          = "telemetry_tick_budget_alarm_last_ratio_bits"
	metricKeyKeyframeJournalSize               = "telemetry_keyframe_journal_size"
	metricKeyKeyframeOldestSequence            = "telemetry_keyframe_oldest_sequence"
	metricKeyKeyframeNewestSequence            = "telemetry_keyframe_newest_sequence"
	metricKeyKeyframeRequestsTotal             = "telemetry_keyframe_requests_total"
	metricKeyKeyframeRequestLatencyMs          = "telemetry_keyframe_request_latency_millis"
	metricKeyKeyframeNackExpiredTotal          = "telemetry_keyframe_nacks_expired_total"
	metricKeyKeyframeNackRateLimited           = "telemetry_keyframe_nacks_rate_limited_total"
	metricKeyEffectsActiveGauge                = "telemetry_effects_active_gauge"
	metricKeyEffectsSpawnedTotalPrefix         = "telemetry_effects_spawned_total"
	metricKeyEffectsUpdatedTotalPrefix         = "telemetry_effects_updated_total"
	metricKeyEffectsEndedTotalPrefix           = "telemetry_effects_ended_total"
	metricKeyEffectsSpatialOverflow            = "telemetry_effects_spatial_overflow_total"
	metricKeyEffectTriggersEnqueued            = "telemetry_effect_triggers_enqueued_total"
	metricKeyEffectParityHitsTotalPrefix       = "telemetry_effect_parity_hits_total"
	metricKeyEffectParityMissesTotalPrefix     = "telemetry_effect_parity_misses_total"
	metricKeyEffectParityVictimsTotalPrefix    = "telemetry_effect_parity_victims_total"
	metricKeyEffectParityFirstHitLatencyPrefix = "telemetry_effect_parity_first_hit_latency_ticks_total"
	metricKeyEffectParityFirstHitSamplesPrefix = "telemetry_effect_parity_first_hit_samples_total"
	metricKeyJournalDropsTotalPrefix           = "telemetry_journal_drops_total"
	metricKeyCommandDropsTotalPrefix           = "telemetry_command_drops_total"
	metricKeySubscriberQueueDepth              = "telemetry_ws_queue_depth"
	metricKeySubscriberQueueMaxDepth           = "telemetry_ws_queue_max_depth"
	metricKeySubscriberQueueDropsTotal         = "telemetry_ws_queue_drops_total"
	metricKeyBroadcastQueueDepth               = "telemetry_broadcast_queue_depth"
	metricKeyBroadcastQueueMaxDepth            = "telemetry_broadcast_queue_max_depth"
	metricKeyBroadcastQueueDropsTotal          = "telemetry_broadcast_queue_drops_total"
)

type telemetryMetricsAdapter struct {
	metrics telemetry.Metrics
}

func (a *telemetryMetricsAdapter) Attach(metrics telemetry.Metrics) {
	a.metrics = metrics
}

func (a *telemetryMetricsAdapter) add(key string, delta uint64) {
	if a == nil || a.metrics == nil || key == "" || delta == 0 {
		return
	}
	a.metrics.Add(key, delta)
}

func (a *telemetryMetricsAdapter) store(key string, value uint64) {
	if a == nil || a.metrics == nil || key == "" {
		return
	}
	a.metrics.Store(key, value)
}

func (a *telemetryMetricsAdapter) layeredKey(base, primary, secondary string) string {
	if base == "" {
		return ""
	}
	normalizedPrimary := normalizeMetricKey(primary)
	normalizedSecondary := ""
	if secondary != "" {
		normalizedSecondary = normalizeMetricKey(secondary)
	}
	var b strings.Builder
	// account for slashes and normalized values when sizing the builder
	length := len(base) + 1 + len(normalizedPrimary)
	if normalizedSecondary != "" {
		length += 1 + len(normalizedSecondary)
	}
	b.Grow(length)
	b.WriteString(base)
	b.WriteByte('/')
	b.WriteString(normalizedPrimary)
	if normalizedSecondary != "" {
		b.WriteByte('/')
		b.WriteString(normalizedSecondary)
	}
	return b.String()
}

func (a *telemetryMetricsAdapter) RecordBroadcast(bytes, entities int) {
	if a == nil || a.metrics == nil {
		return
	}
	if bytes < 0 {
		bytes = 0
	}
	if entities < 0 {
		entities = 0
	}
	a.metrics.Add(metricKeyBroadcastTotal, 1)
	a.metrics.Add(metricKeyBroadcastBytesTotal, uint64(bytes))
	a.metrics.Add(metricKeyBroadcastEntitiesTotal, uint64(entities))
	a.metrics.Store(metricKeyBroadcastLastBytes, uint64(bytes))
	a.metrics.Store(metricKeyBroadcastLastEntities, uint64(entities))
}

func (a *telemetryMetricsAdapter) RecordTickDuration(duration time.Duration, total uint64) {
	if a == nil || a.metrics == nil {
		return
	}
	millis := duration.Milliseconds()
	if millis < 0 {
		millis = 0
	}
	a.metrics.Store(metricKeyTickDurationMillis, uint64(millis))
	a.metrics.Store(metricKeyTickTotal, total)
}

func (a *telemetryMetricsAdapter) RecordTickBudgetOverrun(duration, budget time.Duration, ratio float64, streak, max uint64) {
	if a == nil || a.metrics == nil {
		return
	}
	millis := duration.Milliseconds()
	if millis < 0 {
		millis = 0
	}
	budgetMillis := budget.Milliseconds()
	if budgetMillis < 0 {
		budgetMillis = 0
	}
	a.metrics.Add(metricKeyTickBudgetOverrunTotal, 1)
	a.metrics.Store(metricKeyTickBudgetOverrunLastMs, uint64(millis))
	a.metrics.Store(metricKeyTickBudgetBudgetMillis, uint64(budgetMillis))
	a.metrics.Store(metricKeyTickBudgetStreakCurrent, streak)
	a.metrics.Store(metricKeyTickBudgetStreakMax, max)
	a.metrics.Store(metricKeyTickBudgetRatioBits, math.Float64bits(ratio))
}

func (a *telemetryMetricsAdapter) ResetTickBudgetStreak() {
	if a == nil || a.metrics == nil {
		return
	}
	a.metrics.Store(metricKeyTickBudgetStreakCurrent, 0)
}

func (a *telemetryMetricsAdapter) RecordTickBudgetAlarm(tick uint64, ratio float64) {
	if a == nil || a.metrics == nil {
		return
	}
	a.metrics.Add(metricKeyTickBudgetAlarmTotal, 1)
	a.metrics.Store(metricKeyTickBudgetAlarmLastTick, tick)
	a.metrics.Store(metricKeyTickBudgetAlarmRatioBits, math.Float64bits(ratio))
}

func (a *telemetryMetricsAdapter) RecordKeyframeJournal(size uint64, oldest, newest uint64) {
	if a == nil || a.metrics == nil {
		return
	}
	a.store(metricKeyKeyframeJournalSize, size)
	a.store(metricKeyKeyframeOldestSequence, oldest)
	a.store(metricKeyKeyframeNewestSequence, newest)
}

func (a *telemetryMetricsAdapter) RecordKeyframeRequest(success bool, latencyMillis uint64) {
	if a == nil || a.metrics == nil {
		return
	}
	a.add(metricKeyKeyframeRequestsTotal, 1)
	if success {
		a.store(metricKeyKeyframeRequestLatencyMs, latencyMillis)
	}
}

func (a *telemetryMetricsAdapter) IncrementKeyframeExpired() {
	a.add(metricKeyKeyframeNackExpiredTotal, 1)
}

func (a *telemetryMetricsAdapter) IncrementKeyframeRateLimited() {
	a.add(metricKeyKeyframeNackRateLimited, 1)
}

func (a *telemetryMetricsAdapter) RecordEffectsActive(count uint64) {
	a.store(metricKeyEffectsActiveGauge, count)
}

func (a *telemetryMetricsAdapter) RecordEffectSpawned(effectType, producer string) {
	key := a.layeredKey(metricKeyEffectsSpawnedTotalPrefix, effectType, producer)
	a.add(key, 1)
}

func (a *telemetryMetricsAdapter) RecordEffectUpdated(effectType, mutation string) {
	key := a.layeredKey(metricKeyEffectsUpdatedTotalPrefix, effectType, mutation)
	a.add(key, 1)
}

func (a *telemetryMetricsAdapter) RecordEffectEnded(effectType, reason string) {
	key := a.layeredKey(metricKeyEffectsEndedTotalPrefix, effectType, reason)
	a.add(key, 1)
}

func (a *telemetryMetricsAdapter) RecordEffectSpatialOverflow(effectType string) {
	key := a.layeredKey(metricKeyEffectsSpatialOverflow, effectType, "")
	a.add(key, 1)
}

func (a *telemetryMetricsAdapter) RecordEffectTrigger(triggerType string) {
	key := a.layeredKey(metricKeyEffectTriggersEnqueued, triggerType, "")
	a.add(key, 1)
}

func (a *telemetryMetricsAdapter) RecordJournalDrop(reason string) {
	key := a.layeredKey(metricKeyJournalDropsTotalPrefix, reason, "")
	a.add(key, 1)
}

func (a *telemetryMetricsAdapter) RecordCommandDrop(reason, commandType string) {
	key := a.layeredKey(metricKeyCommandDropsTotalPrefix, reason, commandType)
	a.add(key, 1)
}

func (a *telemetryMetricsAdapter) RecordSubscriberQueueDepth(depth uint64) {
	if a == nil || a.metrics == nil {
		return
	}
	a.metrics.Store(metricKeySubscriberQueueDepth, depth)
}

func (a *telemetryMetricsAdapter) RecordSubscriberQueueMaxDepth(depth uint64) {
	if a == nil || a.metrics == nil {
		return
	}
	a.metrics.Store(metricKeySubscriberQueueMaxDepth, depth)
}

func (a *telemetryMetricsAdapter) IncrementSubscriberQueueDrops() {
	a.add(metricKeySubscriberQueueDropsTotal, 1)
}

func (a *telemetryMetricsAdapter) RecordBroadcastQueueDepth(depth uint64) {
	if a == nil || a.metrics == nil {
		return
	}
	a.metrics.Store(metricKeyBroadcastQueueDepth, depth)
}

func (a *telemetryMetricsAdapter) RecordBroadcastQueueMaxDepth(depth uint64) {
	if a == nil || a.metrics == nil {
		return
	}
	a.metrics.Store(metricKeyBroadcastQueueMaxDepth, depth)
}

func (a *telemetryMetricsAdapter) IncrementBroadcastQueueDrops() {
	a.add(metricKeyBroadcastQueueDropsTotal, 1)
}

func (a *telemetryMetricsAdapter) RecordEffectParity(summary worldpkg.EffectTelemetrySummary) {
	if a == nil || a.metrics == nil {
		return
	}
	if summary.Hits > 0 {
		hitsKey := a.layeredKey(metricKeyEffectParityHitsTotalPrefix, summary.EffectType, "")
		a.add(hitsKey, uint64(summary.Hits))
		if summary.SpawnTick > 0 && summary.FirstHitTick >= summary.SpawnTick {
			latency := summary.FirstHitTick - summary.SpawnTick
			if latency < 0 {
				latency = 0
			}
			latencyKey := a.layeredKey(metricKeyEffectParityFirstHitLatencyPrefix, summary.EffectType, "")
			a.add(latencyKey, uint64(latency))
			samplesKey := a.layeredKey(metricKeyEffectParityFirstHitSamplesPrefix, summary.EffectType, "")
			a.add(samplesKey, 1)
		}
	} else {
		missesKey := a.layeredKey(metricKeyEffectParityMissesTotalPrefix, summary.EffectType, "")
		a.add(missesKey, 1)
	}
	if bucket := victimsBucket(summary.UniqueVictims); bucket != "" {
		victimsKey := a.layeredKey(metricKeyEffectParityVictimsTotalPrefix, summary.EffectType, bucket)
		a.add(victimsKey, 1)
	}
}

type simpleCounter struct {
	data sync.Map
}

func (c *simpleCounter) add(key string, delta uint64) {
	if c == nil {
		return
	}
	normalized := normalizeMetricKey(key)
	if delta == 0 {
		return
	}
	current, _ := c.data.LoadOrStore(normalized, &atomic.Uint64{})
	counter := current.(*atomic.Uint64)
	counter.Add(delta)
}

func (c *simpleCounter) snapshot() map[string]uint64 {
	if c == nil {
		return nil
	}
	result := make(map[string]uint64)
	c.data.Range(func(key, value any) bool {
		strKey, ok := key.(string)
		if !ok {
			return true
		}
		if counter, ok := value.(*atomic.Uint64); ok {
			result[strKey] = counter.Load()
		}
		return true
	})
	if len(result) == 0 {
		return nil
	}
	return result
}

type layeredCounter struct {
	buckets sync.Map // string -> *simpleCounter
}

func (c *layeredCounter) add(primary, secondary string, delta uint64) {
	if c == nil || delta == 0 {
		return
	}
	normalizedPrimary := normalizeMetricKey(primary)
	normalizedSecondary := normalizeMetricKey(secondary)
	bucketAny, _ := c.buckets.LoadOrStore(normalizedPrimary, &simpleCounter{})
	if bucket, ok := bucketAny.(*simpleCounter); ok {
		bucket.add(normalizedSecondary, delta)
	}
}

func (c *layeredCounter) snapshot() map[string]map[string]uint64 {
	if c == nil {
		return nil
	}
	result := make(map[string]map[string]uint64)
	c.buckets.Range(func(key, value any) bool {
		primary, ok := key.(string)
		if !ok {
			return true
		}
		if bucket, ok := value.(*simpleCounter); ok {
			snapshot := bucket.snapshot()
			if len(snapshot) > 0 {
				result[primary] = snapshot
			}
		}
		return true
	})
	if len(result) == 0 {
		return nil
	}
	return result
}

type effectParitySummary = worldpkg.EffectTelemetrySummary

type effectParityTotals struct {
	Hits                 uint64
	Damage               float64
	Misses               uint64
	FirstHitLatencyTicks uint64
	FirstHitSamples      uint64
	VictimBuckets        map[string]uint64
}

type effectParityAggregator struct {
	mu     sync.Mutex
	totals map[string]*effectParityTotals
}

func (a *effectParityAggregator) record(summary effectParitySummary) {
	if a == nil {
		return
	}
	normalizedType := normalizeMetricKey(summary.EffectType)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.totals == nil {
		a.totals = make(map[string]*effectParityTotals)
	}
	totals := a.totals[normalizedType]
	if totals == nil {
		totals = &effectParityTotals{VictimBuckets: make(map[string]uint64)}
		a.totals[normalizedType] = totals
	}
	if summary.Hits > 0 {
		totals.Hits += uint64(summary.Hits)
		if summary.TotalDamage > 0 {
			totals.Damage += summary.TotalDamage
		}
		if summary.SpawnTick > 0 && summary.FirstHitTick >= summary.SpawnTick {
			latency := summary.FirstHitTick - summary.SpawnTick
			if latency < 0 {
				latency = 0
			}
			totals.FirstHitLatencyTicks += uint64(latency)
			totals.FirstHitSamples++
		}
	} else {
		totals.Misses++
	}
	bucket := victimsBucket(summary.UniqueVictims)
	if bucket != "" {
		totals.VictimBuckets[bucket]++
	}
}

func (a *effectParityAggregator) snapshot(totalTicks uint64) map[string]telemetryEffectParityEntry {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.totals) == 0 {
		return nil
	}
	result := make(map[string]telemetryEffectParityEntry, len(a.totals))
	for effectType, totals := range a.totals {
		if totals == nil {
			continue
		}
		result[effectType] = totals.toSnapshot(totalTicks)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func victimsBucket(count int) string {
	switch {
	case count <= 0:
		return "0"
	case count == 1:
		return "1"
	case count == 2:
		return "2"
	case count == 3:
		return "3"
	default:
		return "4+"
	}
}

func tickBudgetOverrunBucket(duration, budget time.Duration) string {
	if budget <= 0 {
		return "unknown"
	}
	ratio := float64(duration) / float64(budget)
	switch {
	case ratio < 1.5:
		return "over_1x"
	case ratio < 2.0:
		return "over_1_5x"
	case ratio < 3.0:
		return "over_2x"
	default:
		return "over_gt3x"
	}
}

type telemetryEffectParityEntry struct {
	Hits                 uint64            `json:"hits"`
	HitsPer1kTicks       float64           `json:"hitsPer1kTicks"`
	Damage               float64           `json:"damage"`
	DamagePer1kTicks     float64           `json:"damagePer1kTicks"`
	Misses               uint64            `json:"misses"`
	FirstHitLatencyTicks float64           `json:"firstHitLatencyTicks,omitempty"`
	FirstHitLatencyMs    float64           `json:"firstHitLatencyMillis,omitempty"`
	VictimBuckets        map[string]uint64 `json:"victimBuckets,omitempty"`
}

func (t *effectParityTotals) toSnapshot(totalTicks uint64) telemetryEffectParityEntry {
	entry := telemetryEffectParityEntry{
		Hits:   t.Hits,
		Damage: t.Damage,
		Misses: t.Misses,
	}
	ticks := float64(totalTicks)
	if ticks > 0 {
		entry.HitsPer1kTicks = float64(t.Hits) * 1000.0 / ticks
		entry.DamagePer1kTicks = t.Damage * 1000.0 / ticks
	}
	if t.FirstHitSamples > 0 {
		avgTicks := float64(t.FirstHitLatencyTicks) / float64(t.FirstHitSamples)
		entry.FirstHitLatencyTicks = avgTicks
		entry.FirstHitLatencyMs = avgTicks * 1000.0 / float64(tickRate)
	}
	if len(t.VictimBuckets) > 0 {
		copy := make(map[string]uint64, len(t.VictimBuckets))
		for bucket, count := range t.VictimBuckets {
			copy[bucket] = count
		}
		entry.VictimBuckets = copy
	}
	return entry
}

func normalizeMetricKey(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

type telemetryCounters struct {
	metrics        telemetry.Metrics
	metricsAdapter telemetryMetricsAdapter

	bytesSent                    atomic.Uint64
	entitiesSent                 atomic.Uint64
	tickDurationMillis           atomic.Int64
	lastBroadcastBytes           atomic.Uint64
	lastBroadcastEntities        atomic.Uint64
	debug                        bool
	keyframeJournalSize          atomic.Uint64
	keyframeOldestSequence       atomic.Uint64
	keyframeNewestSequence       atomic.Uint64
	keyframeRequests             atomic.Uint64
	keyframeNacksExpired         atomic.Uint64
	keyframeNacksRateLimited     atomic.Uint64
	keyframeRequestLatencyMillis atomic.Uint64

	effectsSpawnedTotal    layeredCounter
	effectsUpdatedTotal    layeredCounter
	effectsEndedTotal      layeredCounter
	effectsActiveGauge     atomic.Int64
	effectsSpatialOverflow simpleCounter
	triggerEnqueued        simpleCounter
	journalDrops           simpleCounter

	commandDrops layeredCounter

	tickBudgetOverruns               simpleCounter
	tickBudgetLastOverrunMillis      atomic.Int64
	tickBudgetConsecutiveOverruns    atomic.Uint64
	tickBudgetMaxConsecutiveOverruns atomic.Uint64
	tickBudgetAlarms                 atomic.Uint64
	tickBudgetLastAlarmTick          atomic.Uint64
	tickBudgetLastAlarmRatio         atomic.Uint64

	totalTicks   atomic.Uint64
	effectParity effectParityAggregator

	subscriberQueueDepth    atomic.Int64
	subscriberQueueMaxDepth atomic.Int64
	subscriberQueueDrops    atomic.Uint64

	broadcastQueueDepth    atomic.Int64
	broadcastQueueMaxDepth atomic.Int64
	broadcastQueueDrops    atomic.Uint64
}

type telemetrySnapshot struct {
	BytesSent                uint64                           `json:"bytesSent"`
	EntitiesSent             uint64                           `json:"entitiesSent"`
	TickDuration             int64                            `json:"tickDurationMillis"`
	KeyframeJournalSize      uint64                           `json:"keyframeJournalSize"`
	KeyframeOldestSequence   uint64                           `json:"keyframeOldestSequence"`
	KeyframeNewestSequence   uint64                           `json:"keyframeNewestSequence"`
	KeyframeRequests         uint64                           `json:"keyframeRequests"`
	KeyframeNacksExpired     uint64                           `json:"keyframeNacksExpired"`
	KeyframeNacksRateLimited uint64                           `json:"keyframeNacksRateLimited"`
	KeyframeRequestLatencyMs uint64                           `json:"keyframeRequestLatencyMs"`
	Effects                  telemetryEffectsSnapshot         `json:"effects"`
	EffectTriggers           telemetryEffectTriggersSnapshot  `json:"effectTriggers"`
	JournalDrops             map[string]uint64                `json:"journalDrops,omitempty"`
	CommandDrops             map[string]map[string]uint64     `json:"commandDrops,omitempty"`
	SubscriberQueues         telemetrySubscriberQueueSnapshot `json:"subscriberQueues"`
	BroadcastQueue           telemetryBroadcastQueueSnapshot  `json:"broadcastQueue"`
	EffectParity             telemetryEffectParitySnapshot    `json:"effectParity"`
	TickBudget               telemetryTickBudgetSnapshot      `json:"tickBudget"`
}

type telemetryEffectsSnapshot struct {
	SpawnedTotal    map[string]map[string]uint64 `json:"spawnedTotal,omitempty"`
	UpdatedTotal    map[string]map[string]uint64 `json:"updatedTotal,omitempty"`
	EndedTotal      map[string]map[string]uint64 `json:"endedTotal,omitempty"`
	ActiveGauge     int64                        `json:"activeGauge"`
	SpatialOverflow map[string]uint64            `json:"spatialOverflow,omitempty"`
}

type telemetryEffectTriggersSnapshot struct {
	EnqueuedTotal map[string]uint64 `json:"enqueuedTotal,omitempty"`
}

type telemetrySubscriberQueueSnapshot struct {
	Depth             int64   `json:"depth"`
	MaxDepth          int64   `json:"maxDepth"`
	Drops             uint64  `json:"drops"`
	DropRatePerSecond float64 `json:"dropRatePerSecond"`
}

type telemetryBroadcastQueueSnapshot struct {
	Depth             int64   `json:"depth"`
	MaxDepth          int64   `json:"maxDepth"`
	Drops             uint64  `json:"drops"`
	DropRatePerSecond float64 `json:"dropRatePerSecond"`
}

type telemetryEffectParitySnapshot struct {
	TotalTicks uint64                                `json:"totalTicks"`
	Entries    map[string]telemetryEffectParityEntry `json:"entries,omitempty"`
}

type telemetryTickBudgetSnapshot struct {
	BudgetMillis      int64             `json:"budgetMillis"`
	LastOverrunMillis int64             `json:"lastOverrunMillis,omitempty"`
	CurrentStreak     uint64            `json:"currentStreak"`
	MaxStreak         uint64            `json:"maxStreak"`
	Overruns          map[string]uint64 `json:"overruns,omitempty"`
	AlarmCount        uint64            `json:"alarmCount"`
	LastAlarmTick     uint64            `json:"lastAlarmTick,omitempty"`
	LastAlarmRatio    float64           `json:"lastAlarmRatio,omitempty"`
}

func newTelemetryCounters(metrics telemetry.Metrics) *telemetryCounters {
	t := &telemetryCounters{metrics: metrics}
	t.metricsAdapter.Attach(metrics)
	if os.Getenv("DEBUG_TELEMETRY") == "1" {
		t.debug = true
	}
	return t
}

func (t *telemetryCounters) AttachMetrics(metrics telemetry.Metrics) {
	if t == nil {
		return
	}
	t.metrics = metrics
	t.metricsAdapter.Attach(metrics)
}

func (t *telemetryCounters) RecordBroadcast(bytes, entities int) {
	if bytes < 0 {
		bytes = 0
	}
	if entities < 0 {
		entities = 0
	}
	t.bytesSent.Add(uint64(bytes))
	t.entitiesSent.Add(uint64(entities))
	t.lastBroadcastBytes.Store(uint64(bytes))
	t.lastBroadcastEntities.Store(uint64(entities))
	t.metricsAdapter.RecordBroadcast(bytes, entities)
}

func (t *telemetryCounters) RecordTickDuration(duration time.Duration) {
	millis := duration.Milliseconds()
	if millis < 0 {
		millis = 0
	}
	t.tickDurationMillis.Store(millis)
	total := t.totalTicks.Add(1)
	t.metricsAdapter.RecordTickDuration(duration, total)
	if t.debug {
		effects := t.effectsActiveGauge.Load()
		spawned := t.effectsSpawnedTotal.snapshot()
		updated := t.effectsUpdatedTotal.snapshot()
		ended := t.effectsEndedTotal.snapshot()
		overflow := t.effectsSpatialOverflow.snapshot()
		triggers := t.triggerEnqueued.snapshot()
		fmt.Printf(
			"[telemetry] tick=%dms bytes=%d totalBytes=%d entities=%d totalEntities=%d effectsActive=%d spawned=%v updated=%v ended=%v overflow=%v triggers=%v\n",
			millis,
			t.lastBroadcastBytes.Load(),
			t.bytesSent.Load(),
			t.lastBroadcastEntities.Load(),
			t.entitiesSent.Load(),
			effects,
			spawned,
			updated,
			ended,
			overflow,
			triggers,
		)
	}
}

func (t *telemetryCounters) RecordTickBudgetOverrun(duration, budget time.Duration) uint64 {
	if t == nil {
		return 0
	}
	if budget <= 0 {
		budget = time.Second / time.Duration(tickRate)
	}
	bucket := tickBudgetOverrunBucket(duration, budget)
	if bucket != "" {
		t.tickBudgetOverruns.add(bucket, 1)
	}
	millis := duration.Milliseconds()
	if millis < 0 {
		millis = 0
	}
	t.tickBudgetLastOverrunMillis.Store(millis)
	streak := t.tickBudgetConsecutiveOverruns.Add(1)
	for {
		current := t.tickBudgetMaxConsecutiveOverruns.Load()
		if streak <= current {
			break
		}
		if t.tickBudgetMaxConsecutiveOverruns.CompareAndSwap(current, streak) {
			break
		}
	}
	ratio := float64(0)
	if budget > 0 {
		ratio = float64(duration) / float64(budget)
	}
	maxStreak := t.tickBudgetMaxConsecutiveOverruns.Load()
	t.metricsAdapter.RecordTickBudgetOverrun(duration, budget, ratio, streak, maxStreak)
	return streak
}

func (t *telemetryCounters) RecordTickBudgetAlarm(tick uint64, ratio float64) {
	if t == nil {
		return
	}
	t.tickBudgetAlarms.Add(1)
	if tick > 0 {
		t.tickBudgetLastAlarmTick.Store(tick)
	}
	bits := math.Float64bits(ratio)
	t.tickBudgetLastAlarmRatio.Store(bits)
	t.metricsAdapter.RecordTickBudgetAlarm(tick, ratio)
}

func (t *telemetryCounters) ResetTickBudgetOverrunStreak() {
	if t == nil {
		return
	}
	t.tickBudgetConsecutiveOverruns.Store(0)
	t.metricsAdapter.ResetTickBudgetStreak()
}

func (t *telemetryCounters) RecordKeyframeJournal(size int, oldest, newest uint64) {
	if size < 0 {
		size = 0
	}
	t.keyframeJournalSize.Store(uint64(size))
	t.keyframeOldestSequence.Store(oldest)
	t.keyframeNewestSequence.Store(newest)
	t.metricsAdapter.RecordKeyframeJournal(uint64(size), oldest, newest)
}

func (t *telemetryCounters) RecordKeyframeRequest(latency time.Duration, success bool) {
	t.keyframeRequests.Add(1)
	var latencyMillis uint64
	if success {
		raw := latency.Milliseconds()
		if raw < 0 {
			raw = 0
		}
		latencyMillis = uint64(raw)
		t.keyframeRequestLatencyMillis.Store(latencyMillis)
	}
	t.metricsAdapter.RecordKeyframeRequest(success, latencyMillis)
}

func (t *telemetryCounters) IncrementKeyframeExpired() {
	t.keyframeNacksExpired.Add(1)
	t.metricsAdapter.IncrementKeyframeExpired()
}

func (t *telemetryCounters) IncrementKeyframeRateLimited() {
	t.keyframeNacksRateLimited.Add(1)
	t.metricsAdapter.IncrementKeyframeRateLimited()
}

func (t *telemetryCounters) RecordEffectSpawned(effectType, producer string) {
	if t == nil {
		return
	}
	t.effectsSpawnedTotal.add(effectType, producer, 1)
	t.metricsAdapter.RecordEffectSpawned(effectType, producer)
}

func (t *telemetryCounters) RecordEffectUpdated(effectType, mutation string) {
	if t == nil {
		return
	}
	t.effectsUpdatedTotal.add(effectType, mutation, 1)
	t.metricsAdapter.RecordEffectUpdated(effectType, mutation)
}

func (t *telemetryCounters) RecordEffectEnded(effectType, reason string) {
	if t == nil {
		return
	}
	t.effectsEndedTotal.add(effectType, reason, 1)
	t.metricsAdapter.RecordEffectEnded(effectType, reason)
}

func (t *telemetryCounters) RecordEffectSpatialOverflow(effectType string) {
	if t == nil {
		return
	}
	t.effectsSpatialOverflow.add(effectType, 1)
	t.metricsAdapter.RecordEffectSpatialOverflow(effectType)
}

func (t *telemetryCounters) RecordEffectsActive(count int) {
	if t == nil {
		return
	}
	if count < 0 {
		count = 0
	}
	t.effectsActiveGauge.Store(int64(count))
	t.metricsAdapter.RecordEffectsActive(uint64(count))
}

func (t *telemetryCounters) RecordEffectTrigger(triggerType string) {
	if t == nil {
		return
	}
	t.triggerEnqueued.add(triggerType, 1)
	t.metricsAdapter.RecordEffectTrigger(triggerType)
}

func (t *telemetryCounters) RecordJournalDrop(reason string) {
	if t == nil {
		return
	}
	t.journalDrops.add(reason, 1)
	t.metricsAdapter.RecordJournalDrop(reason)
}

func (t *telemetryCounters) RecordCommandDropped(reason string, cmdType string) {
	if t == nil {
		return
	}
	if reason == "" {
		reason = "unknown"
	}
	secondary := cmdType
	if secondary == "" {
		secondary = "unknown"
	}
	t.commandDrops.add(reason, secondary, 1)
	t.metricsAdapter.RecordCommandDrop(reason, secondary)
}

func (t *telemetryCounters) RecordSubscriberQueueDepth(depth int) {
	if t == nil {
		return
	}
	if depth < 0 {
		depth = 0
	}
	depth64 := int64(depth)
	t.subscriberQueueDepth.Store(depth64)
	for {
		current := t.subscriberQueueMaxDepth.Load()
		if depth64 <= current {
			break
		}
		if t.subscriberQueueMaxDepth.CompareAndSwap(current, depth64) {
			break
		}
	}
	t.metricsAdapter.RecordSubscriberQueueDepth(uint64(depth))
	maxDepth := t.subscriberQueueMaxDepth.Load()
	if maxDepth < 0 {
		maxDepth = 0
	}
	t.metricsAdapter.RecordSubscriberQueueMaxDepth(uint64(maxDepth))
}

func (t *telemetryCounters) RecordSubscriberQueueDrop(depth int) {
	if t == nil {
		return
	}
	t.RecordSubscriberQueueDepth(depth)
	t.subscriberQueueDrops.Add(1)
	t.metricsAdapter.IncrementSubscriberQueueDrops()
}

func (t *telemetryCounters) RecordBroadcastQueueDepth(depth int) {
	if t == nil {
		return
	}
	if depth < 0 {
		depth = 0
	}
	depth64 := int64(depth)
	t.broadcastQueueDepth.Store(depth64)
	for {
		current := t.broadcastQueueMaxDepth.Load()
		if depth64 <= current {
			break
		}
		if t.broadcastQueueMaxDepth.CompareAndSwap(current, depth64) {
			break
		}
	}
	t.metricsAdapter.RecordBroadcastQueueDepth(uint64(depth))
	maxDepth := t.broadcastQueueMaxDepth.Load()
	if maxDepth < 0 {
		maxDepth = 0
	}
	t.metricsAdapter.RecordBroadcastQueueMaxDepth(uint64(maxDepth))
}

func (t *telemetryCounters) RecordBroadcastQueueDrop(depth int) {
	if t == nil {
		return
	}
	t.RecordBroadcastQueueDepth(depth)
	t.broadcastQueueDrops.Add(1)
	t.metricsAdapter.IncrementBroadcastQueueDrops()
}

func (t *telemetryCounters) RecordEffectParity(summary worldpkg.EffectTelemetrySummary) {
	if t == nil {
		return
	}
	t.effectParity.record(summary)
	t.metricsAdapter.RecordEffectParity(summary)
}

func (t *telemetryCounters) DebugEnabled() bool {
	return t.debug
}

func (t *telemetryCounters) Snapshot() telemetrySnapshot {
	totalTicks := t.totalTicks.Load()
	tickBudget := time.Second / time.Duration(tickRate)
	depth := t.subscriberQueueDepth.Load()
	maxDepth := t.subscriberQueueMaxDepth.Load()
	drops := t.subscriberQueueDrops.Load()
	broadcastDepth := t.broadcastQueueDepth.Load()
	broadcastMaxDepth := t.broadcastQueueMaxDepth.Load()
	broadcastDrops := t.broadcastQueueDrops.Load()
	var dropRate float64
	var broadcastDropRate float64
	if totalTicks > 0 {
		dropRate = float64(drops) * float64(tickRate) / float64(totalTicks)
		broadcastDropRate = float64(broadcastDrops) * float64(tickRate) / float64(totalTicks)
	}
	snapshot := telemetrySnapshot{
		BytesSent:                t.bytesSent.Load(),
		EntitiesSent:             t.entitiesSent.Load(),
		TickDuration:             t.tickDurationMillis.Load(),
		KeyframeJournalSize:      t.keyframeJournalSize.Load(),
		KeyframeOldestSequence:   t.keyframeOldestSequence.Load(),
		KeyframeNewestSequence:   t.keyframeNewestSequence.Load(),
		KeyframeRequests:         t.keyframeRequests.Load(),
		KeyframeNacksExpired:     t.keyframeNacksExpired.Load(),
		KeyframeNacksRateLimited: t.keyframeNacksRateLimited.Load(),
		KeyframeRequestLatencyMs: t.keyframeRequestLatencyMillis.Load(),
		Effects: telemetryEffectsSnapshot{
			SpawnedTotal:    t.effectsSpawnedTotal.snapshot(),
			UpdatedTotal:    t.effectsUpdatedTotal.snapshot(),
			EndedTotal:      t.effectsEndedTotal.snapshot(),
			ActiveGauge:     t.effectsActiveGauge.Load(),
			SpatialOverflow: t.effectsSpatialOverflow.snapshot(),
		},
		EffectTriggers: telemetryEffectTriggersSnapshot{
			EnqueuedTotal: t.triggerEnqueued.snapshot(),
		},
		JournalDrops: t.journalDrops.snapshot(),
		CommandDrops: t.commandDrops.snapshot(),
		SubscriberQueues: telemetrySubscriberQueueSnapshot{
			Depth:             depth,
			MaxDepth:          maxDepth,
			Drops:             drops,
			DropRatePerSecond: dropRate,
		},
		BroadcastQueue: telemetryBroadcastQueueSnapshot{
			Depth:             broadcastDepth,
			MaxDepth:          broadcastMaxDepth,
			Drops:             broadcastDrops,
			DropRatePerSecond: broadcastDropRate,
		},
		EffectParity: telemetryEffectParitySnapshot{
			TotalTicks: totalTicks,
			Entries:    t.effectParity.snapshot(totalTicks),
		},
	}
	tickBudgetSnapshot := telemetryTickBudgetSnapshot{
		BudgetMillis:  tickBudget.Milliseconds(),
		CurrentStreak: t.tickBudgetConsecutiveOverruns.Load(),
		MaxStreak:     t.tickBudgetMaxConsecutiveOverruns.Load(),
		Overruns:      t.tickBudgetOverruns.snapshot(),
		AlarmCount:    t.tickBudgetAlarms.Load(),
	}
	if last := t.tickBudgetLastOverrunMillis.Load(); last > 0 {
		tickBudgetSnapshot.LastOverrunMillis = last
	}
	if tickBudgetSnapshot.AlarmCount > 0 {
		if tick := t.tickBudgetLastAlarmTick.Load(); tick > 0 {
			tickBudgetSnapshot.LastAlarmTick = tick
		}
		if bits := t.tickBudgetLastAlarmRatio.Load(); bits != 0 {
			tickBudgetSnapshot.LastAlarmRatio = math.Float64frombits(bits)
		}
	}
	snapshot.TickBudget = tickBudgetSnapshot
	return snapshot
}
