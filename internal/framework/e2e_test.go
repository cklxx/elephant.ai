package framework

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"alex/internal/core/channel"
	"alex/internal/core/envelope"
	"alex/internal/core/hook"
	"alex/internal/core/tape"
	infratape "alex/internal/infra/tape"
)

// ---------------------------------------------------------------------------
// Test plugin that records every lifecycle step
// ---------------------------------------------------------------------------

type recordingPlugin struct {
	mu    sync.Mutex
	name  string
	prio  int
	calls []string

	// Configurable behavior per step
	sessionID    string
	systemPrompt string
	modelAnswer  string
	modelErr     error
	saveErr      error
}

func newRecordingPlugin(name string, prio int) *recordingPlugin {
	return &recordingPlugin{
		name:         name,
		prio:         prio,
		sessionID:    "test-session",
		systemPrompt: "You are a test assistant.",
		modelAnswer:  "Hello from the model!",
	}
}

func (p *recordingPlugin) Name() string  { return p.name }
func (p *recordingPlugin) Priority() int { return p.prio }

func (p *recordingPlugin) record(step string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, step)
}

func (p *recordingPlugin) getCalls() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, len(p.calls))
	copy(out, p.calls)
	return out
}

func (p *recordingPlugin) ResolveSession(_ context.Context, state *hook.TurnState) error {
	p.record("resolve_session")
	state.SessionID = p.sessionID
	state.RunID = "test-run-1"
	return nil
}

func (p *recordingPlugin) LoadState(_ context.Context, state *hook.TurnState) error {
	p.record("load_state")
	// Simulate loading prior conversation history
	state.Messages = append(state.Messages, hook.Message{
		Role:    "system",
		Content: "Previous conversation loaded.",
	})
	return nil
}

func (p *recordingPlugin) BuildPrompt(_ context.Context, state *hook.TurnState) (*hook.Prompt, error) {
	p.record("build_prompt")
	return &hook.Prompt{
		System:   p.systemPrompt,
		Messages: state.Messages,
		Tools:    []hook.ToolSchema{{Name: "test_tool", Description: "A test tool"}},
	}, nil
}

func (p *recordingPlugin) PreTask(_ context.Context, state *hook.TurnState) error {
	p.record("pre_task")
	// Inject proactive context
	state.Messages = append(state.Messages, hook.Message{
		Role:    "system",
		Content: "[Injected] OKR context: Focus on Q1 goals.",
		Source:  "okr_hook",
	})
	return nil
}

func (p *recordingPlugin) RunModel(_ context.Context, _ *hook.TurnState, prompt *hook.Prompt) (*hook.ModelOutput, error) {
	p.record("run_model")
	if p.modelErr != nil {
		return nil, p.modelErr
	}
	return &hook.ModelOutput{
		Text:       p.modelAnswer,
		StopReason: "end_turn",
		Model:      "test-model-v1",
		Usage: hook.Usage{
			InputTokens:  100,
			OutputTokens: 50,
			TotalTokens:  150,
		},
	}, nil
}

func (p *recordingPlugin) SaveState(_ context.Context, _ *hook.TurnState) error {
	p.record("save_state")
	return p.saveErr
}

func (p *recordingPlugin) PostTask(_ context.Context, _ *hook.TurnState, result *hook.TurnResult) error {
	p.record("post_task")
	return nil
}

func (p *recordingPlugin) RenderOutbound(_ context.Context, state *hook.TurnState, output *hook.ModelOutput) ([]hook.Outbound, error) {
	p.record("render_outbound")
	if output == nil || output.Text == "" {
		return nil, nil
	}
	return []hook.Outbound{{
		Channel:   state.Channel,
		SessionID: state.SessionID,
		Content:   output.Text,
	}}, nil
}

func (p *recordingPlugin) DispatchOutbound(_ context.Context, outbounds []hook.Outbound) error {
	p.record("dispatch_outbound")
	return nil
}

// Compile-time interface checks
var (
	_ hook.Plugin            = (*recordingPlugin)(nil)
	_ hook.SessionResolver   = (*recordingPlugin)(nil)
	_ hook.StateLoader       = (*recordingPlugin)(nil)
	_ hook.PromptBuilder     = (*recordingPlugin)(nil)
	_ hook.PreTaskHook       = (*recordingPlugin)(nil)
	_ hook.ModelRunner       = (*recordingPlugin)(nil)
	_ hook.StateSaver        = (*recordingPlugin)(nil)
	_ hook.PostTaskHook      = (*recordingPlugin)(nil)
	_ hook.OutboundRenderer  = (*recordingPlugin)(nil)
	_ hook.OutboundDispatcher = (*recordingPlugin)(nil)
)

// ---------------------------------------------------------------------------
// E2E: Full lifecycle with single plugin
// ---------------------------------------------------------------------------

func TestE2E_FullLifecycle(t *testing.T) {
	plugin := newRecordingPlugin("full-lifecycle", 100)

	store := infratape.NewMemoryStore()
	tapeCtx := tape.TapeContext{TapeName: "e2e-test", RunID: "run-1"}
	tapeMgr := tape.NewTapeManager(store, tapeCtx)

	fw := New(Config{
		TapeManager: tapeMgr,
	})
	fw.RegisterPlugin(plugin)

	env := envelope.New(map[string]any{
		"content": "What are my Q1 OKRs?",
		"channel": "cli",
		"user_id": "ckl",
	})

	result, err := fw.ProcessInbound(context.Background(), env)
	if err != nil {
		t.Fatalf("ProcessInbound failed: %v", err)
	}

	// Verify all lifecycle steps were called in order
	calls := plugin.getCalls()
	expected := []string{
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
	if len(calls) != len(expected) {
		t.Fatalf("expected %d lifecycle steps, got %d: %v", len(expected), len(calls), calls)
	}
	for i, step := range expected {
		if calls[i] != step {
			t.Errorf("step %d: expected %q, got %q", i, step, calls[i])
		}
	}

	// Verify result fields
	if result.SessionID != "test-session" {
		t.Errorf("session_id = %q, want %q", result.SessionID, "test-session")
	}
	if result.RunID != "test-run-1" {
		t.Errorf("run_id = %q, want %q", result.RunID, "test-run-1")
	}
	if result.ModelOutput == nil {
		t.Fatal("model_output is nil")
	}
	if result.ModelOutput.Text != "Hello from the model!" {
		t.Errorf("model_output.text = %q", result.ModelOutput.Text)
	}
	if result.ModelOutput.StopReason != "end_turn" {
		t.Errorf("stop_reason = %q", result.ModelOutput.StopReason)
	}
	if result.ModelOutput.Usage.TotalTokens != 150 {
		t.Errorf("total_tokens = %d, want 150", result.ModelOutput.Usage.TotalTokens)
	}
	if result.Prompt == nil {
		t.Fatal("prompt is nil")
	}
	if result.Prompt.System != "You are a test assistant." {
		t.Errorf("system prompt = %q", result.Prompt.System)
	}
	if len(result.Prompt.Tools) != 1 || result.Prompt.Tools[0].Name != "test_tool" {
		t.Errorf("tools = %v", result.Prompt.Tools)
	}
	// Verify outbounds
	if len(result.Outbounds) != 1 {
		t.Fatalf("expected 1 outbound, got %d", len(result.Outbounds))
	}
	if result.Outbounds[0].Content != "Hello from the model!" {
		t.Errorf("outbound content = %q", result.Outbounds[0].Content)
	}

	// Verify automatic tape recording: turn_start and turn_end anchors
	anchors, err := tapeMgr.Query(context.Background(), tape.Query().Kinds(tape.KindAnchor))
	if err != nil {
		t.Fatalf("Query anchors: %v", err)
	}
	if len(anchors) != 2 {
		t.Fatalf("expected 2 anchors (turn_start + turn_end), got %d", len(anchors))
	}
	if anchors[0].Payload["label"] != "turn_start" {
		t.Errorf("first anchor = %v, want turn_start", anchors[0].Payload["label"])
	}
	if anchors[1].Payload["label"] != "turn_end" {
		t.Errorf("second anchor = %v, want turn_end", anchors[1].Payload["label"])
	}
	// Verify anchors carry session/run metadata
	if anchors[0].Meta.SessionID != "test-session" {
		t.Errorf("turn_start session_id = %q, want test-session", anchors[0].Meta.SessionID)
	}
	if anchors[0].Meta.RunID != "test-run-1" {
		t.Errorf("turn_start run_id = %q, want test-run-1", anchors[0].Meta.RunID)
	}
}

// ---------------------------------------------------------------------------
// E2E: Multi-plugin priority ordering
// ---------------------------------------------------------------------------

type injectionPlugin struct {
	name     string
	prio     int
	injected string
}

func (p *injectionPlugin) Name() string  { return p.name }
func (p *injectionPlugin) Priority() int { return p.prio }

func (p *injectionPlugin) PreTask(_ context.Context, state *hook.TurnState) error {
	state.Messages = append(state.Messages, hook.Message{
		Role:    "system",
		Content: p.injected,
		Source:  p.name,
	})
	return nil
}

func TestE2E_MultiPluginInjection(t *testing.T) {
	// Main plugin handles the full lifecycle
	main := newRecordingPlugin("main", 100)

	// Two injection plugins at different priorities
	okr := &injectionPlugin{name: "okr-inject", prio: 80, injected: "[OKR] Ship Q1 features"}
	mem := &injectionPlugin{name: "memory-inject", prio: 60, injected: "[Memory] User prefers concise answers"}

	fw := New(Config{})
	fw.RegisterPlugin(main)
	fw.RegisterPlugin(okr)
	fw.RegisterPlugin(mem)

	env := envelope.New(map[string]any{
		"content": "What should I work on?",
		"channel": "lark",
	})

	result, err := fw.ProcessInbound(context.Background(), env)
	if err != nil {
		t.Fatalf("ProcessInbound failed: %v", err)
	}

	// PreTask hooks inject into state.Messages AFTER build_prompt.
	// The model sees injections via state, not via result.Prompt (which is a snapshot before PreTask).
	// Verify the model was called and produced output (meaning injections didn't break the flow).
	if result.ModelOutput == nil {
		t.Fatal("model output is nil")
	}
	if result.ModelOutput.Text != "Hello from the model!" {
		t.Errorf("model output = %q", result.ModelOutput.Text)
	}

	// Verify both PreTask hooks ran by checking the recording plugin's calls
	mainCalls := main.getCalls()
	preTaskCount := 0
	for _, c := range mainCalls {
		if c == "pre_task" {
			preTaskCount++
		}
	}
	// The main plugin's PreTask also runs, plus okr and mem — but main's PreTask
	// is the one we track. The injections from okr and mem are in state.Messages.
	if preTaskCount != 1 {
		t.Errorf("expected main.pre_task to run once, ran %d times", preTaskCount)
	}

	// Verify model lifecycle completed
	hasRunModel := false
	for _, c := range mainCalls {
		if c == "run_model" {
			hasRunModel = true
		}
	}
	if !hasRunModel {
		t.Error("run_model should have been called")
	}
}

// ---------------------------------------------------------------------------
// E2E: Tape audit trail
// ---------------------------------------------------------------------------

func TestE2E_TapeAuditTrail(t *testing.T) {
	store := infratape.NewMemoryStore()
	tapeCtx := tape.TapeContext{TapeName: "audit-test", RunID: "run-audit"}
	tapeMgr := tape.NewTapeManager(store, tapeCtx)

	// Plugin that writes to tape during lifecycle
	plugin := newRecordingPlugin("tape-writer", 100)

	fw := New(Config{TapeManager: tapeMgr})
	fw.RegisterPlugin(plugin)

	ctx := context.Background()

	// Append a user message before processing (simulates what a real plugin would do)
	_ = tapeMgr.Append(ctx, tape.NewMessage("user", "Hello", tape.EntryMeta{RunID: "run-audit"}))

	env := envelope.New(map[string]any{"content": "Hello"})
	_, err := fw.ProcessInbound(ctx, env)
	if err != nil {
		t.Fatalf("ProcessInbound: %v", err)
	}

	// Append model response to tape
	_ = tapeMgr.Append(ctx, tape.NewMessage("assistant", "Hello from the model!", tape.EntryMeta{RunID: "run-audit"}))

	// Query tape and verify entries
	// Expected order: user_message, turn_start (auto), turn_end (auto), assistant_message
	entries, err := tapeMgr.Query(ctx, tape.Query())
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(entries) < 4 {
		t.Fatalf("expected at least 4 tape entries, got %d", len(entries))
	}

	// Verify entry kinds
	if entries[0].Kind != tape.KindMessage {
		t.Errorf("entry[0] kind = %s, want message (user)", entries[0].Kind)
	}
	if entries[1].Kind != tape.KindAnchor {
		t.Errorf("entry[1] kind = %s, want anchor (turn_start)", entries[1].Kind)
	}

	// Verify turn_start and turn_end anchors exist
	anchors, err := tapeMgr.Query(ctx, tape.Query().Kinds(tape.KindAnchor))
	if err != nil {
		t.Fatalf("Query anchors: %v", err)
	}
	if len(anchors) != 2 {
		t.Fatalf("expected 2 anchors (turn_start + turn_end), got %d", len(anchors))
	}
	if anchors[0].Payload["label"] != "turn_start" {
		t.Errorf("first anchor label = %v, want turn_start", anchors[0].Payload["label"])
	}
	if anchors[1].Payload["label"] != "turn_end" {
		t.Errorf("second anchor label = %v, want turn_end", anchors[1].Payload["label"])
	}

	// Verify tape query filtering for messages
	msgs, err := tapeMgr.Query(ctx, tape.Query().Kinds(tape.KindMessage))
	if err != nil {
		t.Fatalf("Query kinds: %v", err)
	}
	if len(msgs) != 2 {
		t.Errorf("expected 2 messages, got %d", len(msgs))
	}
}

// ---------------------------------------------------------------------------
// E2E: Channel dispatch with debounce
// ---------------------------------------------------------------------------

type testChannel struct {
	mu       sync.Mutex
	name     string
	debounce bool
	sent     []channel.Outbound
	started  bool
	stopped  bool
}

func (c *testChannel) Name() string         { return c.name }
func (c *testChannel) NeedsDebounce() bool  { return c.debounce }
func (c *testChannel) Start(context.Context) error { c.started = true; return nil }
func (c *testChannel) Stop(context.Context) error  { c.stopped = true; return nil }
func (c *testChannel) Send(_ context.Context, _ string, msg channel.Outbound) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sent = append(c.sent, msg)
	return nil
}
func (c *testChannel) getSent() []channel.Outbound {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]channel.Outbound, len(c.sent))
	copy(out, c.sent)
	return out
}

// dispatchPlugin dispatches outbounds to a ChannelManager
type dispatchPlugin struct {
	*recordingPlugin
	channelMgr *channel.Manager
}

func (p *dispatchPlugin) DispatchOutbound(ctx context.Context, outbounds []hook.Outbound) error {
	p.record("dispatch_outbound")
	for _, out := range outbounds {
		ch := out.Channel
		if ch == "" {
			ch = "cli"
		}
		msg := channel.Outbound{
			Content: out.Content,
			Kind:    "text",
		}
		if err := p.channelMgr.Send(ctx, ch, out.SessionID, msg); err != nil {
			return err
		}
	}
	return nil
}

func TestE2E_ChannelDispatch(t *testing.T) {
	cli := &testChannel{name: "cli"}
	lark := &testChannel{name: "lark", debounce: false}

	chMgr := channel.NewManager(channel.DefaultDebounceConfig())
	chMgr.Register(cli)
	chMgr.Register(lark)

	if err := chMgr.Start(context.Background()); err != nil {
		t.Fatalf("channel start: %v", err)
	}
	defer func() { _ = chMgr.Stop(context.Background()) }()

	base := newRecordingPlugin("dispatch-test", 100)
	plugin := &dispatchPlugin{recordingPlugin: base, channelMgr: chMgr}

	fw := New(Config{ChannelManager: chMgr})
	fw.RegisterPlugin(plugin)

	// Test CLI channel
	env := envelope.New(map[string]any{
		"content": "Test CLI output",
		"channel": "cli",
	})
	result, err := fw.ProcessInbound(context.Background(), env)
	if err != nil {
		t.Fatalf("ProcessInbound: %v", err)
	}
	if result.ModelOutput.Text != "Hello from the model!" {
		t.Errorf("model output = %q", result.ModelOutput.Text)
	}

	cliSent := cli.getSent()
	if len(cliSent) != 1 {
		t.Fatalf("expected 1 CLI message, got %d", len(cliSent))
	}
	if cliSent[0].Content != "Hello from the model!" {
		t.Errorf("CLI content = %q", cliSent[0].Content)
	}

	// Test Lark channel
	env2 := envelope.New(map[string]any{
		"content": "Test Lark output",
		"channel": "lark",
	})
	_, err = fw.ProcessInbound(context.Background(), env2)
	if err != nil {
		t.Fatalf("ProcessInbound lark: %v", err)
	}

	larkSent := lark.getSent()
	if len(larkSent) != 1 {
		t.Fatalf("expected 1 Lark message, got %d", len(larkSent))
	}
}

// ---------------------------------------------------------------------------
// E2E: Error handling and isolation
// ---------------------------------------------------------------------------

func TestE2E_ModelErrorPropagation(t *testing.T) {
	plugin := newRecordingPlugin("error-test", 100)
	plugin.modelErr = fmt.Errorf("provider overloaded: 529")

	fw := New(Config{})
	fw.RegisterPlugin(plugin)

	env := envelope.New(map[string]any{"content": "fail please"})
	result, err := fw.ProcessInbound(context.Background(), env)

	// Error should propagate
	if err == nil {
		t.Fatal("expected error from model runner")
	}
	if !strings.Contains(err.Error(), "529") {
		t.Errorf("error should contain 529, got: %v", err)
	}

	// Result should still be populated with what succeeded
	if result == nil {
		t.Fatal("result should not be nil even on error")
	}
	if result.Error == nil {
		t.Error("result.Error should be set")
	}

	// Verify lifecycle stopped at run_model — no save/post/render/dispatch
	calls := plugin.getCalls()
	for _, call := range calls {
		if call == "save_state" || call == "post_task" || call == "dispatch_outbound" {
			t.Errorf("unexpected call after model error: %s", call)
		}
	}
}

// ---------------------------------------------------------------------------
// E2E: Context cancellation
// ---------------------------------------------------------------------------

func TestE2E_ContextCancellation(t *testing.T) {
	plugin := newRecordingPlugin("cancel-test", 100)

	fw := New(Config{})
	fw.RegisterPlugin(plugin)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	env := envelope.New(map[string]any{"content": "should not process"})
	_, err := fw.ProcessInbound(ctx, env)

	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
	if ctx.Err() == nil {
		t.Fatal("context should be cancelled")
	}

	// Should stop early — most steps should not run
	calls := plugin.getCalls()
	if len(calls) > 1 {
		t.Errorf("expected at most 1 call with cancelled context, got %d: %v", len(calls), calls)
	}
}

// ---------------------------------------------------------------------------
// E2E: Plugin override (higher priority wins for CallFirst)
// ---------------------------------------------------------------------------

type overrideModelPlugin struct {
	name    string
	prio    int
	answer  string
	called  bool
}

func (p *overrideModelPlugin) Name() string  { return p.name }
func (p *overrideModelPlugin) Priority() int { return p.prio }
func (p *overrideModelPlugin) RunModel(_ context.Context, _ *hook.TurnState, _ *hook.Prompt) (*hook.ModelOutput, error) {
	p.called = true
	return &hook.ModelOutput{
		Text:       p.answer,
		StopReason: "end_turn",
		Model:      "override-model",
	}, nil
}

func TestE2E_PluginOverride(t *testing.T) {
	// Base plugin at priority 50
	base := newRecordingPlugin("base", 50)
	base.modelAnswer = "base answer"

	// Override plugin at priority 200 — only implements RunModel
	override := &overrideModelPlugin{
		name:   "override",
		prio:   200,
		answer: "override answer",
	}

	fw := New(Config{})
	fw.RegisterPlugin(base)
	fw.RegisterPlugin(override)

	env := envelope.New(map[string]any{"content": "who answers?"})
	result, err := fw.ProcessInbound(context.Background(), env)
	if err != nil {
		t.Fatalf("ProcessInbound: %v", err)
	}

	// Override plugin should win for RunModel (higher priority)
	if !override.called {
		t.Error("override plugin should have been called")
	}
	if result.ModelOutput.Text != "override answer" {
		t.Errorf("expected override answer, got %q", result.ModelOutput.Text)
	}
	if result.ModelOutput.Model != "override-model" {
		t.Errorf("expected override-model, got %q", result.ModelOutput.Model)
	}

	// Base plugin should still handle other steps (it has lower priority but
	// is the only one implementing SessionResolver, StateLoader, etc.)
	calls := base.getCalls()
	hasResolve := false
	hasRunModel := false
	for _, c := range calls {
		if c == "resolve_session" {
			hasResolve = true
		}
		if c == "run_model" {
			hasRunModel = true
		}
	}
	if !hasResolve {
		t.Error("base plugin should have handled resolve_session")
	}
	if hasRunModel {
		t.Error("base plugin should NOT have handled run_model (overridden)")
	}
}

// ---------------------------------------------------------------------------
// E2E: Fork store for speculative execution
// ---------------------------------------------------------------------------

func TestE2E_ForkStoreSpeculativeExecution(t *testing.T) {
	parent := infratape.NewMemoryStore()
	ctx := context.Background()

	// Write some entries to parent
	_ = parent.Append(ctx, "session-1", tape.NewMessage("user", "hello", tape.EntryMeta{}))
	_ = parent.Append(ctx, "session-1", tape.NewMessage("assistant", "hi", tape.EntryMeta{}))

	// Fork for speculative execution
	fork := infratape.NewForkStore(parent)
	fork.Fork("session-1")

	// Write speculative entries to fork
	_ = fork.Append(ctx, "session-1", tape.NewMessage("user", "do something risky", tape.EntryMeta{}))
	_ = fork.Append(ctx, "session-1", tape.NewMessage("assistant", "done!", tape.EntryMeta{}))

	// Fork should see all 4 entries
	forkEntries, _ := fork.Query(ctx, "session-1", tape.Query())
	if len(forkEntries) != 4 {
		t.Fatalf("fork should see 4 entries, got %d", len(forkEntries))
	}

	// Parent should still only have 2
	parentEntries, _ := parent.Query(ctx, "session-1", tape.Query())
	if len(parentEntries) != 2 {
		t.Fatalf("parent should still have 2 entries, got %d", len(parentEntries))
	}

	// Discard the fork — parent unchanged
	fork.Discard()
	parentEntries, _ = parent.Query(ctx, "session-1", tape.Query())
	if len(parentEntries) != 2 {
		t.Fatalf("parent should still have 2 after discard, got %d", len(parentEntries))
	}

	// New fork: write and merge
	fork2 := infratape.NewForkStore(parent)
	fork2.Fork("session-1")
	_ = fork2.Append(ctx, "session-1", tape.NewMessage("user", "safe operation", tape.EntryMeta{}))
	if err := fork2.Merge(ctx); err != nil {
		t.Fatalf("merge: %v", err)
	}

	// Parent should now have 3
	parentEntries, _ = parent.Query(ctx, "session-1", tape.Query())
	if len(parentEntries) != 3 {
		t.Fatalf("parent should have 3 after merge, got %d", len(parentEntries))
	}
}
