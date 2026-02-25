package preparation

import (
	"context"
	"errors"
	"fmt"
	"testing"

	appconfig "alex/internal/app/agent/config"
	appcontext "alex/internal/app/agent/context"
	"alex/internal/app/subscription"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	llm "alex/internal/domain/agent/ports/llm"
	storage "alex/internal/domain/agent/ports/storage"
)

type scriptedCompletion struct {
	content string
	err     error
}

type scriptedLLMClient struct {
	model string
	plan  []scriptedCompletion
	calls int
}

func (c *scriptedLLMClient) Complete(_ context.Context, _ ports.CompletionRequest) (*ports.CompletionResponse, error) {
	idx := c.calls
	c.calls++
	if idx >= len(c.plan) {
		return &ports.CompletionResponse{Content: ""}, nil
	}
	step := c.plan[idx]
	if step.err != nil {
		return nil, step.err
	}
	return &ports.CompletionResponse{Content: step.content}, nil
}

func (c *scriptedLLMClient) Model() string { return c.model }

type scriptedFactory struct {
	clients map[string]llm.LLMClient
	calls   []string
}

func (f *scriptedFactory) GetClient(provider, model string, cfg llm.LLMConfig) (llm.LLMClient, error) {
	return f.GetIsolatedClient(provider, model, cfg)
}

func (f *scriptedFactory) GetIsolatedClient(provider, model string, _ llm.LLMConfig) (llm.LLMClient, error) {
	key := provider + "|" + model
	f.calls = append(f.calls, key)
	client, ok := f.clients[key]
	if !ok {
		return nil, fmt.Errorf("missing scripted client for %s", key)
	}
	return client, nil
}

func (f *scriptedFactory) DisableRetry() {}

func TestPreparePinnedSelectionFallsBackAfterRateLimit(t *testing.T) {
	session := &storage.Session{ID: "session-pinned-fallback", Messages: nil, Metadata: map[string]string{}}
	store := &stubSessionStore{session: session}
	primary := &scriptedLLMClient{
		model: "gpt-5.3-codex-spark",
		plan: []scriptedCompletion{
			{err: errors.New("usage_limit_reached")},
			{content: "primary-should-not-run"},
		},
	}
	fallback := &scriptedLLMClient{
		model: "kimi-for-coding",
		plan: []scriptedCompletion{
			{content: "fallback-first"},
			{content: "fallback-second"},
		},
	}
	factory := &scriptedFactory{
		clients: map[string]llm.LLMClient{
			"codex|gpt-5.3-codex-spark":  primary,
			"openrouter|kimi-for-coding": fallback,
		},
	}

	service := NewExecutionPreparationService(ExecutionPreparationDeps{
		LLMFactory:   factory,
		ToolRegistry: &registryWithList{defs: []ports.ToolDefinition{{Name: "shell"}}},
		SessionStore: store,
		ContextMgr:   stubContextManager{},
		Parser:       stubParser{},
		Config: appconfig.Config{
			LLMProvider:   "openrouter",
			LLMModel:      "kimi-for-coding",
			MaxIterations: 3,
		},
		Logger:       agent.NoopLogger{},
		EventEmitter: agent.NoopEventListener{},
	})

	ctx := appcontext.WithLLMSelection(context.Background(), subscription.ResolvedSelection{
		Provider: "codex",
		Model:    "gpt-5.3-codex-spark",
		APIKey:   "selection-key",
		BaseURL:  "https://chatgpt.com/backend-api/codex",
		Pinned:   true,
	})
	env, err := service.Prepare(ctx, "Do the thing", session.ID)
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	req := ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "hello"}},
	}
	resp, err := env.Services.LLM.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("first completion failed: %v", err)
	}
	if resp == nil || resp.Content != "fallback-first" {
		t.Fatalf("expected fallback-first response, got %#v", resp)
	}

	resp, err = env.Services.LLM.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("second completion failed: %v", err)
	}
	if resp == nil || resp.Content != "fallback-second" {
		t.Fatalf("expected fallback-second response, got %#v", resp)
	}

	if primary.calls != 1 {
		t.Fatalf("expected pinned client once, got %d", primary.calls)
	}
	if fallback.calls != 2 {
		t.Fatalf("expected fallback client twice, got %d", fallback.calls)
	}
	if len(factory.calls) != 2 ||
		factory.calls[0] != "codex|gpt-5.3-codex-spark" ||
		factory.calls[1] != "openrouter|kimi-for-coding" {
		t.Fatalf("unexpected client init calls: %#v", factory.calls)
	}
}

func TestPreparePinnedSelectionDoesNotFallbackOnNonRateLimit(t *testing.T) {
	session := &storage.Session{ID: "session-pinned-no-fallback", Messages: nil, Metadata: map[string]string{}}
	store := &stubSessionStore{session: session}
	primary := &scriptedLLMClient{
		model: "gpt-5.3-codex-spark",
		plan: []scriptedCompletion{
			{err: errors.New("connection reset by peer")},
		},
	}
	fallback := &scriptedLLMClient{
		model: "kimi-for-coding",
		plan:  []scriptedCompletion{{content: "fallback"}},
	}
	factory := &scriptedFactory{
		clients: map[string]llm.LLMClient{
			"codex|gpt-5.3-codex-spark":  primary,
			"openrouter|kimi-for-coding": fallback,
		},
	}

	service := NewExecutionPreparationService(ExecutionPreparationDeps{
		LLMFactory:   factory,
		ToolRegistry: &registryWithList{defs: []ports.ToolDefinition{{Name: "shell"}}},
		SessionStore: store,
		ContextMgr:   stubContextManager{},
		Parser:       stubParser{},
		Config: appconfig.Config{
			LLMProvider:   "openrouter",
			LLMModel:      "kimi-for-coding",
			MaxIterations: 3,
		},
		Logger:       agent.NoopLogger{},
		EventEmitter: agent.NoopEventListener{},
	})

	ctx := appcontext.WithLLMSelection(context.Background(), subscription.ResolvedSelection{
		Provider: "codex",
		Model:    "gpt-5.3-codex-spark",
		APIKey:   "selection-key",
		BaseURL:  "https://chatgpt.com/backend-api/codex",
		Pinned:   true,
	})
	env, err := service.Prepare(ctx, "Do the thing", session.ID)
	if err != nil {
		t.Fatalf("prepare failed: %v", err)
	}

	_, err = env.Services.LLM.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "hello"}},
	})
	if err == nil || err.Error() != "connection reset by peer" {
		t.Fatalf("expected original non-rate-limit error, got %v", err)
	}
	if primary.calls != 1 {
		t.Fatalf("expected pinned client once, got %d", primary.calls)
	}
	if fallback.calls != 0 {
		t.Fatalf("expected fallback client not called, got %d", fallback.calls)
	}
}
