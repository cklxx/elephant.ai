package lark

import (
	"context"
	"strings"
	"testing"
	"time"

	"alex/internal/delivery/channels"
	agent "alex/internal/domain/agent/ports/agent"
	portsllm "alex/internal/domain/agent/ports/llm"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
)

// tieredTestGateway builds a Gateway wired for tiered delivery unit tests.
// The stub LLM client records calls so tests can assert whether rephrase was invoked.
func tieredTestGateway(stub *rephraseStubLLMClient, rec *RecordingMessenger) *Gateway {
	return &Gateway{
		cfg: Config{
			AppID:                  "test",
			AppSecret:              "secret",
			DeliveryShortThreshold: defaultDeliveryShortThreshold,
			DeliveryDocThreshold:   defaultDeliveryDocThreshold,
		},
		logger:     logging.OrNop(nil),
		messenger:  rec,
		llmFactory: &rephraseStubFactory{client: stub},
		llmProfile: runtimeconfig.LLMProfile{Provider: "openai", Model: "gpt-4o-mini"},
	}
}

func TestTieredDelivery_ShortAnswer_NoRephrase(t *testing.T) {
	stub := &rephraseStubLLMClient{resp: "should not be called"}
	rec := NewRecordingMessenger()
	gw := tieredTestGateway(stub, rec)

	// 100 runes — well under shortThreshold (300)
	shortAnswer := strings.Repeat("测", 100)
	result := &agent.TaskResult{Answer: shortAnswer}

	reply := gw.tieredDelivery(context.Background(), "chat1", "msg1", result, nil)

	// Should return shaped reply directly, no LLM call.
	stub.mu.Lock()
	callCount := len(stub.reqs)
	stub.mu.Unlock()
	if callCount != 0 {
		t.Fatalf("expected 0 LLM calls for short answer, got %d", callCount)
	}
	if !strings.Contains(reply, "测") {
		t.Fatalf("expected reply to contain original content, got %q", reply)
	}
}

func TestTieredDelivery_MediumAnswer_Rephrased(t *testing.T) {
	stub := &rephraseStubLLMClient{resp: "简洁版本的回答"}
	rec := NewRecordingMessenger()
	gw := tieredTestGateway(stub, rec)

	// 500 runes — between short (300) and doc (800)
	mediumAnswer := strings.Repeat("中", 500)
	result := &agent.TaskResult{Answer: mediumAnswer}

	reply := gw.tieredDelivery(context.Background(), "chat1", "msg1", result, nil)

	stub.mu.Lock()
	callCount := len(stub.reqs)
	maxTok := 0
	if callCount > 0 {
		maxTok = stub.reqs[0].MaxTokens
	}
	stub.mu.Unlock()

	if callCount != 1 {
		t.Fatalf("expected 1 LLM call for medium answer, got %d", callCount)
	}
	if maxTok != 400 {
		t.Fatalf("expected maxTokens=400, got %d", maxTok)
	}
	if reply != "简洁版本的回答" {
		t.Fatalf("expected rephrased reply, got %q", reply)
	}
}

func TestTieredDelivery_LongAnswer_DocCreated(t *testing.T) {
	stub := &rephraseStubLLMClient{resp: "摘要内容…"}
	rec := NewRecordingMessenger()
	gw := tieredTestGateway(stub, rec)

	// Provide a mock messenger that supports file upload so overflowToDoc's
	// truncateWithDoc fallback succeeds.
	rec.SetUploadFileResult("fk_test_123", nil)

	// 1000 runes — above doc threshold (800)
	longAnswer := strings.Repeat("长", 1000)
	result := &agent.TaskResult{Answer: longAnswer}

	reply := gw.tieredDelivery(context.Background(), "chat1", "msg1", result, nil)

	// overflowToDoc should have been called. The stub messenger's UploadFile
	// returns a file key, so a file message should have been dispatched
	// (via ReplyMessage since replyToID is non-empty and "file" is not standalone).
	allCalls := rec.Calls()
	hasFileMsg := false
	for _, c := range allCalls {
		if c.MsgType == "file" {
			hasFileMsg = true
			break
		}
	}
	if !hasFileMsg {
		t.Fatal("expected a file message from overflowToDoc fallback")
	}

	// The reply should have been rephrased (LLM called on the summary).
	stub.mu.Lock()
	callCount := len(stub.reqs)
	stub.mu.Unlock()
	if callCount != 1 {
		t.Fatalf("expected 1 LLM call for long answer summary, got %d", callCount)
	}
	if reply != "摘要内容…" {
		t.Fatalf("expected rephrased summary, got %q", reply)
	}
}

func TestTieredDelivery_DocFails_SplitMessageFallback(t *testing.T) {
	stub := &rephraseStubLLMClient{resp: "should not be called"}
	rec := NewRecordingMessenger()
	gw := tieredTestGateway(stub, rec)
	// No client and no messenger — both doc and file paths fail,
	// forcing overflowToDoc to return hard-truncated text.
	gw.client = nil
	gw.messenger = nil

	longAnswer := strings.Repeat("回", 1000)
	result := &agent.TaskResult{Answer: longAnswer}

	reply := gw.tieredDelivery(context.Background(), "chat1", "msg1", result, nil)

	// When both doc and file fail, tieredDelivery returns truncated text
	// (≤800 runes) with a notice instead of dumping the full content.
	if !strings.Contains(reply, "已截断显示") {
		t.Fatalf("expected truncation notice, got: %s", reply[:min(len(reply), 200)])
	}
	runeLen := len([]rune(reply))
	if runeLen > 850 {
		t.Fatalf("expected truncated reply (≤~850 runes with notice), got %d runes", runeLen)
	}
}

func TestTieredDelivery_ExactBoundary800Runes(t *testing.T) {
	stub := &rephraseStubLLMClient{resp: "简洁版本"}
	rec := NewRecordingMessenger()
	gw := tieredTestGateway(stub, rec)

	// Exactly 800 runes — should be medium tier (≤ docThreshold), not doc
	exactAnswer := strings.Repeat("边", 800)
	result := &agent.TaskResult{Answer: exactAnswer}

	reply := gw.tieredDelivery(context.Background(), "chat1", "msg1", result, nil)

	// Should be rephrased (medium tier), not overflow to doc.
	stub.mu.Lock()
	callCount := len(stub.reqs)
	stub.mu.Unlock()
	if callCount != 1 {
		t.Fatalf("expected 1 LLM call for boundary answer, got %d", callCount)
	}
	if reply != "简洁版本" {
		t.Fatalf("expected rephrased reply, got %q", reply)
	}
}

func TestFlush_UsesSmartContent(t *testing.T) {
	stub := &rephraseStubLLMClient{resp: "**已完成** 任务结果"}
	rec := NewRecordingMessenger()
	g := &Gateway{
		messenger:  rec,
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
		taskID:        "bg-1",
		description:   "测试任务",
		startedAt:     time.Now().Add(-2 * time.Minute),
		status:        taskStatusCompleted,
		pendingSummary: "一些**markdown**结果",
		progressMsgID: "msg-1",
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}

	ln.flush(tracker, true)

	updates := rec.CallsByMethod(MethodUpdateMessage)
	if len(updates) == 0 {
		t.Fatal("expected at least one UpdateMessage call")
	}
	lastUpdate := updates[len(updates)-1]
	// If the rephrased text contains markdown (bold), smartContent should
	// use interactive/post format instead of plain text.
	if lastUpdate.MsgType == "text" && strings.Contains(lastUpdate.Content, "**") {
		t.Fatalf("expected post format for markdown content, got text with content %q", lastUpdate.Content)
	}
}

func TestFlush_FallbackFormattedNewMessage(t *testing.T) {
	stub := &rephraseStubLLMClient{resp: "**完成** 任务说明"}
	rec := NewRecordingMessenger()
	// Force UpdateMessage to fail so it falls back to dispatchMessage.
	rec.SetUpdateMessageError(context.DeadlineExceeded)
	g := &Gateway{
		messenger:  rec,
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
		taskID:        "bg-2",
		description:   "测试回退",
		startedAt:     time.Now().Add(-1 * time.Minute),
		status:        taskStatusCompleted,
		pendingSummary: "回退结果",
		progressMsgID: "msg-2",
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}

	ln.flush(tracker, true)

	// UpdateMessage fails → falls back to dispatchMessage (ReplyMessage since replyToID is set).
	replies := rec.CallsByMethod(MethodReplyMessage)
	if len(replies) == 0 {
		t.Fatal("expected fallback ReplyMessage after UpdateMessage failure")
	}
	lastReply := replies[len(replies)-1]
	// The fallback should also use smartContent (not hardcoded "text").
	if lastReply.MsgType == "text" && strings.Contains(lastReply.Content, "**") {
		t.Fatalf("expected post format for markdown fallback, got text with content %q", lastReply.Content)
	}
}

// Ensure the stub factory satisfies the interface.
var _ portsllm.LLMClientFactory = (*rephraseStubFactory)(nil)

// Ensure the channel base config compiles with our test setup.
var _ channels.AgentExecutor = (*e2eExecutor)(nil)
