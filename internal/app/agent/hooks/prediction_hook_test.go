package hooks

import (
	"context"
	"strings"
	"testing"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/infra/memory"
	runtimeconfig "alex/internal/shared/config"
	id "alex/internal/shared/utils/id"
)

func TestPredictionHookSavesPredictions(t *testing.T) {
	dir := t.TempDir()
	engine := memory.NewMarkdownEngine(dir)
	if err := engine.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	stubClient := &stubLLMClient{
		response: "- User will review test coverage\n- User may deploy to staging\n- User will check CI results",
	}
	factory := &stubLLMFactory{client: stubClient}

	hook := NewPredictionHook(engine, factory, nil, nil, PredictionConfig{
		Enabled: true,
		Profile: runtimeconfig.LLMProfile{Provider: "stub", Model: "stub"},
	})

	ctx := appcontext.WithMemoryPolicy(context.Background(), appcontext.MemoryPolicy{Enabled: true, AutoCapture: true})
	ctx = id.WithUserID(ctx, "test-user")

	err := hook.OnTaskCompleted(ctx, TaskResultInfo{
		TaskInput: "implement login feature",
		Answer:    "implemented login with OAuth2",
		UserID:    "test-user",
	})
	if err != nil {
		t.Fatalf("OnTaskCompleted: %v", err)
	}

	predictions, err := engine.LoadPredictions(context.Background(), "test-user")
	if err != nil {
		t.Fatalf("LoadPredictions: %v", err)
	}
	if len(predictions) == 0 {
		t.Fatal("expected predictions to be saved")
	}
	if len(predictions) > 3 {
		t.Errorf("expected at most 3 predictions, got %d", len(predictions))
	}
}

func TestPredictionHookDisabledSkips(t *testing.T) {
	dir := t.TempDir()
	engine := memory.NewMarkdownEngine(dir)
	_ = engine.EnsureSchema(context.Background())

	hook := NewPredictionHook(engine, &stubLLMFactory{client: &stubLLMClient{}}, nil, nil, PredictionConfig{
		Enabled: false,
		Profile: runtimeconfig.LLMProfile{Provider: "stub", Model: "stub"},
	})

	ctx := appcontext.WithMemoryPolicy(context.Background(), appcontext.MemoryPolicy{Enabled: true, AutoCapture: true})
	err := hook.OnTaskCompleted(ctx, TaskResultInfo{TaskInput: "test"})
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}

	predictions, _ := engine.LoadPredictions(context.Background(), "")
	if len(predictions) != 0 {
		t.Error("expected no predictions when disabled")
	}
}

func TestPredictionHookRecordsQueryCategory(t *testing.T) {
	dir := t.TempDir()
	engine := memory.NewMarkdownEngine(dir)
	_ = engine.EnsureSchema(context.Background())
	tracker := memory.NewQueryTracker(dir)

	hook := NewPredictionHook(engine, &stubLLMFactory{client: &stubLLMClient{
		response: "- prediction one",
	}}, tracker, nil, PredictionConfig{
		Enabled: true,
		Profile: runtimeconfig.LLMProfile{Provider: "stub", Model: "stub"},
	})

	ctx := appcontext.WithMemoryPolicy(context.Background(), appcontext.MemoryPolicy{Enabled: true, AutoCapture: true})

	_ = hook.OnTaskCompleted(ctx, TaskResultInfo{
		TaskInput: "fix the bug in user registration",
		Answer:    "fixed the null pointer",
		UserID:    "test-user",
	})

	dist, err := tracker.Load(context.Background(), "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if dist.Total != 1 {
		t.Errorf("Total = %d, want 1", dist.Total)
	}
	if dist.Counts[memory.CategoryCode] != 1 {
		t.Errorf("Code count = %d, want 1", dist.Counts[memory.CategoryCode])
	}
}

func TestPredictionHookLLMFailureNonBlocking(t *testing.T) {
	dir := t.TempDir()
	engine := memory.NewMarkdownEngine(dir)
	_ = engine.EnsureSchema(context.Background())

	hook := NewPredictionHook(engine, &stubLLMFactory{
		client: &stubLLMClient{err: context.DeadlineExceeded},
	}, nil, nil, PredictionConfig{
		Enabled: true,
		Profile: runtimeconfig.LLMProfile{Provider: "stub", Model: "stub"},
	})

	ctx := appcontext.WithMemoryPolicy(context.Background(), appcontext.MemoryPolicy{Enabled: true, AutoCapture: true})
	err := hook.OnTaskCompleted(ctx, TaskResultInfo{
		TaskInput: "some task",
		Answer:    "some answer",
	})
	// LLM failure should not propagate.
	if err != nil {
		t.Fatalf("expected nil on LLM failure, got: %v", err)
	}
}

func TestBuildPredictionPrompt(t *testing.T) {
	prompt := buildPredictionPrompt(TaskResultInfo{
		TaskInput: "implement feature X",
		Answer:    "done with tests",
		ToolCalls: []ToolResultInfo{
			{ToolName: "shell_exec", Success: true},
			{ToolName: "read_file", Success: true},
		},
	})
	if !strings.Contains(prompt, "implement feature X") {
		t.Error("expected task input in prompt")
	}
	if !strings.Contains(prompt, "shell_exec") {
		t.Error("expected tool names in prompt")
	}
}

func TestParsePredictionLines(t *testing.T) {
	input := "- First prediction\n- Second prediction\n- Third prediction\n- Fourth should be dropped"
	lines := parsePredictionLines(input)
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "First prediction" {
		t.Errorf("expected 'First prediction', got %q", lines[0])
	}
}
