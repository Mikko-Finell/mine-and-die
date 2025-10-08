package main

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

type telemetryCounters struct {
	bytesSent             atomic.Uint64
	entitiesSent          atomic.Uint64
	tickDurationMillis    atomic.Int64
	lastBroadcastBytes    atomic.Uint64
	lastBroadcastEntities atomic.Uint64
	debug                 bool
}

type telemetrySnapshot struct {
	BytesSent    uint64 `json:"bytesSent"`
	EntitiesSent uint64 `json:"entitiesSent"`
	TickDuration int64  `json:"tickDurationMillis"`
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

func (t *telemetryCounters) Snapshot() telemetrySnapshot {
	return telemetrySnapshot{
		BytesSent:    t.bytesSent.Load(),
		EntitiesSent: t.entitiesSent.Load(),
		TickDuration: t.tickDurationMillis.Load(),
	}
}
