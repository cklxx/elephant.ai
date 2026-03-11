package lark

import (
	"context"
	"sync"
	"testing"
	"time"

	"alex/internal/delivery/channels"
	ports "alex/internal/domain/agent/ports"
	portsllm "alex/internal/domain/agent/ports/llm"
	"alex/internal/shared/config"
	"alex/internal/shared/logging"
)

// ---------------------------------------------------------------------------
// Stub LLM helpers (mirrors rephrase_test.go pattern)
// ---------------------------------------------------------------------------

type intentStubLLMClient struct {
	mu   sync.Mutex
	resp string
	err  error
	reqs []ports.CompletionRequest
}

func (c *intentStubLLMClient) Complete(_ context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	c.mu.Lock()
	c.reqs = append(c.reqs, req)
	c.mu.Unlock()
	if c.err != nil {
		return nil, c.err
	}
	return &ports.CompletionResponse{Content: c.resp}, nil
}

func (c *intentStubLLMClient) Model() string { return "stub-intent" }

func (c *intentStubLLMClient) lastReqs() []ports.CompletionRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]ports.CompletionRequest, len(c.reqs))
	copy(out, c.reqs)
	return out
}

type intentStubFactory struct {
	client portsllm.LLMClient
}

func (f *intentStubFactory) GetClient(_, _ string, _ portsllm.LLMConfig) (portsllm.LLMClient, error) {
	return f.client, nil
}

func (f *intentStubFactory) GetIsolatedClient(_, _ string, _ portsllm.LLMConfig) (portsllm.LLMClient, error) {
	return f.client, nil
}

func (f *intentStubFactory) DisableRetry() {}

// newIntentGateway builds a minimal Gateway wired with the stub LLM factory
// and BtwIntentRouterEnabled set.
func newIntentGateway(t *testing.T, stub *intentStubLLMClient, enabled bool, modelOverride string) *Gateway {
	t.Helper()
	en := enabled
	gw := &Gateway{
		cfg: Config{
			BaseConfig: channels.BaseConfig{
				SessionPrefix: "test",
				AllowDirect:   true,
			},
			BtwIntentRouterEnabled: &en,
			BtwIntentRouterModel:   modelOverride,
		},
		llmFactory: &intentStubFactory{client: stub},
		llmProfile: config.LLMProfile{Provider: "openai", Model: "gpt-4o-mini"},
		logger:     logging.OrNop(nil),
		now:        time.Now,
	}
	return gw
}

// ---------------------------------------------------------------------------
// Tests for classifyBtwIntent
// ---------------------------------------------------------------------------

func TestClassifyBtwIntent_ReturnsINJECT(t *testing.T) {
	stub := &intentStubLLMClient{resp: "INJECT"}
	g := newIntentGateway(t, stub, true, "")

	got := g.classifyBtwIntent(context.Background(), "write a report on Q1 metrics", "please focus on APAC region")
	if got != "INJECT" {
		t.Fatalf("expected INJECT, got %q", got)
	}
}

func TestClassifyBtwIntent_ReturnsBTW(t *testing.T) {
	stub := &intentStubLLMClient{resp: "BTW"}
	g := newIntentGateway(t, stub, true, "")

	got := g.classifyBtwIntent(context.Background(), "write a report on Q1 metrics", "what's the weather today?")
	if got != "BTW" {
		t.Fatalf("expected BTW, got %q", got)
	}
}

func TestClassifyBtwIntent_ToleratesTrailingPunctuation(t *testing.T) {
	stub := &intentStubLLMClient{resp: "INJECT."}
	g := newIntentGateway(t, stub, true, "")

	got := g.classifyBtwIntent(context.Background(), "build a dashboard", "add a bar chart section")
	if got != "INJECT" {
		t.Fatalf("expected INJECT even with trailing punctuation, got %q", got)
	}
}

func TestClassifyBtwIntent_FallbackOnError(t *testing.T) {
	stub := &intentStubLLMClient{err: &intentStubErr{"simulated LLM error"}}
	g := newIntentGateway(t, stub, true, "")

	got := g.classifyBtwIntent(context.Background(), "some task", "some message")
	if got != "BTW" {
		t.Fatalf("expected BTW fallback on error, got %q", got)
	}
}

func TestClassifyBtwIntent_FallbackWhenFactoryNil(t *testing.T) {
	en := true
	g := &Gateway{
		cfg: Config{
			BtwIntentRouterEnabled: &en,
		},
		llmFactory: nil,
		logger:     logging.OrNop(nil),
		now:        time.Now,
	}
	got := g.classifyBtwIntent(context.Background(), "task", "msg")
	if got != "BTW" {
		t.Fatalf("expected BTW when factory is nil, got %q", got)
	}
}

func TestClassifyBtwIntent_ModelOverride(t *testing.T) {
	stub := &intentStubLLMClient{resp: "INJECT"}
	g := newIntentGateway(t, stub, true, "claude-3-haiku")

	got := g.classifyBtwIntent(context.Background(), "task", "msg")
	if got != "INJECT" {
		t.Fatalf("expected INJECT, got %q", got)
	}
	// Verify the resolved profile used the override model.
	profile := g.resolveIntentRouterProfile()
	if profile.Model != "claude-3-haiku" {
		t.Fatalf("expected model override claude-3-haiku, got %q", profile.Model)
	}
}

func TestClassifyBtwIntent_PromptContainsTaskAndMessage(t *testing.T) {
	stub := &intentStubLLMClient{resp: "BTW"}
	g := newIntentGateway(t, stub, true, "")

	taskDesc := "refactor the auth module"
	newMsg := "order me a pizza"
	_ = g.classifyBtwIntent(context.Background(), taskDesc, newMsg)

	reqs := stub.lastReqs()
	if len(reqs) == 0 {
		t.Fatal("expected at least one LLM request")
	}
	userContent := reqs[0].Messages[1].Content
	if !contains(userContent, taskDesc) {
		t.Errorf("expected user prompt to contain taskDesc %q, got %q", taskDesc, userContent)
	}
	if !contains(userContent, newMsg) {
		t.Errorf("expected user prompt to contain newMsg %q, got %q", newMsg, userContent)
	}
}

// ---------------------------------------------------------------------------
// Tests for btwIntentRouterEnabled / resolveIntentRouterProfile
// ---------------------------------------------------------------------------

func TestBtwIntentRouterEnabled_Default(t *testing.T) {
	g := &Gateway{cfg: Config{}}
	if g.btwIntentRouterEnabled() {
		t.Fatal("expected disabled by default")
	}
}

func TestBtwIntentRouterEnabled_True(t *testing.T) {
	en := true
	g := &Gateway{cfg: Config{BtwIntentRouterEnabled: &en}}
	if !g.btwIntentRouterEnabled() {
		t.Fatal("expected enabled when *bool=true")
	}
}

func TestResolveIntentRouterProfile_NoOverride(t *testing.T) {
	g := &Gateway{
		cfg:        Config{BtwIntentRouterModel: ""},
		llmProfile: config.LLMProfile{Provider: "openai", Model: "gpt-4o"},
	}
	p := g.resolveIntentRouterProfile()
	if p.Model != "gpt-4o" {
		t.Fatalf("expected gpt-4o, got %q", p.Model)
	}
}

func TestResolveIntentRouterProfile_WithOverride(t *testing.T) {
	g := &Gateway{
		cfg:        Config{BtwIntentRouterModel: "gpt-4o-mini"},
		llmProfile: config.LLMProfile{Provider: "openai", Model: "gpt-4o"},
	}
	p := g.resolveIntentRouterProfile()
	if p.Model != "gpt-4o-mini" {
		t.Fatalf("expected override gpt-4o-mini, got %q", p.Model)
	}
	if p.Provider != "openai" {
		t.Fatalf("expected provider openai preserved, got %q", p.Provider)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type intentStubErr struct{ msg string }

func (e *intentStubErr) Error() string { return e.msg }

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}())
}
