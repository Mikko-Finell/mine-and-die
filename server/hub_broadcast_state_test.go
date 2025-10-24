package server

import (
	"context"
	"sync"
	"testing"
	"time"

	"mine-and-die/server/logging"
)

type stubClock struct {
	mu       sync.Mutex
	times    []time.Time
	index    int
	observed []time.Time
}

func newStubClock(times []time.Time) *stubClock {
	cloned := make([]time.Time, len(times))
	copy(cloned, times)
	return &stubClock{times: cloned}
}

func (c *stubClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.times) == 0 {
		now := time.Now()
		c.observed = append(c.observed, now)
		return now
	}

	var current time.Time
	if c.index < len(c.times) {
		current = c.times[c.index]
		c.index++
	} else {
		current = c.times[len(c.times)-1]
	}
	c.observed = append(c.observed, current)
	return current
}

func (c *stubClock) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.index = 0
	c.observed = nil
}

func (c *stubClock) Observed() []time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	snapshot := make([]time.Time, len(c.observed))
	copy(snapshot, c.observed)
	return snapshot
}

type stubPublisherWithClock struct {
	clock logging.Clock
}

func (p stubPublisherWithClock) Publish(context.Context, logging.Event) {}

func (p stubPublisherWithClock) Clock() logging.Clock {
	return p.clock
}

type recordingSubscriberConn struct {
	mu        sync.Mutex
	deadlines []time.Time
	writes    int
}

func (c *recordingSubscriberConn) Write([]byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writes++
	return nil
}

func (c *recordingSubscriberConn) SetWriteDeadline(deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deadlines = append(c.deadlines, deadline)
	return nil
}

func (c *recordingSubscriberConn) Close() error { return nil }

func (c *recordingSubscriberConn) snapshot() (int, []time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	copiedDeadlines := make([]time.Time, len(c.deadlines))
	copy(copiedDeadlines, c.deadlines)
	return c.writes, copiedDeadlines
}

func (c *recordingSubscriberConn) waitWrites(t *testing.T, expected int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		writes := c.writes
		c.mu.Unlock()
		if writes >= expected {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	c.mu.Lock()
	writes := c.writes
	c.mu.Unlock()
	t.Fatalf("expected %d writes, got %d", expected, writes)
}

func TestBroadcastStateRefreshesDeadlinesPerSubscriber(t *testing.T) {
	base := time.Unix(1_700_000_000, 0).UTC()
	clock := newStubClock([]time.Time{
		base,
		base.Add(45 * time.Second),
		base.Add(90 * time.Second),
		base.Add(135 * time.Second),
	})
	hub := NewHubWithConfig(DefaultHubConfig(), stubPublisherWithClock{clock: clock})

	clock.Reset()

	subscriberIDs := []string{"alpha", "bravo", "charlie"}
	connections := make(map[string]*recordingSubscriberConn, len(subscriberIDs))

	hub.mu.Lock()
	for _, id := range subscriberIDs {
		conn := &recordingSubscriberConn{}
		connections[id] = conn
		sub := newSubscriber(conn)
		hub.subscribers[id] = sub
		t.Cleanup(sub.Close)
	}
	hub.mu.Unlock()

	hub.broadcastState(nil, nil, nil, nil)

	for _, conn := range connections {
		conn.waitWrites(t, 1)
	}

	observed := clock.Observed()
	if len(observed) != len(subscriberIDs)+1 {
		t.Fatalf("expected %d clock reads, got %d", len(subscriberIDs)+1, len(observed))
	}

	remaining := make(map[int64]int, len(subscriberIDs))
	for _, ts := range observed[1:] {
		remaining[ts.UnixNano()]++
	}

	for id, conn := range connections {
		writes, deadlines := conn.snapshot()
		if writes != 1 {
			t.Fatalf("subscriber %s expected 1 write, got %d", id, writes)
		}
		if len(deadlines) != 1 {
			t.Fatalf("subscriber %s expected 1 deadline, got %d", id, len(deadlines))
		}
		baseTime := deadlines[0].Add(-writeWait)
		key := baseTime.UnixNano()
		count := remaining[key]
		if count == 0 {
			t.Fatalf("subscriber %s deadline %s not backed by clock observations %v", id, baseTime, observed)
		}
		if count == 1 {
			delete(remaining, key)
		} else {
			remaining[key] = count - 1
		}
	}

	if len(remaining) != 0 {
		t.Fatalf("unmatched clock observations remained: %v", remaining)
	}
}
