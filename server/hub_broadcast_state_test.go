package server

import (
	"context"
	"errors"
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

type recordingQueueTelemetry struct {
	mu     sync.Mutex
	depths []int
	drops  []int
}

func (t *recordingQueueTelemetry) RecordSubscriberQueueDepth(depth int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.depths = append(t.depths, depth)
}

func (t *recordingQueueTelemetry) RecordSubscriberQueueDrop(depth int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.drops = append(t.drops, depth)
}

func (t *recordingQueueTelemetry) snapshot() ([]int, []int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	depths := make([]int, len(t.depths))
	copy(depths, t.depths)
	drops := make([]int, len(t.drops))
	copy(drops, t.drops)
	return depths, drops
}

type blockingSubscriberConn struct {
	mu     sync.Mutex
	writes int
	block  chan struct{}
}

func newBlockingSubscriberConn() *blockingSubscriberConn {
	return &blockingSubscriberConn{block: make(chan struct{}, subscriberSendQueueSize*2)}
}

func (c *blockingSubscriberConn) Write([]byte) error {
	<-c.block
	c.mu.Lock()
	c.writes++
	c.mu.Unlock()
	return nil
}

func (c *blockingSubscriberConn) SetWriteDeadline(time.Time) error { return nil }

func (c *blockingSubscriberConn) Close() error { return nil }

func (c *blockingSubscriberConn) allow(count int) {
	for i := 0; i < count; i++ {
		c.block <- struct{}{}
	}
}

func (c *blockingSubscriberConn) waitWrites(t *testing.T, expected int) {
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
		sub := newSubscriber(conn, nil)
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

func TestSubscriberRecordsQueueTelemetry(t *testing.T) {
	telemetry := &recordingQueueTelemetry{}
	conn := newBlockingSubscriberConn()
	sub := newSubscriber(conn, telemetry)
	t.Cleanup(sub.Close)

	conn.allow(1)
	if err := sub.Write([]byte("initial")); err != nil {
		t.Fatalf("expected initial write to succeed, got %v", err)
	}
	conn.waitWrites(t, 1)

	for i := 0; i < subscriberSendQueueSize; i++ {
		if err := sub.EnqueueBroadcast(time.Now(), []byte("payload")); err != nil {
			t.Fatalf("unexpected enqueue error at %d: %v", i, err)
		}
	}

	if err := sub.EnqueueBroadcast(time.Now(), []byte("filler")); err != nil {
		t.Fatalf("expected final queue slot to accept, got %v", err)
	}

	err := sub.EnqueueBroadcast(time.Now(), []byte("overflow"))
	if !errors.Is(err, errSubscriberQueueFull) {
		t.Fatalf("expected subscriber queue full error, got %v", err)
	}

	conn.allow(subscriberSendQueueSize + 1)
	conn.waitWrites(t, subscriberSendQueueSize+2)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		depths, _ := telemetry.snapshot()
		if len(depths) > 0 && depths[len(depths)-1] == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	depths, drops := telemetry.snapshot()
	if len(depths) == 0 || depths[len(depths)-1] != 0 {
		t.Fatalf("expected final queue depth to be 0, got %v", depths)
	}
	maxDepth := 0
	for _, depth := range depths {
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	if maxDepth != subscriberSendQueueSize {
		t.Fatalf("expected max queue depth %d, got %d", subscriberSendQueueSize, maxDepth)
	}
	if len(drops) != 1 {
		t.Fatalf("expected exactly one drop, got %d", len(drops))
	}
	if drops[0] != subscriberSendQueueSize {
		t.Fatalf("expected drop depth %d, got %d", subscriberSendQueueSize, drops[0])
	}
}
