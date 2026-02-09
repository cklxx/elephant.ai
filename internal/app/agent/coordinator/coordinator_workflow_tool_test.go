package coordinator

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	appconfig "alex/internal/app/agent/config"
	"alex/internal/domain/agent"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	llm "alex/internal/domain/agent/ports/llm"
	"alex/internal/domain/agent/ports/mocks"
	storage "alex/internal/domain/agent/ports/storage"
	tools "alex/internal/domain/agent/ports/tools"
	materialports "alex/internal/domain/materials/ports"
	"alex/internal/domain/workflow"
	"alex/internal/infra/tools/builtin/shared"
	"alex/internal/shared/utils/id"
)

type stubLLMFactory struct{ client llm.LLMClient }

func (f stubLLMFactory) GetClient(provider, model string, cfg llm.LLMConfig) (llm.LLMClient, error) {
	if f.client == nil {
		return nil, fmt.Errorf("no llm client configured")
	}
	return f.client, nil
}

func (f stubLLMFactory) GetIsolatedClient(provider, model string, cfg llm.LLMConfig) (llm.LLMClient, error) {
	return f.GetClient(provider, model, cfg)
}

func (f stubLLMFactory) DisableRetry() {}

type capturingListener struct {
	mu     sync.Mutex
	events []agent.AgentEvent
}

func (c *capturingListener) OnEvent(evt agent.AgentEvent) {
	c.mu.Lock()
	c.events = append(c.events, evt)
	c.mu.Unlock()
}

func (c *capturingListener) envelopes(eventName string) []*domain.WorkflowEventEnvelope {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []*domain.WorkflowEventEnvelope
	for _, evt := range c.events {
		env, ok := evt.(*domain.WorkflowEventEnvelope)
		if !ok || env == nil {
			continue
		}
		if env.Event == eventName {
			out = append(out, env)
		}
	}
	return out
}

func TestExecuteTaskRunsToolWorkflowEndToEnd(t *testing.T) {
	callCount := 0
	llm := &mocks.MockLLMClient{CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
		callCount++
		switch callCount {
		case 1:
			return &ports.CompletionResponse{
				Content: "I will call a tool",
				ToolCalls: []ports.ToolCall{{
					ID:   "call-1",
					Name: "echo",
					Arguments: map[string]any{
						"text": "hello world",
					},
				}},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{TotalTokens: 11},
			}, nil
		default:
			return &ports.CompletionResponse{Content: "Final answer with [note.txt]", StopReason: "stop", Usage: ports.TokenUsage{TotalTokens: 7}}, nil
		}
	}}

	registry := &mocks.MockToolRegistry{
		GetFunc: func(name string) (tools.ToolExecutor, error) {
			switch name {
			case "echo":
				return &mocks.MockToolExecutor{ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
					return &ports.ToolResult{
						CallID:  call.ID,
						Content: "echo: " + fmt.Sprint(call.Arguments["text"]),
						Attachments: map[string]ports.Attachment{
							"note.txt": {Name: "note.txt", MediaType: "text/plain", Data: "aGVsbG8=", Source: call.Name},
						},
					}, nil
				}}, nil
			default:
				return nil, fmt.Errorf("tool %s not found", name)
			}
		},
		ListFunc: func() []ports.ToolDefinition { return []ports.ToolDefinition{{Name: "echo"}} },
	}

	sessionStore := &stubSessionStore{}
	listener := &capturingListener{}

	coordinator := NewAgentCoordinator(
		stubLLMFactory{client: llm},
		registry,
		sessionStore,
		stubContextManager{},
		nil,
		&mocks.MockParser{},
		nil,
		appconfig.Config{
			LLMProvider:   "mock",
			LLMModel:      "tool-e2e",
			MaxIterations: 6,
			Temperature:   0.2,
		},
	)

	ctx := agent.WithOutputContext(context.Background(), &agent.OutputContext{Level: agent.LevelCore})
	result, err := coordinator.ExecuteTask(ctx, "please run echo", "session-e2e", listener)
	if err != nil {
		t.Fatalf("ExecuteTask returned error: %v", err)
	}

	if result == nil || result.Workflow == nil {
		t.Fatalf("expected workflow snapshot on result (nil_result=%v, nil_workflow=%v)", result == nil, result != nil && result.Workflow == nil)
	}
	if result.Workflow.Phase != workflow.PhaseSucceeded {
		t.Fatalf("unexpected workflow phase: %s", result.Workflow.Phase)
	}

	nodes := make(map[string]workflow.NodeSnapshot)
	for _, node := range result.Workflow.Nodes {
		nodes[node.ID] = node
	}

	for _, id := range []string{"prepare", "execute", "react:context", "react:iter:1:think", "react:iter:1:tools", "react:iter:1:tool:call-1", "react:finalize"} {
		if node, ok := nodes[id]; !ok || node.Status != workflow.NodeStatusSucceeded {
			t.Fatalf("expected node %s to succeed (found=%v, status=%s)", id, ok, node.Status)
		}
	}

	var toolMessageAttachments int
	for _, msg := range result.Messages {
		if msg.Source == ports.MessageSourceToolResult {
			toolMessageAttachments += len(msg.Attachments)
		}
	}
	if toolMessageAttachments == 0 {
		t.Fatalf("expected tool attachments to propagate into result messages")
	}

	completed := listener.envelopes("workflow.node.completed")
	if len(completed) == 0 {
		t.Fatalf("expected workflow.node.completed envelopes to be emitted")
	}
	hasToolStep := false
	for _, env := range completed {
		stepDesc, _ := env.Payload["step_description"].(string)
		if stepDesc == "" {
			stepDesc = env.NodeID
		}
		if strings.Contains(stepDesc, "react:iter:1:tool:call-1") {
			hasToolStep = true
			rawWorkflow, ok := env.Payload["workflow"]
			if !ok || rawWorkflow == nil {
				t.Fatalf("expected workflow snapshot on step envelope")
			}
			wfSnap, ok := rawWorkflow.(*workflow.WorkflowSnapshot)
			if !ok || wfSnap == nil {
				t.Fatalf("expected workflow snapshot type on step envelope")
			}
			if wfSnap.Phase != workflow.PhaseRunning && wfSnap.Phase != workflow.PhaseSucceeded {
				t.Fatalf("unexpected workflow phase on step envelope: %s", wfSnap.Phase)
			}
			iter, _ := env.Payload["iteration"].(int)
			if iter == 0 {
				if floatIter, ok := env.Payload["iteration"].(float64); ok {
					iter = int(floatIter)
				}
			}
			if iter != 1 {
				t.Fatalf("expected iteration 1 on tool completion, got %d", iter)
			}
		}
	}
	if !hasToolStep {
		t.Fatalf("expected tool call step completion envelope")
	}
}

func TestExecuteTaskInjectsSchedulerIntoToolContext(t *testing.T) {
	callCount := 0
	llmClient := &mocks.MockLLMClient{CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
		callCount++
		if callCount == 1 {
			return &ports.CompletionResponse{
				Content: "inspect scheduler",
				ToolCalls: []ports.ToolCall{{
					ID:   "call-scheduler",
					Name: "inspect_scheduler",
				}},
				StopReason: "tool_calls",
				Usage:      ports.TokenUsage{TotalTokens: 5},
			}, nil
		}
		return &ports.CompletionResponse{
			Content:    "done",
			StopReason: "stop",
			Usage:      ports.TokenUsage{TotalTokens: 3},
		}, nil
	}}

	expectedScheduler := &struct{ Name string }{Name: "runtime-scheduler"}
	schedulerObserved := false

	registry := &mocks.MockToolRegistry{
		GetFunc: func(name string) (tools.ToolExecutor, error) {
			if name != "inspect_scheduler" {
				return nil, fmt.Errorf("tool %s not found", name)
			}
			return &mocks.MockToolExecutor{
				ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
					schedulerObserved = shared.SchedulerFromContext(ctx) == expectedScheduler
					return &ports.ToolResult{CallID: call.ID, Content: "scheduler inspected"}, nil
				},
			}, nil
		},
		ListFunc: func() []ports.ToolDefinition {
			return []ports.ToolDefinition{{Name: "inspect_scheduler"}}
		},
	}

	coordinator := NewAgentCoordinator(
		stubLLMFactory{client: llmClient},
		registry,
		&stubSessionStore{},
		stubContextManager{},
		nil,
		&mocks.MockParser{},
		nil,
		appconfig.Config{
			LLMProvider:   "mock",
			LLMModel:      "scheduler-injection",
			MaxIterations: 4,
			Temperature:   0.1,
		},
	)
	coordinator.SetScheduler(expectedScheduler)

	_, err := coordinator.ExecuteTask(context.Background(), "check scheduler context", "session-scheduler", nil)
	if err != nil {
		t.Fatalf("ExecuteTask returned error: %v", err)
	}
	if !schedulerObserved {
		t.Fatalf("expected scheduler service to be injected into tool execution context")
	}
}

func TestExecuteTaskPropagatesSessionIDToWorkflowEnvelope(t *testing.T) {
	llm := &mocks.MockLLMClient{CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
		return &ports.CompletionResponse{
			Content:    "final answer",
			StopReason: "stop",
			Usage:      ports.TokenUsage{TotalTokens: 3},
		}, nil
	}}

	sessionStore := &stubSessionStore{}
	listener := &capturingListener{}

	coordinator := NewAgentCoordinator(
		stubLLMFactory{client: llm},
		stubToolRegistry{},
		sessionStore,
		stubContextManager{},
		nil,
		&mocks.MockParser{},
		nil,
		appconfig.Config{
			LLMProvider:   "mock",
			LLMModel:      "session-propagation",
			MaxIterations: 1,
			Temperature:   0.2,
		},
	)

	ctx := agent.WithOutputContext(context.Background(), &agent.OutputContext{Level: agent.LevelCore})
	_, err := coordinator.ExecuteTask(ctx, "test session propagation", "session-e2e", listener)
	if err != nil {
		t.Fatalf("ExecuteTask returned error: %v", err)
	}

	started := listener.envelopes("workflow.node.started")
	if len(started) == 0 {
		t.Fatalf("expected workflow.node.started envelopes to be emitted")
	}

	foundPrepare := false
	for _, env := range started {
		if env.NodeID != "prepare" {
			continue
		}
		foundPrepare = true
		if env.GetSessionID() != "session-e2e" {
			t.Fatalf("expected prepare step session_id=session-e2e, got %q", env.GetSessionID())
		}
		break
	}
	if !foundPrepare {
		t.Fatalf("expected prepare step envelope to be emitted")
	}
}

func TestExecuteTaskUsesEnsuredTaskIDForPrepareEnvelope(t *testing.T) {
	llm := &mocks.MockLLMClient{CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
		return &ports.CompletionResponse{
			Content:    "final answer",
			StopReason: "stop",
			Usage:      ports.TokenUsage{TotalTokens: 3},
		}, nil
	}}

	sessionStore := &stubSessionStore{}
	listener := &capturingListener{}

	coordinator := NewAgentCoordinator(
		stubLLMFactory{client: llm},
		stubToolRegistry{},
		sessionStore,
		stubContextManager{},
		nil,
		&mocks.MockParser{},
		nil,
		appconfig.Config{
			LLMProvider:   "mock",
			LLMModel:      "prepare-task-id",
			MaxIterations: 1,
			Temperature:   0.2,
		},
	)

	ctx := id.WithIDs(context.Background(), id.IDs{
		SessionID:   "session-e2e",
		RunID:       "task-sub-1",
		ParentRunID: "task-parent",
	})
	ctx = agent.WithOutputContext(ctx, &agent.OutputContext{
		Level:        agent.LevelSubagent,
		SessionID:    "session-e2e",
		TaskID:       "task-parent",
		ParentTaskID: "task-parent",
	})

	result, err := coordinator.ExecuteTask(ctx, "test prepare task", "session-e2e", listener)
	if err != nil {
		t.Fatalf("ExecuteTask returned error: %v", err)
	}
	if result.RunID != "task-sub-1" {
		t.Fatalf("expected run id task-sub-1, got %q", result.RunID)
	}

	started := listener.envelopes("workflow.node.started")
	if len(started) == 0 {
		t.Fatalf("expected workflow.node.started envelopes to be emitted")
	}

	var prepare *domain.WorkflowEventEnvelope
	for _, env := range started {
		if env.NodeID == "prepare" {
			prepare = env
			break
		}
	}
	if prepare == nil {
		t.Fatalf("expected prepare step envelope to be emitted")
	}
	if prepare.GetRunID() != result.RunID {
		t.Fatalf("expected prepare step run_id=%q, got %q", result.RunID, prepare.GetRunID())
	}
	if prepare.GetParentRunID() != "task-parent" {
		t.Fatalf("expected prepare step parent_run_id=task-parent, got %q", prepare.GetParentRunID())
	}
}

type recordingMigrator struct {
	called     bool
	request    materialports.MigrationRequest
	normalized map[string]ports.Attachment
}

func (m *recordingMigrator) Normalize(ctx context.Context, req materialports.MigrationRequest) (map[string]ports.Attachment, error) {
	m.called = true
	m.request = req
	if m.normalized != nil {
		return m.normalized, nil
	}
	return req.Attachments, nil
}

func TestSaveSessionAfterExecutionMigratesAttachments(t *testing.T) {
	coordinator := NewAgentCoordinator(
		stubLLMFactory{client: &mocks.MockLLMClient{}},
		stubToolRegistry{},
		&stubSessionStore{},
		stubContextManager{},
		nil,
		stubParser{},
		nil,
		appconfig.Config{LLMProvider: "mock", LLMModel: "session-migrate", MaxIterations: 1},
	)

	migrator := &recordingMigrator{
		normalized: map[string]ports.Attachment{
			"frame.png": {Name: "frame.png", MediaType: "image/png", URI: "/api/attachments/frame.png"},
		},
	}
	coordinator.SetAttachmentMigrator(migrator)

	session := &storage.Session{ID: "session-migrate", Metadata: map[string]string{}}
	result := &agent.TaskResult{
		SessionID: "session-migrate",
		Messages: []ports.Message{
			{
				Source: ports.MessageSourceToolResult,
				Attachments: map[string]ports.Attachment{
					"frame.png": {Name: "frame.png", MediaType: "image/png", Data: "ZmFrZQ=="},
				},
			},
		},
	}

	if err := coordinator.SaveSessionAfterExecution(context.Background(), session, result); err != nil {
		t.Fatalf("SaveSessionAfterExecution failed: %v", err)
	}

	if !migrator.called {
		t.Fatalf("expected attachment migrator to be called")
	}
	if session.Attachments == nil {
		t.Fatalf("expected session attachments to be set")
	}
	att, ok := session.Attachments["frame.png"]
	if !ok {
		t.Fatalf("expected migrated attachment to be stored on session")
	}
	if att.URI != "/api/attachments/frame.png" {
		t.Fatalf("expected migrated attachment URI, got %q", att.URI)
	}
}
