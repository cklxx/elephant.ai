package domain

import (
	"testing"
	"time"

	"alex/internal/agent/ports"
)

func TestBaseEventAccessors(t *testing.T) {
	ts := time.Now()
	level := ports.LevelSubagent
	sessionID := "session-123"

	taskID := "task-123"
	parentTaskID := ""
	event := NewTaskAnalysisEvent(level, sessionID, taskID, parentTaskID, "analyze", "goal", ts)
	if got := event.Timestamp(); !got.Equal(ts) {
		t.Fatalf("expected timestamp %v, got %v", ts, got)
	}
	if event.GetAgentLevel() != level {
		t.Fatalf("expected level %v, got %v", level, event.GetAgentLevel())
	}
	if event.GetSessionID() != sessionID {
		t.Fatalf("expected sessionID %s, got %s", sessionID, event.GetSessionID())
	}
	if event.GetTaskID() != taskID {
		t.Fatalf("expected taskID %s, got %s", taskID, event.GetTaskID())
	}
	if event.GetParentTaskID() != parentTaskID {
		t.Fatalf("expected parentTaskID %s, got %s", parentTaskID, event.GetParentTaskID())
	}
}

func TestEventTypeImplementations(t *testing.T) {
	base := newBaseEventWithIDs(ports.LevelCore, "sess", "task", "parent", time.Now())

	cases := []struct {
		name string
		evt  AgentEvent
		want string
	}{
		{"task_analysis", &TaskAnalysisEvent{BaseEvent: base}, "task_analysis"},
		{"iteration_start", &IterationStartEvent{BaseEvent: base}, "iteration_start"},
		{"thinking", &ThinkingEvent{BaseEvent: base}, "thinking"},
		{"think_complete", &ThinkCompleteEvent{BaseEvent: base}, "think_complete"},
		{"assistant_message", &AssistantMessageEvent{BaseEvent: base}, "assistant_message"},
		{"tool_call_start", &ToolCallStartEvent{BaseEvent: base}, "tool_call_start"},
		{"tool_call_stream", &ToolCallStreamEvent{BaseEvent: base}, "tool_call_stream"},
		{"tool_call_complete", &ToolCallCompleteEvent{BaseEvent: base}, "tool_call_complete"},
		{"iteration_complete", &IterationCompleteEvent{BaseEvent: base}, "iteration_complete"},
		{"task_complete", &TaskCompleteEvent{BaseEvent: base}, "task_complete"},
		{"error", &ErrorEvent{BaseEvent: base}, "error"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.evt.EventType(); got != tc.want {
				t.Fatalf("expected event type %s, got %s", tc.want, got)
			}
		})
	}
}

func TestEventListenerFunc(t *testing.T) {
	var captured AgentEvent
	listener := EventListenerFunc(func(evt AgentEvent) {
		captured = evt
	})

	evt := &ThinkingEvent{BaseEvent: newBaseEventWithIDs(ports.LevelCore, "sess", "task", "parent", time.Now())}
	listener.OnEvent(evt)

	if captured != evt {
		t.Fatalf("expected listener to capture event %p, got %p", evt, captured)
	}
}
