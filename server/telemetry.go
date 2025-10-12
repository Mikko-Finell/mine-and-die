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

func (t *telemetryCounters) DebugEnabled() bool {
	return t.debug
}

func (t *telemetryCounters) Snapshot() telemetrySnapshot {
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
	}
}
