package app

import (
	"context"
	"strings"
	"sync"
	"testing"

	"alex/internal/agent/ports"
)

type recordingLLMFactory struct {
	mu    sync.Mutex
	calls []string
}

func (f *recordingLLMFactory) GetClient(provider, model string, config ports.LLMConfig) (ports.LLMClient, error) {
	return f.GetIsolatedClient(provider, model, config)
}

func (f *recordingLLMFactory) GetIsolatedClient(provider, model string, config ports.LLMConfig) (ports.LLMClient, error) {
	f.mu.Lock()
	f.calls = append(f.calls, provider+"|"+model)
	f.mu.Unlock()
	return &triageClient{model: model}, nil
}

func (f *recordingLLMFactory) DisableRetry() {}

func (f *recordingLLMFactory) CallModels() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

type triageClient struct {
	model string
}

func (c *triageClient) Model() string { return c.model }

func (c *triageClient) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	if len(req.Messages) > 0 && req.Messages[0].Role == "system" &&
		strings.Contains(req.Messages[0].Content, "fast task triage assistant") {
		return &ports.CompletionResponse{Content: `{
  "complexity":"simple",
  "recommended_model":"small",
  "task_name":"Fix typo in README",
  "goal":"Fix a typo",
  "approach":"Edit the file",
  "success_criteria":["README builds"],
  "steps":[{"description":"Edit README.md","rationale":"Correct the typo","needs_external_context":false}],
  "retrieval":{"should_retrieve":false,"local_queries":[],"search_queries":[],"crawl_urls":[],"knowledge_gaps":[],"notes":""}
}`}, nil
	}
	return &ports.CompletionResponse{Content: "ok"}, nil
}

func TestPrepareUsesSmallModelForSimpleTasksAndSetsTitleFromPreanalysis(t *testing.T) {
	session := &ports.Session{ID: "session-preanalysis-simple", Messages: nil, Metadata: map[string]string{}}
	store := &stubSessionStore{session: session}
	factory := &recordingLLMFactory{}

	service := NewExecutionPreparationService(ExecutionPreparationDeps{
		LLMFactory:   factory,
		ToolRegistry: &registryWithList{defs: []ports.ToolDefinition{{Name: "shell"}}},
		SessionStore: store,
		ContextMgr:   stubContextManager{},
		Parser:       stubParser{},
		Config: Config{
			LLMProvider:      "mock-default",
			LLMModel:         "default-model",
			LLMSmallProvider: "mock-small",
			LLMSmallModel:    "small-model",
			MaxIterations:    3,
		},
		Logger:       ports.NoopLogger{},
		EventEmitter: ports.NoopEventListener{},
	})

	_, err := service.Prepare(context.Background(), "Fix a typo in README", session.ID)
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	if got := strings.TrimSpace(session.Metadata["title"]); got != "Fix typo in README" {
		t.Fatalf("expected session title from pre-analysis, got %q", got)
	}

	modelCalls := factory.CallModels()
	if len(modelCalls) < 2 {
		t.Fatalf("expected at least 2 LLM factory calls (preanalysis + execution), got %v", modelCalls)
	}
	if modelCalls[0] != "mock-small|small-model" {
		t.Fatalf("expected preanalysis to use small model, got %q", modelCalls[0])
	}
	if modelCalls[len(modelCalls)-1] != "mock-small|small-model" {
		t.Fatalf("expected execution to use small model for simple tasks, got %q", modelCalls[len(modelCalls)-1])
	}
}

func TestParseTaskAnalysisExtractsRecommendedModel(t *testing.T) {
	analysis, recommended := parseTaskAnalysis(`noise prefix
{"complexity":"complex","recommended_model":"default","task_name":"Ship feature","goal":"g","approach":"a","success_criteria":["s"],"steps":[],"retrieval":{"should_retrieve":false}}
noise suffix`)
	if analysis == nil {
		t.Fatal("expected analysis")
	}
	if analysis.Complexity != "complex" {
		t.Fatalf("complexity: expected complex, got %q", analysis.Complexity)
	}
	if recommended != "default" {
		t.Fatalf("recommended: expected default, got %q", recommended)
	}
}

