package domain

import (
	"errors"
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
)

func TestNewInputReceivedEvent(t *testing.T) {
	ts := time.Now()
	atts := map[string]ports.Attachment{"f": {Name: "f"}}
	e := NewInputReceivedEvent(agent.LevelCore, "s", "r", "", "do something", atts, ts)
	if e.Kind != types.EventInputReceived {
		t.Errorf("wrong kind: %s", e.Kind)
	}
	if e.Data.Task != "do something" {
		t.Errorf("wrong task: %s", e.Data.Task)
	}
	// Verify deep copy
	atts["f"] = ports.Attachment{Name: "modified"}
	if e.Data.Attachments["f"].Name != "f" {
		t.Error("attachments should be cloned")
	}
}

func TestNewInputReceivedEvent_NilAttachments(t *testing.T) {
	e := NewInputReceivedEvent(agent.LevelCore, "s", "r", "", "task", nil, time.Now())
	if e.Data.Attachments != nil {
		t.Error("expected nil attachments")
	}
}

func TestNewNodeStartedEvent(t *testing.T) {
	base := NewBaseEvent(agent.LevelCore, "s", "r", "", time.Now())
	e := NewNodeStartedEvent(base, 1, 5, 0, "step 1", "input", nil)
	if e.Kind != types.EventNodeStarted {
		t.Errorf("wrong kind: %s", e.Kind)
	}
	if e.Data.TotalIters != 5 {
		t.Errorf("expected 5 total iters, got %d", e.Data.TotalIters)
	}
}

func TestNewNodeOutputDeltaEvent(t *testing.T) {
	base := NewBaseEvent(agent.LevelCore, "s", "r", "", time.Now())
	e := NewNodeOutputDeltaEvent(base, 1, 3, "hello", false, time.Now(), "gpt-4")
	if e.Kind != types.EventNodeOutputDelta {
		t.Errorf("wrong kind: %s", e.Kind)
	}
	if e.Data.Delta != "hello" {
		t.Errorf("wrong delta: %s", e.Data.Delta)
	}
	if e.Data.Final {
		t.Error("expected non-final")
	}
}

func TestNewToolStartedEvent(t *testing.T) {
	base := NewBaseEvent(agent.LevelCore, "s", "r", "", time.Now())
	args := map[string]interface{}{"path": "/tmp"}
	e := NewToolStartedEvent(base, 1, "call-1", "read_file", args)
	if e.Kind != types.EventToolStarted {
		t.Errorf("wrong kind: %s", e.Kind)
	}
	if e.Data.ToolName != "read_file" {
		t.Errorf("wrong tool: %s", e.Data.ToolName)
	}
}

func TestNewToolCompletedEvent(t *testing.T) {
	base := NewBaseEvent(agent.LevelCore, "s", "r", "", time.Now())
	testErr := errors.New("timeout")
	e := NewToolCompletedEvent(base, "call-1", "bash", "output", testErr, time.Second, nil, nil)
	if e.Kind != types.EventToolCompleted {
		t.Errorf("wrong kind: %s", e.Kind)
	}
	if e.Data.ErrorStr != "timeout" {
		t.Errorf("expected timeout, got %s", e.Data.ErrorStr)
	}
	if e.Data.Error == nil {
		t.Error("expected non-nil Error field")
	}
}

func TestNewToolCompletedEvent_NilError(t *testing.T) {
	base := NewBaseEvent(agent.LevelCore, "s", "r", "", time.Now())
	e := NewToolCompletedEvent(base, "call-1", "bash", "ok", nil, time.Second, nil, nil)
	if e.Data.ErrorStr != "" {
		t.Errorf("expected empty error string, got %s", e.Data.ErrorStr)
	}
}

func TestNewResultFinalEvent(t *testing.T) {
	base := NewBaseEvent(agent.LevelCore, "s", "r", "", time.Now())
	e := NewResultFinalEvent(base, "done", 10, 5000, "end_turn", 2*time.Second, true, true, nil)
	if e.Kind != types.EventResultFinal {
		t.Errorf("wrong kind: %s", e.Kind)
	}
	if e.Data.FinalAnswer != "done" {
		t.Errorf("wrong final answer: %s", e.Data.FinalAnswer)
	}
}

func TestNewResultCancelledEvent(t *testing.T) {
	e := NewResultCancelledEvent(agent.LevelCore, "s", "r", "", "user cancelled", "user", time.Now())
	if e.Kind != types.EventResultCancelled {
		t.Errorf("wrong kind: %s", e.Kind)
	}
	if e.Data.RequestedBy != "user" {
		t.Errorf("expected user, got %s", e.Data.RequestedBy)
	}
}

func TestNewNodeFailedEvent(t *testing.T) {
	base := NewBaseEvent(agent.LevelCore, "s", "r", "", time.Now())
	e := NewNodeFailedEvent(base, 3, "execution", errors.New("oom"), true)
	if e.Kind != types.EventNodeFailed {
		t.Errorf("wrong kind: %s", e.Kind)
	}
	if !e.Data.Recoverable {
		t.Error("expected recoverable")
	}
	if e.Data.ErrorStr != "oom" {
		t.Errorf("expected oom, got %s", e.Data.ErrorStr)
	}
}

func TestNewNodeFailedEvent_NilError(t *testing.T) {
	base := NewBaseEvent(agent.LevelCore, "s", "r", "", time.Now())
	e := NewNodeFailedEvent(base, 1, "phase", nil, false)
	if e.Data.ErrorStr != "" {
		t.Errorf("expected empty error string, got %s", e.Data.ErrorStr)
	}
}
