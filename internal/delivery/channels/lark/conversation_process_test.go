package lark

import (
	"context"
	"sync"
	"testing"
	"time"

	"alex/internal/delivery/channels"
	ports "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	portsllm "alex/internal/domain/agent/ports/llm"
	storage "alex/internal/domain/agent/ports/storage"
	"alex/internal/shared/config"
	"alex/internal/shared/logging"
	"alex/internal/shared/utils"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// ---------------------------------------------------------------------------
// Stub LLM client that supports tool calls
// ---------------------------------------------------------------------------

type convStubLLMClient struct {
	mu        sync.Mutex
	resp      string
	toolCalls []ports.ToolCall
	err       error
	reqs      []ports.CompletionRequest
}

func (c *convStubLLMClient) Complete(_ context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	c.mu.Lock()
	c.reqs = append(c.reqs, req)
	c.mu.Unlock()
	if c.err != nil {
		return nil, c.err
	}
	return &ports.CompletionResponse{Content: c.resp, ToolCalls: c.toolCalls}, nil
}

func (c *convStubLLMClient) Model() string { return "stub-conv" }

func (c *convStubLLMClient) lastReqs() []ports.CompletionRequest {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]ports.CompletionRequest, len(c.reqs))
	copy(out, c.reqs)
	return out
}

type convStubFactory struct {
	client portsllm.LLMClient
}

func (f *convStubFactory) GetClient(_, _ string, _ portsllm.LLMConfig) (portsllm.LLMClient, error) {
	return f.client, nil
}

func (f *convStubFactory) GetIsolatedClient(_, _ string, _ portsllm.LLMConfig) (portsllm.LLMClient, error) {
	return f.client, nil
}

func (f *convStubFactory) DisableRetry() {}

// ---------------------------------------------------------------------------
// Recording messenger for verifying dispatch calls
// ---------------------------------------------------------------------------

type convRecordingMessenger struct {
	mu       sync.Mutex
	messages []convSentMessage
}

type convSentMessage struct {
	chatID  string
	replyTo string
	msgType string
	content string
}

func (m *convRecordingMessenger) SendMessage(_ context.Context, chatID, msgType, content string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, convSentMessage{chatID: chatID, msgType: msgType, content: content})
	return "om_" + chatID, nil
}

func (m *convRecordingMessenger) ReplyMessage(_ context.Context, replyTo, msgType, content string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, convSentMessage{replyTo: replyTo, msgType: msgType, content: content})
	return "om_reply", nil
}

func (m *convRecordingMessenger) UpdateMessage(context.Context, string, string, string) error { return nil }
func (m *convRecordingMessenger) AddReaction(context.Context, string, string) (string, error) {
	return "", nil
}
func (m *convRecordingMessenger) DeleteReaction(context.Context, string, string) error { return nil }
func (m *convRecordingMessenger) UploadImage(context.Context, []byte) (string, error) {
	return "", nil
}
func (m *convRecordingMessenger) UploadFile(context.Context, []byte, string, string) (string, error) {
	return "", nil
}
func (m *convRecordingMessenger) ListMessages(context.Context, string, int) ([]*larkim.Message, error) {
	return nil, nil
}

func (m *convRecordingMessenger) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.messages)
}

func (m *convRecordingMessenger) last() convSentMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.messages) == 0 {
		return convSentMessage{}
	}
	return m.messages[len(m.messages)-1]
}

// ---------------------------------------------------------------------------
// Stub agent executor
// ---------------------------------------------------------------------------

type convStubAgentExecutor struct {
	result *agent.TaskResult
	err    error
}

func (e *convStubAgentExecutor) EnsureSession(_ context.Context, sessionID string) (*storage.Session, error) {
	return &storage.Session{ID: sessionID}, nil
}

func (e *convStubAgentExecutor) ExecuteTask(_ context.Context, _ string, _ string, _ agent.EventListener) (*agent.TaskResult, error) {
	return e.result, e.err
}

// ---------------------------------------------------------------------------
// Gateway builder
// ---------------------------------------------------------------------------

func newConvGateway(t *testing.T, stub *convStubLLMClient, enabled bool) *Gateway {
	t.Helper()
	en := enabled
	rec := &convRecordingMessenger{}
	gw := &Gateway{
		cfg: Config{
			BaseConfig: channels.BaseConfig{
				SessionPrefix: "test",
				AllowDirect:   true,
			},
			ConversationProcessEnabled: &en,
		},
		agent: &convStubAgentExecutor{
			result: &agent.TaskResult{Answer: "done"},
		},
		llmFactory: &convStubFactory{client: stub},
		llmProfile: config.LLMProfile{Provider: "openai", Model: "gpt-4o-mini"},
		logger:     logging.OrNop(nil),
		messenger:  rec,
		now:        time.Now,
	}
	return gw
}

func getRecorder(g *Gateway) *convRecordingMessenger {
	// The messenger might be wrapped by injectCaptureHub; unwrap if needed.
	if hub, ok := g.messenger.(*injectCaptureHub); ok {
		return hub.inner.(*convRecordingMessenger)
	}
	return g.messenger.(*convRecordingMessenger)
}

// ---------------------------------------------------------------------------
// Tests for conversationLLM
// ---------------------------------------------------------------------------

func TestConversationLLM_ReturnsTextReply(t *testing.T) {
	stub := &convStubLLMClient{resp: "你好！有什么可以帮你的？"}
	g := newConvGateway(t, stub, true)
	snap := workerSnapshot{Phase: slotIdle}

	reply, toolCalls := g.conversationLLM(context.Background(), "你好", snap)
	if reply != "你好！有什么可以帮你的？" {
		t.Fatalf("expected reply text, got %q", reply)
	}
	if len(toolCalls) != 0 {
		t.Fatalf("expected no tool calls, got %d", len(toolCalls))
	}
}

func TestConversationLLM_ReturnsToolCall(t *testing.T) {
	stub := &convStubLLMClient{
		resp: "好，我来看一下。",
		toolCalls: []ports.ToolCall{
			{ID: "tc1", Name: "dispatch_worker", Arguments: map[string]any{"task": "重构 auth 模块"}},
		},
	}
	g := newConvGateway(t, stub, true)
	snap := workerSnapshot{Phase: slotIdle}

	reply, toolCalls := g.conversationLLM(context.Background(), "重构 auth 模块", snap)
	if reply != "好，我来看一下。" {
		t.Fatalf("expected confirmation reply, got %q", reply)
	}
	if len(toolCalls) != 1 || toolCalls[0].Name != "dispatch_worker" {
		t.Fatalf("expected 1 dispatch_worker tool call, got %v", toolCalls)
	}
}

func TestConversationLLM_IncludesWorkerStatus(t *testing.T) {
	stub := &convStubLLMClient{resp: "已经跑了45秒，正在处理中。"}
	g := newConvGateway(t, stub, true)
	snap := workerSnapshot{Phase: slotRunning, TaskDesc: "build dashboard", Elapsed: 45 * time.Second}

	g.conversationLLM(context.Background(), "做得怎么样了？", snap)

	reqs := stub.lastReqs()
	if len(reqs) == 0 {
		t.Fatal("expected at least one LLM request")
	}
	userContent := reqs[0].Messages[1].Content
	if !strContains(userContent, "build dashboard") {
		t.Errorf("user prompt should contain task desc, got %q", userContent)
	}
}

func TestConversationLLM_IncludesDispatchWorkerTool(t *testing.T) {
	stub := &convStubLLMClient{resp: "ok"}
	g := newConvGateway(t, stub, true)
	snap := workerSnapshot{Phase: slotIdle}

	g.conversationLLM(context.Background(), "hello", snap)

	reqs := stub.lastReqs()
	if len(reqs) == 0 {
		t.Fatal("expected at least one LLM request")
	}
	if len(reqs[0].Tools) != 1 || reqs[0].Tools[0].Name != "dispatch_worker" {
		t.Fatalf("expected 1 dispatch_worker tool, got %v", reqs[0].Tools)
	}
}

func TestConversationLLM_FallbackOnError(t *testing.T) {
	stub := &convStubLLMClient{err: &convStubErr{"timeout"}}
	g := newConvGateway(t, stub, true)
	snap := workerSnapshot{Phase: slotIdle}

	reply, toolCalls := g.conversationLLM(context.Background(), "hello", snap)
	if reply != "" || len(toolCalls) != 0 {
		t.Fatalf("expected empty on error, got reply=%q toolCalls=%d", reply, len(toolCalls))
	}
}

func TestConversationLLM_FallbackWhenFactoryNil(t *testing.T) {
	en := true
	g := &Gateway{
		cfg:        Config{ConversationProcessEnabled: &en},
		llmFactory: nil,
		logger:     logging.OrNop(nil),
		now:        time.Now,
	}
	reply, toolCalls := g.conversationLLM(context.Background(), "hi", workerSnapshot{Phase: slotIdle})
	if reply != "" || len(toolCalls) != 0 {
		t.Fatal("expected empty when factory nil")
	}
}

// ---------------------------------------------------------------------------
// Tests for handleViaConversationProcess
// ---------------------------------------------------------------------------

func TestHandleViaConversationProcess_DirectReply(t *testing.T) {
	stub := &convStubLLMClient{resp: "你好！"}
	g := newConvGateway(t, stub, true)
	rec := getRecorder(g)

	msg := &incomingMessage{chatID: "chat1", messageID: "msg1", content: "你好"}
	slot := &sessionSlot{}

	handled := g.handleViaConversationProcess(context.Background(), msg, slot)
	if !handled {
		t.Fatal("expected handled=true")
	}
	if rec.count() != 1 {
		t.Fatalf("expected 1 message sent, got %d", rec.count())
	}
}

func TestHandleViaConversationProcess_AlwaysReturnsTrue(t *testing.T) {
	stub := &convStubLLMClient{err: &convStubErr{"fail"}}
	g := newConvGateway(t, stub, true)

	msg := &incomingMessage{chatID: "chat1", content: "hello"}
	slot := &sessionSlot{}

	if !g.handleViaConversationProcess(context.Background(), msg, slot) {
		t.Fatal("expected handled=true even on LLM failure")
	}
}

func TestHandleViaConversationProcess_SpawnsWorkerOnToolCall(t *testing.T) {
	stub := &convStubLLMClient{
		resp: "好的",
		toolCalls: []ports.ToolCall{
			{ID: "tc1", Name: "dispatch_worker", Arguments: map[string]any{"task": "重构 auth"}},
		},
	}
	g := newConvGateway(t, stub, true)

	msg := &incomingMessage{chatID: "chat1", chatType: "p2p", messageID: "msg1", senderID: "user1", content: "重构 auth"}
	slot := g.getOrCreateSlot("chat1")

	g.handleViaConversationProcess(context.Background(), msg, slot)

	// Verify slot transitioned to running (worker goroutine launched).
	slot.mu.Lock()
	phase := slot.phase
	desc := slot.taskDesc
	slot.mu.Unlock()
	if phase != slotRunning {
		t.Fatalf("expected slotRunning, got %d", phase)
	}
	if desc != "重构 auth" {
		t.Fatalf("expected taskDesc='重构 auth', got %q", desc)
	}

	// Wait for the worker goroutine to finish.
	g.taskWG.Wait()

	// After worker completes, slot should be idle again.
	slot.mu.Lock()
	phase = slot.phase
	slot.mu.Unlock()
	if phase != slotIdle {
		t.Fatalf("expected slotIdle after worker completes, got %d", phase)
	}
}

func TestHandleViaConversationProcess_InjectsIntoRunningWorker(t *testing.T) {
	stub := &convStubLLMClient{
		resp: "收到，已传达。",
		toolCalls: []ports.ToolCall{
			{ID: "tc1", Name: "dispatch_worker", Arguments: map[string]any{"task": "用 PostgreSQL"}},
		},
	}
	g := newConvGateway(t, stub, true)

	// Pre-populate a running worker with an inputCh.
	inputCh := make(chan agent.UserInput, 16)
	slot := g.getOrCreateSlot("chat1")
	slot.mu.Lock()
	slot.phase = slotRunning
	slot.inputCh = inputCh
	slot.sessionID = "sess-1"
	slot.taskDesc = "重构 auth 模块"
	slot.lastTouched = time.Now()
	slot.mu.Unlock()
	g.activeSlots.Store("chat1", slot)

	msg := &incomingMessage{chatID: "chat1", messageID: "msg2", senderID: "user1", content: "用 PostgreSQL"}

	g.handleViaConversationProcess(context.Background(), msg, slot)

	// Verify the message was injected into the existing inputCh.
	select {
	case input := <-inputCh:
		if input.Content != "用 PostgreSQL" {
			t.Fatalf("expected injected content '用 PostgreSQL', got %q", input.Content)
		}
	default:
		t.Fatal("expected message to be injected into worker inputCh")
	}

	// Slot should still be running (not replaced).
	slot.mu.Lock()
	phase := slot.phase
	slot.mu.Unlock()
	if phase != slotRunning {
		t.Fatalf("expected slot still running, got phase=%d", phase)
	}
}

// ---------------------------------------------------------------------------
// Tests for conversationProcessEnabled
// ---------------------------------------------------------------------------

func TestConversationProcessEnabled_DefaultFalse(t *testing.T) {
	g := &Gateway{cfg: Config{}}
	if g.conversationProcessEnabled() {
		t.Fatal("expected disabled by default")
	}
}

func TestConversationProcessEnabled_True(t *testing.T) {
	en := true
	g := &Gateway{cfg: Config{ConversationProcessEnabled: &en}}
	if !g.conversationProcessEnabled() {
		t.Fatal("expected enabled")
	}
}

// ---------------------------------------------------------------------------
// Tests for workerSnapshot
// ---------------------------------------------------------------------------

func TestWorkerSnapshot_StatusSummary_Idle(t *testing.T) {
	snap := workerSnapshot{Phase: slotIdle}
	if snap.StatusSummary() != "idle" {
		t.Fatalf("expected 'idle', got %q", snap.StatusSummary())
	}
}

func TestWorkerSnapshot_StatusSummary_Running(t *testing.T) {
	snap := workerSnapshot{
		Phase: slotRunning, TaskDesc: "build dashboard", Elapsed: 45 * time.Second,
	}
	s := snap.StatusSummary()
	if !strContains(s, "build dashboard") || !strContains(s, "45s") {
		t.Errorf("unexpected summary: %q", s)
	}
}

func TestSnapshotWorker_Idle(t *testing.T) {
	g := &Gateway{logger: logging.OrNop(nil), now: time.Now}
	if !g.snapshotWorker("nonexistent").IsIdle() {
		t.Fatal("expected idle")
	}
}

func TestSnapshotWorker_Running(t *testing.T) {
	g := &Gateway{logger: logging.OrNop(nil), now: time.Now}
	slot := &sessionSlot{phase: slotRunning, taskDesc: "refactor auth"}
	slot.lastTouched = time.Now().Add(-30 * time.Second)
	g.activeSlots.Store("chat1", slot)

	snap := g.snapshotWorker("chat1")
	if !snap.IsRunning() || snap.TaskDesc != "refactor auth" {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}
}

// ---------------------------------------------------------------------------
// Tests for truncateLog
// ---------------------------------------------------------------------------

func TestTruncateLog(t *testing.T) {
	if got := utils.Truncate("hello", 10, "…"); got != "hello" {
		t.Fatalf("got %q", got)
	}
	if got := utils.Truncate("hello world", 5, "…"); got != "hell…" {
		t.Fatalf("got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type convStubErr struct{ msg string }

func (e *convStubErr) Error() string { return e.msg }

func strContains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && func() bool {
		for i := 0; i <= len(s)-len(sub); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	}())
}
