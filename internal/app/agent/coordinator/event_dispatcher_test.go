package coordinator

import (
	"context"
	"sync"
	"testing"
	"time"

	domain "alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	toolspolicy "alex/internal/infra/tools"

	"github.com/prometheus/client_golang/prometheus"
)

type recordingEventListener struct {
	mu     sync.Mutex
	events []agent.AgentEvent
}

func (l *recordingEventListener) OnEvent(event agent.AgentEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

func (l *recordingEventListener) snapshot() []agent.AgentEvent {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]agent.AgentEvent, len(l.events))
	copy(out, l.events)
	return out
}

func TestNewEventDispatcher_NilListener(t *testing.T) {
	dispatcher := NewEventDispatcher(nil, nil, EventDispatcherOptions{})
	if dispatcher == nil {
		t.Fatalf("dispatcher = nil")
	}
	if dispatcher.Listener() != nil {
		t.Fatalf("expected nil listener when base listener is nil")
	}
	if got := dispatcher.Title(); got != "" {
		t.Fatalf("Title() = %q, want empty", got)
	}
	dispatcher.Flush(context.Background(), "")
}

func TestEventDispatcher_TranslatesAndEnrichesToolCompleted(t *testing.T) {
	collector, err := toolspolicy.NewSLACollector(prometheus.NewRegistry())
	if err != nil {
		t.Fatalf("NewSLACollector() error = %v", err)
	}
	collector.RecordExecution("read_file", 10*time.Millisecond, nil)

	sink := &recordingEventListener{}
	dispatcher := NewEventDispatcher(sink, collector, EventDispatcherOptions{})

	base := domain.NewBaseEvent(agent.LevelCore, "session-1", "run-1", "", time.Now())
	evt := domain.NewToolCompletedEvent(base, "call-1", "read_file", "ok", nil, 10*time.Millisecond, nil, nil)
	dispatcher.Listener().OnEvent(evt)
	dispatcher.Flush(context.Background(), "run-1")

	events := sink.snapshot()
	if len(events) != 1 {
		t.Fatalf("received %d events, want 1", len(events))
	}
	envelope, ok := events[0].(*domain.WorkflowEventEnvelope)
	if !ok {
		t.Fatalf("event type = %T, want *domain.WorkflowEventEnvelope", events[0])
	}
	if envelope.Event != types.EventToolCompleted {
		t.Fatalf("envelope.Event = %q, want %q", envelope.Event, types.EventToolCompleted)
	}
	rawSLA, ok := envelope.Payload["tool_sla"]
	if !ok {
		t.Fatalf("tool_sla payload missing")
	}
	sla, ok := rawSLA.(map[string]any)
	if !ok {
		t.Fatalf("tool_sla type = %T, want map[string]any", rawSLA)
	}
	if got, _ := sla["tool_name"].(string); got != "read_file" {
		t.Fatalf("tool_sla.tool_name = %q, want %q", got, "read_file")
	}
}

func TestEventDispatcher_PlanTitleRecorder(t *testing.T) {
	sink := &recordingEventListener{}
	var callbacks []string
	var mu sync.Mutex

	dispatcher := NewEventDispatcher(sink, nil, EventDispatcherOptions{
		EnablePlanTitle: true,
		OnPlanTitle: func(title string) {
			mu.Lock()
			defer mu.Unlock()
			callbacks = append(callbacks, title)
		},
	})

	base := domain.NewBaseEvent(agent.LevelCore, "session-1", "run-plan", "", time.Now())
	first := domain.NewToolCompletedEvent(base, "call-1", "plan", "ok", nil, 0, map[string]any{
		"session_title": "Plan Alpha",
	}, nil)
	second := domain.NewToolCompletedEvent(base, "call-2", "plan", "ok", nil, 0, map[string]any{
		"session_title": "Plan Beta",
	}, nil)

	dispatcher.Listener().OnEvent(first)
	dispatcher.Listener().OnEvent(second)
	dispatcher.Flush(context.Background(), "run-plan")

	if got := dispatcher.Title(); got != "Plan Alpha" {
		t.Fatalf("Title() = %q, want %q", got, "Plan Alpha")
	}
	mu.Lock()
	defer mu.Unlock()
	if len(callbacks) != 1 {
		t.Fatalf("callback count = %d, want 1", len(callbacks))
	}
	if callbacks[0] != "Plan Alpha" {
		t.Fatalf("first callback title = %q, want %q", callbacks[0], "Plan Alpha")
	}
}
