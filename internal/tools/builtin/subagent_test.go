package builtin

import (
	"testing"
	"time"

	"alex/internal/agent/ports"
)

type stubEvent struct{}

func (stubEvent) EventType() string               { return "tool_call_complete" }
func (stubEvent) Timestamp() time.Time            { return time.Unix(0, 0) }
func (stubEvent) GetAgentLevel() ports.AgentLevel { return ports.AgentLevel("subagent") }
func (stubEvent) GetSessionID() string            { return "session" }
func (stubEvent) GetTaskID() string               { return "task" }
func (stubEvent) GetParentTaskID() string         { return "parent" }

type stubCoreEvent struct{}

func (stubCoreEvent) EventType() string               { return "assistant_message" }
func (stubCoreEvent) Timestamp() time.Time            { return time.Unix(0, 0) }
func (stubCoreEvent) GetAgentLevel() ports.AgentLevel { return ports.LevelCore }
func (stubCoreEvent) GetSessionID() string            { return "session" }
func (stubCoreEvent) GetTaskID() string               { return "task" }
func (stubCoreEvent) GetParentTaskID() string         { return "parent" }

func TestSubtaskEventEventTypeMatchesOriginal(t *testing.T) {
	evt := &SubtaskEvent{OriginalEvent: stubEvent{}}

	if got := evt.EventType(); got != "tool_call_complete" {
		t.Fatalf("expected event type to match original event, got %q", got)
	}
}

func TestSubtaskEventGetAgentLevelDefaultsToSubagent(t *testing.T) {
	evt := &SubtaskEvent{OriginalEvent: stubCoreEvent{}}

	if got := evt.GetAgentLevel(); got != ports.LevelSubagent {
		t.Fatalf("expected core events to be elevated to subagent, got %q", got)
	}
}

func TestSubtaskEventGetAgentLevelRespectsExistingLevel(t *testing.T) {
	evt := &SubtaskEvent{OriginalEvent: stubEvent{}}

	if got := evt.GetAgentLevel(); got != ports.AgentLevel("subagent") {
		t.Fatalf("expected non-core agent level to be preserved, got %q", got)
	}
}

func TestSubtaskEventGetAgentLevelNilOriginal(t *testing.T) {
	evt := &SubtaskEvent{}

	if got := evt.GetAgentLevel(); got != ports.LevelSubagent {
		t.Fatalf("expected nil original events to default to subagent level, got %q", got)
	}
}
