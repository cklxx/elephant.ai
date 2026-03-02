package lark

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	ports "alex/internal/domain/agent/ports"
	portsllm "alex/internal/domain/agent/ports/llm"
	runtimeconfig "alex/internal/shared/config"
)

type rephraseStubLLMClient struct {
	mu   sync.Mutex
	resp string
	err  error
	reqs []ports.CompletionRequest
}

func (c *rephraseStubLLMClient) Complete(_ context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	c.mu.Lock()
	c.reqs = append(c.reqs, req)
	c.mu.Unlock()
	if c.err != nil {
		return nil, c.err
	}
	return &ports.CompletionResponse{Content: c.resp}, nil
}

func (c *rephraseStubLLMClient) Model() string { return "stub-rephrase" }

type rephraseStubFactory struct {
	client portsllm.LLMClient
}

func (f *rephraseStubFactory) GetClient(_, _ string, _ portsllm.LLMConfig) (portsllm.LLMClient, error) {
	return f.client, nil
}

func (f *rephraseStubFactory) GetIsolatedClient(_, _ string, _ portsllm.LLMConfig) (portsllm.LLMClient, error) {
	return f.client, nil
}

func (f *rephraseStubFactory) DisableRetry() {}

func TestRephraseForUser_Success(t *testing.T) {
	stub := &rephraseStubLLMClient{resp: "数据库查询已优化，P99 降至 15ms。"}
	g := &Gateway{
		llmFactory: &rephraseStubFactory{client: stub},
		llmProfile: runtimeconfig.LLMProfile{Provider: "openai", Model: "gpt-4o-mini"},
	}

	raw := "task_id: abc123\nstatus: completed\nmerge: merged\n\n优化了数据库查询..."
	result := g.rephraseForUser(context.Background(), raw, rephraseBackground)

	if result != "数据库查询已优化，P99 降至 15ms。" {
		t.Fatalf("expected rephrased text, got %q", result)
	}

	stub.mu.Lock()
	defer stub.mu.Unlock()
	if len(stub.reqs) != 1 {
		t.Fatalf("expected 1 LLM request, got %d", len(stub.reqs))
	}
	if stub.reqs[0].Temperature != 0.3 {
		t.Fatalf("expected temperature 0.3, got %v", stub.reqs[0].Temperature)
	}
	if stub.reqs[0].MaxTokens != 400 {
		t.Fatalf("expected maxTokens %d, got %d", 400, stub.reqs[0].MaxTokens)
	}
	if len(stub.reqs[0].Messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(stub.reqs[0].Messages))
	}
	if !strings.Contains(stub.reqs[0].Messages[0].Content, "后台任务") {
		t.Fatalf("expected background system prompt, got %q", stub.reqs[0].Messages[0].Content)
	}
}

func TestRephraseForUser_ForegroundPrompt(t *testing.T) {
	stub := &rephraseStubLLMClient{resp: "简洁回答"}
	g := &Gateway{
		llmFactory: &rephraseStubFactory{client: stub},
		llmProfile: runtimeconfig.LLMProfile{Provider: "openai", Model: "gpt-4o-mini"},
	}

	g.rephraseForUser(context.Background(), "一些长文本", rephraseForeground)

	stub.mu.Lock()
	defer stub.mu.Unlock()
	if len(stub.reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(stub.reqs))
	}
	if !strings.Contains(stub.reqs[0].Messages[0].Content, "简洁易读") {
		t.Fatalf("expected foreground system prompt, got %q", stub.reqs[0].Messages[0].Content)
	}
}

func TestRephraseForUser_NilFactory(t *testing.T) {
	g := &Gateway{}
	raw := "some raw output"
	result := g.rephraseForUser(context.Background(), raw, rephraseBackground)
	if result != raw {
		t.Fatalf("expected raw text returned when factory is nil, got %q", result)
	}
}

func TestRephraseForUser_NilGateway(t *testing.T) {
	var g *Gateway
	raw := "some raw output"
	result := g.rephraseForUser(context.Background(), raw, rephraseBackground)
	if result != raw {
		t.Fatalf("expected raw text returned when gateway is nil, got %q", result)
	}
}

func TestRephraseForUser_EmptyProfile(t *testing.T) {
	stub := &rephraseStubLLMClient{resp: "should not be called"}
	g := &Gateway{
		llmFactory: &rephraseStubFactory{client: stub},
		llmProfile: runtimeconfig.LLMProfile{},
	}

	raw := "some raw output"
	result := g.rephraseForUser(context.Background(), raw, rephraseBackground)
	if result != raw {
		t.Fatalf("expected raw text when profile is empty, got %q", result)
	}

	stub.mu.Lock()
	defer stub.mu.Unlock()
	if len(stub.reqs) != 0 {
		t.Fatalf("expected no LLM calls with empty profile, got %d", len(stub.reqs))
	}
}

func TestRephraseForUser_LLMError(t *testing.T) {
	stub := &rephraseStubLLMClient{err: errors.New("llm unavailable")}
	g := &Gateway{
		llmFactory: &rephraseStubFactory{client: stub},
		llmProfile: runtimeconfig.LLMProfile{Provider: "openai", Model: "gpt-4o-mini"},
	}

	raw := "some raw output"
	result := g.rephraseForUser(context.Background(), raw, rephraseBackground)
	if result != raw {
		t.Fatalf("expected raw text on LLM error, got %q", result)
	}
}

func TestRephraseForUser_EmptyResponse(t *testing.T) {
	stub := &rephraseStubLLMClient{resp: "   "}
	g := &Gateway{
		llmFactory: &rephraseStubFactory{client: stub},
		llmProfile: runtimeconfig.LLMProfile{Provider: "openai", Model: "gpt-4o-mini"},
	}

	raw := "some raw output"
	result := g.rephraseForUser(context.Background(), raw, rephraseBackground)
	if result != raw {
		t.Fatalf("expected raw text on empty LLM response, got %q", result)
	}
}

func TestRephraseForUser_DisabledByConfig(t *testing.T) {
	stub := &rephraseStubLLMClient{resp: "should not be called"}
	disabled := false
	g := &Gateway{
		cfg:        Config{RephraseEnabled: &disabled},
		llmFactory: &rephraseStubFactory{client: stub},
		llmProfile: runtimeconfig.LLMProfile{Provider: "openai", Model: "gpt-4o-mini"},
	}

	raw := "some raw output"
	result := g.rephraseForUser(context.Background(), raw, rephraseBackground)
	if result != raw {
		t.Fatalf("expected raw text when disabled, got %q", result)
	}

	stub.mu.Lock()
	defer stub.mu.Unlock()
	if len(stub.reqs) != 0 {
		t.Fatalf("expected no LLM calls when disabled, got %d", len(stub.reqs))
	}
}

func TestRephraseForUser_TruncatesLongInput(t *testing.T) {
	stub := &rephraseStubLLMClient{resp: "short"}
	g := &Gateway{
		llmFactory: &rephraseStubFactory{client: stub},
		llmProfile: runtimeconfig.LLMProfile{Provider: "openai", Model: "gpt-4o-mini"},
	}

	raw := strings.Repeat("中", rephraseMaxInput+500)
	g.rephraseForUser(context.Background(), raw, rephraseBackground)

	stub.mu.Lock()
	defer stub.mu.Unlock()
	if len(stub.reqs) != 1 {
		t.Fatalf("expected 1 request, got %d", len(stub.reqs))
	}
	userContent := stub.reqs[0].Messages[1].Content
	if len([]rune(userContent)) != rephraseMaxInput {
		t.Fatalf("expected input truncated to %d runes, got %d", rephraseMaxInput, len([]rune(userContent)))
	}
}

func TestRephraseForUser_BlankInput(t *testing.T) {
	stub := &rephraseStubLLMClient{resp: "should not be called"}
	g := &Gateway{
		llmFactory: &rephraseStubFactory{client: stub},
		llmProfile: runtimeconfig.LLMProfile{Provider: "openai", Model: "gpt-4o-mini"},
	}

	raw := "   "
	result := g.rephraseForUser(context.Background(), raw, rephraseBackground)
	if result != raw {
		t.Fatalf("expected raw text on blank input, got %q", result)
	}
}

func TestBackgroundCompletion_NoTechnicalFields(t *testing.T) {
	stub := &rephraseStubLLMClient{resp: "已完成数据库优化，P99 降至 15ms。"}
	recorder := NewRecordingMessenger()
	g := &Gateway{
		messenger:  recorder,
		llmFactory: &rephraseStubFactory{client: stub},
		llmProfile: runtimeconfig.LLMProfile{Provider: "openai", Model: "gpt-4o-mini"},
	}

	ln := newBackgroundProgressListener(
		context.Background(),
		nil,
		g,
		"chat-1",
		"om_parent",
		nil,
		50*time.Millisecond,
		10*time.Minute,
	)
	defer ln.Close()

	tracker := &bgTaskTracker{
		taskID:         "bg-1",
		description:    "优化数据库查询",
		startedAt:      time.Now().Add(-5 * time.Minute),
		status:         "completed",
		mergeStatus:    "merged",
		pendingSummary: "优化了 users 表查询，全表扫描改为索引查询",
		progressMsgID:  "msg-1",
		stopCh:         make(chan struct{}),
		doneCh:         make(chan struct{}),
	}

	ln.flush(tracker, true)

	updates := recorder.CallsByMethod("UpdateMessage")
	if len(updates) == 0 {
		t.Fatal("expected at least one UpdateMessage call")
	}
	text := updates[len(updates)-1].Content

	if strings.Contains(text, "task_id") {
		t.Fatalf("completion message should not contain task_id, got %q", text)
	}
	if strings.Contains(text, "status:") {
		t.Fatalf("completion message should not contain status: field, got %q", text)
	}
	if strings.Contains(text, "merge:") {
		t.Fatalf("completion message should not contain merge:, got %q", text)
	}
	if !strings.Contains(text, "已完成数据库优化") {
		t.Fatalf("expected rephrased content, got %q", text)
	}
}
