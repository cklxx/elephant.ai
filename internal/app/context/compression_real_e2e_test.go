package context

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/ports/mocks"
	"alex/internal/domain/agent/react"
)

// ---------------------------------------------------------------------------
// Real-injection E2E tests: wire the real context manager (real token
// estimator, real compression plan, real flush hook) through the real
// ReactEngine.SolveTask() pipeline. Only the LLM is a stub (external
// boundary).
// ---------------------------------------------------------------------------

func realContextManager(opts ...Option) agent.ContextManager {
	return NewManager(opts...)
}

func buildE2EConversation(turns int, contentSize int) []ports.Message {
	msgs := []ports.Message{
		{Role: "system", Content: "You are a helpful assistant.", Source: ports.MessageSourceSystemPrompt},
	}
	for i := 0; i < turns; i++ {
		body := fmt.Sprintf("turn-%d %s", i, strings.Repeat("x", contentSize))
		msgs = append(msgs,
			ports.Message{Role: "user", Content: body, Source: ports.MessageSourceUserInput},
			ports.Message{Role: "assistant", Content: body, Source: ports.MessageSourceAssistantReply},
		)
	}
	return msgs
}

func captureE2ELLM() (*mocks.MockLLMClient, func() ports.CompletionRequest) {
	var captured ports.CompletionRequest
	var mu sync.Mutex

	llm := &mocks.MockLLMClient{
		StreamCompleteFunc: func(_ context.Context, req ports.CompletionRequest, cb ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
			mu.Lock()
			captured = req
			mu.Unlock()
			if cb.OnContentDelta != nil {
				cb.OnContentDelta(ports.ContentDelta{Delta: "ok", Final: true})
			}
			return &ports.CompletionResponse{
				Content:    "ok",
				StopReason: "stop",
				Usage:      ports.TokenUsage{TotalTokens: 10},
			}, nil
		},
	}
	return llm, func() ports.CompletionRequest {
		mu.Lock()
		defer mu.Unlock()
		return captured
	}
}

func newE2EEngine(tokenLimit int) *react.ReactEngine {
	return react.NewReactEngine(react.ReactEngineConfig{
		MaxIterations: 1,
		Logger:        agent.NoopLogger{},
		Clock:         agent.SystemClock{},
		CompletionDefaults: react.CompletionDefaults{
			ContextTokenLimit: &tokenLimit,
		},
	})
}

func solveOnce(t *testing.T, engine *react.ReactEngine, msgs []ports.Message, ctxMgr agent.ContextManager, llm *mocks.MockLLMClient) {
	t.Helper()
	_, err := engine.SolveTask(context.Background(), "test task", &react.TaskState{
		Messages: msgs,
	}, react.Services{
		LLM:          llm,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      ctxMgr,
	})
	if err != nil {
		t.Fatalf("SolveTask error: %v", err)
	}
}

// TestRealE2E_CompressOverLimit wires the real context manager into the real
// ReactEngine and verifies that compression fires when messages exceed budget.
func TestRealE2E_CompressOverLimit(t *testing.T) {
	llm, getReq := captureE2ELLM()
	ctxMgr := realContextManager()

	// Real token estimator: 20 turns × 1000 chars ≈ ~5500 tokens.
	// Limit 4000 so 70% threshold (~2800) is exceeded → compression fires.
	engine := newE2EEngine(4000)

	msgs := buildE2EConversation(20, 1000)
	originalCount := len(msgs)

	// Pre-verify: real estimator says these exceed 70% of limit.
	estimated := ctxMgr.EstimateTokens(msgs)
	t.Logf("Pre-compression: %d messages, ~%d tokens, limit=4000", len(msgs), estimated)

	solveOnce(t, engine, msgs, ctxMgr, llm)

	req := getReq()
	t.Logf("Post-compression: %d messages sent to LLM", len(req.Messages))
	if len(req.Messages) >= originalCount {
		t.Fatalf("expected compressed messages < %d, got %d", originalCount, len(req.Messages))
	}

	// A compression or trim summary must exist.
	hasSummary := false
	for _, msg := range req.Messages {
		if strings.Contains(msg.Content, "Earlier context compressed") ||
			strings.Contains(msg.Content, "Context trimmed") ||
			strings.Contains(msg.Content, "context truncated") {
			hasSummary = true
			break
		}
	}
	if !hasSummary {
		for i, msg := range req.Messages {
			t.Logf("  msg[%d] role=%s source=%s content=%.80s", i, msg.Role, msg.Source, msg.Content)
		}
		t.Error("expected a compression/trim summary in sent messages")
	}
}

// TestRealE2E_PreservesRecentTurn verifies the most recent user turn survives
// real compression.
func TestRealE2E_PreservesRecentTurn(t *testing.T) {
	llm, getReq := captureE2ELLM()
	ctxMgr := realContextManager()

	// 8 turns × 200 chars, limit generous enough that AutoCompact (not
	// forceFit) handles it. The real estimator puts this around ~900 tokens.
	engine := newE2EEngine(4000)

	msgs := buildE2EConversation(8, 200)
	lastUserContent := msgs[len(msgs)-2].Content

	solveOnce(t, engine, msgs, ctxMgr, llm)

	req := getReq()
	found := false
	for _, msg := range req.Messages {
		if msg.Role == "user" && strings.Contains(msg.Content, "turn-7") {
			found = true
			break
		}
	}
	if !found {
		// Also check if content was truncated but the turn identifier survives.
		for _, msg := range req.Messages {
			if msg.Role == "user" && msg.Content == lastUserContent {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("most recent user turn (turn-7) not found in compressed output")
		for i, msg := range req.Messages {
			t.Logf("  msg[%d] role=%s content=%.80s", i, msg.Role, msg.Content)
		}
	}
}

// TestRealE2E_NoCompressionBelowThreshold verifies no compression fires when
// messages fit comfortably within the budget.
func TestRealE2E_NoCompressionBelowThreshold(t *testing.T) {
	llm, getReq := captureE2ELLM()
	ctxMgr := realContextManager()

	engine := newE2EEngine(200_000)

	msgs := buildE2EConversation(3, 50)
	originalCount := len(msgs)

	solveOnce(t, engine, msgs, ctxMgr, llm)

	req := getReq()
	if len(req.Messages) < originalCount {
		t.Errorf("expected >= %d messages (no compression), got %d", originalCount, len(req.Messages))
	}
}

// TestRealE2E_FlushHookCalledBeforeCompaction verifies that the real
// MemoryFlushHook receives messages before they are compressed away.
func TestRealE2E_FlushHookCalledBeforeCompaction(t *testing.T) {
	var flushedContent string
	var mu sync.Mutex

	saveFn := func(_ context.Context, content string, _ map[string]string) error {
		mu.Lock()
		flushedContent = content
		mu.Unlock()
		return nil
	}
	flushHook := NewMemoryFlushHook(saveFn)
	ctxMgr := realContextManager(WithFlushHook(flushHook))
	llm, _ := captureE2ELLM()

	engine := newE2EEngine(4000)

	msgs := buildE2EConversation(20, 1000)
	solveOnce(t, engine, msgs, ctxMgr, llm)

	mu.Lock()
	content := flushedContent
	mu.Unlock()

	if content == "" {
		t.Error("flush hook was not called — expected it to receive messages before compaction")
	}
	if !strings.Contains(content, "[User messages]") {
		t.Errorf("flush content missing user section, got: %s", content[:min(len(content), 200)])
	}
}

// TestRealE2E_PreservesImportantAndCheckpoint verifies that system, important,
// and checkpoint messages survive real compression.
func TestRealE2E_PreservesImportantAndCheckpoint(t *testing.T) {
	llm, getReq := captureE2ELLM()
	ctxMgr := realContextManager()

	// Use a generous limit so AutoCompact or aggressive trim can work
	// without falling through to forceFit (which strips to system+last only).
	engine := newE2EEngine(8000)

	msgs := []ports.Message{
		{Role: "system", Content: "System prompt", Source: ports.MessageSourceSystemPrompt},
		{Role: "system", Content: "IMPORTANT: never do X", Source: ports.MessageSourceImportant},
		{Role: "user", Content: strings.Repeat("old question ", 30), Source: ports.MessageSourceUserInput},
		{Role: "assistant", Content: strings.Repeat("old answer ", 30), Source: ports.MessageSourceAssistantReply},
		{Role: "user", Content: "checkpoint summary", Source: ports.MessageSourceCheckpoint},
		{Role: "user", Content: strings.Repeat("mid question ", 20), Source: ports.MessageSourceUserInput},
		{Role: "assistant", Content: strings.Repeat("mid answer ", 20), Source: ports.MessageSourceAssistantReply},
		{Role: "user", Content: "latest question", Source: ports.MessageSourceUserInput},
		{Role: "assistant", Content: "latest answer", Source: ports.MessageSourceAssistantReply},
	}

	estimated := ctxMgr.EstimateTokens(msgs)
	t.Logf("Messages: %d, estimated tokens: %d, limit: 8000", len(msgs), estimated)

	solveOnce(t, engine, msgs, ctxMgr, llm)

	req := getReq()
	t.Logf("LLM received %d messages", len(req.Messages))

	hasImportant := false
	hasCheckpoint := false
	for i, msg := range req.Messages {
		t.Logf("  msg[%d] role=%s source=%s content=%.60s", i, msg.Role, msg.Source, msg.Content)
		if msg.Source == ports.MessageSourceImportant {
			hasImportant = true
		}
		if msg.Source == ports.MessageSourceCheckpoint {
			hasCheckpoint = true
		}
	}
	if !hasImportant {
		t.Error("Important message lost after real compression")
	}
	if !hasCheckpoint {
		t.Error("Checkpoint message lost after real compression")
	}
}

// TestRealE2E_StressMultiRound runs 500 rounds through the real context
// manager and verifies compression keeps messages bounded.
func TestRealE2E_StressMultiRound(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in -short mode")
	}

	ctxMgr := realContextManager()

	const (
		totalRounds        = 500
		contextWindowLimit = 8_000
		userContentSize    = 200
	)

	messages := []ports.Message{
		{Role: "system", Content: "You are a helpful assistant.", Source: ports.MessageSourceSystemPrompt},
	}

	var compressionCount int
	var maxMessageCount int

	for round := 0; round < totalRounds; round++ {
		userMsg := ports.Message{
			Role:    "user",
			Content: fmt.Sprintf("Round %d question: %s", round, strings.Repeat("q", userContentSize)),
			Source:  ports.MessageSourceUserInput,
		}
		messages = append(messages, userMsg)

		if ctxMgr.ShouldCompress(messages, contextWindowLimit) {
			compacted, didCompact := ctxMgr.AutoCompact(messages, contextWindowLimit)
			if didCompact {
				compressionCount++
				messages = compacted
			}

			estimated := ctxMgr.EstimateTokens(messages)
			if signal := BudgetCheck(estimated, contextWindowLimit, defaultThreshold, 0.85); signal == BudgetAggressiveTrim {
				messages = AggressiveTrim(messages, 2)
			}
		}

		assistantMsg := ports.Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Round %d reply: %s", round, strings.Repeat("a", userContentSize)),
			Source:  ports.MessageSourceAssistantReply,
		}
		messages = append(messages, assistantMsg)

		if len(messages) > maxMessageCount {
			maxMessageCount = len(messages)
		}
	}

	if compressionCount == 0 {
		t.Error("real compression never fired across 500 rounds")
	}

	if maxMessageCount > 200 {
		t.Errorf("max message count %d — compression not bounding growth", maxMessageCount)
	}

	foundSystem := false
	for _, msg := range messages {
		if msg.Source == ports.MessageSourceSystemPrompt {
			foundSystem = true
			break
		}
	}
	if !foundSystem {
		t.Error("system prompt lost after 500 rounds of real compression")
	}

	t.Logf("Real stress: %d rounds, %d compressions, peak %d messages, final %d messages",
		totalRounds, compressionCount, maxMessageCount, len(messages))
}
