package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/async"
	"alex/internal/shared/logging"
)

type batchEventAppender interface {
	AppendBatch(ctx context.Context, events []agent.AgentEvent) error
}

// AsyncEventHistoryStore wraps an EventHistoryStore and pushes Append operations
// onto a background flusher so agent streaming paths do not block on I/O.
//
// Stream/Delete/HasSessionEvents wait for in-flight writes so replay endpoints
// have a consistent view.
type AsyncEventHistoryStore struct {
	inner EventHistoryStore

	ch            chan agent.AgentEvent
	flushRequests chan chan error
	closeOnce     sync.Once
	done          chan struct{}

	batchSize     int
	flushInterval time.Duration
	appendTimeout time.Duration
	queueCapacity int
	logger        logging.Logger

	enqueuedEvents  atomic.Int64
	queueFullEvents atomic.Int64
	flushBatches    atomic.Int64
	flushFailures   atomic.Int64
	flushedEvents   atomic.Int64
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

func WithAsyncHistoryQueueCapacity(capacity int) AsyncEventHistoryStoreOption {
	return func(s *AsyncEventHistoryStore) {
		if capacity > 0 {
			s.queueCapacity = capacity
		}
	}
}

var ErrAsyncHistoryQueueFull = errors.New("async event history queue full")

const (
	DefaultAsyncHistoryBatchSize     = 200
	DefaultAsyncHistoryFlushInterval = 250 * time.Millisecond
	DefaultAsyncHistoryAppendTimeout = 50 * time.Millisecond
	DefaultAsyncHistoryQueueCapacity = 8192
)

func NewAsyncEventHistoryStore(inner EventHistoryStore, opts ...AsyncEventHistoryStoreOption) *AsyncEventHistoryStore {
	store := &AsyncEventHistoryStore{
		inner:         inner,
		batchSize:     DefaultAsyncHistoryBatchSize,
		flushInterval: DefaultAsyncHistoryFlushInterval,
		appendTimeout: DefaultAsyncHistoryAppendTimeout,
		queueCapacity: DefaultAsyncHistoryQueueCapacity,
		logger:        logging.NewComponentLogger("AsyncEventHistoryStore"),
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(store)
	}
	store.ch = make(chan agent.AgentEvent, store.queueCapacity)
	store.flushRequests = make(chan chan error, 16)
	store.done = make(chan struct{})
	if inner != nil {
		async.Go(store.logger, "eventHistory.asyncStore", func() {
			store.run()
		})
	}
	return store
}

func (s *AsyncEventHistoryStore) Append(ctx context.Context, event agent.AgentEvent) error {
	if s == nil || s.inner == nil || event == nil {
		return nil
	}

	select {
	case s.ch <- event:
		s.enqueuedEvents.Add(1)
		return nil
	default:
		// Avoid blocking latency-sensitive streaming paths. If the queue is full,
		// fall back to a short context-aware wait and surface backpressure.
		timeout := s.appendTimeout
		if timeout <= 0 {
			timeout = DefaultAsyncHistoryAppendTimeout
		}
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case s.ch <- event:
			s.enqueuedEvents.Add(1)
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			count := s.queueFullEvents.Add(1)
			if shouldSampleCounter(count) {
				logging.OrNop(s.logger).Warn("Async event history queue full (count=%d depth=%d cap=%d)", count, len(s.ch), cap(s.ch))
			}
			return ErrAsyncHistoryQueueFull
		}
	}
}

func (s *AsyncEventHistoryStore) Stream(ctx context.Context, filter EventHistoryFilter, fn func(agent.AgentEvent) error) error {
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

	buffer := make([]agent.AgentEvent, 0, s.batchSize)
	minBackoff := 250 * time.Millisecond
	maxBackoff := 5 * time.Second
	baseBackoff := s.flushInterval
	if baseBackoff <= 0 {
		baseBackoff = minBackoff
	}
	if baseBackoff < minBackoff {
		baseBackoff = minBackoff
	}
	var failureCount int
	var nextFlush time.Time

	flushBuffer := func() error {
		if len(buffer) == 0 {
			return nil
		}
		batchLen := len(buffer)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if batcher, ok := s.inner.(batchEventAppender); ok {
			if err := batcher.AppendBatch(ctx, buffer); err != nil {
				return err
			}
			buffer = buffer[:0]
			s.flushBatches.Add(1)
			s.flushedEvents.Add(int64(batchLen))
			return nil
		}

		for i, event := range buffer {
			if appendErr := s.inner.Append(ctx, event); appendErr != nil {
				buffer = buffer[i:]
				return appendErr
			}
		}
		buffer = buffer[:0]
		s.flushBatches.Add(1)
		s.flushedEvents.Add(int64(batchLen))
		return nil
	}

	applyBackoff := func() {
		failureCount++
		backoff := baseBackoff
		for i := 1; i < failureCount && backoff < maxBackoff; i++ {
			backoff *= 2
			if backoff >= maxBackoff {
				backoff = maxBackoff
				break
			}
		}
		nextFlush = time.Now().Add(backoff)
	}

	resetBackoff := func() {
		failureCount = 0
		nextFlush = time.Time{}
	}

	tryFlush := func(force bool) error {
		if !force && !nextFlush.IsZero() && time.Now().Before(nextFlush) {
			return nil
		}
		if err := flushBuffer(); err != nil {
			applyBackoff()
			count := s.flushFailures.Add(1)
			if shouldSampleCounter(count) {
				logging.OrNop(s.logger).Warn("Failed to flush async event history batch (count=%d depth=%d pending=%d): %v", count, len(s.ch), len(buffer), err)
			}
			return err
		}
		if failureCount > 0 {
			resetBackoff()
		}
		return nil
	}

	for {
		select {
		case <-s.done:
			if err := flushBuffer(); err != nil {
				count := s.flushFailures.Add(1)
				if shouldSampleCounter(count) {
					logging.OrNop(s.logger).Warn("Failed to flush async event history on shutdown (count=%d depth=%d pending=%d): %v", count, len(s.ch), len(buffer), err)
				}
			}
			return
		case event := <-s.ch:
			if event == nil {
				continue
			}
			buffer = append(buffer, event)
			if len(buffer) >= s.batchSize {
				_ = tryFlush(false)
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
						_ = tryFlush(false)
					}
				default:
					goto drained
				}
			}
		drained:
			resp <- tryFlush(true)
		case <-ticker.C:
			_ = tryFlush(false)
		}
	}
}

type AsyncEventHistoryStats struct {
	QueueDepth      int   `json:"queue_depth"`
	QueueCapacity   int   `json:"queue_capacity"`
	EnqueuedEvents  int64 `json:"enqueued_events"`
	QueueFullEvents int64 `json:"queue_full_events"`
	FlushBatches    int64 `json:"flush_batches"`
	FlushFailures   int64 `json:"flush_failures"`
	FlushedEvents   int64 `json:"flushed_events"`
}

func (s *AsyncEventHistoryStore) Stats() AsyncEventHistoryStats {
	if s == nil {
		return AsyncEventHistoryStats{}
	}
	depth := 0
	capacity := 0
	if s.ch != nil {
		depth = len(s.ch)
		capacity = cap(s.ch)
	}
	return AsyncEventHistoryStats{
		QueueDepth:      depth,
		QueueCapacity:   capacity,
		EnqueuedEvents:  s.enqueuedEvents.Load(),
		QueueFullEvents: s.queueFullEvents.Load(),
		FlushBatches:    s.flushBatches.Load(),
		FlushFailures:   s.flushFailures.Load(),
		FlushedEvents:   s.flushedEvents.Load(),
	}
}
