package logging

import (
	"context"
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type Clock interface {
	Now() time.Time
}

type ClockFunc func() time.Time

func (f ClockFunc) Now() time.Time {
	return f()
}

type Sink interface {
	Write(Event) error
	Close(context.Context) error
}

type NamedSink struct {
	Name string
	Sink Sink
}

type Router struct {
	cfg          Config
	queue        chan Event
	sinks        []*sinkWorker
	clock        Clock
	fallback     *log.Logger
	ctx          context.Context
	cancel       context.CancelFunc
	closed       atomic.Bool
	minSeverity  Severity
	fields       map[string]any
	wg           sync.WaitGroup
	dispatchOnce sync.Once

	eventsTotal  atomic.Uint64
	droppedTotal atomic.Uint64
	lastDropLog  atomic.Int64
}

type RouterStats struct {
	EventsTotal  uint64
	DroppedTotal uint64
}

func NewRouter(clock Clock, cfg Config, namedSinks []NamedSink) (*Router, error) {
	if clock == nil {
		clock = ClockFunc(time.Now)
	}
	bufferSize := cfg.BufferSize
	if bufferSize <= 0 {
		bufferSize = 512
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &Router{
		cfg:         cfg,
		queue:       make(chan Event, bufferSize),
		clock:       clock,
		fallback:    log.New(os.Stderr, "[logging] ", log.LstdFlags),
		ctx:         ctx,
		cancel:      cancel,
		minSeverity: cfg.MinimumSeverity,
		fields:      cfg.CloneFields(),
	}

	sinkBuffer := bufferSize
	if sinkBuffer > 1024 {
		sinkBuffer = 1024
	}
	if sinkBuffer < 32 {
		sinkBuffer = 32
	}

	for _, named := range namedSinks {
		if named.Sink == nil {
			continue
		}
		worker := newSinkWorker(named.Name, named.Sink, sinkBuffer, r.fallback)
		r.sinks = append(r.sinks, worker)
	}

	r.start()
	return r, nil
}

func (r *Router) start() {
	r.dispatchOnce.Do(func() {
		r.wg.Add(1)
		go func() {
			defer func() {
				for _, worker := range r.sinks {
					close(worker.events)
				}
				r.wg.Done()
			}()
			for {
				select {
				case <-r.ctx.Done():
					r.drain()
					return
				case event := <-r.queue:
					r.forward(event)
				}
			}
		}()

		for _, worker := range r.sinks {
			r.wg.Add(1)
			go func(w *sinkWorker) {
				defer r.wg.Done()
				w.run()
			}(worker)
		}
	})
}

func (r *Router) drain() {
	for {
		select {
		case event := <-r.queue:
			r.forward(event)
		default:
			return
		}
	}
}

func (r *Router) forward(event Event) {
	if event.Severity < r.minSeverity {
		return
	}
	if event.Time.IsZero() {
		event.Time = r.clock.Now()
	}
	if len(r.fields) > 0 {
		event = cloneForFields(event)
		if event.Extra == nil {
			event.Extra = make(map[string]any, len(r.fields))
		}
		for k, v := range r.fields {
			if _, exists := event.Extra[k]; !exists {
				event.Extra[k] = v
			}
		}
	}
	r.eventsTotal.Add(1)
	for _, worker := range r.sinks {
		worker.enqueue(event)
	}
}

func (r *Router) Publish(ctx context.Context, event Event) {
	if event.Type == "" {
		return
	}
	if r.closed.Load() {
		return
	}
	select {
	case r.queue <- event:
	default:
		r.handleDrop(event)
	}
}

func (r *Router) handleDrop(event Event) {
	r.droppedTotal.Add(1)
	interval := r.cfg.DropWarnInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	now := time.Now().UnixNano()
	next := r.lastDropLog.Load()
	if next == 0 || now >= next {
		if r.lastDropLog.CompareAndSwap(next, now+interval.Nanoseconds()) {
			r.fallback.Printf("dropping event type=%s tick=%d", event.Type, event.Tick)
		}
	}
}

func (r *Router) Close(ctx context.Context) error {
	if !r.closed.CompareAndSwap(false, true) {
		<-ctx.Done()
		return ctx.Err()
	}
	r.cancel()
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	var firstErr error
	for _, worker := range r.sinks {
		if err := worker.sink.Close(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (r *Router) Stats() RouterStats {
	return RouterStats{
		EventsTotal:  r.eventsTotal.Load(),
		DroppedTotal: r.droppedTotal.Load(),
	}
}

func (r *Router) Sink(name string) Sink {
	for _, worker := range r.sinks {
		if worker.name == name {
			return worker.sink
		}
	}
	return nil
}

type sinkWorker struct {
	name      string
	sink      Sink
	events    chan Event
	fallback  *log.Logger
	failures  int
	nextRetry time.Time
}

func newSinkWorker(name string, sink Sink, buffer int, fallback *log.Logger) *sinkWorker {
	if buffer <= 0 {
		buffer = 32
	}
	return &sinkWorker{
		name:     name,
		sink:     sink,
		events:   make(chan Event, buffer),
		fallback: fallback,
	}
}

func (w *sinkWorker) enqueue(event Event) {
	cloned := cloneForFields(event)
	select {
	case w.events <- cloned:
	default:
		w.reportDrop(event)
	}
}

func (w *sinkWorker) run() {
	for event := range w.events {
		w.waitUntilReady()
		if err := w.sink.Write(event); err != nil {
			w.fail(err)
		} else {
			w.failures = 0
			w.nextRetry = time.Time{}
		}
	}
}

func (w *sinkWorker) waitUntilReady() {
	if w.failures == 0 {
		return
	}
	for {
		now := time.Now()
		if w.nextRetry.IsZero() || now.After(w.nextRetry) || now.Equal(w.nextRetry) {
			return
		}
		time.Sleep(time.Until(w.nextRetry))
	}
}

func (w *sinkWorker) fail(err error) {
	if err == nil {
		return
	}
	w.failures++
	delay := time.Duration(1<<min(w.failures, 5)) * time.Second
	w.nextRetry = time.Now().Add(delay)
	w.fallback.Printf("sink %s failed: %v (retry in %s)", w.name, err, delay)
}

func (w *sinkWorker) reportDrop(event Event) {
	w.fallback.Printf("sink %s backlog full dropping event type=%s", w.name, event.Type)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
