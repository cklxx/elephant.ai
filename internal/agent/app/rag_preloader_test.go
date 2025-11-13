package app

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"alex/internal/agent/ports"
)

type recordingTool struct {
	name   string
	calls  []ports.ToolCall
	result ports.ToolResult
}

func (t *recordingTool) Execute(_ context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	t.calls = append(t.calls, call)
	res := t.result
	return &res, nil
}

func (t *recordingTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{Name: t.name}
}

func (t *recordingTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{Name: t.name}
}

type toolRegistryStub struct {
	tools map[string]*recordingTool
}

func (r *toolRegistryStub) Register(ports.ToolExecutor) error { return nil }

func (r *toolRegistryStub) Get(name string) (ports.ToolExecutor, error) {
	if r.tools == nil {
		return nil, fmt.Errorf("tool %s not found", name)
	}
	tool, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("tool %s not found", name)
	}
	return tool, nil
}

func (r *toolRegistryStub) List() []ports.ToolDefinition { return nil }

func (r *toolRegistryStub) Unregister(string) error { return nil }

func TestRAGPreloaderSkipsWithoutActions(t *testing.T) {
	preloader := newRAGPreloader(ports.NoopLogger{})
	env := &ports.ExecutionEnvironment{
		Session: &ports.Session{Metadata: map[string]string{}},
		State:   &ports.TaskState{},
		RAGDirectives: &ports.RAGDirectives{
			Query: "ignored",
		},
	}

	if err := preloader.apply(context.Background(), env); err != nil {
		t.Fatalf("expected no error when directives empty, got %v", err)
	}
	if env.Session.Metadata["rag_last_directives"] != "skip" {
		t.Fatalf("expected skip metadata, got %q", env.Session.Metadata["rag_last_directives"])
	}
	if len(env.State.ToolResults) != 0 {
		t.Fatalf("expected no tool results, got %d", len(env.State.ToolResults))
	}
}

func TestRAGPreloaderExecutesActions(t *testing.T) {
	codeTool := &recordingTool{name: "code_search", result: ports.ToolResult{Content: "code"}}
	searchTool := &recordingTool{name: "web_search", result: ports.ToolResult{Content: "search"}}
	fetchTool := &recordingTool{name: "web_fetch", result: ports.ToolResult{Content: "page"}}
	registry := &toolRegistryStub{tools: map[string]*recordingTool{
		"code_search": codeTool,
		"web_search":  searchTool,
		"web_fetch":   fetchTool,
	}}

	env := &ports.ExecutionEnvironment{
		Services: ports.ServiceBundle{ToolExecutor: registry},
		Session:  &ports.Session{ID: "sess-1", Metadata: map[string]string{}},
		State:    &ports.TaskState{},
		RAGDirectives: &ports.RAGDirectives{
			Query:        "find news",
			UseRetrieval: true,
			UseSearch:    true,
			UseCrawl:     true,
			SearchSeeds:  []string{"seed-one"},
			CrawlSeeds:   []string{"https://example.com/a", "https://example.com/b"},
			Justification: map[string]float64{
				"total_score": 0.9,
			},
		},
	}

	preloader := newRAGPreloader(ports.NoopLogger{})
	if err := preloader.apply(context.Background(), env); err != nil {
		t.Fatalf("preloader returned error: %v", err)
	}

	if len(codeTool.calls) != 1 || codeTool.calls[0].Arguments["query"] != "find news" {
		t.Fatalf("expected code_search to run with query, got %#v", codeTool.calls)
	}
	if len(searchTool.calls) != 1 {
		t.Fatalf("expected web_search to run once")
	}
	if searchTool.calls[0].Arguments["query"] != "find news seed-one" {
		t.Fatalf("expected web_search query to include seeds, got %#v", searchTool.calls[0].Arguments)
	}
	if len(fetchTool.calls) != 2 {
		t.Fatalf("expected web_fetch to run for each seed, got %d", len(fetchTool.calls))
	}

	if got := env.Session.Metadata["rag_last_directives"]; got != "retrieve+search+crawl" {
		t.Fatalf("metadata rag_last_directives mismatch: %q", got)
	}
	if env.Session.Metadata["rag_plan_search_seeds"] != "seed-one" {
		t.Fatalf("expected search seeds metadata preserved")
	}
	if env.Session.Metadata["rag_plan_crawl_seeds"] == "" {
		t.Fatalf("expected crawl seeds metadata to be set")
	}
	if len(env.State.ToolResults) != 4 {
		t.Fatalf("expected tool results appended, got %d", len(env.State.ToolResults))
	}
	for _, result := range env.State.ToolResults {
		flag, ok := result.Metadata["rag_preload"].(bool)
		if !ok || !flag {
			t.Fatalf("expected rag_preload metadata flag on result, got %#v", result.Metadata)
		}
	}
}

func TestRAGPreloaderSkipsWhenRetrievalToolUnavailable(t *testing.T) {
	env := &ports.ExecutionEnvironment{
		Services: ports.ServiceBundle{ToolExecutor: &toolRegistryStub{}},
		Session:  &ports.Session{ID: "sess-2", Metadata: map[string]string{}},
		State:    &ports.TaskState{},
		RAGDirectives: &ports.RAGDirectives{
			Query:        "inspect logs",
			UseRetrieval: true,
		},
	}

	preloader := newRAGPreloader(ports.NoopLogger{})
	if err := preloader.apply(context.Background(), env); err != nil {
		t.Fatalf("expected preloader to succeed without code_search tool, got %v", err)
	}
	if len(env.State.ToolResults) != 0 {
		t.Fatalf("expected no tool results when retrieval tool missing, got %d", len(env.State.ToolResults))
	}
	if len(env.State.Messages) == 0 {
		t.Fatalf("expected directive summary message to be recorded")
	}
	summary := env.State.Messages[len(env.State.Messages)-1].Content
	if !strings.Contains(summary, "SKIP") {
		t.Fatalf("expected summary to indicate skipped actions, got %q", summary)
	}
	if !strings.Contains(summary, "Retrieval skipped because code_search tool is unavailable.") {
		t.Fatalf("expected summary to record retrieval skip reason, got %q", summary)
	}
}
