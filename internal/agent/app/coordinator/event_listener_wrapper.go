package coordinator

import (
	"strings"
	"sync"
	"time"

	"alex/internal/agent/domain"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/agent/types"
)

const (
	defaultEventQueueSize  = 256
	defaultEventIdlePeriod = 10 * time.Minute
)

// SerializingEventListener ensures per-run event ordering and thread safety.
type SerializingEventListener struct {
	next        agent.EventListener
	mu          sync.Mutex
	queues      map[string]*eventQueue
	idleTimeout time.Duration
}

type eventQueue struct {
	ch   chan agent.AgentEvent
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
	queue.ch <- event
}

func (s *SerializingEventListener) getQueue(runID string) *eventQueue {
	s.mu.Lock()
	defer s.mu.Unlock()
	if queue, ok := s.queues[runID]; ok {
		return queue
	}
	queue := &eventQueue{ch: make(chan agent.AgentEvent, defaultEventQueueSize)}
	s.queues[runID] = queue
	go s.runQueue(runID, queue)
	return queue
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
		close(q.ch)
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
		if final, ok := event.(*domain.WorkflowResultFinalEvent); ok {
			return final.StreamFinished
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
