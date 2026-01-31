package hooks

import (
	"context"
	"testing"

	appcontext "alex/internal/agent/app/context"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/memory"
	id "alex/internal/utils/id"
)

type refreshMemoryService struct {
	entries []memory.Entry
}

func (s *refreshMemoryService) Save(_ context.Context, entry memory.Entry) (memory.Entry, error) {
	return entry, nil
}

func (s *refreshMemoryService) Recall(_ context.Context, _ memory.Query) ([]memory.Entry, error) {
	return s.entries, nil
}

func TestIterationRefreshHook_InjectsMemories(t *testing.T) {
	svc := &refreshMemoryService{
		entries: []memory.Entry{{Content: "Remember to keep configs in YAML."}},
	}
	hook := NewIterationRefreshHook(svc, nil, IterationRefreshConfig{DefaultInterval: 1, MaxTokens: 200})

	ctx := id.WithUserID(context.Background(), "u1")
	ctx = appcontext.WithMemoryPolicy(ctx, appcontext.MemoryPolicy{
		Enabled:         true,
		RefreshEnabled:  true,
		RefreshInterval: 1,
	})

	state := &agent.TaskState{
		Messages: []ports.Message{
			{Role: "system", Content: "System prompt", Source: ports.MessageSourceSystemPrompt},
		},
		ToolResults: []ports.ToolResult{{Content: "config migration"}},
	}

	result := hook.OnIteration(ctx, state, 1)
	if result.MemoriesInjected != 1 {
		t.Fatalf("expected 1 memory injected, got %d", result.MemoriesInjected)
	}
	if len(state.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(state.Messages))
	}
	if state.Messages[1].Role != "system" || state.Messages[1].Source != ports.MessageSourceProactive {
		t.Fatalf("expected proactive system message, got role=%q source=%q", state.Messages[1].Role, state.Messages[1].Source)
	}
	if state.Messages[1].Content == "" {
		t.Fatal("expected injected content")
	}
}

func TestIterationRefreshHook_SkipsWhenPolicyDisabled(t *testing.T) {
	svc := &refreshMemoryService{
		entries: []memory.Entry{{Content: "memory"}},
	}
	hook := NewIterationRefreshHook(svc, nil, IterationRefreshConfig{DefaultInterval: 1})

	ctx := id.WithUserID(context.Background(), "u1")
	ctx = appcontext.WithMemoryPolicy(ctx, appcontext.MemoryPolicy{
		Enabled:        false,
		RefreshEnabled: false,
	})

	state := &agent.TaskState{ToolResults: []ports.ToolResult{{Content: "config"}}}
	result := hook.OnIteration(ctx, state, 1)
	if result.MemoriesInjected != 0 {
		t.Fatalf("expected no memory injected, got %d", result.MemoriesInjected)
	}
	if len(state.Messages) != 0 {
		t.Fatalf("expected no messages injected, got %d", len(state.Messages))
	}
}

func TestIterationRefreshHook_SkipsWithoutUserID(t *testing.T) {
	svc := &refreshMemoryService{
		entries: []memory.Entry{{Content: "memory"}},
	}
	hook := NewIterationRefreshHook(svc, nil, IterationRefreshConfig{DefaultInterval: 1})

	ctx := appcontext.WithMemoryPolicy(context.Background(), appcontext.MemoryPolicy{
		Enabled:         true,
		RefreshEnabled:  true,
		RefreshInterval: 1,
	})
	state := &agent.TaskState{ToolResults: []ports.ToolResult{{Content: "config"}}}

	result := hook.OnIteration(ctx, state, 1)
	if result.MemoriesInjected != 0 {
		t.Fatalf("expected no memory injected, got %d", result.MemoriesInjected)
	}
}
