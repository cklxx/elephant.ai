package hooks

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/domain/agent/ports"
	portsllm "alex/internal/domain/agent/ports/llm"
	"alex/internal/infra/memory"
	id "alex/internal/shared/utils/id"
)

type stubLLMClient struct {
	response string
	err      error
	lastReq  ports.CompletionRequest
}

func (s *stubLLMClient) Complete(_ context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	s.lastReq = req
	if s.err != nil {
		return nil, s.err
	}
	return &ports.CompletionResponse{Content: s.response}, nil
}

func (s *stubLLMClient) Model() string { return "stub" }

type stubLLMFactory struct {
	client       portsllm.LLMClient
	err          error
	lastProvider string
	lastModel    string
}

func (s *stubLLMFactory) GetClient(provider, model string, _ portsllm.LLMConfig) (portsllm.LLMClient, error) {
	return s.GetIsolatedClient(provider, model, portsllm.LLMConfig{})
}

func (s *stubLLMFactory) GetIsolatedClient(provider, model string, _ portsllm.LLMConfig) (portsllm.LLMClient, error) {
	s.lastProvider = provider
	s.lastModel = model
	if s.err != nil {
		return nil, s.err
	}
	return s.client, nil
}

func (s *stubLLMFactory) DisableRetry() {}

func TestMemoryCaptureHook_WritesDailyLog(t *testing.T) {
	root := t.TempDir()
	engine := memory.NewMarkdownEngine(root)
	if err := engine.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	llm := &stubLLMClient{response: "- Decision: Use SQLite\n- Prefers YAML configs"}
	factory := &stubLLMFactory{client: llm}
	hook := NewMemoryCaptureHook(engine, factory, nil, MemoryCaptureConfig{
		Enabled:  true,
		Provider: "mock",
		Model:    "small",
	})
	fixed := time.Date(2026, 2, 3, 11, 0, 0, 0, time.UTC)
	hook.clock = func() time.Time { return fixed }

	ctx := appcontext.WithMemoryPolicy(context.Background(), appcontext.MemoryPolicy{
		Enabled:     true,
		AutoCapture: true,
	})
	ctx = id.WithUserID(ctx, "user-1")
	if err := hook.OnTaskCompleted(ctx, TaskResultInfo{
		TaskInput: "Decide config format",
		Answer:    "Use YAML only.",
		UserID:    "user-1",
	}); err != nil {
		t.Fatalf("OnTaskCompleted: %v", err)
	}

	content, err := engine.LoadDaily(ctx, "user-1", fixed)
	if err != nil {
		t.Fatalf("LoadDaily: %v", err)
	}
	if !strings.Contains(content, "Decision: Use SQLite") {
		t.Fatalf("expected LLM summary in daily log, got: %s", content)
	}
}

func TestMemoryCaptureHook_RespectsPolicy(t *testing.T) {
	root := t.TempDir()
	engine := memory.NewMarkdownEngine(root)
	if err := engine.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	llm := &stubLLMClient{response: "- Should not write"}
	factory := &stubLLMFactory{client: llm}
	hook := NewMemoryCaptureHook(engine, factory, nil, MemoryCaptureConfig{
		Enabled:  true,
		Provider: "mock",
		Model:    "small",
	})
	ctx := appcontext.WithMemoryPolicy(context.Background(), appcontext.MemoryPolicy{
		Enabled:     true,
		AutoCapture: false,
	})
	ctx = id.WithUserID(ctx, "user-1")
	if err := hook.OnTaskCompleted(ctx, TaskResultInfo{
		TaskInput: "Do not capture",
		UserID:    "user-1",
	}); err != nil {
		t.Fatalf("OnTaskCompleted: %v", err)
	}
	dailyDir := filepath.Join(root, "memory")
	entries, err := os.ReadDir(dailyDir)
	if err == nil && len(entries) > 0 {
		t.Fatalf("expected no daily log written when auto capture disabled")
	}
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking daily dir: %v", err)
	}
}

func TestMemoryCaptureHook_FallbackOnLLMError(t *testing.T) {
	root := t.TempDir()
	engine := memory.NewMarkdownEngine(root)
	if err := engine.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	llm := &stubLLMClient{err: errors.New("boom")}
	factory := &stubLLMFactory{client: llm}
	hook := NewMemoryCaptureHook(engine, factory, nil, MemoryCaptureConfig{
		Enabled:  true,
		Provider: "mock",
		Model:    "small",
	})
	fixed := time.Date(2026, 2, 3, 12, 0, 0, 0, time.UTC)
	hook.clock = func() time.Time { return fixed }
	ctx := appcontext.WithMemoryPolicy(context.Background(), appcontext.MemoryPolicy{
		Enabled:     true,
		AutoCapture: true,
	})
	ctx = id.WithUserID(ctx, "user-2")
	if err := hook.OnTaskCompleted(ctx, TaskResultInfo{
		TaskInput: "Summarize task",
		Answer:    "Completed successfully.",
		UserID:    "user-2",
	}); err != nil {
		t.Fatalf("OnTaskCompleted: %v", err)
	}
	content, err := engine.LoadDaily(ctx, "user-2", fixed)
	if err != nil {
		t.Fatalf("LoadDaily: %v", err)
	}
	if !strings.Contains(content, "Task: Summarize task") {
		t.Fatalf("expected fallback content, got: %s", content)
	}
}

func TestMemoryCaptureHook_PrefersSmallModel(t *testing.T) {
	root := t.TempDir()
	engine := memory.NewMarkdownEngine(root)
	if err := engine.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	llm := &stubLLMClient{response: "- ok"}
	factory := &stubLLMFactory{client: llm}
	hook := NewMemoryCaptureHook(engine, factory, nil, MemoryCaptureConfig{
		Enabled:    true,
		Provider:   "main-provider",
		Model:      "main-model",
		SmallModel: "small-model",
	})
	ctx := appcontext.WithMemoryPolicy(context.Background(), appcontext.MemoryPolicy{
		Enabled:     true,
		AutoCapture: true,
	})
	ctx = id.WithUserID(ctx, "user-3")
	if err := hook.OnTaskCompleted(ctx, TaskResultInfo{
		TaskInput: "Test model selection",
		UserID:    "user-3",
	}); err != nil {
		t.Fatalf("OnTaskCompleted: %v", err)
	}
	if factory.lastModel != "small-model" {
		t.Fatalf("expected small model, got %q", factory.lastModel)
	}
	if factory.lastProvider != "main-provider" {
		t.Fatalf("expected provider to fall back to main-provider, got %q", factory.lastProvider)
	}
}
