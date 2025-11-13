package app

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"alex/internal/agent/ports"
)

func TestRAGPreloaderAppendResultAddsSystemMessageWithoutToolCallID(t *testing.T) {
	preloader := newRAGPreloader(nil)
	env := &ports.ExecutionEnvironment{
		State: &ports.TaskState{},
	}

	result := ports.ToolResult{
		CallID:  "call-123",
		Content: "preloaded context",
		Attachments: map[string]ports.Attachment{
			"note.md": {
				Name: "note.md",
			},
		},
	}

	preloader.appendResult(env, result)

	if len(env.State.Messages) != 1 {
		t.Fatalf("expected one message, got %d", len(env.State.Messages))
	}

	msg := env.State.Messages[0]
	if msg.Role != "user" {
		t.Fatalf("expected user role, got %q", msg.Role)
	}
	if msg.ToolCallID != "" {
		t.Fatalf("expected tool_call_id to be empty, got %q", msg.ToolCallID)
	}
	if msg.Source != ports.MessageSourceToolResult {
		t.Fatalf("expected tool result source, got %q", msg.Source)
	}
	if msg.Attachments == nil || msg.Attachments["note.md"].Name != "note.md" {
		t.Fatalf("expected attachment to be preserved, got %+v", msg.Attachments)
	}

	if len(env.State.ToolResults) != 1 || env.State.ToolResults[0].CallID != "call-123" {
		t.Fatalf("expected tool result to be tracked, got %+v", env.State.ToolResults)
	}
}

type recordingTool struct {
	lastCall ports.ToolCall
}

func (t *recordingTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	t.lastCall = call
	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  "ok",
		Metadata: map[string]any{},
	}, nil
}

func (t *recordingTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{Name: "web_search"}
}

func (t *recordingTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{Name: "web_search"}
}

type singleToolRegistry struct {
	tool ports.ToolExecutor
}

func (r *singleToolRegistry) Register(tool ports.ToolExecutor) error { return nil }

func (r *singleToolRegistry) Get(name string) (ports.ToolExecutor, error) {
	if name == "web_search" {
		return r.tool, nil
	}
	return nil, fmt.Errorf("tool not found: %s", name)
}

func (r *singleToolRegistry) List() []ports.ToolDefinition { return nil }

func (r *singleToolRegistry) Unregister(name string) error { return nil }

type stubLLM struct {
	content string
	err     error
}

func (s *stubLLM) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &ports.CompletionResponse{Content: s.content}, nil
}

func (s *stubLLM) Model() string { return "stub" }

func TestRAGPreloaderUsesLLMGeneratedQuery(t *testing.T) {
	tool := &recordingTool{}
	registry := &singleToolRegistry{tool: tool}
	env := &ports.ExecutionEnvironment{
		Services: ports.ServiceBundle{
			LLM:          &stubLLM{content: "Search Query: vector db 2024 benchmarks\nConfidence: high"},
			ToolExecutor: registry,
		},
		Session: &ports.Session{ID: "sess", Metadata: map[string]string{}},
		State:   &ports.TaskState{},
		RAGDirectives: &ports.RAGDirectives{
			Query:       "latest vector database improvements",
			UseSearch:   true,
			SearchSeeds: []string{"2024 benchmarks"},
		},
	}

	preloader := newRAGPreloader(ports.NoopLogger{})
	if err := preloader.apply(context.Background(), env); err != nil {
		t.Fatalf("apply returned error: %v", err)
	}

	query, ok := tool.lastCall.Arguments["query"].(string)
	if !ok {
		t.Fatalf("expected query argument, got %#v", tool.lastCall.Arguments)
	}
	if query != "vector db 2024 benchmarks" {
		t.Fatalf("expected LLM generated query, got %q", query)
	}
}

func TestRAGPreloaderFallsBackWhenLLMUnavailable(t *testing.T) {
	tool := &recordingTool{}
	registry := &singleToolRegistry{tool: tool}
	env := &ports.ExecutionEnvironment{
		Services: ports.ServiceBundle{
			LLM:          &stubLLM{err: errors.New("boom")},
			ToolExecutor: registry,
		},
		Session: &ports.Session{ID: "sess", Metadata: map[string]string{}},
		State:   &ports.TaskState{},
		RAGDirectives: &ports.RAGDirectives{
			Query:       "observability best practices",
			UseSearch:   true,
			SearchSeeds: []string{"otel collector", "2024"},
		},
	}

	preloader := newRAGPreloader(ports.NoopLogger{})
	if err := preloader.apply(context.Background(), env); err != nil {
		t.Fatalf("apply returned error: %v", err)
	}

	query, ok := tool.lastCall.Arguments["query"].(string)
	if !ok {
		t.Fatalf("expected query argument, got %#v", tool.lastCall.Arguments)
	}
	if query != "observability best practices otel collector 2024" {
		t.Fatalf("expected fallback query with seeds, got %q", query)
	}
}
