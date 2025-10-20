package server

import (
	"testing"
	"time"

	effectcontract "mine-and-die/server/effects/contract"
)

func TestJournalEvictsByCount(t *testing.T) {
	journal := newJournal(2, time.Minute)

	first := journal.RecordKeyframe(keyframe{Sequence: 1, Tick: 10})
	if first.Size != 1 {
		t.Fatalf("expected size 1 after first record, got %d", first.Size)
	}

	second := journal.RecordKeyframe(keyframe{Sequence: 2, Tick: 11})
	if second.Size != 2 {
		t.Fatalf("expected size 2 after second record, got %d", second.Size)
	}
	if second.OldestSequence != 1 || second.NewestSequence != 2 {
		t.Fatalf("unexpected window after second record: oldest=%d newest=%d", second.OldestSequence, second.NewestSequence)
	}

	third := journal.RecordKeyframe(keyframe{Sequence: 3, Tick: 12})
	if third.Size != 2 {
		t.Fatalf("expected size to remain at capacity, got %d", third.Size)
	}
	if third.OldestSequence != 2 || third.NewestSequence != 3 {
		t.Fatalf("unexpected window after eviction: oldest=%d newest=%d", third.OldestSequence, third.NewestSequence)
	}
	if len(third.Evicted) == 0 {
		t.Fatalf("expected eviction metadata when exceeding capacity")
	}
	if third.Evicted[0].Sequence != 1 || third.Evicted[0].Reason != "count" {
		t.Fatalf("unexpected eviction record: %+v", third.Evicted[0])
	}
}

func TestJournalEvictsByAge(t *testing.T) {
	journal := newJournal(4, 5*time.Millisecond)

	journal.RecordKeyframe(keyframe{Sequence: 1, Tick: 5})
	time.Sleep(10 * time.Millisecond)
	result := journal.RecordKeyframe(keyframe{Sequence: 2, Tick: 6})

	if result.Size != 1 {
		t.Fatalf("expected journal to trim expired frames, size=%d", result.Size)
	}
	if len(result.Evicted) == 0 {
		t.Fatalf("expected eviction metadata for expired frame")
	}
	eviction := result.Evicted[0]
	if eviction.Sequence != 1 {
		t.Fatalf("expected sequence 1 to expire, got %d", eviction.Sequence)
	}
	if eviction.Reason != "expired" {
		t.Fatalf("expected expired reason, got %s", eviction.Reason)
	}
	if result.OldestSequence != 2 || result.NewestSequence != 2 {
		t.Fatalf("unexpected window after expiry: oldest=%d newest=%d", result.OldestSequence, result.NewestSequence)
	}
}

func TestJournalRecordsEffectEventsWithSequences(t *testing.T) {
	journal := newJournal(0, 0)
	journal.AttachTelemetry(newTelemetryCounters(nil))

	extra := map[string]int{"damage": 15}
	params := map[string]int{"damage": 15}
	spawn := journal.RecordEffectSpawn(effectcontract.EffectSpawnEvent{
		Tick: 10,
		Instance: effectcontract.EffectInstance{
			ID: "effect-1",
			DeliveryState: effectcontract.EffectDeliveryState{
				Geometry: effectcontract.EffectGeometry{Shape: effectcontract.GeometryShapeCircle, Radius: 4},
			},
			BehaviorState: effectcontract.EffectBehaviorState{TicksRemaining: 5, Extra: extra},
			Params:        params,
			Replication:   effectcontract.ReplicationSpec{SendSpawn: true, SendUpdates: true, SendEnd: true},
		},
	})

	if spawn.Seq != 1 {
		t.Fatalf("expected spawn sequence 1, got %d", spawn.Seq)
	}
	if spawn.Instance.ID != "effect-1" {
		t.Fatalf("expected spawn instance id to be preserved")
	}

	extra["damage"] = 30
	if spawn.Instance.BehaviorState.Extra["damage"] != 15 {
		t.Fatalf("expected spawn clone to protect behavior state map")
	}

	params["damage"] = 25
	if spawn.Instance.Params["damage"] != 15 {
		t.Fatalf("expected spawn clone to protect params map")
	}

	updateParams := map[string]int{"damage": 20}
	update := journal.RecordEffectUpdate(effectcontract.EffectUpdateEvent{
		Tick:   11,
		ID:     "effect-1",
		Params: updateParams,
	})
	if update.Seq != 2 {
		t.Fatalf("expected update sequence 2, got %d", update.Seq)
	}
	updateParams["damage"] = 35
	if update.Params["damage"] != 20 {
		t.Fatalf("expected update params to be cloned from input")
	}

	end := journal.RecordEffectEnd(effectcontract.EffectEndEvent{Tick: 12, ID: "effect-1", Reason: effectcontract.EndReasonExpired})
	if end.Seq != 3 {
		t.Fatalf("expected end sequence 3, got %d", end.Seq)
	}

	snapshot := journal.SnapshotEffectEvents()
	if len(snapshot.Spawns) != 1 || len(snapshot.Updates) != 1 || len(snapshot.Ends) != 1 {
		t.Fatalf("expected snapshot to include all staged events: %+v", snapshot)
	}
	if snapshot.LastSeqByID["effect-1"] != 3 {
		t.Fatalf("expected snapshot cursor to report last seq 3, got %d", snapshot.LastSeqByID["effect-1"])
	}

	drained := journal.DrainEffectEvents()
	if len(drained.Spawns) != 1 || len(drained.Updates) != 1 || len(drained.Ends) != 1 {
		t.Fatalf("expected drained batch to include one of each event, got %+v", drained)
	}
	if drained.LastSeqByID["effect-1"] != 3 {
		t.Fatalf("expected drained cursor to report seq 3, got %d", drained.LastSeqByID["effect-1"])
	}

	// Subsequent drains should be empty until new events are recorded.
	cleared := journal.DrainEffectEvents()
	if len(cleared.Spawns) != 0 || len(cleared.Updates) != 0 || len(cleared.Ends) != 0 {
		t.Fatalf("expected cleared batch after drain, got %+v", cleared)
	}

	respawn := journal.RecordEffectSpawn(effectcontract.EffectSpawnEvent{Tick: 20, Instance: effectcontract.EffectInstance{ID: "effect-1"}})
	if respawn.Seq != 1 {
		t.Fatalf("expected new spawn to reset sequence, got %d", respawn.Seq)
	}
}

func TestJournalDropsNonMonotonicSequences(t *testing.T) {
	journal := newJournal(0, 0)
	telemetry := newTelemetryCounters(nil)
	journal.AttachTelemetry(telemetry)

	spawn := journal.RecordEffectSpawn(effectcontract.EffectSpawnEvent{Tick: 1, Instance: effectcontract.EffectInstance{ID: "effect-1"}})
	if spawn.Seq != 1 {
		t.Fatalf("expected spawn sequence 1, got %d", spawn.Seq)
	}

	accepted := journal.RecordEffectUpdate(effectcontract.EffectUpdateEvent{Tick: 2, ID: "effect-1", Seq: 2})
	if accepted.Seq != 2 {
		t.Fatalf("expected update sequence 2, got %d", accepted.Seq)
	}

	duplicate := journal.RecordEffectUpdate(effectcontract.EffectUpdateEvent{Tick: 3, ID: "effect-1", Seq: 2})
	if duplicate.ID != "" || duplicate.Seq != 0 {
		t.Fatalf("expected duplicate sequence to be dropped, got %+v", duplicate)
	}

	regression := journal.RecordEffectUpdate(effectcontract.EffectUpdateEvent{Tick: 4, ID: "effect-1", Seq: 1})
	if regression.ID != "" || regression.Seq != 0 {
		t.Fatalf("expected regressed sequence to be dropped, got %+v", regression)
	}

	snapshot := telemetry.Snapshot()
	if snapshot.JournalDrops[metricJournalNonMonotonicSeq] != 2 {
		t.Fatalf("expected two non-monotonic drops, got %d", snapshot.JournalDrops[metricJournalNonMonotonicSeq])
	}
}

func TestJournalDropsUnknownEffectUpdates(t *testing.T) {
	journal := newJournal(0, 0)
	telemetry := newTelemetryCounters(nil)
	journal.AttachTelemetry(telemetry)

	dropped := journal.RecordEffectUpdate(effectcontract.EffectUpdateEvent{Tick: 5, ID: "effect-x", Seq: 1})
	if dropped.ID != "" || dropped.Seq != 0 {
		t.Fatalf("expected unknown effect update to be dropped, got %+v", dropped)
	}

	endDrop := journal.RecordEffectEnd(effectcontract.EffectEndEvent{Tick: 6, ID: "effect-x", Seq: 2})
	if endDrop != (effectcontract.EffectEndEvent{}) {
		t.Fatalf("expected unknown effect end to be dropped, got %+v", endDrop)
	}

	snapshot := telemetry.Snapshot()
	if snapshot.JournalDrops[metricJournalUnknownIDUpdate] != 2 {
		t.Fatalf("expected two unknown id drops, got %d", snapshot.JournalDrops[metricJournalUnknownIDUpdate])
	}
}

func TestJournalResyncHintOnLostSpawn(t *testing.T) {
	journal := newJournal(0, 0)

	journal.RecordEffectUpdate(effectcontract.EffectUpdateEvent{Tick: 5, ID: "effect-x"})
	signal, ok := journal.ConsumeResyncHint()
	if !ok {
		t.Fatalf("expected resync hint after unknown update")
	}
	if signal.LostSpawns != 1 {
		t.Fatalf("expected lost spawn count 1, got %d", signal.LostSpawns)
	}
	if len(signal.Reasons) == 0 || signal.Reasons[0].Kind != metricJournalUnknownIDUpdate {
		t.Fatalf("expected reason to record unknown id drop, got %+v", signal.Reasons)
	}
	if _, ok := journal.ConsumeResyncHint(); ok {
		t.Fatalf("expected resync hint to reset after consumption")
	}
}

func TestJournalDropsUpdatesAfterEnd(t *testing.T) {
	journal := newJournal(0, 0)
	telemetry := newTelemetryCounters(nil)
	journal.AttachTelemetry(telemetry)

	journal.RecordEffectSpawn(effectcontract.EffectSpawnEvent{Tick: 10, Instance: effectcontract.EffectInstance{ID: "effect-1"}})
	end := journal.RecordEffectEnd(effectcontract.EffectEndEvent{Tick: 12, ID: "effect-1"})
	if end.Seq != 2 {
		t.Fatalf("expected end sequence 2, got %d", end.Seq)
	}

	dropped := journal.RecordEffectUpdate(effectcontract.EffectUpdateEvent{Tick: 13, ID: "effect-1", Seq: 3})
	if dropped.ID != "" || dropped.Seq != 0 {
		t.Fatalf("expected update after end to be dropped, got %+v", dropped)
	}

	snapshot := telemetry.Snapshot()
	if snapshot.JournalDrops[metricJournalUpdateAfterEnd] != 1 {
		t.Fatalf("expected update-after-end metric to increment, got %d", snapshot.JournalDrops[metricJournalUpdateAfterEnd])
	}

	journal.DrainEffectEvents()

	allowed := journal.RecordEffectUpdate(effectcontract.EffectUpdateEvent{Tick: 20, ID: "effect-1", Seq: 4})
	if allowed.ID != "" || allowed.Seq != 0 {
		t.Fatalf("expected update to unknown id after drain to be dropped, got %+v", allowed)
	}
	snapshot = telemetry.Snapshot()
	if snapshot.JournalDrops[metricJournalUnknownIDUpdate] == 0 {
		t.Fatalf("expected unknown id metric to increment after drain")
	}
}
