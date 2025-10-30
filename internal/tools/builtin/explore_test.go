package builtin

import (
	"context"
	"encoding/json"
	"reflect"
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
	rawResults, err := json.Marshal([]map[string]any{
		{
			"index":       0,
			"task":        "[LOCAL] Assess repository health — Focus on auth package.",
			"answer":      "Reviewed code paths\nAuthentication looks solid.",
			"iterations":  2,
			"tokens_used": 50,
		},
		{
			"index": 1,
			"task":  "[WEB] Assess repository health — Research Go security best practices.",
			"error": "timeout",
		},
		{
			"index":       2,
			"task":        "[CUSTOM] Summarize findings for stakeholders",
			"answer":      "Custom note for leadership.",
			"iterations":  1,
			"tokens_used": 10,
		},
	})
	if err != nil {
		t.Fatalf("marshal results: %v", err)
	}

	mock.result = &ports.ToolResult{
		CallID:  "subagent-call",
		Content: "subagent raw output",
		Metadata: map[string]any{
			"results":          string(rawResults),
			"success_count":    2,
			"failure_count":    1,
			"total_tasks":      3,
			"total_tokens":     60,
			"total_iterations": 3,
		},
	}

	tool := NewExplore(mock)
	args := map[string]any{
		"objective":    "Assess repository health",
		"local_scope":  []any{"auth package"},
		"web_scope":    []string{"Go security best practices"},
		"custom_tasks": []any{"Summarize findings for stakeholders"},
		"notes":        "Prioritize authentication concerns",
		"mode":         "serial",
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

	subtasksArg, ok := mock.lastCall.Arguments["subtasks"].([]any)
	if !ok {
		t.Fatalf("expected subtasks to be []any, got %T", mock.lastCall.Arguments["subtasks"])
	}
	if len(subtasksArg) != 3 {
		t.Fatalf("expected 3 subtasks, got %d", len(subtasksArg))
	}
	for _, sub := range subtasksArg {
		str, _ := sub.(string)
		if !strings.Contains(str, "Notes: Prioritize authentication concerns") {
			t.Fatalf("expected notes propagated in subtask %q", str)
		}
	}
	if mode, ok := mock.lastCall.Arguments["mode"].(string); !ok || mode != "serial" {
		t.Fatalf("expected mode 'serial', got %v", mock.lastCall.Arguments["mode"])
	}

	if !strings.Contains(result.Content, "Delegated objective \"Assess repository health\"") {
		t.Fatalf("summary missing objective: %s", result.Content)
	}
	if !strings.Contains(result.Content, "Highlights:") {
		t.Fatalf("expected highlights in summary: %s", result.Content)
	}
	if !strings.Contains(result.Content, "Shared notes: Prioritize authentication concerns") {
		t.Fatalf("expected notes mention in summary: %s", result.Content)
	}

	metadata := result.Metadata
	expectedScope := []string{"auth package"}
	if got := metadata["local_scope"].([]string); !reflect.DeepEqual(got, expectedScope) {
		t.Fatalf("unexpected local scope metadata: %#v", got)
	}
	if got := metadata["web_scope"].([]string); !reflect.DeepEqual(got, []string{"Go security best practices"}) {
		t.Fatalf("unexpected web scope metadata: %#v", got)
	}
	if got := metadata["custom_tasks"].([]string); !reflect.DeepEqual(got, []string{"Summarize findings for stakeholders"}) {
		t.Fatalf("unexpected custom tasks metadata: %#v", got)
	}

	highlights, ok := metadata["summary_highlights"].([]string)
	if !ok {
		t.Fatalf("expected highlights slice, got %T", metadata["summary_highlights"])
	}
	expectedHighlights := []string{
		"Task 1: Reviewed code paths",
		"Task 2 failed: timeout",
		"Task 3: Custom note for leadership.",
	}
	if !reflect.DeepEqual(highlights, expectedHighlights) {
		t.Fatalf("unexpected highlights: %#v", highlights)
	}

	delegation, ok := metadata["delegation"].(map[string]any)
	if !ok {
		t.Fatalf("expected delegation metadata map, got %T", metadata["delegation"])
	}
	if _, ok := delegation["result_metadata"].(map[string]any); !ok {
		t.Fatalf("expected raw metadata in delegation details")
	}
	callDetails, ok := delegation["call"].(map[string]any)
	if !ok {
		t.Fatalf("expected call details in delegation metadata")
	}
	argsDetails, ok := callDetails["arguments"].(map[string]any)
	if !ok {
		t.Fatalf("expected call arguments in delegation metadata")
	}
	if sentMode, ok := argsDetails["mode"].(string); !ok || sentMode != "serial" {
		t.Fatalf("delegation metadata lost mode: %v", argsDetails["mode"])
	}
}

func TestExploreDefaultSubtaskWhenNoScopes(t *testing.T) {
	mock := &mockSubagentExecutor{}
	rawResults, err := json.Marshal([]map[string]any{{
		"index":  0,
		"task":   "[CUSTOM] Evaluate new feature",
		"answer": "Done",
	}})
	if err != nil {
		t.Fatalf("marshal results: %v", err)
	}
	mock.result = &ports.ToolResult{
		CallID:  "subagent",
		Content: "ok",
		Metadata: map[string]any{
			"results":       string(rawResults),
			"success_count": 1,
			"failure_count": 0,
		},
	}

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
	subtasksArg := mock.lastCall.Arguments["subtasks"].([]any)
	if len(subtasksArg) != 1 {
		t.Fatalf("expected single fallback subtask, got %d", len(subtasksArg))
	}
	if !strings.HasPrefix(subtasksArg[0].(string), "[CUSTOM] Evaluate new feature") {
		t.Fatalf("unexpected fallback subtask: %s", subtasksArg[0])
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

func TestExploreCountsFailuresForEmptyErrorObject(t *testing.T) {
	results := []delegationSubtask{{
		Index: 0,
		Task:  "[LOCAL] Inspect",
		Error: map[string]any{},
	}}

	success, failure := countDelegationOutcomes(results, map[string]any{"failure_count": 1})
	if success != 0 || failure != 1 {
		t.Fatalf("expected 0 success/1 failure, got %d/%d", success, failure)
	}

	highlights := buildDelegationHighlights(results)
	if len(highlights) != 1 {
		t.Fatalf("expected single highlight, got %d", len(highlights))
	}
	if !strings.Contains(highlights[0], "Task 1 failed") {
		t.Fatalf("expected failure highlight, got %q", highlights[0])
	}
}
