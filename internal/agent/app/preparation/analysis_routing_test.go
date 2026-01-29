package preparation

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	appconfig "alex/internal/agent/app/config"
	appcontext "alex/internal/agent/app/context"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	llm "alex/internal/agent/ports/llm"
	storage "alex/internal/agent/ports/storage"
	"alex/internal/subscription"
)

type recordingLLMFactory struct {
	mu    sync.Mutex
	calls []string
}

func (f *recordingLLMFactory) GetClient(provider, model string, config llm.LLMConfig) (llm.LLMClient, error) {
	return f.GetIsolatedClient(provider, model, config)
}

func (f *recordingLLMFactory) GetIsolatedClient(provider, model string, config llm.LLMConfig) (llm.LLMClient, error) {
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

func TestPrepareRunsPreanalysisAsyncAndPersistsTitleInBackground(t *testing.T) {
	session := &storage.Session{ID: "session-preanalysis-simple", Messages: nil, Metadata: map[string]string{}}
	store := &stubSessionStore{session: session}
	factory := &recordingLLMFactory{}

	service := NewExecutionPreparationService(ExecutionPreparationDeps{
		LLMFactory:   factory,
		ToolRegistry: &registryWithList{defs: []ports.ToolDefinition{{Name: "shell"}}},
		SessionStore: store,
		ContextMgr:   stubContextManager{},
		Parser:       stubParser{},
		Config: appconfig.Config{
			LLMProvider:      "mock-default",
			LLMModel:         "default-model",
			LLMSmallProvider: "mock-small",
			LLMSmallModel:    "small-model",
			MaxIterations:    3,
		},
		Logger:       agent.NoopLogger{},
		EventEmitter: agent.NoopEventListener{},
	})

	env, err := service.Prepare(context.Background(), "Fix a typo in README", session.ID)
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	// Pre-analysis is async: Prepare should use the default model (not small)
	// because the triage result is not yet available.
	modelCalls := factory.CallModels()
	if len(modelCalls) < 1 {
		t.Fatalf("expected at least 1 LLM factory call, got %v", modelCalls)
	}
	// The first synchronous call should be the execution client using the default model.
	if modelCalls[0] != "mock-default|default-model" {
		t.Fatalf("expected execution to use default model, got %q", modelCalls[0])
	}

	// TaskAnalysis should be nil since quickTriageTask doesn't match a normal task.
	if env.TaskAnalysis != nil {
		t.Fatalf("expected nil TaskAnalysis for async preanalysis, got %+v", env.TaskAnalysis)
	}

	// Wait for the async goroutine to persist the title.
	// Use the thread-safe SessionTitle() accessor to avoid racing with the goroutine.
	deadline := time.After(2 * time.Second)
	for {
		if got := store.SessionTitle(); got != "" {
			if got != "Fix typo in README" {
				t.Fatalf("expected async title 'Fix typo in README', got %q", got)
			}
			break
		}
		select {
		case <-deadline:
			t.Fatal("timed out waiting for async title persistence")
		case <-time.After(10 * time.Millisecond):
		}
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

func TestPrepareSkipsLLMPreanalysisForGreeting(t *testing.T) {
	session := &storage.Session{ID: "session-preanalysis-greeting", Messages: nil, Metadata: map[string]string{}}
	store := &stubSessionStore{session: session}
	factory := &recordingLLMFactory{}

	service := NewExecutionPreparationService(ExecutionPreparationDeps{
		LLMFactory:   factory,
		ToolRegistry: &registryWithList{defs: []ports.ToolDefinition{{Name: "shell"}}},
		SessionStore: store,
		ContextMgr:   stubContextManager{},
		Parser:       stubParser{},
		Config: appconfig.Config{
			LLMProvider:      "mock-default",
			LLMModel:         "default-model",
			LLMSmallProvider: "mock-small",
			LLMSmallModel:    "small-model",
			MaxIterations:    3,
		},
		Logger:       agent.NoopLogger{},
		EventEmitter: agent.NoopEventListener{},
	})

	env, err := service.Prepare(context.Background(), "nihao", session.ID)
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	// quickTriageTask sets the title synchronously on the returned env.Session.
	if got := strings.TrimSpace(env.Session.Metadata["title"]); got != "Greeting" {
		t.Fatalf("expected session title Greeting, got %q", got)
	}

	modelCalls := factory.CallModels()
	if len(modelCalls) != 1 {
		t.Fatalf("expected 1 LLM factory call (execution client only), got %v", modelCalls)
	}
	if modelCalls[0] != "mock-small|small-model" {
		t.Fatalf("expected greeting to use small model directly, got %q", modelCalls[0])
	}
}

func TestPrepareUsesPinnedSelectionAndSkipsSmallModel(t *testing.T) {
	session := &storage.Session{ID: "session-pinned", Messages: nil, Metadata: map[string]string{}}
	store := &stubSessionStore{session: session}
	factory := &recordingLLMFactory{}

	service := NewExecutionPreparationService(ExecutionPreparationDeps{
		LLMFactory:   factory,
		ToolRegistry: &registryWithList{defs: []ports.ToolDefinition{{Name: "shell"}}},
		SessionStore: store,
		ContextMgr:   stubContextManager{},
		Parser:       stubParser{},
		Config: appconfig.Config{
			LLMProvider:      "mock-default",
			LLMModel:         "default-model",
			LLMSmallProvider: "mock-small",
			LLMSmallModel:    "small-model",
			MaxIterations:    3,
		},
		Logger:       agent.NoopLogger{},
		EventEmitter: agent.NoopEventListener{},
	})

	ctx := appcontext.WithLLMSelection(context.Background(), subscription.ResolvedSelection{
		Provider: "codex",
		Model:    "gpt-5.2-codex",
		APIKey:   "tok",
		BaseURL:  "https://chatgpt.com/backend-api/codex",
		Headers:  map[string]string{"ChatGPT-Account-Id": "acct"},
		Pinned:   true,
	})

	_, err := service.Prepare(ctx, "Do the thing", session.ID)
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	modelCalls := factory.CallModels()
	if len(modelCalls) != 1 || modelCalls[0] != "codex|gpt-5.2-codex" {
		t.Fatalf("expected pinned model only, got %v", modelCalls)
	}
}
