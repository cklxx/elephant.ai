package coordinator

import (
	"context"
	"strings"
	"sync"
	"time"

	"alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
)

const (
	defaultEventQueueSize  = 256
	defaultEventIdlePeriod = 10 * time.Minute
)

type flushBarrierEvent struct {
	runID string
	done  chan struct{}
}

func (e *flushBarrierEvent) EventType() string               { return "serializing.flush" }
func (e *flushBarrierEvent) Timestamp() time.Time            { return time.Now() }
func (e *flushBarrierEvent) GetAgentLevel() agent.AgentLevel { return agent.LevelCore }
func (e *flushBarrierEvent) GetSessionID() string            { return "" }
func (e *flushBarrierEvent) GetRunID() string                { return e.runID }
func (e *flushBarrierEvent) GetParentRunID() string          { return "" }
func (e *flushBarrierEvent) GetCorrelationID() string        { return "" }
func (e *flushBarrierEvent) GetCausationID() string          { return "" }
func (e *flushBarrierEvent) GetEventID() string              { return "" }
func (e *flushBarrierEvent) GetSeq() uint64                  { return 0 }

// SerializingEventListener ensures per-run event ordering and thread safety.
type SerializingEventListener struct {
	next        agent.EventListener
	mu          sync.Mutex
	queues      map[string]*eventQueue
	idleTimeout time.Duration
}

type eventQueue struct {
	ch   chan agent.AgentEvent
	done chan struct{}
	once sync.Once
}

// NewSerializingEventListener wraps a listener with per-run serialization.
func NewSerializingEventListener(next agent.EventListener) *SerializingEventListener {
	if next == nil {
		return nil
	}
	return &SerializingEventListener{
		next:        next,
		queues:      make(map[string]*eventQueue),
		idleTimeout: defaultEventIdlePeriod,
	}
}

// OnEvent enqueues events to be delivered in order per run.
func (s *SerializingEventListener) OnEvent(event agent.AgentEvent) {
	if s == nil || s.next == nil || event == nil {
		return
	}
	runID := strings.TrimSpace(event.GetRunID())
	if runID == "" {
		runID = "unknown"
	}
	queue := s.getQueue(runID)
	select {
	case <-queue.done:
		return
	default:
	}
	select {
	case queue.ch <- event:
	case <-queue.done:
	}
}

// Flush waits until all events queued before the flush barrier have been
// delivered to the wrapped listener for the given runID.
func (s *SerializingEventListener) Flush(ctx context.Context, runID string) {
	if s == nil || s.next == nil {
		return
	}

	runID = strings.TrimSpace(runID)
	if runID == "" {
		runID = "unknown"
	}

	queue := s.getQueueIfExists(runID)
	if queue == nil {
		return
	}

	barrier := &flushBarrierEvent{runID: runID, done: make(chan struct{})}
	select {
	case <-queue.done:
		return
	default:
	}

	select {
	case queue.ch <- barrier:
	case <-queue.done:
		return
	case <-ctx.Done():
		return
	}

	select {
	case <-barrier.done:
	case <-queue.done:
	case <-ctx.Done():
	}
}

func (s *SerializingEventListener) getQueue(runID string) *eventQueue {
	s.mu.Lock()
	defer s.mu.Unlock()
	if queue, ok := s.queues[runID]; ok {
		return queue
	}
	queue := &eventQueue{ch: make(chan agent.AgentEvent, defaultEventQueueSize)}
	queue.done = make(chan struct{})
	s.queues[runID] = queue
	go s.runQueue(runID, queue)
	return queue
}

func (s *SerializingEventListener) getQueueIfExists(runID string) *eventQueue {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.queues[runID]
}

func (s *SerializingEventListener) runQueue(runID string, queue *eventQueue) {
	timer := time.NewTimer(s.idleTimeout)
	defer timer.Stop()
	for {
		select {
		case event, ok := <-queue.ch:
			if !ok {
				return
			}
			if barrier, ok := event.(*flushBarrierEvent); ok && barrier != nil {
				close(barrier.done)
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(s.idleTimeout)
				continue
			}
			s.next.OnEvent(event)
			if isTerminalEvent(event) {
				s.removeQueue(runID, queue)
				return
			}
			if !timer.Stop() {
				<-timer.C
			}
			timer.Reset(s.idleTimeout)
		case <-timer.C:
			s.removeQueue(runID, queue)
			return
		}
	}
}

func (s *SerializingEventListener) removeQueue(runID string, queue *eventQueue) {
	s.mu.Lock()
	if current, ok := s.queues[runID]; ok && current == queue {
		delete(s.queues, runID)
	}
	s.mu.Unlock()
	queue.close()
}

func (q *eventQueue) close() {
	if q == nil {
		return
	}
	q.once.Do(func() {
		close(q.done)
	})
}

func isTerminalEvent(event agent.AgentEvent) bool {
	if event == nil {
		return false
	}
	switch event.EventType() {
	case types.EventResultCancelled:
		return true
	case types.EventResultFinal:
		if e, ok := event.(*domain.Event); ok {
			return e.Data.StreamFinished
		}
		if envelope, ok := event.(*domain.WorkflowEventEnvelope); ok {
			return envelopeStreamFinished(envelope)
		}
	}
	return false
}

func envelopeStreamFinished(envelope *domain.WorkflowEventEnvelope) bool {
	if envelope == nil || len(envelope.Payload) == 0 {
		return false
	}
	raw, ok := envelope.Payload["stream_finished"]
	if !ok {
		return false
	}
	finished, ok := raw.(bool)
	return ok && finished
}
