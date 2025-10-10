package main

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

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
}

type telemetrySnapshot struct {
	BytesSent                uint64 `json:"bytesSent"`
	EntitiesSent             uint64 `json:"entitiesSent"`
	TickDuration             int64  `json:"tickDurationMillis"`
	KeyframeJournalSize      uint64 `json:"keyframeJournalSize"`
	KeyframeOldestSequence   uint64 `json:"keyframeOldestSequence"`
	KeyframeNewestSequence   uint64 `json:"keyframeNewestSequence"`
	KeyframeRequests         uint64 `json:"keyframeRequests"`
	KeyframeNacksExpired     uint64 `json:"keyframeNacksExpired"`
	KeyframeNacksRateLimited uint64 `json:"keyframeNacksRateLimited"`
	KeyframeRequestLatencyMs uint64 `json:"keyframeRequestLatencyMs"`
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
		fmt.Printf(
			"[telemetry] tick=%dms bytes=%d totalBytes=%d entities=%d totalEntities=%d\n",
			millis,
			t.lastBroadcastBytes.Load(),
			t.bytesSent.Load(),
			t.lastBroadcastEntities.Load(),
			t.entitiesSent.Load(),
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
	}
}
