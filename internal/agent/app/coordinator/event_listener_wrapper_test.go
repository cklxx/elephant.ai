package coordinator

import (
	"sync"
	"testing"
	"time"

	"alex/internal/agent/domain"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/agent/types"
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

func newFinalEvent(runID string, finished bool) *domain.WorkflowResultFinalEvent {
	base := domain.NewBaseEvent(agent.LevelCore, "s1", runID, "", time.Now())
	base.SetSeq(1)
	return &domain.WorkflowResultFinalEvent{
		BaseEvent:      base,
		StreamFinished: finished,
	}
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
