package lark

import (
	"context"
	"strings"
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

func (m *convRecordingMessenger) all() []convSentMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]convSentMessage, len(m.messages))
	copy(out, m.messages)
	return out
}

// ---------------------------------------------------------------------------
// Stub agent executor
// ---------------------------------------------------------------------------

type convStubAgentExecutor struct {
	result  *agent.TaskResult
	err     error
	startCh chan struct{} // if non-nil, ExecuteTask blocks until closed
}

func (e *convStubAgentExecutor) EnsureSession(_ context.Context, sessionID string) (*storage.Session, error) {
	return &storage.Session{ID: sessionID}, nil
}

func (e *convStubAgentExecutor) ExecuteTask(_ context.Context, _ string, _ string, _ agent.EventListener) (*agent.TaskResult, error) {
	if e.startCh != nil {
		<-e.startCh
	}
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
		logger:    logging.OrNop(nil),
		messenger: rec,
		now:       time.Now,
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
// Tests for conversationLLMWithList
// ---------------------------------------------------------------------------

func TestConversationLLMWithList_ReturnsTextReply(t *testing.T) {
	stub := &convStubLLMClient{resp: "你好！有什么可以帮你的？"}
	g := newConvGateway(t, stub, true)
	workers := workerSnapshotList{}

	reply, toolCalls, err := g.conversationLLMWithList(context.Background(), "u1", "你好", workers, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "你好！有什么可以帮你的？" {
		t.Fatalf("expected reply text, got %q", reply)
	}
	if len(toolCalls) != 0 {
		t.Fatalf("expected no tool calls, got %d", len(toolCalls))
	}
}

func TestConversationLLMWithList_ReturnsToolCall(t *testing.T) {
	stub := &convStubLLMClient{
		resp: "好，我来看一下。",
		toolCalls: []ports.ToolCall{
			{ID: "tc1", Name: "dispatch_worker", Arguments: map[string]any{"task": "重构 auth 模块"}},
		},
	}
	g := newConvGateway(t, stub, true)
	workers := workerSnapshotList{}

	reply, toolCalls, err := g.conversationLLMWithList(context.Background(), "u1", "重构 auth 模块", workers, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reply != "好，我来看一下。" {
		t.Fatalf("expected confirmation reply, got %q", reply)
	}
	if len(toolCalls) != 1 || toolCalls[0].Name != "dispatch_worker" {
		t.Fatalf("expected 1 dispatch_worker tool call, got %v", toolCalls)
	}
}

func TestConversationLLMWithList_IncludesWorkerStatus(t *testing.T) {
	stub := &convStubLLMClient{resp: "已经跑了45秒，正在处理中。"}
	g := newConvGateway(t, stub, true)
	workers := workerSnapshotList{
		Snapshots: []workerSnapshot{{Phase: slotRunning, TaskDesc: "build dashboard", Elapsed: 45 * time.Second}},
	}

	_, _, err := g.conversationLLMWithList(context.Background(), "u1", "做得怎么样了？", workers, "")
	if err != nil {
		t.Fatalf("conversationLLMWithList: %v", err)
	}

	reqs := stub.lastReqs()
	if len(reqs) == 0 {
		t.Fatal("expected at least one LLM request")
	}
	userContent := reqs[0].Messages[1].Content
	if !strContains(userContent, "build dashboard") {
		t.Errorf("user prompt should contain task desc, got %q", userContent)
	}
}

func TestConversationLLMWithList_IncludesTools(t *testing.T) {
	stub := &convStubLLMClient{resp: "ok"}
	g := newConvGateway(t, stub, true)
	workers := workerSnapshotList{}

	_, _, _ = g.conversationLLMWithList(context.Background(), "u1", "hello", workers, "")

	reqs := stub.lastReqs()
	if len(reqs) == 0 {
		t.Fatal("expected at least one LLM request")
	}
	if len(reqs[0].Tools) != 2 {
		t.Fatalf("expected 2 tools (dispatch_worker, stop_worker), got %d", len(reqs[0].Tools))
	}
	names := make(map[string]bool)
	for _, tool := range reqs[0].Tools {
		names[tool.Name] = true
	}
	if !names["dispatch_worker"] || !names["stop_worker"] {
		t.Fatalf("expected dispatch_worker and stop_worker tools, got %v", reqs[0].Tools)
	}
}

func TestConversationLLMWithList_FallbackOnError(t *testing.T) {
	stub := &convStubLLMClient{err: &convStubErr{"timeout"}}
	g := newConvGateway(t, stub, true)
	workers := workerSnapshotList{}

	reply, toolCalls, err := g.conversationLLMWithList(context.Background(), "u1", "hello", workers, "")
	if err == nil {
		t.Fatal("expected error")
	}
	if reply != "" || len(toolCalls) != 0 {
		t.Fatalf("expected empty on error, got reply=%q toolCalls=%d", reply, len(toolCalls))
	}
}

func TestConversationLLMWithList_FallbackWhenFactoryNil(t *testing.T) {
	en := true
	g := &Gateway{
		cfg:        Config{ConversationProcessEnabled: &en},
		llmFactory: nil,
		logger:     logging.OrNop(nil),
		now:        time.Now,
	}
	_, _, err := g.conversationLLMWithList(context.Background(), "u1", "hi", workerSnapshotList{}, "")
	if err == nil {
		t.Fatal("expected error when factory nil")
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

	handled := g.handleViaConversationProcess(context.Background(), msg)
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

	if !g.handleViaConversationProcess(context.Background(), msg) {
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
	// Use a blocking executor so the worker goroutine stays in slotRunning
	// until we explicitly release it — avoids a race where the goroutine
	// completes before the test checks slot phase.
	blockCh := make(chan struct{})
	g := newConvGateway(t, stub, true)
	g.agent = &convStubAgentExecutor{
		result:  &agent.TaskResult{Answer: "done"},
		startCh: blockCh,
	}

	msg := &incomingMessage{chatID: "chat1", chatType: "p2p", messageID: "msg1", senderID: "user1", content: "重构 auth"}

	g.handleViaConversationProcess(context.Background(), msg)

	// Verify a slot was allocated in the chatSlotMap and is running.
	slotMap := g.getOrCreateSlotMap("chat1")
	var foundRunning bool
	var foundDesc string
	slotMap.forEachSlot(func(taskID string, s *sessionSlot) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.phase == slotRunning {
			foundRunning = true
			foundDesc = s.taskDesc
		}
	})
	if !foundRunning {
		t.Fatal("expected a slot in slotRunning phase")
	}
	if foundDesc != "重构 auth" {
		t.Fatalf("expected taskDesc='重构 auth', got %q", foundDesc)
	}

	// Release the worker goroutine and wait for it to finish.
	close(blockCh)
	g.taskWG.Wait()

	// After worker completes, slot should be idle again.
	var stillRunning bool
	slotMap.forEachSlot(func(taskID string, s *sessionSlot) {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.phase == slotRunning {
			stillRunning = true
		}
	})
	if stillRunning {
		t.Fatal("expected no slots still running after worker completes")
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

	// Pre-populate a running worker in the chatSlotMap.
	inputCh := make(chan agent.UserInput, 16)
	slotMap := g.getOrCreateSlotMap("chat1")
	slot, _, _, _ := slotMap.allocateSlotIfCapacity(5, time.Now())
	if slot == nil {
		t.Fatal("failed to allocate slot")
	}
	slot.mu.Lock()
	slot.phase = slotRunning
	slot.inputCh = inputCh
	slot.sessionID = "sess-1"
	slot.taskDesc = "重构 auth 模块"
	slot.lastTouched = time.Now()
	slot.mu.Unlock()

	msg := &incomingMessage{chatID: "chat1", messageID: "msg2", senderID: "user1", content: "用 PostgreSQL"}

	g.handleViaConversationProcess(context.Background(), msg)

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
	if snap.StatusSummary("zh") != "idle" {
		t.Fatalf("expected 'idle', got %q", snap.StatusSummary("zh"))
	}
}

func TestWorkerSnapshot_StatusSummary_Running(t *testing.T) {
	snap := workerSnapshot{
		Phase: slotRunning, TaskDesc: "build dashboard", Elapsed: 45 * time.Second,
	}
	s := snap.StatusSummary("zh")
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
// Tests for dispatchFormattedReply
// ---------------------------------------------------------------------------

func TestDispatchFormattedReply_PlainText(t *testing.T) {
	stub := &convStubLLMClient{resp: "ok"}
	g := newConvGateway(t, stub, true)
	rec := getRecorder(g)

	g.dispatchFormattedReply(context.Background(), "chat1", "msg1", "你好")

	if rec.count() != 1 {
		t.Fatalf("expected 1 message, got %d", rec.count())
	}
	m := rec.last()
	if m.msgType != "text" {
		t.Fatalf("expected text, got %q", m.msgType)
	}
}

func TestDispatchFormattedReply_Markdown(t *testing.T) {
	stub := &convStubLLMClient{resp: "ok"}
	g := newConvGateway(t, stub, true)
	rec := getRecorder(g)

	g.dispatchFormattedReply(context.Background(), "chat1", "msg1", "**bold** text")

	if rec.count() != 1 {
		t.Fatalf("expected 1 message, got %d", rec.count())
	}
	m := rec.last()
	if m.msgType != "post" {
		t.Fatalf("expected post for markdown, got %q", m.msgType)
	}
}

func TestDispatchFormattedReply_EmptyAfterShape(t *testing.T) {
	stub := &convStubLLMClient{resp: "ok"}
	g := newConvGateway(t, stub, true)
	rec := getRecorder(g)

	g.dispatchFormattedReply(context.Background(), "chat1", "msg1", "")

	if rec.count() != 0 {
		t.Fatalf("expected 0 messages for empty text, got %d", rec.count())
	}
}

func TestDispatchFormattedReply_SplitsLongMessage(t *testing.T) {
	stub := &convStubLLMClient{resp: "ok"}
	g := newConvGateway(t, stub, true)
	rec := getRecorder(g)

	// Two heading sections should produce 2 chunks.
	long := "## Section 1\n\nContent one.\n\n## Section 2\n\nContent two."
	g.dispatchFormattedReply(context.Background(), "chat1", "msg1", long)

	if rec.count() < 2 {
		t.Fatalf("expected >=2 messages for multi-section content, got %d", rec.count())
	}
}

func TestDispatchFormattedReply_ShapeReply7CApplied(t *testing.T) {
	stub := &convStubLLMClient{resp: "ok"}
	g := newConvGateway(t, stub, true)
	rec := getRecorder(g)

	// ShapeReply7C strips horizontal rules (---) — verify it runs.
	g.dispatchFormattedReply(context.Background(), "chat1", "msg1", "Hello\n\n---\n\nWorld")

	if rec.count() == 0 {
		t.Fatal("expected at least 1 message")
	}
	// Verify no dispatched message contains a horizontal rule.
	rec.mu.Lock()
	for _, m := range rec.messages {
		if strContains(m.content, "---") {
			t.Errorf("ShapeReply7C should strip horizontal rules, got content containing '---'")
		}
	}
	rec.mu.Unlock()
}

func TestHandleViaConversationProcess_UsesFormattedPipeline(t *testing.T) {
	stub := &convStubLLMClient{resp: "**重点说明**：结果已就绪"}
	g := newConvGateway(t, stub, true)
	rec := getRecorder(g)

	msg := &incomingMessage{chatID: "chat1", messageID: "msg1", content: "结果呢？"}

	g.handleViaConversationProcess(context.Background(), msg)

	if rec.count() != 1 {
		t.Fatalf("expected 1 message, got %d", rec.count())
	}
	m := rec.last()
	// Markdown content should be sent as post, not text.
	if m.msgType != "post" {
		t.Fatalf("expected post for markdown reply, got %q", m.msgType)
	}
}

// ---------------------------------------------------------------------------
// Tests for naturalizeReply
// ---------------------------------------------------------------------------

func TestNaturalizeReply_StripsTrailingPeriod(t *testing.T) {
	cases := []struct{ in, want string }{
		{"已完成。", "已完成"},
		{"好的。", "好"},       // base trailing period, then casual "好的"→"好"
		{"没问题", "没问题"},    // no period, no change
		{"", ""},              // empty passthrough
		{"Done.", "Done."},    // ASCII period untouched (only Chinese period removed)
	}
	for _, tc := range cases {
		got := naturalizeReply(tc.in, 1)
		if got != tc.want {
			t.Errorf("naturalizeReply(%q, 1) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNaturalizeReply_BaseTierSubstitutions(t *testing.T) {
	cases := []struct{ in, want string }{
		{"请稍等", "等下"},
		{"您好，我是助手", "你好，我是助手"},
		{"非常感谢", "谢了"},
		{"非常抱歉打扰", "抱歉打扰"},
		{"好的，来看看", "好，来看看"},
		{"可以的", "行"},
		{"收到了", "收到"},
		{"没有问题", "没问题"},
	}
	for _, tc := range cases {
		got := naturalizeReply(tc.in, 0)
		if got != tc.want {
			t.Errorf("naturalizeReply(%q, 0) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNaturalizeReply_CasualTierApplied(t *testing.T) {
	// level=1 applies casual rules on top of base rules.
	if got := naturalizeReply("好的", 1); got != "好" {
		t.Errorf("casual: naturalizeReply(\"好的\", 1) = %q, want \"好\"", got)
	}
	if got := naturalizeReply("知道了", 1); got != "知道" {
		t.Errorf("casual: naturalizeReply(\"知道了\", 1) = %q, want \"知道\"", got)
	}
}

func TestNaturalizeReply_CasualTierNotAppliedAtLevel0(t *testing.T) {
	// level=0 should leave casual-only patterns unchanged.
	if got := naturalizeReply("好的", 0); got != "好的" {
		t.Errorf("neutral: naturalizeReply(\"好的\", 0) = %q, want \"好的\"", got)
	}
}

func TestNaturalizeReply_EnglishPeriodUntouched(t *testing.T) {
	// ASCII "." should NOT be stripped — only Chinese "。".
	if got := naturalizeReply("Stopped.", 0); got != "Stopped." {
		t.Errorf("naturalizeReply(\"Stopped.\", 0) = %q, want \"Stopped.\"", got)
	}
}

// ---------------------------------------------------------------------------
// Tests for detectFormalityLevel
// ---------------------------------------------------------------------------

func TestDetectFormalityLevel_P2P(t *testing.T) {
	if detectFormalityLevel("p2p", "") != 1 {
		t.Error("p2p chat should return casual level 1")
	}
}

func TestDetectFormalityLevel_Group(t *testing.T) {
	if detectFormalityLevel("group", "") != 0 {
		t.Error("group chat should return neutral level 0")
	}
}

func TestDetectFormalityLevel_Unknown(t *testing.T) {
	if detectFormalityLevel("", "") != 0 {
		t.Error("unknown chat type should return neutral level 0")
	}
}

func TestDetectFormalityLevel_MemoryNeutral(t *testing.T) {
	// Memory keyword "外部客户" should override p2p → neutral.
	if detectFormalityLevel("p2p", "这是外部客户的群") != 0 {
		t.Error("memory with 外部客户 should override p2p to neutral level 0")
	}
	if detectFormalityLevel("p2p", "This is a client chat") != 0 {
		t.Error("memory with 'client' should override p2p to neutral level 0")
	}
	if detectFormalityLevel("group", "external partner channel") != 0 {
		t.Error("memory with 'external' should return neutral level 0")
	}
}

func TestDetectFormalityLevel_MemoryCasual(t *testing.T) {
	// Memory keyword "同事" should override group → casual.
	if detectFormalityLevel("group", "都是同事") != 1 {
		t.Error("memory with 同事 should override group to casual level 1")
	}
	if detectFormalityLevel("group", "Colleague chat") != 1 {
		t.Error("memory with 'colleague' should override group to casual level 1")
	}
	if detectFormalityLevel("group", "teammate daily standup") != 1 {
		t.Error("memory with 'teammate' should override group to casual level 1")
	}
}

func TestDetectFormalityLevel_MemoryNeutralTakesPrecedence(t *testing.T) {
	// When both neutral and casual keywords appear, neutral wins (scanned first).
	if detectFormalityLevel("p2p", "同事 but also 外部客户") != 0 {
		t.Error("neutral keywords should take precedence over casual")
	}
}

// ---------------------------------------------------------------------------
// Integration: naturalizeReply is applied in handleViaConversationProcess
// ---------------------------------------------------------------------------

func TestHandleViaConversationProcess_NaturalizeApplied(t *testing.T) {
	// LLM returns a reply with a trailing Chinese period — it should be stripped.
	stub := &convStubLLMClient{resp: "已完成分析。"}
	g := newConvGateway(t, stub, true)
	rec := getRecorder(g)

	msg := &incomingMessage{chatID: "chat1", messageID: "msg1", chatType: "p2p", content: "帮我分析"}
	g.handleViaConversationProcess(context.Background(), msg)

	if rec.count() == 0 {
		t.Fatal("expected a reply")
	}
	// Check all sent messages — with fragmented replies there may be multiple.
	for _, m := range rec.all() {
		if strContains(m.content, "。") {
			t.Errorf("naturalizeReply should strip trailing 。, got content: %q", m.content)
		}
	}
}

// ---------------------------------------------------------------------------
// Tests: previously fast-pathed messages now go through LLM
// ---------------------------------------------------------------------------

func TestHandleViaConversationProcess_NoFastPath_DispatchGoesToLLM(t *testing.T) {
	stub := &convStubLLMClient{
		resp: "好 查一下",
		toolCalls: []ports.ToolCall{
			{ID: "tc1", Name: "dispatch_worker", Arguments: map[string]any{"task": "查一下昨天日报"}},
		},
	}
	g := newConvGateway(t, stub, true)

	msg := &incomingMessage{chatID: "chat1", chatType: "p2p", messageID: "msg1", senderID: "user1", content: "帮我查一下昨天日报"}
	g.handleViaConversationProcess(context.Background(), msg)

	// Verify the LLM was actually called (not bypassed by fast-path).
	reqs := stub.lastReqs()
	if len(reqs) == 0 {
		t.Fatal("expected LLM call, but none recorded — message was fast-pathed")
	}

	g.taskWG.Wait()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type convStubErr struct{ msg string }

func (e *convStubErr) Error() string { return e.msg }

// ---------------------------------------------------------------------------
// Tests for stop_worker
// ---------------------------------------------------------------------------

func TestHandleViaConversationProcess_StopsWorkerOnToolCall(t *testing.T) {
	stub := &convStubLLMClient{
		resp: "好的，已停止。",
		toolCalls: []ports.ToolCall{
			{ID: "tc1", Name: "stop_worker", Arguments: map[string]any{"task_id": ""}},
		},
	}
	g := newConvGateway(t, stub, true)

	// Pre-populate a running worker in the chatSlotMap with a cancel func.
	cancelled := false
	slotMap := g.getOrCreateSlotMap("chat1")
	slot, _, _, _ := slotMap.allocateSlotIfCapacity(5, time.Now())
	if slot == nil {
		t.Fatal("failed to allocate slot")
	}
	slot.mu.Lock()
	slot.phase = slotRunning
	slot.inputCh = make(chan agent.UserInput, 16)
	slot.sessionID = "sess-1"
	slot.taskToken = 1
	slot.taskCancel = func() { cancelled = true }
	slot.lastTouched = time.Now()
	slot.mu.Unlock()

	msg := &incomingMessage{chatID: "chat1", messageID: "msg1", senderID: "user1", content: "停一下"}

	g.handleViaConversationProcess(context.Background(), msg)

	if !cancelled {
		t.Fatal("expected worker to be cancelled")
	}

	// Verify intentionalCancelToken was set.
	slot.mu.Lock()
	intentional := slot.intentionalCancelToken
	slot.mu.Unlock()
	if intentional != 1 {
		t.Fatalf("expected intentionalCancelToken=1, got %d", intentional)
	}
}

func TestStopWorkerExtended_NoopWhenEmpty(t *testing.T) {
	stub := &convStubLLMClient{resp: "ok"}
	g := newConvGateway(t, stub, true)

	slotMap := &chatSlotMap{}
	// Should not panic when no workers exist.
	g.executeStopWorkerExtended(context.Background(), slotMap, "")
}

// ---------------------------------------------------------------------------
// Tests for chat history in conversationLLMWithList
// ---------------------------------------------------------------------------

func TestConversationLLMWithList_IncludesChatHistory(t *testing.T) {
	stub := &convStubLLMClient{resp: "ok"}
	g := newConvGateway(t, stub, true)
	workers := workerSnapshotList{}

	_, _, _ = g.conversationLLMWithList(context.Background(), "u1", "hello", workers, "user: 之前的消息\nassistant: 之前的回复")

	reqs := stub.lastReqs()
	if len(reqs) == 0 {
		t.Fatal("expected at least one LLM request")
	}
	userContent := reqs[0].Messages[1].Content
	if !strContains(userContent, "Recent chat") {
		t.Errorf("expected chat history header in prompt, got %q", userContent)
	}
	if !strContains(userContent, "之前的消息") {
		t.Errorf("expected chat history content in prompt, got %q", userContent)
	}
}

func TestConversationLLMWithList_NoChatHistoryWhenEmpty(t *testing.T) {
	stub := &convStubLLMClient{resp: "ok"}
	g := newConvGateway(t, stub, true)
	workers := workerSnapshotList{}

	_, _, _ = g.conversationLLMWithList(context.Background(), "u1", "hello", workers, "")

	reqs := stub.lastReqs()
	if len(reqs) == 0 {
		t.Fatal("expected at least one LLM request")
	}
	userContent := reqs[0].Messages[1].Content
	if strContains(userContent, "Recent chat") {
		t.Errorf("should not include chat history header when empty, got %q", userContent)
	}
}

// ---------------------------------------------------------------------------
// Tests for workerSnapshot with RecentProgress
// ---------------------------------------------------------------------------

func TestWorkerSnapshot_StatusSummary_WithProgress(t *testing.T) {
	snap := workerSnapshot{
		Phase:          slotRunning,
		TaskDesc:       "build dashboard",
		Elapsed:        45 * time.Second,
		RecentProgress: []string{"▶ read_file", "✓ read_file (100ms)"},
	}
	s := snap.StatusSummary("zh")
	if !strContains(s, "最近进展") {
		t.Errorf("expected progress section, got %q", s)
	}
	if !strContains(s, "read_file") {
		t.Errorf("expected progress entry, got %q", s)
	}
}

func TestSessionSlot_AppendProgress_RingBuffer(t *testing.T) {
	slot := &sessionSlot{}
	for i := 0; i < maxSlotProgress+3; i++ {
		slot.appendProgress("step")
	}
	slot.mu.Lock()
	n := len(slot.recentProgress)
	slot.mu.Unlock()
	if n != maxSlotProgress {
		t.Fatalf("expected max %d entries, got %d", maxSlotProgress, n)
	}
}

func TestSnapshotWorker_CopiesRecentProgress(t *testing.T) {
	g := &Gateway{logger: logging.OrNop(nil), now: time.Now}
	slot := &sessionSlot{phase: slotRunning, taskDesc: "test", recentProgress: []string{"▶ bash"}}
	slot.lastTouched = time.Now()
	g.activeSlots.Store("chat1", slot)

	snap := g.snapshotWorker("chat1")
	if len(snap.RecentProgress) != 1 || snap.RecentProgress[0] != "▶ bash" {
		t.Fatalf("expected copied progress, got %v", snap.RecentProgress)
	}

	// Mutating the snapshot should not affect the slot.
	snap.RecentProgress[0] = "mutated"
	slot.mu.Lock()
	orig := slot.recentProgress[0]
	slot.mu.Unlock()
	if orig != "▶ bash" {
		t.Fatal("snapshot mutation leaked to slot")
	}
}

// ---------------------------------------------------------------------------
// Tests for sendFragmentedReply (no longer splits — single message)
// ---------------------------------------------------------------------------

func TestSendFragmentedReply_SingleMessage(t *testing.T) {
	stub := &convStubLLMClient{resp: "ok"}
	g := newConvGateway(t, stub, true)
	rec := getRecorder(g)

	g.sendFragmentedReply(context.Background(), "chat1", "msg1", "好", 0)

	if rec.count() != 1 {
		t.Fatalf("expected 1 message, got %d", rec.count())
	}
}

func TestSendFragmentedReply_LongReplyStaysSingle(t *testing.T) {
	stub := &convStubLLMClient{resp: "ok"}
	g := newConvGateway(t, stub, true)
	rec := getRecorder(g)

	g.sendFragmentedReply(context.Background(), "chat1", "msg1", "还在跑还在跑还在跑还在跑呢，明天出结果给你", 0)

	if rec.count() != 1 {
		t.Fatalf("expected 1 message (no splitting), got %d", rec.count())
	}
}

func TestSendFragmentedReply_EmptyReply(t *testing.T) {
	stub := &convStubLLMClient{resp: "ok"}
	g := newConvGateway(t, stub, true)
	rec := getRecorder(g)

	g.sendFragmentedReply(context.Background(), "chat1", "msg1", "", 0)

	if rec.count() != 0 {
		t.Fatalf("expected 0 messages for empty reply, got %d", rec.count())
	}
}

// ---------------------------------------------------------------------------
// Tests for resolveTaskReferences (cross-task awareness)
// ---------------------------------------------------------------------------

func TestResolveTaskReferences_NoRefs(t *testing.T) {
	slotMap := &chatSlotMap{slots: make(map[string]*sessionSlot)}
	got := resolveTaskReferences("do something", slotMap)
	if got != "do something" {
		t.Fatalf("expected unchanged, got %q", got)
	}
}

func TestResolveTaskReferences_WithResult(t *testing.T) {
	slotMap := &chatSlotMap{slots: map[string]*sessionSlot{
		"#1": {lastResultPreview: "found 3 bugs in auth module"},
	}}
	got := resolveTaskReferences("fix the bugs #1 found", slotMap)
	if !strContains(got, "[#1 result]: found 3 bugs in auth module") {
		t.Errorf("expected #1 result injected, got %q", got)
	}
	if !strContains(got, "fix the bugs #1 found") {
		t.Errorf("original content should be preserved, got %q", got)
	}
}

func TestResolveTaskReferences_MissingResult(t *testing.T) {
	slotMap := &chatSlotMap{slots: map[string]*sessionSlot{
		"#1": {lastResultPreview: ""}, // no result yet
	}}
	got := resolveTaskReferences("use #1 output", slotMap)
	if got != "use #1 output" {
		t.Fatalf("expected unchanged when no result, got %q", got)
	}
}

func TestResolveTaskReferences_MultipleRefs(t *testing.T) {
	slotMap := &chatSlotMap{slots: map[string]*sessionSlot{
		"#1": {lastResultPreview: "result A"},
		"#2": {lastResultPreview: "result B"},
	}}
	got := resolveTaskReferences("combine #1 and #2", slotMap)
	if !strContains(got, "[#1 result]: result A") || !strContains(got, "[#2 result]: result B") {
		t.Errorf("expected both results, got %q", got)
	}
}

func TestResolveTaskReferences_DuplicateRef(t *testing.T) {
	slotMap := &chatSlotMap{slots: map[string]*sessionSlot{
		"#1": {lastResultPreview: "result A"},
	}}
	got := resolveTaskReferences("#1 and also #1", slotMap)
	// Should only include #1 result once.
	count := strings.Count(got, "[#1 result]")
	if count != 1 {
		t.Errorf("expected 1 occurrence of #1 result, got %d in %q", count, got)
	}
}

// ---------------------------------------------------------------------------
// Tests for workerSnapshot with ResultPreview (cross-task awareness)
// ---------------------------------------------------------------------------

func TestWorkerSnapshot_StatusSummary_CompletedWithResult(t *testing.T) {
	snap := workerSnapshot{
		Phase:         slotIdle,
		TaskID:        "#1",
		TaskDesc:      "analyze code",
		ResultPreview: "found 3 issues",
	}
	s := snap.StatusSummary("zh")
	if !strContains(s, "#1 已完成") || !strContains(s, "found 3 issues") {
		t.Errorf("expected completed summary with result, got %q", s)
	}
	sEn := snap.StatusSummary("en")
	if !strContains(sEn, "#1 done") || !strContains(sEn, "found 3 issues") {
		t.Errorf("expected completed summary with result (en), got %q", sEn)
	}
}

func TestSnapshotAllWorkers_IncludesCompletedWithResult(t *testing.T) {
	g := &Gateway{logger: logging.OrNop(nil), now: time.Now}
	slotMap := &chatSlotMap{slots: map[string]*sessionSlot{
		"#1": {phase: slotIdle, taskDesc: "task one", lastResultPreview: "result one"},
		"#2": {phase: slotIdle, taskDesc: "task two", lastResultPreview: ""},
	}}
	g.activeChatSlots.Store("chat1", slotMap)

	list := g.snapshotAllWorkers("chat1", "zh")
	// #1 has result → included; #2 has no result → excluded.
	if len(list.Snapshots) != 1 {
		t.Fatalf("expected 1 snapshot (completed with result), got %d", len(list.Snapshots))
	}
	if list.Snapshots[0].TaskID != "#1" {
		t.Errorf("expected #1, got %q", list.Snapshots[0].TaskID)
	}
}

func TestWorkerSnapshotList_StatusSummary_MixedActiveAndCompleted(t *testing.T) {
	list := workerSnapshotList{
		Snapshots: []workerSnapshot{
			{Phase: slotRunning, TaskID: "#1", TaskDesc: "running task", Elapsed: 10 * time.Second},
			{Phase: slotIdle, TaskID: "#2", TaskDesc: "done task", ResultPreview: "some result"},
		},
		Lang: "zh",
	}
	s := list.StatusSummary()
	if !strContains(s, "1 个任务执行中") || !strContains(s, "1 个已完成") {
		t.Errorf("expected mixed summary, got %q", s)
	}
}

// ---------------------------------------------------------------------------
// Tests for chatSlotMap.resultPreview
// ---------------------------------------------------------------------------

func TestChatSlotMap_ResultPreview(t *testing.T) {
	slotMap := &chatSlotMap{slots: map[string]*sessionSlot{
		"#1": {lastResultPreview: "hello"},
	}}
	if got := slotMap.resultPreview("#1"); got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
	if got := slotMap.resultPreview("#99"); got != "" {
		t.Errorf("expected empty for missing slot, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

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
