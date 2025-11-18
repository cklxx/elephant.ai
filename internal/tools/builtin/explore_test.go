package builtin

import (
	"context"
	"strings"
	"testing"

	"alex/internal/agent/ports"
)

type stubSubagent struct {
	lastCall ports.ToolCall
	result   *ports.ToolResult
	err      error
}

func (s *stubSubagent) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	s.lastCall = call
	if s.err != nil {
		return nil, s.err
	}
	if s.result != nil {
		return s.result, nil
	}
	return &ports.ToolResult{CallID: call.ID, Content: "ok"}, nil
}

func (s *stubSubagent) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{Name: "subagent"}
}

func (s *stubSubagent) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{Name: "subagent"}
}

func TestPhaseToolRequiresObjective(t *testing.T) {
	tool := NewExplore(&stubSubagent{})
	result, err := tool.Execute(context.Background(), ports.ToolCall{ID: "call-empty", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatalf("expected validation error, got nil")
	}
}

func TestExploreDelegatesToSubagent(t *testing.T) {
	stub := &stubSubagent{result: &ports.ToolResult{Metadata: map[string]any{
		"results": []map[string]any{{
			"index":  0,
			"answer": "Mapped cmd/alex",
		}},
	}}}
	tool := NewExplore(stub)
	args := map[string]any{
		"objective":    "Ship research console MVP",
		"local_scope":  []any{"cmd/alex"},
		"web_scope":    []string{"terminal UI best practices"},
		"custom_tasks": []any{"Author rollout checklist"},
		"notes":        "Preserve accessibility",
		"mode":         "serial",
	}

	result, err := tool.Execute(context.Background(), ports.ToolCall{ID: "call-1", Arguments: args})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}

	if stub.lastCall.Name != "subagent" {
		t.Fatalf("expected subagent call, got %s", stub.lastCall.Name)
	}
	subtasks, ok := stub.lastCall.Arguments["subtasks"].([]any)
	if !ok || len(subtasks) != 3 {
		t.Fatalf("unexpected subtasks payload: %#v", stub.lastCall.Arguments["subtasks"])
	}
	first, _ := subtasks[0].(string)
	if first == "" || !strings.HasPrefix(first, "[EXPLORE:LOCAL]") {
		t.Fatalf("expected explore local prefix, got %q", first)
	}

	if result.Metadata["phase"].(string) != "Explore" {
		t.Fatalf("expected phase metadata to be Explore, got %v", result.Metadata["phase"])
	}
	if _, ok := result.Metadata["summary_highlights"].([]string); !ok {
		t.Fatalf("expected highlights slice in metadata")
	}
}

func TestResearchPhaseSubtasks(t *testing.T) {
	stub := &stubSubagent{}
	tool := NewResearch(stub)
	args := map[string]any{
		"objective": "Validate roadmap",
		"web_scope": []string{"compare CLI research consoles"},
	}
	if _, err := tool.Execute(context.Background(), ports.ToolCall{ID: "call-2", Arguments: args}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	subtasks, ok := stub.lastCall.Arguments["subtasks"].([]any)
	if !ok || len(subtasks) == 0 {
		t.Fatalf("expected subtasks from research tool")
	}
	first, _ := subtasks[0].(string)
	if first == "" || !strings.HasPrefix(first, "[RESEARCH:WEB]") {
		t.Fatalf("expected research web prefix, got %q", first)
	}
}

func TestPhaseToolValidatesScopeTypes(t *testing.T) {
	stub := &stubSubagent{}
	tool := NewBuild(stub)
	args := map[string]any{
		"objective":   "Check",
		"local_scope": "not-an-array",
	}
	result, err := tool.Execute(context.Background(), ports.ToolCall{ID: "call-invalid", Arguments: args})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatalf("expected validation error for invalid scope type")
	}
}
