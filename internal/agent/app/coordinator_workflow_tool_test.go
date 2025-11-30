package app

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/agent/ports/mocks"
	"alex/internal/workflow"
)

type stubLLMFactory struct{ client ports.LLMClient }

func (f stubLLMFactory) GetClient(provider, model string, cfg ports.LLMConfig) (ports.LLMClient, error) {
	if f.client == nil {
		return nil, fmt.Errorf("no llm client configured")
	}
	return f.client, nil
}

func (f stubLLMFactory) GetIsolatedClient(provider, model string, cfg ports.LLMConfig) (ports.LLMClient, error) {
	return f.GetClient(provider, model, cfg)
}

func (f stubLLMFactory) DisableRetry() {}

type capturingListener struct{ events []ports.AgentEvent }

func (c *capturingListener) OnEvent(evt ports.AgentEvent) { c.events = append(c.events, evt) }

func (c *capturingListener) envelopes(eventName string) []*domain.WorkflowEventEnvelope {
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
		GetFunc: func(name string) (ports.ToolExecutor, error) {
			if name != "echo" {
				return nil, fmt.Errorf("tool %s not found", name)
			}
			return &mocks.MockToolExecutor{ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
				return &ports.ToolResult{
					CallID:  call.ID,
					Content: "echo: " + fmt.Sprint(call.Arguments["text"]),
					Attachments: map[string]ports.Attachment{
						"note.txt": {Name: "note.txt", MediaType: "text/plain", Data: "aGVsbG8=", Source: call.Name},
					},
				}, nil
			}}, nil
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
		&mocks.MockParser{},
		nil,
		Config{
			LLMProvider:   "mock",
			LLMModel:      "tool-e2e",
			MaxIterations: 3,
			Temperature:   0.2,
		},
	)

	ctx := ports.WithOutputContext(context.Background(), &ports.OutputContext{Level: ports.LevelCore})
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
