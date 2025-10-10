package main

import (
	"testing"
	"time"
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
