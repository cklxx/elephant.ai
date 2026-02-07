package react

import (
	"context"
	"testing"

	"alex/internal/domain/agent/ports"
	"alex/internal/domain/agent/ports/mocks"
	tools "alex/internal/domain/agent/ports/tools"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestReactEngine_EmitsTraceSpansForIterationLLMAndTool(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider()
	tp.RegisterSpanProcessor(recorder)
	prevProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(prevProvider)
	})

	callCount := 0
	mockLLM := &mocks.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			callCount++
			if callCount == 1 {
				return &ports.CompletionResponse{
					Content: "先调用工具。",
					ToolCalls: []ports.ToolCall{
						{ID: "call-1", Name: "echo", Arguments: map[string]any{"text": "hello"}},
					},
					StopReason: "tool_calls",
				}, nil
			}
			return &ports.CompletionResponse{
				Content:    "完成。",
				StopReason: "stop",
			}, nil
		},
		ModelFunc: func() string { return "trace-test-model" },
	}

	mockTools := &mocks.MockToolRegistry{
		GetFunc: func(name string) (tools.ToolExecutor, error) {
			return &mocks.MockToolExecutor{
				ExecuteFunc: func(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
					return &ports.ToolResult{CallID: call.ID, Content: "ok"}, nil
				},
			}, nil
		},
	}

	engine := newReactEngineForTest(4)
	state := &TaskState{
		SessionID:   "session-trace",
		RunID:       "run-trace",
		ParentRunID: "parent-trace",
	}
	services := Services{
		LLM:          mockLLM,
		ToolExecutor: mockTools,
		Parser:       &mocks.MockParser{},
		Context:      &mocks.MockContextManager{},
	}

	if _, err := engine.SolveTask(context.Background(), "trace test", state, services); err != nil {
		t.Fatalf("SolveTask returned error: %v", err)
	}

	spans := recorder.Ended()
	if len(spans) == 0 {
		t.Fatalf("expected spans to be recorded")
	}

	counts := map[string]int{}
	for _, span := range spans {
		counts[span.Name()]++
	}

	if counts[traceSpanReactIteration] == 0 {
		t.Fatalf("expected %q span, spans=%v", traceSpanReactIteration, counts)
	}
	if counts[traceSpanLLMGenerate] == 0 {
		t.Fatalf("expected %q span, spans=%v", traceSpanLLMGenerate, counts)
	}
	if counts[traceSpanToolExecute] == 0 {
		t.Fatalf("expected %q span, spans=%v", traceSpanToolExecute, counts)
	}
}
