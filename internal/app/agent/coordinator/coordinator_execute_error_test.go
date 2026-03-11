package coordinator

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	appconfig "alex/internal/app/agent/config"
	appcontext "alex/internal/app/agent/context"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	"alex/internal/domain/workflow"
)

type stubEventDispatcher struct {
	title string
}

func (d stubEventDispatcher) Listener() agent.EventListener { return nil }
func (d stubEventDispatcher) Flush(context.Context, string) {}
func (d stubEventDispatcher) Title() string                 { return d.title }

func TestFinalizeExecution_ReturnsPersistFailure(t *testing.T) {
	store := &ensureSessionStore{
		sessions: map[string]*storage.Session{},
		saveErr:  errors.New("persist failed"),
	}
	coordinator := NewAgentCoordinator(nil, nil, store, nil, nil, nil, nil, appconfig.Config{})

	env := &agent.ExecutionEnvironment{
		Session: &storage.Session{ID: "session-1", Metadata: map[string]string{}},
	}
	result := &agent.TaskResult{Answer: "done"}
	wf := newAgentWorkflow("run-1", slog.Default(), nil, nil)
	wf.start(stagePrepare)
	wf.succeed(stagePrepare, nil)
	wf.start(stageExecute)

	attach := func(result *agent.TaskResult, env *agent.ExecutionEnvironment) *agent.TaskResult {
		return attachWorkflowSnapshot(result, wf, env.Session.ID, "run-1", "parent-1")
	}

	finalResult, err := coordinator.finalizeExecution(
		context.Background(),
		"do work",
		env,
		wf,
		stubEventDispatcher{title: "Planned Title"},
		result,
		nil,
		"run-1",
		"parent-1",
		agent.NoopLogger{},
		attach,
	)
	if err == nil || err.Error() != "failed to save session: persist failed" {
		t.Fatalf("expected wrapped persistence error, got %v", err)
	}
	if finalResult == nil || finalResult.Workflow == nil {
		t.Fatalf("expected workflow snapshot on persist failure, got %+v", finalResult)
	}
	if env.Session.Metadata["title"] != "Planned Title" {
		t.Fatalf("expected plan title propagated before save, got %q", env.Session.Metadata["title"])
	}

	statusByNode := make(map[string]workflow.NodeStatus)
	for _, node := range finalResult.Workflow.Nodes {
		statusByNode[node.ID] = node.Status
	}
	if statusByNode[stagePersist] != workflow.NodeStatusFailed {
		t.Fatalf("expected persist node failed, got %s", statusByNode[stagePersist])
	}
}

func TestFinalizeExecution_SubagentExecutionErrorSkipsPersistence(t *testing.T) {
	store := &ensureSessionStore{
		sessions: map[string]*storage.Session{},
		saveErr:  errors.New("should not be called"),
	}
	coordinator := NewAgentCoordinator(nil, nil, store, nil, nil, nil, nil, appconfig.Config{})
	coordinator.clock = agent.ClockFunc(func() time.Time {
		return time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)
	})

	env := &agent.ExecutionEnvironment{
		Session: &storage.Session{ID: "session-sub", Metadata: map[string]string{}},
	}
	result := &agent.TaskResult{Answer: "partial"}
	execErr := errors.New("tool failed")
	wf := newAgentWorkflow("run-sub", slog.Default(), nil, nil)
	wf.start(stagePrepare)
	wf.succeed(stagePrepare, nil)
	wf.start(stageExecute)

	attach := func(result *agent.TaskResult, env *agent.ExecutionEnvironment) *agent.TaskResult {
		return attachWorkflowSnapshot(result, wf, env.Session.ID, "run-sub", "parent-sub")
	}

	finalResult, err := coordinator.finalizeExecution(
		appcontext.MarkSubagentContext(context.Background()),
		"do sub work",
		env,
		wf,
		stubEventDispatcher{title: "ignored"},
		result,
		execErr,
		"run-sub",
		"parent-sub",
		agent.NoopLogger{},
		attach,
	)
	if err == nil || err.Error() != "task execution failed: tool failed" {
		t.Fatalf("expected wrapped execution error, got %v", err)
	}
	if store.saveCalls != 0 {
		t.Fatalf("expected persistence skipped for subagent, got %d save calls", store.saveCalls)
	}
	if env.Session.Metadata["last_error"] != "tool failed" {
		t.Fatalf("expected last_error metadata stamped, got %q", env.Session.Metadata["last_error"])
	}
	if env.Session.Metadata["last_error_at"] != "2026-03-11T10:00:00Z" {
		t.Fatalf("expected last_error_at metadata stamped, got %q", env.Session.Metadata["last_error_at"])
	}
	if finalResult == nil || finalResult.Workflow == nil {
		t.Fatalf("expected workflow snapshot on execution error, got %+v", finalResult)
	}

	statusByNode := make(map[string]workflow.NodeStatus)
	for _, node := range finalResult.Workflow.Nodes {
		statusByNode[node.ID] = node.Status
	}
	if statusByNode[stageExecute] != workflow.NodeStatusFailed {
		t.Fatalf("expected execute node failed, got %s", statusByNode[stageExecute])
	}
	if statusByNode[stagePersist] != workflow.NodeStatusSucceeded {
		t.Fatalf("expected persist node succeeded with skip marker, got %s", statusByNode[stagePersist])
	}
}
