package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	agentports "alex/internal/agent/ports"
	"alex/internal/logging"
)

type batchEventAppender interface {
	AppendBatch(ctx context.Context, events []agentports.AgentEvent) error
}

// AsyncEventHistoryStore wraps an EventHistoryStore and pushes Append operations
// onto a background flusher so agent streaming paths do not block on I/O.
//
// Stream/Delete/HasSessionEvents wait for in-flight writes so replay endpoints
// have a consistent view.
type AsyncEventHistoryStore struct {
	inner EventHistoryStore

	ch            chan agentports.AgentEvent
	flushRequests chan chan error
	closeOnce     sync.Once
	done          chan struct{}

	batchSize     int
	flushInterval time.Duration
	appendTimeout time.Duration
	logger        logging.Logger
}

type AsyncEventHistoryStoreOption func(*AsyncEventHistoryStore)

func WithAsyncHistoryBatchSize(size int) AsyncEventHistoryStoreOption {
	return func(s *AsyncEventHistoryStore) {
		if size > 0 {
			s.batchSize = size
		}
	}
}

func WithAsyncHistoryFlushInterval(interval time.Duration) AsyncEventHistoryStoreOption {
	return func(s *AsyncEventHistoryStore) {
		if interval > 0 {
			s.flushInterval = interval
		}
	}
}

func WithAsyncHistoryAppendTimeout(timeout time.Duration) AsyncEventHistoryStoreOption {
	return func(s *AsyncEventHistoryStore) {
		if timeout > 0 {
			s.appendTimeout = timeout
		}
	}
}

var ErrAsyncHistoryQueueFull = errors.New("async event history queue full")

func NewAsyncEventHistoryStore(inner EventHistoryStore, opts ...AsyncEventHistoryStoreOption) *AsyncEventHistoryStore {
	store := &AsyncEventHistoryStore{
		inner:         inner,
		ch:            make(chan agentports.AgentEvent, 8192),
		flushRequests: make(chan chan error, 16),
		done:          make(chan struct{}),
		batchSize:     200,
		flushInterval: 250 * time.Millisecond,
		appendTimeout: 50 * time.Millisecond,
		logger:        logging.NewComponentLogger("AsyncEventHistoryStore"),
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(store)
	}
	if inner != nil {
		go store.run()
	}
	return store
}

func (s *AsyncEventHistoryStore) Append(ctx context.Context, event agentports.AgentEvent) error {
	if s == nil || s.inner == nil || event == nil {
		return nil
	}

	select {
	case s.ch <- event:
		return nil
	default:
		// Avoid blocking latency-sensitive streaming paths. If the queue is full,
		// fall back to a short context-aware wait and surface backpressure.
		timeout := s.appendTimeout
		if timeout <= 0 {
			timeout = 50 * time.Millisecond
		}
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case s.ch <- event:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return ErrAsyncHistoryQueueFull
		}
	}
}

func (s *AsyncEventHistoryStore) Stream(ctx context.Context, filter EventHistoryFilter, fn func(agentports.AgentEvent) error) error {
	if s == nil || s.inner == nil {
		return fmt.Errorf("history store not initialized")
	}
	if err := s.flush(ctx); err != nil {
		return err
	}
	return s.inner.Stream(ctx, filter, fn)
}

func (s *AsyncEventHistoryStore) DeleteSession(ctx context.Context, sessionID string) error {
	if s == nil || s.inner == nil {
		return fmt.Errorf("history store not initialized")
	}
	if err := s.flush(ctx); err != nil {
		return err
	}
	return s.inner.DeleteSession(ctx, sessionID)
}

func (s *AsyncEventHistoryStore) HasSessionEvents(ctx context.Context, sessionID string) (bool, error) {
	if s == nil || s.inner == nil {
		return false, fmt.Errorf("history store not initialized")
	}
	if err := s.flush(ctx); err != nil {
		return false, err
	}
	return s.inner.HasSessionEvents(ctx, sessionID)
}

func (s *AsyncEventHistoryStore) Close() error {
	if s == nil {
		return nil
	}
	var err error
	s.closeOnce.Do(func() {
		err = s.flush(context.Background())
		close(s.done)
	})
	return err
}

func (s *AsyncEventHistoryStore) flush(ctx context.Context) error {
	if s == nil || s.inner == nil {
		return nil
	}

	resp := make(chan error, 1)
	select {
	case s.flushRequests <- resp:
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err := <-resp:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *AsyncEventHistoryStore) run() {
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()

	buffer := make([]agentports.AgentEvent, 0, s.batchSize)

	flushBuffer := func() error {
		if len(buffer) == 0 {
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var err error
		if batcher, ok := s.inner.(batchEventAppender); ok {
			err = batcher.AppendBatch(ctx, buffer)
		} else {
			for _, event := range buffer {
				if appendErr := s.inner.Append(ctx, event); appendErr != nil {
					err = appendErr
					break
				}
			}
		}
		buffer = buffer[:0]
		return err
	}

	for {
		select {
		case <-s.done:
			_ = flushBuffer()
			return
		case event := <-s.ch:
			if event == nil {
				continue
			}
			buffer = append(buffer, event)
			if len(buffer) >= s.batchSize {
				if err := flushBuffer(); err != nil {
					logging.OrNop(s.logger).Warn("Failed to flush event history batch: %v", err)
				}
			}
		case resp := <-s.flushRequests:
			// Drain any queued events before fulfilling the flush request.
			for {
				select {
				case event := <-s.ch:
					if event != nil {
						buffer = append(buffer, event)
					}
					if len(buffer) >= s.batchSize {
						if err := flushBuffer(); err != nil {
							logging.OrNop(s.logger).Warn("Failed to flush event history batch: %v", err)
						}
					}
				default:
					goto drained
				}
			}
		drained:
			resp <- flushBuffer()
		case <-ticker.C:
			if err := flushBuffer(); err != nil {
				logging.OrNop(s.logger).Warn("Failed to flush event history batch: %v", err)
			}
		}
	}
}
