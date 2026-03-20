package framework

import (
	"context"
	"testing"

	"alex/internal/core/envelope"
	"alex/internal/core/hook"
)

// testPlugin implements all hook interfaces and tracks call order.
type testPlugin struct {
	name     string
	priority int
	calls    []string
	output   *hook.ModelOutput
}

func (p *testPlugin) Name() string   { return p.name }
func (p *testPlugin) Priority() int  { return p.priority }

func (p *testPlugin) ResolveSession(_ context.Context, state *hook.TurnState) error {
	p.calls = append(p.calls, "resolve_session")
	state.SessionID = "test-session"
	state.RunID = "test-run"
	return nil
}

func (p *testPlugin) LoadState(_ context.Context, _ *hook.TurnState) error {
	p.calls = append(p.calls, "load_state")
	return nil
}

func (p *testPlugin) BuildPrompt(_ context.Context, state *hook.TurnState) (*hook.Prompt, error) {
	p.calls = append(p.calls, "build_prompt")
	return &hook.Prompt{
		System:   "test system",
		Messages: []hook.Message{{Role: "user", Content: state.Input}},
	}, nil
}

func (p *testPlugin) PreTask(_ context.Context, _ *hook.TurnState) error {
	p.calls = append(p.calls, "pre_task")
	return nil
}

func (p *testPlugin) RunModel(_ context.Context, _ *hook.TurnState, _ *hook.Prompt) (*hook.ModelOutput, error) {
	p.calls = append(p.calls, "run_model")
	if p.output != nil {
		return p.output, nil
	}
	return &hook.ModelOutput{Text: "test response"}, nil
}

func (p *testPlugin) SaveState(_ context.Context, _ *hook.TurnState) error {
	p.calls = append(p.calls, "save_state")
	return nil
}

func (p *testPlugin) PostTask(_ context.Context, _ *hook.TurnState, _ *hook.TurnResult) error {
	p.calls = append(p.calls, "post_task")
	return nil
}

func (p *testPlugin) RenderOutbound(_ context.Context, state *hook.TurnState, output *hook.ModelOutput) ([]hook.Outbound, error) {
	p.calls = append(p.calls, "render_outbound")
	return []hook.Outbound{{
		Channel:   state.Channel,
		SessionID: state.SessionID,
		Content:   output.Text,
	}}, nil
}

func (p *testPlugin) DispatchOutbound(_ context.Context, _ []hook.Outbound) error {
	p.calls = append(p.calls, "dispatch_outbound")
	return nil
}

// Verify test plugin implements all interfaces.
var (
	_ hook.Plugin             = (*testPlugin)(nil)
	_ hook.SessionResolver    = (*testPlugin)(nil)
	_ hook.StateLoader        = (*testPlugin)(nil)
	_ hook.PromptBuilder      = (*testPlugin)(nil)
	_ hook.PreTaskHook        = (*testPlugin)(nil)
	_ hook.ModelRunner        = (*testPlugin)(nil)
	_ hook.StateSaver         = (*testPlugin)(nil)
	_ hook.PostTaskHook       = (*testPlugin)(nil)
	_ hook.OutboundRenderer   = (*testPlugin)(nil)
	_ hook.OutboundDispatcher = (*testPlugin)(nil)
)

func TestFrameworkCreation(t *testing.T) {
	fw := New(Config{})
	if fw == nil {
		t.Fatal("expected non-nil Framework")
	}
	if fw.Hooks() == nil {
		t.Fatal("expected non-nil HookRuntime")
	}
}

func TestPluginRegistration(t *testing.T) {
	fw := New(Config{})
	p := &testPlugin{name: "test", priority: 10}
	fw.RegisterPlugin(p)

	plugins := fw.Hooks().Plugins()
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin, got %d", len(plugins))
	}
	if plugins[0].Name() != "test" {
		t.Fatalf("expected plugin name 'test', got %q", plugins[0].Name())
	}
}

func TestProcessInboundLifecycle(t *testing.T) {
	fw := New(Config{})
	p := &testPlugin{name: "test", priority: 10}
	fw.RegisterPlugin(p)

	env := envelope.New(map[string]any{
		"content": "hello",
		"channel": "test-channel",
		"user_id": "user-1",
	})

	result, err := fw.ProcessInbound(context.Background(), env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.SessionID != "test-session" {
		t.Errorf("expected session ID 'test-session', got %q", result.SessionID)
	}
	if result.RunID != "test-run" {
		t.Errorf("expected run ID 'test-run', got %q", result.RunID)
	}
	if result.Input != "hello" {
		t.Errorf("expected input 'hello', got %q", result.Input)
	}

	// Verify model output was captured.
	if result.ModelOutput == nil || result.ModelOutput.Text != "test response" {
		t.Error("expected model output with 'test response'")
	}

	// Verify outbounds were rendered.
	if len(result.Outbounds) != 1 || result.Outbounds[0].Content != "test response" {
		t.Errorf("expected 1 outbound with 'test response', got %v", result.Outbounds)
	}

	// Verify lifecycle steps were called in order.
	expectedOrder := []string{
		"resolve_session",
		"load_state",
		"build_prompt",
		"pre_task",
		"run_model",
		"save_state",
		"post_task",
		"render_outbound",
		"dispatch_outbound",
	}

	if len(p.calls) != len(expectedOrder) {
		t.Fatalf("expected %d calls, got %d: %v", len(expectedOrder), len(p.calls), p.calls)
	}
	for i, expected := range expectedOrder {
		if p.calls[i] != expected {
			t.Errorf("step %d: expected %q, got %q", i, expected, p.calls[i])
		}
	}
}

func TestMultiplePluginPriority(t *testing.T) {
	fw := New(Config{})
	low := &testPlugin{name: "low", priority: 0}
	high := &testPlugin{name: "high", priority: 100}

	fw.RegisterPlugin(low)
	fw.RegisterPlugin(high)

	env := envelope.New(map[string]any{"content": "hello"})

	_, err := fw.ProcessInbound(context.Background(), env)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// High priority plugin should handle CallFirst hooks (session, prompt, model, render).
	// Both should handle CallMany hooks (load_state, save_state, dispatch).
	if len(high.calls) == 0 {
		t.Error("expected high-priority plugin to be called")
	}
	// The high-priority plugin should handle resolve_session first.
	if high.calls[0] != "resolve_session" {
		t.Errorf("expected high-priority to handle resolve_session first, got %q", high.calls[0])
	}
}
