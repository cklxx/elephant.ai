package coordinator

import (
	"context"
	"sync"
	"testing"
	"time"

	"alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
)

type serialRecordingListener struct {
	mu     sync.Mutex
	events []agent.AgentEvent
	wg     sync.WaitGroup
}

func (l *serialRecordingListener) OnEvent(event agent.AgentEvent) {
	l.mu.Lock()
	l.events = append(l.events, event)
	l.mu.Unlock()
	l.wg.Done()
}

func (l *serialRecordingListener) collected() []agent.AgentEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]agent.AgentEvent(nil), l.events...)
}

type stubEvent struct {
	runID     string
	eventType string
}

func (e stubEvent) EventType() string               { return e.eventType }
func (e stubEvent) Timestamp() time.Time            { return time.Now() }
func (e stubEvent) GetAgentLevel() agent.AgentLevel { return agent.LevelCore }
func (e stubEvent) GetSessionID() string            { return "s1" }
func (e stubEvent) GetRunID() string                { return e.runID }
func (e stubEvent) GetParentRunID() string          { return "" }
func (e stubEvent) GetCorrelationID() string        { return "" }
func (e stubEvent) GetCausationID() string          { return "" }
func (e stubEvent) GetEventID() string              { return "" }
func (e stubEvent) GetSeq() uint64                  { return 0 }

type blockingEventListener struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (l *blockingEventListener) OnEvent(event agent.AgentEvent) {
	l.once.Do(func() { close(l.started) })
	<-l.release
}

func TestSerializingEventListener_PerRunOrdering(t *testing.T) {
	listener := &serialRecordingListener{}
	listener.wg.Add(2)
	wrapper := NewSerializingEventListener(listener)

	wrapper.OnEvent(stubEvent{runID: "r1", eventType: "a"})
	wrapper.OnEvent(stubEvent{runID: "r1", eventType: "b"})

	waitForEvents(t, &listener.wg)
	events := listener.collected()
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].EventType() != "a" || events[1].EventType() != "b" {
		t.Fatalf("expected order a then b, got %s then %s", events[0].EventType(), events[1].EventType())
	}
}

func TestSerializingEventListener_TerminalEventClosesQueue(t *testing.T) {
	listener := &serialRecordingListener{}
	listener.wg.Add(1)
	wrapper := NewSerializingEventListener(listener)

	wrapper.OnEvent(newFinalEvent("r2", true))

	waitForEvents(t, &listener.wg)
	events := listener.collected()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

func TestSerializingEventListener_EnvelopeTerminalEvent(t *testing.T) {
	listener := &serialRecordingListener{}
	listener.wg.Add(1)
	wrapper := NewSerializingEventListener(listener)

	envelope := domain.NewWorkflowEnvelopeFromEvent(newFinalEvent("r3", true), types.EventResultFinal)
	envelope.Payload = map[string]any{"stream_finished": true}
	wrapper.OnEvent(envelope)

	waitForEvents(t, &listener.wg)
	events := listener.collected()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

func TestSerializingEventListener_FlushWaitsForInFlightEvents(t *testing.T) {
	bl := &blockingEventListener{
		started: make(chan struct{}),
		release: make(chan struct{}),
	}

	wrapper := NewSerializingEventListener(bl)
	wrapper.OnEvent(stubEvent{runID: "r-flush", eventType: "a"})

	select {
	case <-bl.started:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event to start processing")
	}

	flushed := make(chan struct{})
	go func() {
		defer close(flushed)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		wrapper.Flush(ctx, "r-flush")
	}()

	select {
	case <-flushed:
		t.Fatal("flush returned while event processing was blocked")
	case <-time.After(50 * time.Millisecond):
	}

	close(bl.release)

	select {
	case <-flushed:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for flush to return")
	}
}

func newFinalEvent(runID string, finished bool) *domain.Event {
	base := domain.NewBaseEvent(agent.LevelCore, "s1", runID, "", time.Now())
	base.SetSeq(1)
	return domain.NewResultFinalEvent(base, "", 0, 0, "", 0, false, finished, nil)
}

func waitForEvents(t *testing.T, wg *sync.WaitGroup) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for events")
	}
}
