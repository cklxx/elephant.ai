package react

import (
	"context"
	"strings"
	"testing"

	"alex/internal/domain/agent"
	"alex/internal/domain/agent/ports"
	"alex/internal/domain/agent/ports/mocks"
	"alex/internal/domain/agent/types"
)

type recordingListener struct {
	events []AgentEvent
}

func (r *recordingListener) OnEvent(evt AgentEvent) {
	r.events = append(r.events, evt)
}

func TestReactEngine_FinalAnswerReview_EmitsToolEvents(t *testing.T) {
	scenario := mocks.NewTodoManagementScenario()
	services := Services{
		LLM:          scenario.LLM,
		ToolExecutor: scenario.Registry,
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := newReactEngineForTest(10)
	listener := &recordingListener{}
	engine.SetEventListener(listener)

	state := &TaskState{SessionID: "sess"}
	_, err := engine.SolveTask(context.Background(), "Add tasks and mark first as complete", state, services)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	startedIdx := -1
	completedIdx := -1
	startedCallID := ""
	completedCallID := ""

	for idx, evt := range listener.events {
		if e, ok := evt.(*domain.Event); ok {
			switch e.Kind {
			case types.EventToolStarted:
				if strings.EqualFold(strings.TrimSpace(e.Data.ToolName), "final_answer_review") {
					startedIdx = idx
					startedCallID = strings.TrimSpace(e.Data.CallID)
				}
			case types.EventToolCompleted:
				if strings.EqualFold(strings.TrimSpace(e.Data.ToolName), "final_answer_review") {
					completedIdx = idx
					completedCallID = strings.TrimSpace(e.Data.CallID)
				}
			}
		}
	}

	if startedIdx < 0 {
		t.Fatalf("expected WorkflowToolStartedEvent for final_answer_review")
	}
	if completedIdx < 0 {
		t.Fatalf("expected WorkflowToolCompletedEvent for final_answer_review")
	}
	if !strings.HasPrefix(startedCallID, "final_answer_review:") {
		t.Fatalf("expected call_id prefix final_answer_review:, got %q", startedCallID)
	}
	if completedCallID != startedCallID {
		t.Fatalf("expected completed call_id=%q, got %q", startedCallID, completedCallID)
	}
	if completedIdx <= startedIdx {
		t.Fatalf("expected completed event after started (started=%d completed=%d)", startedIdx, completedIdx)
	}
}

func TestReactEngine_FinalAnswerReview_DoesNotEmitToolEventsWithoutTools(t *testing.T) {
	services := Services{
		LLM: &mocks.MockLLMClient{
			CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
				return &ports.CompletionResponse{
					Content:    "hello",
					StopReason: "stop",
					Usage:      ports.TokenUsage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
				}, nil
			},
		},
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	engine := newReactEngineForTest(10)
	listener := &recordingListener{}
	engine.SetEventListener(listener)

	state := &TaskState{SessionID: "sess"}
	_, err := engine.SolveTask(context.Background(), "hello", state, services)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	for _, evt := range listener.events {
		if e, ok := evt.(*domain.Event); ok && e.Kind == types.EventToolStarted {
			if strings.EqualFold(strings.TrimSpace(e.Data.ToolName), "final_answer_review") {
				t.Fatalf("unexpected final_answer_review tool started event")
			}
		}
	}
}
