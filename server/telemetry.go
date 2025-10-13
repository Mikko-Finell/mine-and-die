package main

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

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

type effectParitySummary struct {
	EffectType    string
	Source        string
	Hits          int
	UniqueVictims int
	TotalDamage   float64
	SpawnTick     Tick
	FirstHitTick  Tick
}

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
	totals map[string]map[string]*effectParityTotals
}

func (a *effectParityAggregator) record(summary effectParitySummary) {
	if a == nil {
		return
	}
	normalizedType := normalizeMetricKey(summary.EffectType)
	normalizedSource := normalizeMetricKey(summary.Source)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.totals == nil {
		a.totals = make(map[string]map[string]*effectParityTotals)
	}
	sources := a.totals[normalizedType]
	if sources == nil {
		sources = make(map[string]*effectParityTotals)
		a.totals[normalizedType] = sources
	}
	totals := sources[normalizedSource]
	if totals == nil {
		totals = &effectParityTotals{VictimBuckets: make(map[string]uint64)}
		sources[normalizedSource] = totals
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

func (a *effectParityAggregator) snapshot(totalTicks uint64) map[string]map[string]telemetryEffectParityEntry {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.totals) == 0 {
		return nil
	}
	result := make(map[string]map[string]telemetryEffectParityEntry, len(a.totals))
	for effectType, sources := range a.totals {
		if len(sources) == 0 {
			continue
		}
		entries := make(map[string]telemetryEffectParityEntry, len(sources))
		for source, totals := range sources {
			if totals == nil {
				continue
			}
			entries[source] = totals.toSnapshot(totalTicks)
		}
		if len(entries) > 0 {
			result[effectType] = entries
		}
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

	effectsSpawnedTotal layeredCounter
	effectsUpdatedTotal layeredCounter
	effectsEndedTotal   layeredCounter
	effectsActiveGauge  atomic.Int64
	triggerEnqueued     simpleCounter
	journalDrops        simpleCounter

	totalTicks   atomic.Uint64
	effectParity effectParityAggregator
}

type telemetrySnapshot struct {
	BytesSent                uint64                          `json:"bytesSent"`
	EntitiesSent             uint64                          `json:"entitiesSent"`
	TickDuration             int64                           `json:"tickDurationMillis"`
	KeyframeJournalSize      uint64                          `json:"keyframeJournalSize"`
	KeyframeOldestSequence   uint64                          `json:"keyframeOldestSequence"`
	KeyframeNewestSequence   uint64                          `json:"keyframeNewestSequence"`
	KeyframeRequests         uint64                          `json:"keyframeRequests"`
	KeyframeNacksExpired     uint64                          `json:"keyframeNacksExpired"`
	KeyframeNacksRateLimited uint64                          `json:"keyframeNacksRateLimited"`
	KeyframeRequestLatencyMs uint64                          `json:"keyframeRequestLatencyMs"`
	Effects                  telemetryEffectsSnapshot        `json:"effects"`
	EffectTriggers           telemetryEffectTriggersSnapshot `json:"effectTriggers"`
	JournalDrops             map[string]uint64               `json:"journalDrops,omitempty"`
	EffectParity             telemetryEffectParitySnapshot   `json:"effectParity"`
}

type telemetryEffectsSnapshot struct {
	SpawnedTotal map[string]map[string]uint64 `json:"spawnedTotal,omitempty"`
	UpdatedTotal map[string]map[string]uint64 `json:"updatedTotal,omitempty"`
	EndedTotal   map[string]map[string]uint64 `json:"endedTotal,omitempty"`
	ActiveGauge  int64                        `json:"activeGauge"`
}

type telemetryEffectTriggersSnapshot struct {
	EnqueuedTotal map[string]uint64 `json:"enqueuedTotal,omitempty"`
}

type telemetryEffectParitySnapshot struct {
	TotalTicks uint64                                           `json:"totalTicks"`
	Entries    map[string]map[string]telemetryEffectParityEntry `json:"entries,omitempty"`
}

func newTelemetryCounters() *telemetryCounters {
	t := &telemetryCounters{}
	if os.Getenv("DEBUG_TELEMETRY") == "1" {
		t.debug = true
	}
	return t
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
}

func (t *telemetryCounters) RecordTickDuration(duration time.Duration) {
	millis := duration.Milliseconds()
	if millis < 0 {
		millis = 0
	}
	t.tickDurationMillis.Store(millis)
	t.totalTicks.Add(1)
	if t.debug {
		effects := t.effectsActiveGauge.Load()
		spawned := t.effectsSpawnedTotal.snapshot()
		updated := t.effectsUpdatedTotal.snapshot()
		ended := t.effectsEndedTotal.snapshot()
		triggers := t.triggerEnqueued.snapshot()
		fmt.Printf(
			"[telemetry] tick=%dms bytes=%d totalBytes=%d entities=%d totalEntities=%d effectsActive=%d spawned=%v updated=%v ended=%v triggers=%v\n",
			millis,
			t.lastBroadcastBytes.Load(),
			t.bytesSent.Load(),
			t.lastBroadcastEntities.Load(),
			t.entitiesSent.Load(),
			effects,
			spawned,
			updated,
			ended,
			triggers,
		)
	}
}

func (t *telemetryCounters) RecordKeyframeJournal(size int, oldest, newest uint64) {
	if size < 0 {
		size = 0
	}
	t.keyframeJournalSize.Store(uint64(size))
	t.keyframeOldestSequence.Store(oldest)
	t.keyframeNewestSequence.Store(newest)
}

func (t *telemetryCounters) RecordKeyframeRequest(latency time.Duration, success bool) {
	t.keyframeRequests.Add(1)
	if success {
		millis := latency.Milliseconds()
		if millis < 0 {
			millis = 0
		}
		t.keyframeRequestLatencyMillis.Store(uint64(millis))
	}
}

func (t *telemetryCounters) IncrementKeyframeExpired() {
	t.keyframeNacksExpired.Add(1)
}

func (t *telemetryCounters) IncrementKeyframeRateLimited() {
	t.keyframeNacksRateLimited.Add(1)
}

func (t *telemetryCounters) RecordEffectSpawned(effectType, producer string) {
	if t == nil {
		return
	}
	t.effectsSpawnedTotal.add(effectType, producer, 1)
}

func (t *telemetryCounters) RecordEffectUpdated(effectType, mutation string) {
	if t == nil {
		return
	}
	t.effectsUpdatedTotal.add(effectType, mutation, 1)
}

func (t *telemetryCounters) RecordEffectEnded(effectType, reason string) {
	if t == nil {
		return
	}
	t.effectsEndedTotal.add(effectType, reason, 1)
}

func (t *telemetryCounters) RecordEffectsActive(count int) {
	if t == nil {
		return
	}
	if count < 0 {
		count = 0
	}
	t.effectsActiveGauge.Store(int64(count))
}

func (t *telemetryCounters) RecordEffectTrigger(triggerType string) {
	if t == nil {
		return
	}
	t.triggerEnqueued.add(triggerType, 1)
}

func (t *telemetryCounters) RecordJournalDrop(reason string) {
	if t == nil {
		return
	}
	t.journalDrops.add(reason, 1)
}

func (t *telemetryCounters) RecordEffectParity(summary effectParitySummary) {
	if t == nil {
		return
	}
	t.effectParity.record(summary)
}

func (t *telemetryCounters) DebugEnabled() bool {
	return t.debug
}

func (t *telemetryCounters) Snapshot() telemetrySnapshot {
	totalTicks := t.totalTicks.Load()
	return telemetrySnapshot{
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
			SpawnedTotal: t.effectsSpawnedTotal.snapshot(),
			UpdatedTotal: t.effectsUpdatedTotal.snapshot(),
			EndedTotal:   t.effectsEndedTotal.snapshot(),
			ActiveGauge:  t.effectsActiveGauge.Load(),
		},
		EffectTriggers: telemetryEffectTriggersSnapshot{
			EnqueuedTotal: t.triggerEnqueued.snapshot(),
		},
		JournalDrops: t.journalDrops.snapshot(),
		EffectParity: telemetryEffectParitySnapshot{
			TotalTicks: totalTicks,
			Entries:    t.effectParity.snapshot(totalTicks),
		},
	}
}
