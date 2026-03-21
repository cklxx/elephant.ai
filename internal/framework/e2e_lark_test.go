package framework

import (
	"context"
	"strings"
	"sync"
	"testing"

	"alex/internal/core/channel"
	"alex/internal/core/envelope"
	"alex/internal/core/hook"
)

// ---------------------------------------------------------------------------
// Lark-specific test plugin that simulates the full Lark message flow
// ---------------------------------------------------------------------------

type larkTestPlugin struct {
	mu    sync.Mutex
	calls []string

	// Simulated state
	sessionMessages []hook.Message
	injectedOKR     bool
	injectedMemory  bool
	modelCalled     bool
	savedState      bool
	dispatched      []hook.Outbound
}

func newLarkTestPlugin() *larkTestPlugin {
	return &larkTestPlugin{}
}

func (p *larkTestPlugin) Name() string  { return "lark-e2e" }
func (p *larkTestPlugin) Priority() int { return 100 }

func (p *larkTestPlugin) record(step string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls = append(p.calls, step)
}

func (p *larkTestPlugin) getCalls() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, len(p.calls))
	copy(out, p.calls)
	return out
}

// ResolveSession — simulates Lark session resolution (lark-{chatID} pattern)
func (p *larkTestPlugin) ResolveSession(_ context.Context, state *hook.TurnState) error {
	p.record("resolve_session")

	chatID, _ := state.Metadata["chat_id"].(string)
	if chatID == "" {
		chatID = "default"
	}
	state.SessionID = "lark-" + chatID
	state.RunID = "run-lark-1"
	state.Channel = "lark"

	// Extract user ID from envelope
	if uid, ok := state.Metadata["user_id"].(string); ok {
		state.UserID = uid
	}

	return nil
}

// LoadState — simulates loading prior conversation + chat context enrichment
func (p *larkTestPlugin) LoadState(_ context.Context, state *hook.TurnState) error {
	p.record("load_state")

	// Simulate loading prior conversation history from session store
	p.sessionMessages = []hook.Message{
		{Role: "user", Content: "之前我们讨论了 Q1 目标"},
		{Role: "assistant", Content: "好的，Q1 的三个关键目标是..."},
	}
	state.Messages = append(state.Messages, p.sessionMessages...)

	// Simulate chat context enrichment (recent IM rounds)
	// This mirrors enrichWithChatContext in task_manager_exec.go
	chatContext := state.GetString("chat_context")
	if chatContext != "" {
		state.Messages = append(state.Messages, hook.Message{
			Role:    "user",
			Content: "[Recent Chat Context]\n" + chatContext,
			Source:  "chat_enrichment",
		})
	}

	return nil
}

// BuildPrompt — simulates prompt building with Lark channel hints
func (p *larkTestPlugin) BuildPrompt(_ context.Context, state *hook.TurnState) (*hook.Prompt, error) {
	p.record("build_prompt")

	return &hook.Prompt{
		System: "You are Alex, an AI assistant deployed on Lark. " +
			"Respond in Chinese. Keep messages under 800 chars for Lark compatibility.",
		Messages: state.Messages,
		Tools: []hook.ToolSchema{
			{Name: "web_search", Description: "Search the web"},
			{Name: "lark_docx", Description: "Read/write Lark documents"},
			{Name: "dispatch_background_task", Description: "Dispatch async task"},
		},
	}, nil
}

// PreTask — simulates OKR context injection (okr_context hook)
func (p *larkTestPlugin) PreTask(_ context.Context, state *hook.TurnState) error {
	p.record("pre_task")

	// Simulate OKR context injection (from hooks/okr_context.go)
	state.Messages = append(state.Messages, hook.Message{
		Role:    "system",
		Content: "[OKR Context] Q1 关键目标: 1. 完成架构重构 2. 上线 Lark 集成 3. 提升测试覆盖率",
		Source:  "okr_hook",
	})
	p.mu.Lock()
	p.injectedOKR = true
	p.mu.Unlock()

	return nil
}

// ModelRunner — simulates LLM response
func (p *larkTestPlugin) RunModel(_ context.Context, state *hook.TurnState, prompt *hook.Prompt) (*hook.ModelOutput, error) {
	p.record("run_model")
	p.mu.Lock()
	p.modelCalled = true
	p.mu.Unlock()

	// Verify injections are visible to model
	hasOKR := false
	hasChatHistory := false
	for _, msg := range state.Messages {
		if strings.Contains(msg.Content, "[OKR Context]") {
			hasOKR = true
		}
		if strings.Contains(msg.Content, "Q1 目标") {
			hasChatHistory = true
		}
	}

	answer := "Q1 架构重构进展：tape + hook-first 架构已完成"
	if hasOKR {
		answer += "（已结合 OKR 上下文）"
	}
	if hasChatHistory {
		answer += "（已参考对话历史）"
	}

	return &hook.ModelOutput{
		Text:       answer,
		StopReason: "end_turn",
		Model:      "claude-sonnet-4-20250514",
		Usage: hook.Usage{
			InputTokens:  500,
			OutputTokens: 120,
			TotalTokens:  620,
		},
	}, nil
}

// StateSaver — simulates session persistence
func (p *larkTestPlugin) SaveState(_ context.Context, state *hook.TurnState) error {
	p.record("save_state")
	p.mu.Lock()
	p.savedState = true
	p.mu.Unlock()
	return nil
}

// PostTask — simulates memory capture + cost logging
func (p *larkTestPlugin) PostTask(_ context.Context, state *hook.TurnState, result *hook.TurnResult) error {
	p.record("post_task")

	// Simulate memory capture hook
	if result != nil && result.ModelOutput != nil {
		state.Set("memory_captured", true)
		p.mu.Lock()
		p.injectedMemory = true
		p.mu.Unlock()
	}

	return nil
}

// OutboundRenderer — simulates Lark reply formatting
func (p *larkTestPlugin) RenderOutbound(_ context.Context, state *hook.TurnState, output *hook.ModelOutput) ([]hook.Outbound, error) {
	p.record("render_outbound")

	if output == nil || output.Text == "" {
		return nil, nil
	}

	// Simulate Lark reply shaping (smartContent)
	content := output.Text

	// Lark messages must be ≤800 chars — simulate chunking
	var outbounds []hook.Outbound
	for len(content) > 0 {
		chunk := content
		if len(chunk) > 800 {
			chunk = content[:800]
			content = content[800:]
		} else {
			content = ""
		}

		outbounds = append(outbounds, hook.Outbound{
			Channel:   "lark",
			SessionID: state.SessionID,
			Content:   chunk,
			Metadata: map[string]any{
				"chat_id":    state.Metadata["chat_id"],
				"msg_type":   "text",
				"reply_to":   state.Metadata["message_id"],
				"user_id":    state.UserID,
			},
		})
	}

	return outbounds, nil
}

// OutboundDispatcher — simulates Lark API dispatch
func (p *larkTestPlugin) DispatchOutbound(_ context.Context, outbounds []hook.Outbound) error {
	p.record("dispatch_outbound")
	p.mu.Lock()
	p.dispatched = append(p.dispatched, outbounds...)
	p.mu.Unlock()
	return nil
}

// Compile-time checks
var (
	_ hook.Plugin            = (*larkTestPlugin)(nil)
	_ hook.SessionResolver   = (*larkTestPlugin)(nil)
	_ hook.StateLoader       = (*larkTestPlugin)(nil)
	_ hook.PromptBuilder     = (*larkTestPlugin)(nil)
	_ hook.PreTaskHook       = (*larkTestPlugin)(nil)
	_ hook.ModelRunner       = (*larkTestPlugin)(nil)
	_ hook.StateSaver        = (*larkTestPlugin)(nil)
	_ hook.PostTaskHook      = (*larkTestPlugin)(nil)
	_ hook.OutboundRenderer  = (*larkTestPlugin)(nil)
	_ hook.OutboundDispatcher = (*larkTestPlugin)(nil)
)

// ---------------------------------------------------------------------------
// E2E: Lark message → Framework → Lark reply
// ---------------------------------------------------------------------------

func TestE2E_LarkMessageInjection(t *testing.T) {
	plugin := newLarkTestPlugin()

	fw := New(Config{})
	fw.RegisterPlugin(plugin)

	// Simulate a Lark P2MessageReceiveV1 event
	env := envelope.New(map[string]any{
		"content":    "Q1 架构重构的进展如何？",
		"channel":    "lark",
		"chat_id":    "oc_abc123",
		"message_id": "om_xyz789",
		"user_id":    "ou_ckl",
		"is_group":   true,
	})

	result, err := fw.ProcessInbound(context.Background(), env)
	if err != nil {
		t.Fatalf("ProcessInbound failed: %v", err)
	}

	// 1. Verify all lifecycle steps ran in order
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
		t.Fatalf("expected %d steps, got %d: %v", len(expected), len(calls), calls)
	}
	for i, step := range expected {
		if calls[i] != step {
			t.Errorf("step %d: expected %q, got %q", i, step, calls[i])
		}
	}

	// 2. Verify session resolution (Lark pattern: lark-{chatID})
	if result.SessionID != "lark-oc_abc123" {
		t.Errorf("session_id = %q, want %q", result.SessionID, "lark-oc_abc123")
	}

	// 3. Verify OKR context was injected
	plugin.mu.Lock()
	okrInjected := plugin.injectedOKR
	plugin.mu.Unlock()
	if !okrInjected {
		t.Error("OKR context should have been injected in PreTask")
	}

	// 4. Verify model received injections (check output contains confirmation)
	if result.ModelOutput == nil {
		t.Fatal("model output is nil")
	}
	if !strings.Contains(result.ModelOutput.Text, "OKR 上下文") {
		t.Errorf("model should have seen OKR injection, got: %s", result.ModelOutput.Text)
	}
	if !strings.Contains(result.ModelOutput.Text, "对话历史") {
		t.Errorf("model should have seen chat history, got: %s", result.ModelOutput.Text)
	}

	// 5. Verify state was saved
	plugin.mu.Lock()
	saved := plugin.savedState
	plugin.mu.Unlock()
	if !saved {
		t.Error("state should have been saved")
	}

	// 6. Verify memory capture hook ran
	plugin.mu.Lock()
	memoryCaptured := plugin.injectedMemory
	plugin.mu.Unlock()
	if !memoryCaptured {
		t.Error("memory capture should have run in PostTask")
	}

	// 7. Verify outbound was dispatched to Lark
	plugin.mu.Lock()
	dispatched := make([]hook.Outbound, len(plugin.dispatched))
	copy(dispatched, plugin.dispatched)
	plugin.mu.Unlock()

	if len(dispatched) == 0 {
		t.Fatal("expected at least 1 dispatched outbound")
	}
	out := dispatched[0]
	if out.Channel != "lark" {
		t.Errorf("outbound channel = %q, want lark", out.Channel)
	}
	if out.SessionID != "lark-oc_abc123" {
		t.Errorf("outbound session = %q", out.SessionID)
	}
	if out.Metadata["chat_id"] != "oc_abc123" {
		t.Errorf("outbound chat_id = %v", out.Metadata["chat_id"])
	}
	if out.Metadata["reply_to"] != "om_xyz789" {
		t.Errorf("outbound reply_to = %v, want om_xyz789", out.Metadata["reply_to"])
	}
	if out.Content == "" {
		t.Error("outbound content should not be empty")
	}

	// 8. Verify prompt had Lark-specific system prompt and tools
	if result.Prompt == nil {
		t.Fatal("prompt is nil")
	}
	if !strings.Contains(result.Prompt.System, "Lark") {
		t.Errorf("system prompt should mention Lark, got: %s", result.Prompt.System)
	}
	hasLarkTool := false
	for _, tool := range result.Prompt.Tools {
		if tool.Name == "lark_docx" {
			hasLarkTool = true
		}
	}
	if !hasLarkTool {
		t.Error("prompt should include lark_docx tool")
	}

	// 9. Verify token usage
	if result.ModelOutput.Usage.TotalTokens != 620 {
		t.Errorf("total_tokens = %d, want 620", result.ModelOutput.Usage.TotalTokens)
	}
}

// ---------------------------------------------------------------------------
// E2E: Lark multi-plugin injection order
// ---------------------------------------------------------------------------

// larkOKRPlugin injects OKR context at high priority
type larkOKRPlugin struct {
	called bool
}

func (p *larkOKRPlugin) Name() string  { return "lark-okr" }
func (p *larkOKRPlugin) Priority() int { return 80 }
func (p *larkOKRPlugin) PreTask(_ context.Context, state *hook.TurnState) error {
	p.called = true
	state.Messages = append(state.Messages, hook.Message{
		Role:    "system",
		Content: "[OKR] Q1: ship tape architecture",
		Source:  "okr",
	})
	return nil
}

// larkMemoryPlugin injects memory context at medium priority
type larkMemoryPlugin struct {
	called bool
}

func (p *larkMemoryPlugin) Name() string  { return "lark-memory" }
func (p *larkMemoryPlugin) Priority() int { return 60 }
func (p *larkMemoryPlugin) PreTask(_ context.Context, state *hook.TurnState) error {
	p.called = true
	state.Messages = append(state.Messages, hook.Message{
		Role:    "system",
		Content: "[Memory] User prefers Chinese, concise responses",
		Source:  "memory",
	})
	return nil
}

// larkPredictionPlugin runs post-task at low priority
type larkPredictionPlugin struct {
	predicted bool
}

func (p *larkPredictionPlugin) Name() string  { return "lark-prediction" }
func (p *larkPredictionPlugin) Priority() int { return 40 }
func (p *larkPredictionPlugin) PostTask(_ context.Context, state *hook.TurnState, _ *hook.TurnResult) error {
	p.predicted = true
	state.Set("prediction", "user will ask about deployment next")
	return nil
}

func TestE2E_LarkMultiPluginInjection(t *testing.T) {
	main := newLarkTestPlugin()
	okr := &larkOKRPlugin{}
	mem := &larkMemoryPlugin{}
	pred := &larkPredictionPlugin{}

	fw := New(Config{})
	fw.RegisterPlugin(main)
	fw.RegisterPlugin(okr)
	fw.RegisterPlugin(mem)
	fw.RegisterPlugin(pred)

	env := envelope.New(map[string]any{
		"content":  "部署进展",
		"channel":  "lark",
		"chat_id":  "oc_deploy",
		"user_id":  "ou_ckl",
	})

	result, err := fw.ProcessInbound(context.Background(), env)
	if err != nil {
		t.Fatalf("ProcessInbound: %v", err)
	}

	// All plugins should have run
	if !okr.called {
		t.Error("OKR plugin should have been called")
	}
	if !mem.called {
		t.Error("Memory plugin should have been called")
	}
	if !pred.predicted {
		t.Error("Prediction plugin should have been called")
	}

	// Model should have produced output
	if result.ModelOutput == nil || result.ModelOutput.Text == "" {
		t.Fatal("model should have produced output")
	}

	// Dispatched to Lark
	plugin := main
	plugin.mu.Lock()
	dispatchCount := len(plugin.dispatched)
	plugin.mu.Unlock()
	if dispatchCount == 0 {
		t.Error("should have dispatched to Lark")
	}
}

// ---------------------------------------------------------------------------
// E2E: Lark channel dispatch via ChannelManager
// ---------------------------------------------------------------------------

func TestE2E_LarkChannelManagerDispatch(t *testing.T) {
	larkCh := &testChannel{name: "lark"}
	cliCh := &testChannel{name: "cli"}

	chMgr := channel.NewManager(channel.DefaultDebounceConfig())
	chMgr.Register(larkCh)
	chMgr.Register(cliCh)
	_ = chMgr.Start(context.Background())
	defer func() { _ = chMgr.Stop(context.Background()) }()

	// Test channel manager routing directly
	ctx := context.Background()
	msg := channel.Outbound{Content: "Lark 回复消息", Kind: "text"}
	if err := chMgr.Send(ctx, "lark", "lark-oc_test", msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	sent := larkCh.getSent()
	if len(sent) != 1 {
		t.Fatalf("expected 1 Lark message, got %d", len(sent))
	}
	if sent[0].Content != "Lark 回复消息" {
		t.Errorf("content = %q", sent[0].Content)
	}

	// CLI should have received nothing
	cliSent := cliCh.getSent()
	if len(cliSent) != 0 {
		t.Errorf("CLI should have 0 messages, got %d", len(cliSent))
	}
}
