package builtin

import (
	"context"
	"testing"

	"alex/internal/core/hook"
)

func TestPluginInterfaces(t *testing.T) {
	p := New()

	// Verify all interface compliance.
	var _ hook.Plugin = p
	var _ hook.SessionResolver = p
	var _ hook.StateLoader = p
	var _ hook.StateSaver = p
	var _ hook.PromptBuilder = p
	var _ hook.ModelRunner = p
	var _ hook.OutboundRenderer = p
	var _ hook.OutboundDispatcher = p
}

func TestPluginNameAndPriority(t *testing.T) {
	p := New()
	if p.Name() != "builtin" {
		t.Errorf("expected name 'builtin', got %q", p.Name())
	}
	if p.Priority() != 0 {
		t.Errorf("expected priority 0, got %d", p.Priority())
	}
}

func TestResolveSession(t *testing.T) {
	p := New()
	ctx := context.Background()

	t.Run("generates ID when empty", func(t *testing.T) {
		state := &hook.TurnState{}
		if err := p.ResolveSession(ctx, state); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if state.SessionID == "" {
			t.Error("expected non-empty session ID")
		}
		if state.RunID == "" {
			t.Error("expected non-empty run ID")
		}
	})

	t.Run("preserves existing ID", func(t *testing.T) {
		state := &hook.TurnState{SessionID: "existing"}
		if err := p.ResolveSession(ctx, state); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if state.SessionID != "existing" {
			t.Errorf("expected session ID 'existing', got %q", state.SessionID)
		}
	})
}

func TestLoadState(t *testing.T) {
	p := New()
	state := &hook.TurnState{}
	if err := p.LoadState(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSaveState(t *testing.T) {
	p := New()
	state := &hook.TurnState{}
	if err := p.SaveState(context.Background(), state); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestBuildPrompt(t *testing.T) {
	p := New()
	state := &hook.TurnState{
		Input: "hello",
		Messages: []hook.Message{
			{Role: "user", Content: "previous"},
		},
	}

	prompt, err := p.BuildPrompt(context.Background(), state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if prompt.System != "You are a helpful assistant." {
		t.Errorf("unexpected system prompt: %q", prompt.System)
	}
	// Should have previous message + current input.
	if len(prompt.Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(prompt.Messages))
	}
	if prompt.Messages[0].Content != "previous" {
		t.Errorf("expected first message 'previous', got %q", prompt.Messages[0].Content)
	}
	if prompt.Messages[1].Content != "hello" {
		t.Errorf("expected second message 'hello', got %q", prompt.Messages[1].Content)
	}
}

func TestRunModel(t *testing.T) {
	p := New()
	_, err := p.RunModel(context.Background(), &hook.TurnState{}, &hook.Prompt{})
	if err != ErrNoModelRunner {
		t.Errorf("expected ErrNoModelRunner, got %v", err)
	}
}

func TestRenderOutbound(t *testing.T) {
	p := New()
	ctx := context.Background()

	t.Run("renders text output", func(t *testing.T) {
		state := &hook.TurnState{Channel: "test", SessionID: "s1"}
		output := &hook.ModelOutput{Text: "response text"}
		outbounds, err := p.RenderOutbound(ctx, state, output)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(outbounds) != 1 {
			t.Fatalf("expected 1 outbound, got %d", len(outbounds))
		}
		if outbounds[0].Content != "response text" {
			t.Errorf("expected content 'response text', got %q", outbounds[0].Content)
		}
		if outbounds[0].Channel != "test" {
			t.Errorf("expected channel 'test', got %q", outbounds[0].Channel)
		}
	})

	t.Run("returns nil for empty output", func(t *testing.T) {
		state := &hook.TurnState{}
		outbounds, err := p.RenderOutbound(ctx, state, &hook.ModelOutput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if outbounds != nil {
			t.Errorf("expected nil outbounds, got %v", outbounds)
		}
	})

	t.Run("returns nil for nil output", func(t *testing.T) {
		state := &hook.TurnState{}
		outbounds, err := p.RenderOutbound(ctx, state, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if outbounds != nil {
			t.Errorf("expected nil outbounds, got %v", outbounds)
		}
	})
}

func TestDispatchOutbound(t *testing.T) {
	p := New()
	err := p.DispatchOutbound(context.Background(), []hook.Outbound{
		{Content: "test"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
