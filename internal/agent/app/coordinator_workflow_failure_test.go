package app

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"alex/internal/agent/ports"
	"alex/internal/llm"
	"alex/internal/workflow"
)

type stubPreparationService struct {
	env *ports.ExecutionEnvironment
	err error
}

func (s stubPreparationService) Prepare(ctx context.Context, task string, sessionID string) (*ports.ExecutionEnvironment, error) {
	if s.err != nil {
		return s.env, s.err
	}
	return s.env, fmt.Errorf("prepare not stubbed")
}

func (s stubPreparationService) SetEnvironmentSummary(string) {}

func (s stubPreparationService) ResolveAgentPreset(ctx context.Context, preset string) string {
	return preset
}

func (s stubPreparationService) ResolveToolPreset(ctx context.Context, preset string) string {
	return preset
}

type cancelAwarePreparationService struct{}

func (cancelAwarePreparationService) Prepare(ctx context.Context, task string, sessionID string) (*ports.ExecutionEnvironment, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return &ports.ExecutionEnvironment{Session: &ports.Session{ID: sessionID}}, nil
}

func (cancelAwarePreparationService) SetEnvironmentSummary(string) {}

func (cancelAwarePreparationService) ResolveAgentPreset(ctx context.Context, preset string) string {
	return preset
}

func (cancelAwarePreparationService) ResolveToolPreset(ctx context.Context, preset string) string {
	return preset
}

func TestExecuteTaskReturnsWorkflowSnapshotOnPrepareFailure(t *testing.T) {
	coordinator := NewAgentCoordinator(
		llm.NewFactory(),
		stubToolRegistry{},
		&stubSessionStore{},
		stubContextManager{},
		nil,
		stubParser{},
		nil,
		Config{LLMProvider: "mock", LLMModel: "prepare-failure"},
	)

	coordinator.prepService = &stubPreparationService{err: errors.New("prepare failed")}

	result, err := coordinator.ExecuteTask(context.Background(), "fail", "", nil)
	if err == nil {
		t.Fatalf("expected prepare error")
	}
	if result == nil || result.Workflow == nil {
		t.Fatalf("expected workflow snapshot even on failure (result_nil=%v)", result == nil)
	}
	if result.Workflow.Phase != workflow.PhaseFailed {
		t.Fatalf("expected failed workflow phase, got %s", result.Workflow.Phase)
	}
	statusByNode := make(map[string]workflow.NodeStatus)
	for _, node := range result.Workflow.Nodes {
		statusByNode[node.ID] = node.Status
	}
	if statusByNode[stagePrepare] != workflow.NodeStatusFailed {
		t.Fatalf("prepare node should be marked failed, got %s", statusByNode[stagePrepare])
	}
}

func TestExecuteTaskReturnsWorkflowSnapshotOnCancellation(t *testing.T) {
	coordinator := NewAgentCoordinator(
		llm.NewFactory(),
		stubToolRegistry{},
		&stubSessionStore{},
		stubContextManager{},
		nil,
		stubParser{},
		nil,
		Config{LLMProvider: "mock", LLMModel: "cancel"},
	)

	coordinator.prepService = cancelAwarePreparationService{}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := coordinator.ExecuteTask(ctx, "cancel", "", nil)
	if err == nil {
		t.Fatalf("expected cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancelled error, got %v", err)
	}
	if result == nil || result.Workflow == nil {
		t.Fatalf("expected workflow snapshot even on cancellation (result_nil=%v)", result == nil)
	}
	if result.Workflow.Phase != workflow.PhaseFailed {
		t.Fatalf("expected failed workflow phase on cancellation, got %s", result.Workflow.Phase)
	}
}
