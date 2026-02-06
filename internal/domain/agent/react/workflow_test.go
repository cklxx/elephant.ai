package react

import (
	"context"
	"reflect"
	"sync"
	"testing"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/ports/mocks"
	tools "alex/internal/domain/agent/ports/tools"
)

type recordingWorkflow struct {
	mu      sync.Mutex
	ensured map[string]any
	started map[string]bool
	success map[string]any
	failed  map[string]error
	order   []string
}

func newRecordingWorkflow() *recordingWorkflow {
	return &recordingWorkflow{
		ensured: make(map[string]any),
		started: make(map[string]bool),
		success: make(map[string]any),
		failed:  make(map[string]error),
	}
}

func (r *recordingWorkflow) EnsureNode(id string, input any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if id == "" {
		return
	}
	if _, ok := r.ensured[id]; !ok {
		r.ensured[id] = input
		r.order = append(r.order, id)
	}
}

func (r *recordingWorkflow) StartNode(id string) {
	r.mu.Lock()
	r.started[id] = true
	r.mu.Unlock()
}

func (r *recordingWorkflow) CompleteNodeSuccess(id string, output any) {
	r.mu.Lock()
	r.started[id] = true
	r.success[id] = output
	r.mu.Unlock()
}

func (r *recordingWorkflow) CompleteNodeFailure(id string, err error) {
	r.mu.Lock()
	r.started[id] = true
	r.failed[id] = err
	r.mu.Unlock()
}

func (r *recordingWorkflow) hasStarted(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.started[id]
}

func (r *recordingWorkflow) successOutput(id string) any {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.success[id]
}

func (r *recordingWorkflow) orderSnapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.order...)
}

func TestReactEngineEmitsWorkflowTransitions(t *testing.T) {
	tracker := newRecordingWorkflow()

	callCount := 0
	mockLLM := &mocks.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			callCount++
			switch callCount {
			case 1:
				return &ports.CompletionResponse{
					Content: "执行一次简单工具调用并返回结果。",
					ToolCalls: []ports.ToolCall{{
						ID:   "call-plan",
						Name: "plan",
						Arguments: map[string]any{
							"run_id":          "task",
							"overall_goal_ui": "执行一次工具调用并返回结果。",
							"complexity":      "simple",
						},
					}},
					StopReason: "tool_calls",
					Usage:      ports.TokenUsage{TotalTokens: 10},
				}, nil
			case 2:
				return &ports.CompletionResponse{
					Content: "执行 echo 工具。",
					ToolCalls: []ports.ToolCall{{
						ID:   "call-1",
						Name: "echo",
						Arguments: map[string]any{
							"text": "hi",
						},
					}},
					StopReason: "tool_calls",
					Usage:      ports.TokenUsage{TotalTokens: 10},
				}, nil
			default:
				return &ports.CompletionResponse{
					Content:    "done",
					StopReason: "stop",
					Usage:      ports.TokenUsage{TotalTokens: 5},
				}, nil
			}
		},
	}

	mockRegistry := &mocks.MockToolRegistry{
		GetFunc: func(name string) (tools.ToolExecutor, error) {
			switch name {
			case "plan":
				return &mocks.MockToolExecutor{
					ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
						return &ports.ToolResult{CallID: call.ID, Content: "ok"}, nil
					},
				}, nil
			default:
				return &mocks.MockToolExecutor{
					ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
						return &ports.ToolResult{
							CallID:  call.ID,
							Content: "ok",
							Attachments: map[string]ports.Attachment{
								"log.txt": {Name: "log.txt", MediaType: "text/plain", Data: "bG9n"},
							},
						}, nil
					},
				}, nil
			}
		},
		ListFunc: func() []ports.ToolDefinition {
			return []ports.ToolDefinition{{Name: "echo"}}
		},
	}

	services := Services{
		LLM:          mockLLM,
		ToolExecutor: mockRegistry,
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := NewReactEngine(ReactEngineConfig{
		MaxIterations: 6,
		Logger:        agent.NoopLogger{},
		Clock:         agent.SystemClock{},
		Workflow:      tracker,
	})

	state := &TaskState{
		SessionID: "sess",
		RunID:     "task",
		PendingUserAttachments: map[string]ports.Attachment{
			"note.txt": {Name: "note.txt", MediaType: "text/plain"},
		},
	}

	result, err := engine.SolveTask(context.Background(), "do work", state, services)
	if err != nil {
		t.Fatalf("SolveTask returned error: %v", err)
	}

	for _, id := range []string{
		"react:context",
		"react:iter:1:think",
		"react:iter:1:plan",
		"react:iter:1:tools",
		"react:iter:1:tool:call-plan",
		"react:iter:2:think",
		"react:iter:2:plan",
		"react:iter:2:tools",
		"react:iter:2:tool:call-1",
		"react:iter:3:think",
		"react:iter:3:plan",
		"react:finalize",
	} {
		if !tracker.hasStarted(id) {
			t.Fatalf("expected workflow node %s to start", id)
		}
	}

	ctxOutput, ok := tracker.successOutput("react:context").(map[string]any)
	if !ok {
		t.Fatalf("context output missing or wrong type: %#v", tracker.successOutput("react:context"))
	}
	pending, ok := ctxOutput["pending_attachments"].(map[string]ports.Attachment)
	if !ok || len(pending) != 1 {
		t.Fatalf("expected pending attachments to be preserved, got: %#v", ctxOutput["pending_attachments"])
	}

	planOutput, ok := tracker.successOutput("react:iter:2:plan").(map[string]any)
	if !ok {
		t.Fatalf("plan output missing: %#v", tracker.successOutput("react:iter:2:plan"))
	}
	if planOutput["tool_calls"] != 1 {
		t.Fatalf("expected plan to record tool count, got %#v", planOutput)
	}

	toolsOutput, ok := tracker.successOutput("react:iter:2:tools").(map[string]any)
	if !ok {
		t.Fatalf("tools output missing: %#v", tracker.successOutput("react:iter:2:tools"))
	}
	resultsVal, ok := toolsOutput["results"].([]ports.ToolResult)
	if !ok || len(resultsVal) != 1 {
		t.Fatalf("expected cloned tool results, got: %#v", toolsOutput["results"])
	}
	if len(resultsVal[0].Attachments) != 1 {
		t.Fatalf("expected tool attachments to be carried over, got: %#v", resultsVal[0].Attachments)
	}

	toolCallOutput, ok := tracker.successOutput("react:iter:2:tool:call-1").(map[string]any)
	if !ok {
		t.Fatalf("tool call output missing: %#v", tracker.successOutput("react:iter:2:tool:call-1"))
	}
	if toolCallOutput["call_id"] != "call-1" || toolCallOutput["tool"] != "echo" {
		t.Fatalf("unexpected tool call metadata: %#v", toolCallOutput)
	}
	resultVal, ok := toolCallOutput["result"].(ports.ToolResult)
	if !ok {
		t.Fatalf("tool call result missing: %#v", toolCallOutput["result"])
	}
	if len(resultVal.Attachments) != 1 {
		t.Fatalf("expected tool call attachments to be preserved, got %#v", resultVal.Attachments)
	}

	finalizeOutput, ok := tracker.successOutput("react:finalize").(map[string]any)
	if !ok {
		t.Fatalf("finalize output missing: %#v", tracker.successOutput("react:finalize"))
	}
	if finalizeOutput["stop_reason"] != result.StopReason {
		t.Fatalf("expected finalize stop reason %s, got %v", result.StopReason, finalizeOutput["stop_reason"])
	}
}

func TestReactEngineAllowsParallelRegularToolCalls(t *testing.T) {
	tracker := newRecordingWorkflow()
	callCount := 0
	mockLLM := &mocks.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			callCount++
			if callCount == 1 {
				return &ports.CompletionResponse{
					Content: "",
					ToolCalls: []ports.ToolCall{{
						ID:   "call-1",
						Name: "alpha",
						Arguments: map[string]any{
							"text": "one",
						},
					}, {
						ID:   "call-2",
						Name: "beta",
						Arguments: map[string]any{
							"text": "two",
						},
					}},
					StopReason: "tool_calls",
				}, nil
			}

			return &ports.CompletionResponse{
				Content:    "done",
				StopReason: "stop",
			}, nil
		},
	}

	mockRegistry := &mocks.MockToolRegistry{
		GetFunc: func(name string) (tools.ToolExecutor, error) {
			return &mocks.MockToolExecutor{ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
				return &ports.ToolResult{CallID: call.ID, Content: call.Name}, nil
			}}, nil
		},
	}

	services := Services{
		LLM:          mockLLM,
		ToolExecutor: mockRegistry,
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := NewReactEngine(ReactEngineConfig{
		MaxIterations: 3,
		Logger:        agent.NoopLogger{},
		Clock:         agent.SystemClock{},
		Workflow:      tracker,
	})

	state := &TaskState{SessionID: "sess", RunID: "task"}
	if _, err := engine.SolveTask(context.Background(), "do work", state, services); err != nil {
		t.Fatalf("SolveTask returned error: %v", err)
	}

	// Regular tool calls in parallel should now succeed without plan gate.
	expectedOrder := []string{
		"react:context",
		"react:iter:1:think",
		"react:iter:1:plan",
		"react:iter:1:tools",
		"react:iter:1:tool:call-1",
		"react:iter:1:tool:call-2",
		"react:iter:2:think",
		"react:iter:2:plan",
		"react:finalize",
	}

	if got := tracker.orderSnapshot(); !reflect.DeepEqual(got, expectedOrder) {
		t.Fatalf("unexpected workflow registration order: got %#v want %#v", got, expectedOrder)
	}
	if len(state.ToolResults) != 2 {
		t.Fatalf("expected 2 tool results, got %d", len(state.ToolResults))
	}
}
