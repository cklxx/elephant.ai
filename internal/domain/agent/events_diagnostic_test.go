package domain

import (
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
)

func TestNewDiagnosticContextCompressionEvent(t *testing.T) {
	ts := time.Now()
	e := NewDiagnosticContextCompressionEvent(agent.LevelCore, "s", "r", "p", 100, 30, ts)
	if e.Kind != types.EventDiagnosticContextCompression {
		t.Errorf("wrong kind: %s", e.Kind)
	}
	if e.Data.OriginalCount != 100 {
		t.Errorf("expected 100, got %d", e.Data.OriginalCount)
	}
	if e.Data.CompressionRate != 30.0 {
		t.Errorf("expected 30.0, got %f", e.Data.CompressionRate)
	}
}

func TestNewDiagnosticContextSnapshotEvent(t *testing.T) {
	ts := time.Now()
	msgs := []ports.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}
	excluded := []ports.Message{{Role: "system", Content: "sys"}}
	e := NewDiagnosticContextSnapshotEvent(agent.LevelCore, "s", "r", "p", 1, 5, "req-1", msgs, excluded, ts)

	if e.Kind != types.EventDiagnosticContextSnapshot {
		t.Errorf("wrong kind: %s", e.Kind)
	}
	if e.Data.ContextMsgCount != 2 {
		t.Errorf("expected 2, got %d", e.Data.ContextMsgCount)
	}
	if e.Data.ExcludedCount != 1 {
		t.Errorf("expected 1, got %d", e.Data.ExcludedCount)
	}
	if e.Data.ContextPreview == "" {
		t.Error("expected non-empty preview")
	}
}

func TestBuildContextPreview_Empty(t *testing.T) {
	if buildContextPreview(nil) != "" {
		t.Error("expected empty string for nil messages")
	}
}

func TestBuildContextPreview_FewMessages(t *testing.T) {
	msgs := []ports.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "world"},
	}
	preview := buildContextPreview(msgs)
	if preview == "" {
		t.Error("expected non-empty preview")
	}
}

func TestBuildContextPreview_ManyMessages(t *testing.T) {
	msgs := make([]ports.Message, 10)
	for i := range msgs {
		msgs[i] = ports.Message{Role: "user", Content: "msg"}
	}
	preview := buildContextPreview(msgs)
	if preview == "" {
		t.Error("expected non-empty preview")
	}
}

func TestBuildContextPreview_LongContent(t *testing.T) {
	longContent := ""
	for i := 0; i < 300; i++ {
		longContent += "x"
	}
	msgs := []ports.Message{{Role: "user", Content: longContent}}
	preview := buildContextPreview(msgs)
	if len(preview) > 2100 {
		t.Error("preview should be capped")
	}
}

func TestNewDiagnosticToolFilteringEvent(t *testing.T) {
	ts := time.Now()
	tools := []string{"tool_a", "tool_b"}
	e := NewDiagnosticToolFilteringEvent(agent.LevelCore, "s", "r", "p", "preset", 10, 2, tools, ts)
	if e.Kind != types.EventDiagnosticToolFiltering {
		t.Errorf("wrong kind: %s", e.Kind)
	}
	if e.Data.FilteredCount != 2 {
		t.Errorf("expected 2, got %d", e.Data.FilteredCount)
	}
	if e.Data.ToolFilterRatio != 20.0 {
		t.Errorf("expected 20.0, got %f", e.Data.ToolFilterRatio)
	}
}

func TestNewDiagnosticEnvironmentSnapshotEvent(t *testing.T) {
	ts := time.Now()
	host := map[string]string{"os": "darwin", "arch": "arm64"}
	e := NewDiagnosticEnvironmentSnapshotEvent(host, ts)
	if e.Kind != types.EventDiagnosticEnvironmentSnapshot {
		t.Errorf("wrong kind: %s", e.Kind)
	}
	// Should be cloned
	host["os"] = "linux"
	if e.Data.Host["os"] != "darwin" {
		t.Error("host map was not cloned")
	}
}

func TestNewDiagnosticContextCheckpointEvent(t *testing.T) {
	base := NewBaseEvent(agent.LevelCore, "s", "r", "", time.Now())
	e := NewDiagnosticContextCheckpointEvent(base, "pruning", 5, 1000, 200, 3000)
	if e.Kind != types.EventDiagnosticContextCheckpoint {
		t.Errorf("wrong kind: %s", e.Kind)
	}
	if e.Data.PrunedMessages != 5 {
		t.Errorf("expected 5, got %d", e.Data.PrunedMessages)
	}
}

func TestNewProactiveContextRefreshEvent(t *testing.T) {
	base := NewBaseEvent(agent.LevelCore, "s", "r", "", time.Now())
	e := NewProactiveContextRefreshEvent(base, 3, 5)
	if e.Kind != types.EventProactiveContextRefresh {
		t.Errorf("wrong kind: %s", e.Kind)
	}
	if e.Data.MemoriesInjected != 5 {
		t.Errorf("expected 5, got %d", e.Data.MemoriesInjected)
	}
}

func TestNewBackgroundTaskEvents(t *testing.T) {
	base := NewBaseEvent(agent.LevelCore, "s", "r", "", time.Now())

	dispatched := NewBackgroundTaskDispatchedEvent(base, "task-1", "desc", "prompt", "claude_code")
	if dispatched.Kind != types.EventBackgroundTaskDispatched {
		t.Errorf("wrong kind: %s", dispatched.Kind)
	}

	completed := NewBackgroundTaskCompletedEvent(base, "task-1", "desc", "completed", "answer", "", time.Second, 5, 1000)
	if completed.Kind != types.EventBackgroundTaskCompleted {
		t.Errorf("wrong kind: %s", completed.Kind)
	}
	if completed.Data.Duration != time.Second {
		t.Errorf("expected 1s, got %v", completed.Data.Duration)
	}
}
