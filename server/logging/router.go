package logging

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
)

// Sink consumes events produced by the router.
type Sink interface {
	Write(Event) error
	Close(context.Context) error
}

// Metrics tracks counters exposed by the router for diagnostics.
type Metrics struct {
	eventsTotal        atomic.Uint64
	eventsDroppedTotal atomic.Uint64
	sinkErrorsTotal    atomic.Uint64
	sinkDisabledTotal  atomic.Uint64
	telemetry          sync.Map // string -> *atomic.Uint64
}

// Snapshot returns a copy of the metrics counters.
func (m *Metrics) Snapshot() map[string]uint64 {
	snapshot := map[string]uint64{
		"events_total":         m.eventsTotal.Load(),
		"events_dropped_total": m.eventsDroppedTotal.Load(),
		"sink_errors_total":    m.sinkErrorsTotal.Load(),
		"sink_disabled_total":  m.sinkDisabledTotal.Load(),
	}
	m.telemetry.Range(func(key, value any) bool {
		name, ok := key.(string)
		if !ok || name == "" {
			return true
		}
		counter, ok := value.(*atomic.Uint64)
		if !ok || counter == nil {
			return true
		}
		snapshot[name] = counter.Load()
		return true
	})
	return snapshot
}

func (m *Metrics) telemetryCounter(key string) *atomic.Uint64 {
	if m == nil || key == "" {
		return nil
	}
	if counter, ok := m.telemetry.Load(key); ok {
		if typed, ok := counter.(*atomic.Uint64); ok {
			return typed
		}
	}
	fresh := &atomic.Uint64{}
	actual, _ := m.telemetry.LoadOrStore(key, fresh)
	if counter, ok := actual.(*atomic.Uint64); ok {
		return counter
	}
	return fresh
}

// TelemetryAdd increments the named telemetry counter by the provided delta.
func (m *Metrics) TelemetryAdd(key string, delta uint64) {
	if m == nil || delta == 0 {
		return
	}
	if counter := m.telemetryCounter(key); counter != nil {
		counter.Add(delta)
	}
}

// TelemetryStore records the provided value for the named telemetry gauge.
func (m *Metrics) TelemetryStore(key string, value uint64) {
	if m == nil {
		return
	}
	if counter := m.telemetryCounter(key); counter != nil {
		counter.Store(value)
	}
}

type sinkEntry struct {
	name string
	sink Sink
	ch   chan Event
	wg   sync.WaitGroup
}

// Router coordinates fan-out from publishers to configured sinks.
type Router struct {
	cfg       Config
	clock     Clock
	fallback  *log.Logger
	queue     chan Event
	sinks     []*sinkEntry
	wg        sync.WaitGroup
	shutdown  chan struct{}
	metrics   Metrics
	onceStop  sync.Once
	sinksStop sync.Once
}

// NewRouter constructs a Router using the provided sinks. Sinks not listed in the
// configuration are closed immediately and counted as disabled.
func NewRouter(cfg Config, clock Clock, fallback *log.Logger, available map[string]Sink) (*Router, error) {
	if cfg.BufferSize <= 0 {
		return nil, errors.New("logging: buffer size must be positive")
	}
	if fallback == nil {
		fallback = log.Default()
	}
	if clock == nil {
		clock = SystemClock{}
	}

	r := &Router{
		cfg:      cfg,
		clock:    clock,
		fallback: fallback,
		queue:    make(chan Event, cfg.BufferSize),
		shutdown: make(chan struct{}),
	}

	seen := make(map[string]struct{}, len(cfg.EnabledSinks))
	for _, name := range cfg.EnabledSinks {
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		sink, ok := available[name]
		if !ok {
			r.metrics.sinkDisabledTotal.Add(1)
			fallback.Printf("logging: sink %q unavailable", name)
			continue
		}
		entry := &sinkEntry{name: name, sink: sink, ch: make(chan Event, cfg.BufferSize)}
		entry.wg.Add(1)
		go func(e *sinkEntry) {
			defer e.wg.Done()
			for event := range e.ch {
				if err := e.sink.Write(event); err != nil {
					fallback.Printf("logging: sink %s write failed: %v", e.name, err)
				}
			}
		}(entry)
		r.sinks = append(r.sinks, entry)
	}

	r.wg.Add(1)
	go r.dispatch()

	return r, nil
}

func (r *Router) dispatch() {
	defer r.wg.Done()
	for {
		select {
		case <-r.shutdown:
			r.drainQueue()
			r.stopSinks()
			return
		case event, ok := <-r.queue:
			if !ok {
				r.stopSinks()
				return
			}
			r.forward(event)
		}
	}
}

func (r *Router) drainQueue() {
	for {
		select {
		case event, ok := <-r.queue:
			if !ok {
				return
			}
			r.forward(event)
		default:
			return
		}
	}
}

func (r *Router) stopSinks() {
	r.sinksStop.Do(func() {
		for _, sink := range r.sinks {
			close(sink.ch)
		}
		for _, sink := range r.sinks {
			sink.wg.Wait()
		}
	})
}

func (r *Router) forward(event Event) {
	for _, sink := range r.sinks {
		select {
		case sink.ch <- event:
		default:
			r.metrics.eventsDroppedTotal.Add(1)
			r.fallback.Printf("logging: sink %s dropping event %s (buffer full)", sink.name, event.Type)
		}
	}
}

// Publish implements Publisher.
func (r *Router) Publish(ctx context.Context, event Event) {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return
		default:
		}
	}

	if event.Severity < r.cfg.MinSeverity {
		return
	}
	if len(r.cfg.Categories) > 0 {
		allowed := false
		for _, cat := range r.cfg.Categories {
			if cat == event.Category {
				allowed = true
				break
			}
		}
		if !allowed {
			return
		}
	}

	if event.Time.IsZero() {
		event.Time = r.clock.Now()
	}
	if event.Extra != nil && len(r.cfg.Metadata) > 0 {
		for k, v := range r.cfg.Metadata {
			if _, exists := event.Extra[k]; !exists {
				event.Extra[k] = v
			}
		}
	} else if len(r.cfg.Metadata) > 0 {
		event.Extra = make(map[string]any, len(r.cfg.Metadata))
		for k, v := range r.cfg.Metadata {
			event.Extra[k] = v
		}
	}

	select {
	case r.queue <- event:
		r.metrics.eventsTotal.Add(1)
	default:
		r.metrics.eventsDroppedTotal.Add(1)
		r.fallback.Printf("logging: dropping event %s (router buffer full)", event.Type)
	}
}

// Close signals the router to flush outstanding events and stop all sinks.
func (r *Router) Close(ctx context.Context) error {
	var err error
	r.onceStop.Do(func() {
		close(r.shutdown)
		close(r.queue)
		r.wg.Wait()
		for _, sink := range r.sinks {
			if cerr := sink.sink.Close(ctx); cerr != nil {
				err = errors.Join(err, fmt.Errorf("sink %s: %w", sink.name, cerr))
				r.metrics.sinkErrorsTotal.Add(1)
			}
		}
	})
	return err
}

// MetricsSnapshot exposes a copy of the router counters.
func (r *Router) MetricsSnapshot() map[string]uint64 {
	return r.metrics.Snapshot()
}

// Metrics exposes the router counters for dependency injection.
func (r *Router) Metrics() *Metrics {
	if r == nil {
		return nil
	}
	return &r.metrics
}

// Clock exposes the time source used by the router.
func (r *Router) Clock() Clock {
	if r == nil {
		return nil
	}
	return r.clock
}
