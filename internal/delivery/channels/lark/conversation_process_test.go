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
// Stub LLM helpers
// ---------------------------------------------------------------------------

type convStubLLMClient struct {
	mu   sync.Mutex
	resp string
	err  error
	reqs []ports.CompletionRequest
}

func (c *convStubLLMClient) Complete(_ context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	c.mu.Lock()
	c.reqs = append(c.reqs, req)
	c.mu.Unlock()
	if c.err != nil {
		return nil, c.err
	}
	return &ports.CompletionResponse{Content: c.resp}, nil
}

func (c *convStubLLMClient) Model() string { return "stub-conv" }

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

func newConvGateway(t *testing.T, stub *convStubLLMClient, enabled bool) *Gateway {
	t.Helper()
	en := enabled
	gw := &Gateway{
		cfg: Config{
			BaseConfig: channels.BaseConfig{
				SessionPrefix: "test",
				AllowDirect:   true,
			},
			ConversationProcessEnabled: &en,
		},
		llmFactory: &convStubFactory{client: stub},
		llmProfile: config.LLMProfile{Provider: "openai", Model: "gpt-4o-mini"},
		logger:     logging.OrNop(nil),
		now:        time.Now,
	}
	return gw
}

// ---------------------------------------------------------------------------
// Tests for parseClassifyVerdict
// ---------------------------------------------------------------------------

func TestParseClassifyVerdict_Answer(t *testing.T) {
	snap := workerSnapshot{Phase: slotIdle}
	if v := parseClassifyVerdict("ANSWER", snap); v != verdictAnswer {
		t.Fatalf("expected verdictAnswer, got %s", verdictString(v))
	}
}

func TestParseClassifyVerdict_AnswerWithTrailing(t *testing.T) {
	snap := workerSnapshot{Phase: slotIdle}
	if v := parseClassifyVerdict("ANSWER.", snap); v != verdictAnswer {
		t.Fatalf("expected verdictAnswer, got %s", verdictString(v))
	}
}

func TestParseClassifyVerdict_Delegate(t *testing.T) {
	snap := workerSnapshot{Phase: slotIdle}
	if v := parseClassifyVerdict("DELEGATE", snap); v != verdictDelegate {
		t.Fatalf("expected verdictDelegate, got %s", verdictString(v))
	}
}

func TestParseClassifyVerdict_RelayWhenRunning(t *testing.T) {
	snap := workerSnapshot{Phase: slotRunning}
	if v := parseClassifyVerdict("RELAY", snap); v != verdictRelay {
		t.Fatalf("expected verdictRelay, got %s", verdictString(v))
	}
}

func TestParseClassifyVerdict_RelayWhenIdle_FallsBackToDelegate(t *testing.T) {
	snap := workerSnapshot{Phase: slotIdle}
	if v := parseClassifyVerdict("RELAY", snap); v != verdictDelegate {
		t.Fatalf("expected verdictDelegate (downgraded from RELAY), got %s", verdictString(v))
	}
}

func TestParseClassifyVerdict_ForkWhenRunning(t *testing.T) {
	snap := workerSnapshot{Phase: slotRunning}
	if v := parseClassifyVerdict("FORK", snap); v != verdictFork {
		t.Fatalf("expected verdictFork, got %s", verdictString(v))
	}
}

func TestParseClassifyVerdict_ForkWhenIdle_FallsBackToDelegate(t *testing.T) {
	snap := workerSnapshot{Phase: slotIdle}
	if v := parseClassifyVerdict("FORK", snap); v != verdictDelegate {
		t.Fatalf("expected verdictDelegate (downgraded from FORK), got %s", verdictString(v))
	}
}

func TestParseClassifyVerdict_UnknownFallsToDelegate(t *testing.T) {
	snap := workerSnapshot{Phase: slotIdle}
	if v := parseClassifyVerdict("SOMETHING_ELSE", snap); v != verdictDelegate {
		t.Fatalf("expected verdictDelegate for unknown, got %s", verdictString(v))
	}
}

func TestParseClassifyVerdict_CaseInsensitive(t *testing.T) {
	snap := workerSnapshot{Phase: slotIdle}
	if v := parseClassifyVerdict("answer", snap); v != verdictAnswer {
		t.Fatalf("expected verdictAnswer for lowercase, got %s", verdictString(v))
	}
}

// ---------------------------------------------------------------------------
// Tests for conversationClassify (with mock LLM)
// ---------------------------------------------------------------------------

func TestConversationClassify_ReturnsAnswer(t *testing.T) {
	stub := &convStubLLMClient{resp: "ANSWER"}
	g := newConvGateway(t, stub, true)
	snap := workerSnapshot{Phase: slotIdle}

	v := g.conversationClassify(context.Background(), "你好", snap)
	if v != verdictAnswer {
		t.Fatalf("expected verdictAnswer, got %s", verdictString(v))
	}
}

func TestConversationClassify_ReturnsDelegate(t *testing.T) {
	stub := &convStubLLMClient{resp: "DELEGATE"}
	g := newConvGateway(t, stub, true)
	snap := workerSnapshot{Phase: slotIdle}

	v := g.conversationClassify(context.Background(), "重构 auth 模块", snap)
	if v != verdictDelegate {
		t.Fatalf("expected verdictDelegate, got %s", verdictString(v))
	}
}

func TestConversationClassify_FallbackOnLLMError(t *testing.T) {
	stub := &convStubLLMClient{err: &convStubErr{"timeout"}}
	g := newConvGateway(t, stub, true)
	snap := workerSnapshot{Phase: slotIdle}

	v := g.conversationClassify(context.Background(), "hello", snap)
	if v != verdictDelegate {
		t.Fatalf("expected verdictDelegate fallback on error, got %s", verdictString(v))
	}
}

func TestConversationClassify_FallbackWhenFactoryNil(t *testing.T) {
	en := true
	g := &Gateway{
		cfg: Config{
			ConversationProcessEnabled: &en,
		},
		llmFactory: nil,
		logger:     logging.OrNop(nil),
		now:        time.Now,
	}
	snap := workerSnapshot{Phase: slotIdle}

	v := g.conversationClassify(context.Background(), "hi", snap)
	if v != verdictDelegate {
		t.Fatalf("expected verdictDelegate when factory nil, got %s", verdictString(v))
	}
}

func TestConversationClassify_PromptIncludesWorkerStatus(t *testing.T) {
	stub := &convStubLLMClient{resp: "RELAY"}
	g := newConvGateway(t, stub, true)
	snap := workerSnapshot{Phase: slotRunning, TaskDesc: "build dashboard", Elapsed: 30 * time.Second}

	g.conversationClassify(context.Background(), "用 PostgreSQL", snap)

	stub.mu.Lock()
	defer stub.mu.Unlock()
	if len(stub.reqs) == 0 {
		t.Fatal("expected at least one LLM request")
	}
	userContent := stub.reqs[0].Messages[1].Content
	if !contains(userContent, "build dashboard") {
		t.Errorf("user prompt should contain task desc, got %q", userContent)
	}
	if !contains(userContent, "用 PostgreSQL") {
		t.Errorf("user prompt should contain user message, got %q", userContent)
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
		t.Fatal("expected enabled when *bool=true")
	}
}

func TestConversationProcessEnabled_False(t *testing.T) {
	en := false
	g := &Gateway{cfg: Config{ConversationProcessEnabled: &en}}
	if g.conversationProcessEnabled() {
		t.Fatal("expected disabled when *bool=false")
	}
}

// ---------------------------------------------------------------------------
// Tests for resolveConversationProfile
// ---------------------------------------------------------------------------

func TestResolveConversationProfile_NoOverride(t *testing.T) {
	g := &Gateway{
		cfg:        Config{ConversationModel: ""},
		llmProfile: config.LLMProfile{Provider: "openai", Model: "gpt-4o"},
	}
	p := g.resolveConversationProfile()
	if p.Model != "gpt-4o" {
		t.Fatalf("expected gpt-4o, got %q", p.Model)
	}
}

func TestResolveConversationProfile_WithOverride(t *testing.T) {
	g := &Gateway{
		cfg:        Config{ConversationModel: "gpt-4o-mini"},
		llmProfile: config.LLMProfile{Provider: "openai", Model: "gpt-4o"},
	}
	p := g.resolveConversationProfile()
	if p.Model != "gpt-4o-mini" {
		t.Fatalf("expected override gpt-4o-mini, got %q", p.Model)
	}
	if p.Provider != "openai" {
		t.Fatalf("expected provider preserved, got %q", p.Provider)
	}
}

// ---------------------------------------------------------------------------
// Tests for handleViaConversationProcess (Phase 1: always returns false)
// ---------------------------------------------------------------------------

func TestHandleViaConversationProcess_Phase1AlwaysFallsThrough(t *testing.T) {
	stub := &convStubLLMClient{resp: "ANSWER"}
	g := newConvGateway(t, stub, true)

	msg := &incomingMessage{
		chatID:  "chat1",
		content: "你好",
	}
	slot := &sessionSlot{}

	handled := g.handleViaConversationProcess(context.Background(), msg, slot)
	if handled {
		t.Fatal("Phase 1 should always return false (fall through)")
	}
}

func TestHandleViaConversationProcess_ClassifyCalled(t *testing.T) {
	stub := &convStubLLMClient{resp: "DELEGATE"}
	g := newConvGateway(t, stub, true)

	msg := &incomingMessage{
		chatID:  "chat1",
		content: "重构 auth",
	}
	slot := &sessionSlot{}

	g.handleViaConversationProcess(context.Background(), msg, slot)

	stub.mu.Lock()
	defer stub.mu.Unlock()
	if len(stub.reqs) == 0 {
		t.Fatal("expected classify LLM call to be made")
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
		Phase:    slotRunning,
		TaskDesc: "build a dashboard",
		Elapsed:  45 * time.Second,
	}
	s := snap.StatusSummary()
	if !contains(s, "build a dashboard") {
		t.Errorf("expected task desc in summary, got %q", s)
	}
	if !contains(s, "45s") {
		t.Errorf("expected elapsed in summary, got %q", s)
	}
}

func TestWorkerSnapshot_StatusSummary_AwaitingInput(t *testing.T) {
	snap := workerSnapshot{Phase: slotAwaitingInput}
	if snap.StatusSummary() != "任务等待用户输入中" {
		t.Fatalf("unexpected summary: %q", snap.StatusSummary())
	}
}

func TestWorkerSnapshot_IsIdle(t *testing.T) {
	if !(workerSnapshot{Phase: slotIdle}).IsIdle() {
		t.Fatal("expected IsIdle=true")
	}
	if (workerSnapshot{Phase: slotRunning}).IsIdle() {
		t.Fatal("expected IsIdle=false for running")
	}
}

func TestWorkerSnapshot_IsRunning(t *testing.T) {
	if !(workerSnapshot{Phase: slotRunning}).IsRunning() {
		t.Fatal("expected IsRunning=true")
	}
	if (workerSnapshot{Phase: slotIdle}).IsRunning() {
		t.Fatal("expected IsRunning=false for idle")
	}
}

func TestSnapshotWorker_Idle(t *testing.T) {
	g := &Gateway{
		logger: logging.OrNop(nil),
		now:    time.Now,
	}
	snap := g.snapshotWorker("nonexistent-chat")
	if !snap.IsIdle() {
		t.Fatal("expected idle snapshot for nonexistent chat")
	}
}

func TestSnapshotWorker_Running(t *testing.T) {
	g := &Gateway{
		logger: logging.OrNop(nil),
		now:    time.Now,
	}
	slot := &sessionSlot{
		phase:    slotRunning,
		taskDesc: "refactor auth",
	}
	slot.lastTouched = time.Now().Add(-30 * time.Second)
	g.activeSlots.Store("chat1", slot)

	snap := g.snapshotWorker("chat1")
	if !snap.IsRunning() {
		t.Fatal("expected running snapshot")
	}
	if snap.TaskDesc != "refactor auth" {
		t.Fatalf("expected task desc, got %q", snap.TaskDesc)
	}
	if snap.Elapsed < 29*time.Second {
		t.Fatalf("expected ~30s elapsed, got %v", snap.Elapsed)
	}
}

// ---------------------------------------------------------------------------
// Tests for verdictString
// ---------------------------------------------------------------------------

func TestVerdictString(t *testing.T) {
	tests := []struct {
		v    conversationVerdict
		want string
	}{
		{verdictAnswer, "ANSWER"},
		{verdictDelegate, "DELEGATE"},
		{verdictRelay, "RELAY"},
		{verdictFork, "FORK"},
		{conversationVerdict(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := verdictString(tt.v); got != tt.want {
			t.Errorf("verdictString(%d) = %q, want %q", tt.v, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

type convStubErr struct{ msg string }

func (e *convStubErr) Error() string { return e.msg }
