package react

import (
	"errors"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
)

type parserStub struct {
	calls      []ToolCall
	err        error
	parseCalls int
}

func (s *parserStub) Parse(_ string) ([]ports.ToolCall, error) {
	s.parseCalls++
	return append([]ports.ToolCall(nil), s.calls...), s.err
}

func (s *parserStub) Validate(_ ports.ToolCall, _ ports.ToolDefinition) error {
	return nil
}

func TestParseToolCallsUsesNativeCallsWhenPresent(t *testing.T) {
	engine := newReactEngineForTest(3)
	parser := &parserStub{err: errors.New("parser should not run")}
	msg := Message{
		ToolCalls: []ToolCall{{ID: "call_1", Name: "shell_exec"}},
	}

	calls, err := engine.parseToolCalls(msg, parser)
	if err != nil {
		t.Fatalf("expected no error for native tool calls, got %v", err)
	}
	if parser.parseCalls != 0 {
		t.Fatalf("expected parser.Parse not called, got %d calls", parser.parseCalls)
	}
	if len(calls) != 1 || calls[0].ID != "call_1" {
		t.Fatalf("unexpected parsed calls: %+v", calls)
	}
}

func TestParseToolCallsReturnsErrorOnMalformedMarkers(t *testing.T) {
	engine := newReactEngineForTest(3)
	parser := &parserStub{}
	msg := Message{
		Content: `<tool_call>{"name":}</tool_call>`,
	}

	calls, err := engine.parseToolCalls(msg, parser)
	if err == nil {
		t.Fatal("expected parse error for malformed tool-call markers")
	}
	if len(calls) != 0 {
		t.Fatalf("expected zero calls on parse failure, got %+v", calls)
	}
}

func TestParseToolCallsAllowsPlainAssistantContent(t *testing.T) {
	engine := newReactEngineForTest(3)
	parser := &parserStub{}
	msg := Message{
		Content: "plain assistant answer",
	}

	calls, err := engine.parseToolCalls(msg, parser)
	if err != nil {
		t.Fatalf("expected no error for plain content, got %v", err)
	}
	if len(calls) != 0 {
		t.Fatalf("expected zero calls for plain content, got %+v", calls)
	}
}

func TestPlanToolsSkipsFinalizationWhenToolCallParseFails(t *testing.T) {
	engine := newReactEngineForTest(3)
	state := &TaskState{}
	runtime := &reactRuntime{
		engine:  engine,
		state:   state,
		tracker: newReactWorkflow(nil),
	}
	iteration := &reactIteration{
		runtime:     runtime,
		index:       1,
		thought:     Message{Role: "assistant", Content: `<tool_call>{"name":}</tool_call>`},
		toolCallErr: errors.New("malformed tool call"),
	}

	result, done, err := iteration.planTools()
	if err != nil {
		t.Fatalf("expected no hard error, got %v", err)
	}
	if done {
		t.Fatal("expected runtime to continue next iteration instead of finalizing")
	}
	if result != nil {
		t.Fatalf("expected nil result while retrying parse, got %+v", result)
	}
	if len(state.Messages) != 1 {
		t.Fatalf("expected one retry guidance message, got %d", len(state.Messages))
	}
	msg := state.Messages[0]
	if msg.Role != "system" {
		t.Fatalf("expected system retry guidance message, got role=%q", msg.Role)
	}
	if msg.Source != ports.MessageSourceSystemPrompt {
		t.Fatalf("expected system prompt source, got %q", msg.Source)
	}
	if !strings.Contains(msg.Content, "malformed tool-call markup") {
		t.Fatalf("expected malformed tool-call guidance, got %q", msg.Content)
	}
}
