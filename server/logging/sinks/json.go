package sinks

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"time"

	"mine-and-die/server/logging"
)

type JSONSink struct {
	mu       sync.Mutex
	writer   *bufio.Writer
	file     *os.File
	cfg      logging.JSONConfig
	buffer   []logging.Event
	ticker   *time.Ticker
	shutdown chan struct{}
}

func NewJSONSink(cfg logging.JSONConfig) (*JSONSink, error) {
	if cfg.FilePath == "" {
		cfg.FilePath = "events.jsonl"
	}
	file, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	maxBatch := cfg.MaxBatch
	if maxBatch <= 0 {
		maxBatch = 32
	}
	flushInterval := cfg.FlushInterval
	if flushInterval <= 0 {
		flushInterval = 2 * time.Second
	}
	sink := &JSONSink{
		writer:   bufio.NewWriter(file),
		file:     file,
		cfg:      cfg,
		buffer:   make([]logging.Event, 0, maxBatch),
		ticker:   time.NewTicker(flushInterval),
		shutdown: make(chan struct{}),
	}
	go sink.loop()
	return sink, nil
}

func (s *JSONSink) loop() {
	for {
		select {
		case <-s.ticker.C:
			s.Flush()
		case <-s.shutdown:
			return
		}
	}
}

func (s *JSONSink) Write(event logging.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.buffer = append(s.buffer, cloneForJSON(event))
	if len(s.buffer) >= cap(s.buffer) {
		return s.flushLocked()
	}
	return nil
}

func (s *JSONSink) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flushLocked()
}

func (s *JSONSink) flushLocked() error {
	if len(s.buffer) == 0 {
		return nil
	}
	encoder := json.NewEncoder(s.writer)
	encoder.SetEscapeHTML(false)
	for _, event := range s.buffer {
		if err := encoder.Encode(event); err != nil {
			return err
		}
	}
	s.buffer = s.buffer[:0]
	return s.writer.Flush()
}

func (s *JSONSink) Close(ctx context.Context) error {
	close(s.shutdown)
	s.ticker.Stop()
	flushErr := s.Flush()
	s.mu.Lock()
	defer s.mu.Unlock()
	var closeErr error
	if s.file != nil {
		cErr := s.file.Close()
		if cErr != nil {
			closeErr = cErr
		}
	}
	if flushErr != nil {
		if closeErr != nil {
			return errors.Join(flushErr, closeErr)
		}
		return flushErr
	}
	return closeErr
}

func cloneForJSON(event logging.Event) logging.Event {
	cloned := event
	if len(event.Targets) > 0 {
		cloned.Targets = append([]logging.EntityRef(nil), event.Targets...)
	}
	if event.Extra != nil {
		copied := make(map[string]any, len(event.Extra))
		for k, v := range event.Extra {
			copied[k] = v
		}
		cloned.Extra = copied
	}
	return cloned
}
