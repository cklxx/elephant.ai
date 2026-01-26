package orchestration

import (
	"context"
	"strings"
	"testing"

	"alex/internal/agent/ports"
)

type mockSubagentExecutor struct {
	lastCall *ports.ToolCall
	result   *ports.ToolResult
	err      error
}

func (m *mockSubagentExecutor) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	copied := call
	m.lastCall = &copied
	if m.result != nil || m.err != nil {
		return m.result, m.err
	}
	return &ports.ToolResult{CallID: call.ID, Metadata: map[string]any{}}, nil
}

func (m *mockSubagentExecutor) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{Name: "subagent"}
}

func (m *mockSubagentExecutor) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{Name: "subagent"}
}

func TestExploreRequiresObjective(t *testing.T) {
	tool := NewExplore(&mockSubagentExecutor{})
	result, err := tool.Execute(context.Background(), ports.ToolCall{ID: "call-empty", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatalf("expected validation error, got nil")
	}
	if !strings.Contains(result.Content, "objective") {
		t.Fatalf("expected error message about objective, got %q", result.Content)
	}
}

func TestExploreDelegationFlow(t *testing.T) {
	mock := &mockSubagentExecutor{}
	mock.result = &ports.ToolResult{
		CallID:  "subagent-call",
		Content: "subagent raw output",
	}

	tool := NewExplore(mock)
	args := map[string]any{
		"objective":    "Assess repository health",
		"local_scope":  []any{"auth package"},
		"web_scope":    []string{"Go security best practices"},
		"custom_tasks": []any{"Summarize findings for stakeholders"},
		"notes":        "Prioritize authentication concerns",
	}
	result, err := tool.Execute(context.Background(), ports.ToolCall{ID: "call-1", Arguments: args})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}

	if mock.lastCall == nil {
		t.Fatalf("expected subagent to be executed")
	}
	if mock.lastCall.Name != "subagent" {
		t.Fatalf("expected subagent call name, got %q", mock.lastCall.Name)
	}

	prompt, ok := mock.lastCall.Arguments["prompt"].(string)
	if !ok {
		t.Fatalf("expected prompt to be string, got %T", mock.lastCall.Arguments["prompt"])
	}
	if !strings.Contains(prompt, "Assess repository health") || !strings.Contains(prompt, "auth package") {
		t.Fatalf("prompt missing scopes/objective: %s", prompt)
	}
	if !strings.Contains(prompt, "Go security best practices") || !strings.Contains(prompt, "Summarize findings for stakeholders") {
		t.Fatalf("prompt missing web/custom tasks: %s", prompt)
	}
	if !strings.Contains(prompt, "Notes: Prioritize authentication concerns") {
		t.Fatalf("expected notes propagated in prompt: %s", prompt)
	}

	if !strings.Contains(result.Content, "Delegated objective \"Assess repository health\"") {
		t.Fatalf("summary missing objective: %s", result.Content)
	}
	if !strings.Contains(result.Content, "Delegated objective \"Assess repository health\"") {
		t.Fatalf("summary missing objective: %s", result.Content)
	}

	metadata := result.Metadata
	if got := metadata["local_scope"].([]string); len(got) != 1 || got[0] != "auth package" {
		t.Fatalf("unexpected local scope metadata: %#v", got)
	}
	if got := metadata["web_scope"].([]string); len(got) != 1 || got[0] != "Go security best practices" {
		t.Fatalf("unexpected web scope metadata: %#v", got)
	}
	if got := metadata["custom_tasks"].([]string); len(got) != 1 || got[0] != "Summarize findings for stakeholders" {
		t.Fatalf("unexpected custom tasks metadata: %#v", got)
	}
	if metadata["prompt"] != prompt {
		t.Fatalf("expected prompt metadata to match")
	}
}

func TestExploreDefaultSubtaskWhenNoScopes(t *testing.T) {
	mock := &mockSubagentExecutor{}
	mock.result = &ports.ToolResult{CallID: "subagent", Content: "ok"}

	tool := NewExplore(mock)
	args := map[string]any{
		"objective": "Evaluate new feature",
	}
	result, err := tool.Execute(context.Background(), ports.ToolCall{ID: "call-default", Arguments: args})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}

	if mock.lastCall == nil {
		t.Fatalf("expected subagent execution")
	}
	prompt, _ := mock.lastCall.Arguments["prompt"].(string)
	if !strings.Contains(prompt, "Evaluate new feature") {
		t.Fatalf("unexpected prompt: %s", prompt)
	}

	if metadata := result.Metadata["custom_tasks"]; metadata != nil {
		if slice, ok := metadata.([]string); !ok || len(slice) != 0 {
			t.Fatalf("expected empty custom tasks metadata, got %#v", metadata)
		}
	}
}

func TestExploreValidatesScopeTypes(t *testing.T) {
	tool := NewExplore(&mockSubagentExecutor{})
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
	if !strings.Contains(result.Content, "local_scope") {
		t.Fatalf("expected message mentioning local_scope, got %q", result.Content)
	}
}
