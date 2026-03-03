package react

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/ports/mocks"
)

// ----------------------------------------------------------------------------
// End-to-end inject tests: verify that the think() pipeline automatically
// compresses context when messages exceed the token budget, and that the
// resulting LLM request stays within limits.
// ----------------------------------------------------------------------------

// buildConversation generates a multi-turn conversation.
// Each turn is one user message + one assistant reply with distinct content.
func buildConversation(turns int, contentSize int) []ports.Message {
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

// contentAwareEstimate returns a token estimator that uses actual content length.
// Roughly: 4 overhead + len(content)/4 per message.
func contentAwareEstimate(msgs []ports.Message) int {
	total := 0
	for _, msg := range msgs {
		total += 4 + len([]rune(msg.Content))/4
	}
	return total
}

// captureLLMRequest returns a mock LLM and a function to retrieve the captured request.
func captureLLMRequest() (*mocks.MockLLMClient, func() ports.CompletionRequest) {
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

	getter := func() ports.CompletionRequest {
		mu.Lock()
		defer mu.Unlock()
		return captured
	}
	return llm, getter
}

// autoCompactKeepLastTurn simulates compression: preserve system messages,
// insert a summary, keep only the last user turn.
func autoCompactKeepLastTurn(msgs []ports.Message, _ int) ([]ports.Message, bool) {
	var preserved, conversation []ports.Message
	for _, msg := range msgs {
		if ports.IsPreservedSource(msg.Source) {
			preserved = append(preserved, msg)
			continue
		}
		conversation = append(conversation, msg)
	}
	kept := ports.KeepRecentTurns(conversation, 1)
	result := make([]ports.Message, 0, len(preserved)+1+len(kept))
	result = append(result, preserved...)
	result = append(result, ports.Message{
		Role:    "assistant",
		Content: "[Earlier context compressed] summary of older turns.",
		Source:  ports.MessageSourceUserHistory,
	})
	result = append(result, kept...)
	return result, true
}

// TestThinkCompressesOverLimitContext verifies that think() compresses messages
// when the estimated token count exceeds the budget, and the LLM request sent
// has fewer messages than the original.
func TestThinkCompressesOverLimitContext(t *testing.T) {
	mockLLM, getReq := captureLLMRequest()

	// Use content-aware estimation. 10 turns with 200-char content ≈ decent size.
	// Set a tight limit so compression fires.
	tokenLimit := 500

	mockCtx := &mocks.MockContextManager{
		EstimateTokensFunc: contentAwareEstimate,
		ShouldCompressFunc: func(msgs []ports.Message, limit int) bool {
			return float64(contentAwareEstimate(msgs)) > float64(limit)*0.70
		},
		AutoCompactFunc:  autoCompactKeepLastTurn,
		BuildSummaryOnlyFunc: func(msgs []ports.Message) (string, int) {
			return "[Earlier context compressed] summary", len(msgs)
		},
	}

	engine := NewReactEngine(ReactEngineConfig{
		MaxIterations: 10,
		Logger:        agent.NoopLogger{},
		Clock:         agent.SystemClock{},
		CompletionDefaults: CompletionDefaults{
			ContextTokenLimit: &tokenLimit,
		},
	})

	msgs := buildConversation(10, 200)
	originalCount := len(msgs)
	state := &agent.TaskState{
		Iterations: 5,
		Messages:   msgs,
	}

	_, err := engine.think(context.Background(), state, agent.ServiceBundle{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      mockCtx,
	})
	if err != nil {
		t.Fatalf("think() error: %v", err)
	}

	req := getReq()
	if len(req.Messages) >= originalCount {
		t.Fatalf("expected compressed message count < %d (original), got %d",
			originalCount, len(req.Messages))
	}

	// Verify system prompt is preserved.
	if req.Messages[0].Role != "system" {
		t.Errorf("first message role = %q, want system", req.Messages[0].Role)
	}

	// Verify a compression/trim summary exists in the sent messages.
	hasSummary := false
	for _, msg := range req.Messages {
		if strings.HasPrefix(msg.Content, "[Earlier context compressed]") ||
			strings.HasPrefix(msg.Content, "[Context trimmed") ||
			strings.Contains(msg.Content, "context truncated") {
			hasSummary = true
			break
		}
	}
	if !hasSummary {
		t.Error("expected a compression summary in sent messages")
	}
}

// TestThinkPreservesRecentTurnAfterCompression verifies that the most recent
// user turn is always preserved after compression.
func TestThinkPreservesRecentTurnAfterCompression(t *testing.T) {
	mockLLM, getReq := captureLLMRequest()

	// Limit chosen so raw messages exceed budget.MessageLimit (limit - 1024),
	// but the AutoCompact result (4 msgs) fits under it.
	// Raw: 17 msgs × ~54 tokens ≈ 918 tokens. MessageLimit = 1500 - 1024 = 476.
	// Compacted: 4 msgs × ~50 tokens ≈ 200, well under 476.
	tokenLimit := 1500

	mockCtx := &mocks.MockContextManager{
		EstimateTokensFunc: contentAwareEstimate,
		ShouldCompressFunc: func(_ []ports.Message, _ int) bool { return true },
		AutoCompactFunc:    autoCompactKeepLastTurn,
		BuildSummaryOnlyFunc: func(msgs []ports.Message) (string, int) {
			return "[Earlier context compressed] summary", len(msgs)
		},
	}

	engine := NewReactEngine(ReactEngineConfig{
		MaxIterations: 10,
		Logger:        agent.NoopLogger{},
		Clock:         agent.SystemClock{},
		CompletionDefaults: CompletionDefaults{
			ContextTokenLimit: &tokenLimit,
		},
	})

	msgs := buildConversation(8, 200)
	state := &agent.TaskState{
		Iterations: 3,
		Messages:   msgs,
	}
	// The last user message has unique content: "turn-7 xxx..."
	lastUserContent := msgs[len(msgs)-2].Content

	_, err := engine.think(context.Background(), state, agent.ServiceBundle{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      mockCtx,
	})
	if err != nil {
		t.Fatalf("think() error: %v", err)
	}

	req := getReq()

	// AutoCompact should have reduced message count.
	if len(req.Messages) >= len(msgs) {
		t.Fatalf("expected compression to reduce messages, got %d (original %d)",
			len(req.Messages), len(msgs))
	}

	// The last user message must survive.
	found := false
	for _, msg := range req.Messages {
		if msg.Role == "user" && msg.Content == lastUserContent {
			found = true
			break
		}
	}
	if !found {
		t.Error("most recent user message not found in compressed output")
	}
}

// TestThinkDeferredSummaryAppliedOnBudgetExceed verifies that when a pending
// deferred summary exists and the budget is exceeded, the summary is applied
// immediately rather than waiting for the delay.
func TestThinkDeferredSummaryAppliedOnBudgetExceed(t *testing.T) {
	mockLLM, getReq := captureLLMRequest()

	// The limit is tight enough that the raw messages exceed it.
	tokenLimit := 500

	mockCtx := &mocks.MockContextManager{
		EstimateTokensFunc: contentAwareEstimate,
		ShouldCompressFunc: func(_ []ports.Message, _ int) bool { return true },
		AutoCompactFunc:    autoCompactKeepLastTurn,
		BuildSummaryOnlyFunc: func(msgs []ports.Message) (string, int) {
			return "[Earlier context compressed] deferred summary", len(msgs)
		},
	}

	engine := NewReactEngine(ReactEngineConfig{
		MaxIterations: 10,
		Logger:        agent.NoopLogger{},
		Clock:         agent.SystemClock{},
		CompletionDefaults: CompletionDefaults{
			ContextTokenLimit: &tokenLimit,
		},
	})

	msgs := buildConversation(6, 100)
	state := &agent.TaskState{
		Iterations:             8,
		Messages:               msgs,
		PendingSummary:         "[Earlier context compressed] from earlier.",
		PendingSummaryAtIter:   3,
		PendingSummaryMsgCount: 7,
	}

	_, err := engine.think(context.Background(), state, agent.ServiceBundle{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      mockCtx,
	})
	if err != nil {
		t.Fatalf("think() error: %v", err)
	}

	// Pending summary should have been cleared.
	if state.PendingSummary != "" {
		t.Errorf("expected PendingSummary to be cleared, got %q", state.PendingSummary)
	}

	req := getReq()
	if len(req.Messages) >= len(msgs) {
		t.Errorf("expected fewer messages after deferred summary apply, got %d (original %d)",
			len(req.Messages), len(msgs))
	}
}

// TestThinkAggressiveTrimFallback verifies that when AutoCompact is insufficient,
// the aggressive trim cascade activates and reduces messages to fit.
func TestThinkAggressiveTrimFallback(t *testing.T) {
	mockLLM, getReq := captureLLMRequest()

	tokenLimit := 400

	mockCtx := &mocks.MockContextManager{
		EstimateTokensFunc: contentAwareEstimate,
		ShouldCompressFunc: func(_ []ports.Message, _ int) bool { return true },
		// AutoCompact returns messages unchanged — compression "fails" to help.
		AutoCompactFunc: func(msgs []ports.Message, _ int) ([]ports.Message, bool) {
			return msgs, false
		},
		BuildSummaryOnlyFunc: func(msgs []ports.Message) (string, int) {
			return "[Earlier context compressed] summary", len(msgs)
		},
	}

	engine := NewReactEngine(ReactEngineConfig{
		MaxIterations: 10,
		Logger:        agent.NoopLogger{},
		Clock:         agent.SystemClock{},
		CompletionDefaults: CompletionDefaults{
			ContextTokenLimit: &tokenLimit,
		},
	})

	// 10 turns with moderate content — way over 400 token limit.
	msgs := buildConversation(10, 100)
	state := &agent.TaskState{
		Iterations: 10,
		Messages:   msgs,
	}

	_, err := engine.think(context.Background(), state, agent.ServiceBundle{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      mockCtx,
	})
	if err != nil {
		t.Fatalf("think() error: %v", err)
	}

	req := getReq()
	// Aggressive trim should dramatically reduce message count from 21.
	if len(req.Messages) >= len(msgs) {
		t.Errorf("expected aggressive trim to reduce messages, got %d (original %d)",
			len(req.Messages), len(msgs))
	}

	// System prompt must survive.
	hasSystem := false
	for _, msg := range req.Messages {
		if msg.Source == ports.MessageSourceSystemPrompt {
			hasSystem = true
			break
		}
	}
	if !hasSystem {
		t.Error("system prompt lost after aggressive trim")
	}
}

// TestThinkProactivePhaseGeneratesDeferredSummary verifies Phase A: when
// token usage crosses the compression threshold but is still under the hard
// limit, a deferred summary is generated (not applied) and stored on state.
func TestThinkProactivePhaseGeneratesDeferredSummary(t *testing.T) {
	mockLLM, _ := captureLLMRequest()

	// Use a generous limit so messages fit, but ShouldCompress still fires.
	tokenLimit := 100000

	summaryRequested := false
	mockCtx := &mocks.MockContextManager{
		EstimateTokensFunc: contentAwareEstimate,
		ShouldCompressFunc: func(_ []ports.Message, _ int) bool {
			return true // Always says "yes, you should compress"
		},
		BuildSummaryOnlyFunc: func(msgs []ports.Message) (string, int) {
			summaryRequested = true
			return "[Earlier context compressed] proactive summary", 5
		},
	}

	engine := NewReactEngine(ReactEngineConfig{
		MaxIterations: 10,
		Logger:        agent.NoopLogger{},
		Clock:         agent.SystemClock{},
		CompletionDefaults: CompletionDefaults{
			ContextTokenLimit: &tokenLimit,
		},
	})

	state := &agent.TaskState{
		Iterations: 2,
		Messages:   buildConversation(6, 200),
	}

	_, err := engine.think(context.Background(), state, agent.ServiceBundle{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      mockCtx,
	})
	if err != nil {
		t.Fatalf("think() error: %v", err)
	}

	if !summaryRequested {
		t.Error("expected BuildSummaryOnly to be called in Phase A (proactive)")
	}
	if state.PendingSummary == "" {
		t.Error("expected PendingSummary to be set after Phase A")
	}
	if state.PendingSummaryAtIter != 2 {
		t.Errorf("expected PendingSummaryAtIter=2, got %d", state.PendingSummaryAtIter)
	}
}

// TestThinkContextStatusInjectedAfterCompression verifies that the context
// status message is injected into the LLM request when compression occurs,
// and that the phase is not "ok".
func TestThinkContextStatusInjectedAfterCompression(t *testing.T) {
	mockLLM, getReq := captureLLMRequest()

	// Tight limit that AutoCompact can satisfy, so compression occurs but
	// the result fits.
	tokenLimit := 50000

	mockCtx := &mocks.MockContextManager{
		EstimateTokensFunc: func(msgs []ports.Message) int {
			// Return a high token count to push phase past "ok".
			return len(msgs) * 5000
		},
		ShouldCompressFunc: func(_ []ports.Message, _ int) bool { return true },
		AutoCompactFunc:    autoCompactKeepLastTurn,
		BuildSummaryOnlyFunc: func(msgs []ports.Message) (string, int) {
			return "[Earlier context compressed] summary", len(msgs)
		},
	}

	engine := NewReactEngine(ReactEngineConfig{
		MaxIterations: 10,
		Logger:        agent.NoopLogger{},
		Clock:         agent.SystemClock{},
		CompletionDefaults: CompletionDefaults{
			ContextTokenLimit: &tokenLimit,
		},
	})

	state := &agent.TaskState{
		Iterations: 5,
		Messages:   buildConversation(8, 20),
	}

	_, err := engine.think(context.Background(), state, agent.ServiceBundle{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      mockCtx,
	})
	if err != nil {
		t.Fatalf("think() error: %v", err)
	}

	req := getReq()
	hasCtxStatus := false
	for _, msg := range req.Messages {
		if strings.Contains(msg.Content, "<ctx") && strings.Contains(msg.Content, "phase=") {
			hasCtxStatus = true
			if msg.Role != "system" {
				t.Errorf("ctx status role = %q, want system", msg.Role)
			}
			if strings.Contains(msg.Content, `phase="ok"`) {
				t.Error("expected non-ok phase after compression")
			}
			break
		}
	}
	if !hasCtxStatus {
		t.Error("expected context status message to be injected after compression")
	}
}

// TestThinkNoCompressionBelowThreshold verifies that no compression occurs
// when the message token count is well below the threshold.
func TestThinkNoCompressionBelowThreshold(t *testing.T) {
	mockLLM, getReq := captureLLMRequest()

	tokenLimit := 100000

	mockCtx := &mocks.MockContextManager{
		EstimateTokensFunc: func(msgs []ports.Message) int {
			return len(msgs) * 100 // Very low usage
		},
		ShouldCompressFunc: func(_ []ports.Message, _ int) bool { return false },
	}

	engine := NewReactEngine(ReactEngineConfig{
		MaxIterations: 10,
		Logger:        agent.NoopLogger{},
		Clock:         agent.SystemClock{},
		CompletionDefaults: CompletionDefaults{
			ContextTokenLimit: &tokenLimit,
		},
	})

	msgs := buildConversation(3, 50)
	state := &agent.TaskState{
		Iterations: 1,
		Messages:   msgs,
	}
	originalCount := len(msgs)

	_, err := engine.think(context.Background(), state, agent.ServiceBundle{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      mockCtx,
	})
	if err != nil {
		t.Fatalf("think() error: %v", err)
	}

	req := getReq()
	if len(req.Messages) != originalCount {
		t.Errorf("expected all %d messages to be sent (no compression), got %d",
			originalCount, len(req.Messages))
	}

	// No context status should be injected for ok phase.
	for _, msg := range req.Messages {
		if strings.Contains(msg.Content, "<ctx") {
			t.Error("unexpected context status injection when under threshold")
		}
	}
}

// TestThinkAutoCompactBringsTokensUnderBudget verifies that after AutoCompact
// fires through the full think() pipeline, the estimated tokens of the final
// LLM request stay within the overall token limit. This catches the critical
// gap in existing tests that only check message count, not token count.
func TestThinkAutoCompactBringsTokensUnderBudget(t *testing.T) {
	mockLLM, getReq := captureLLMRequest()

	// Budget math: MessageLimit = 3024 - 1024 (safety) = 2000.
	// 20 turns × 200-char content: 41 msgs × ~55 tokens = ~2211 → exceeds 2000.
	// After AutoCompact (keep last turn): 4 msgs × ~55 = ~220 → fits.
	tokenLimit := 3024

	mockCtx := &mocks.MockContextManager{
		EstimateTokensFunc: contentAwareEstimate,
		ShouldCompressFunc: func(msgs []ports.Message, limit int) bool {
			return float64(contentAwareEstimate(msgs)) > float64(limit)*0.70
		},
		AutoCompactFunc:      autoCompactKeepLastTurn,
		BuildSummaryOnlyFunc: func(msgs []ports.Message) (string, int) { return "", 0 },
	}

	engine := NewReactEngine(ReactEngineConfig{
		MaxIterations: 10,
		Logger:        agent.NoopLogger{},
		Clock:         agent.SystemClock{},
		CompletionDefaults: CompletionDefaults{
			ContextTokenLimit: &tokenLimit,
		},
	})

	msgs := buildConversation(20, 200)
	state := &agent.TaskState{Iterations: 5, Messages: msgs}

	_, err := engine.think(context.Background(), state, agent.ServiceBundle{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      mockCtx,
	})
	if err != nil {
		t.Fatalf("think() error: %v", err)
	}

	req := getReq()
	estimated := contentAwareEstimate(req.Messages)
	if estimated > tokenLimit {
		t.Errorf("estimated tokens %d exceed token limit %d after compression", estimated, tokenLimit)
	}
	if len(req.Messages) >= len(msgs) {
		t.Fatalf("expected compression to reduce messages, got %d (original %d)", len(req.Messages), len(msgs))
	}

	// AutoCompact should suffice — no aggressive trim marker.
	for _, msg := range req.Messages {
		if strings.HasPrefix(msg.Content, "[Context trimmed") {
			t.Error("unexpected aggressive trim — AutoCompact should have been sufficient")
		}
	}
}

// TestThinkForceFitLastResort verifies that when messages are so large that
// even aggressive trim with 1 turn exceeds the budget, forceFitMessagesToLimit
// truncates content to bring tokens under the limit.
func TestThinkForceFitLastResort(t *testing.T) {
	mockLLM, getReq := captureLLMRequest()

	// MessageLimit = 2048 - 1024 = 1024.
	// 2 turns × 5000-char content: each conv msg ≈ 1254 tokens. Total ≈ 5035.
	// After aggressive trim (1 turn): 4 msgs ≈ 2545 tokens → still > 1024.
	// Force-fit halves largest messages until fit.
	tokenLimit := 2048

	mockCtx := &mocks.MockContextManager{
		EstimateTokensFunc: contentAwareEstimate,
		ShouldCompressFunc: func(_ []ports.Message, _ int) bool { return true },
		AutoCompactFunc:    func(msgs []ports.Message, _ int) ([]ports.Message, bool) { return msgs, false },
		BuildSummaryOnlyFunc: func(msgs []ports.Message) (string, int) {
			return "[Earlier context compressed] summary", len(msgs)
		},
	}

	engine := NewReactEngine(ReactEngineConfig{
		MaxIterations: 10,
		Logger:        agent.NoopLogger{},
		Clock:         agent.SystemClock{},
		CompletionDefaults: CompletionDefaults{
			ContextTokenLimit: &tokenLimit,
		},
	})

	msgs := buildConversation(2, 5000)
	state := &agent.TaskState{Iterations: 3, Messages: msgs}

	_, err := engine.think(context.Background(), state, agent.ServiceBundle{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      mockCtx,
	})
	if err != nil {
		t.Fatalf("think() error: %v", err)
	}

	req := getReq()
	estimated := contentAwareEstimate(req.Messages)
	if estimated > tokenLimit {
		t.Errorf("force-fit failed: estimated tokens %d exceed limit %d", estimated, tokenLimit)
	}
	// At least the system prompt should survive.
	if req.Messages[0].Role != "system" {
		t.Errorf("first message role = %q, want system", req.Messages[0].Role)
	}
}

// TestThinkDeferredSummaryAppliedNaturallyAfterDelay verifies Phase B:
// a pending deferred summary is applied after the configured delay (2 turns)
// even when the budget is not exceeded — the normal, non-emergency path.
func TestThinkDeferredSummaryAppliedNaturallyAfterDelay(t *testing.T) {
	mockLLM, getReq := captureLLMRequest()

	// Generous limit — messages fit comfortably.
	tokenLimit := 50000

	mockCtx := &mocks.MockContextManager{
		EstimateTokensFunc:   contentAwareEstimate,
		ShouldCompressFunc:   func(_ []ports.Message, _ int) bool { return false },
		BuildSummaryOnlyFunc: func(_ []ports.Message) (string, int) { return "", 0 },
	}

	engine := NewReactEngine(ReactEngineConfig{
		MaxIterations: 10,
		Logger:        agent.NoopLogger{},
		Clock:         agent.SystemClock{},
		CompletionDefaults: CompletionDefaults{
			ContextTokenLimit: &tokenLimit,
		},
	})

	msgs := buildConversation(4, 50) // 9 messages, ~135 tokens
	state := &agent.TaskState{
		Iterations:             5,
		Messages:               msgs,
		PendingSummary:         "[Earlier context compressed] from iter 2.",
		PendingSummaryAtIter:   2,
		PendingSummaryMsgCount: 7,
	}
	lastUserContent := msgs[len(msgs)-2].Content

	_, err := engine.think(context.Background(), state, agent.ServiceBundle{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      mockCtx,
	})
	if err != nil {
		t.Fatalf("think() error: %v", err)
	}

	if state.PendingSummary != "" {
		t.Errorf("PendingSummary should be cleared after natural apply, got %q", state.PendingSummary)
	}

	req := getReq()

	// Fewer messages than original — summary replaced older messages.
	if len(req.Messages) >= len(msgs) {
		t.Errorf("expected fewer messages after deferred apply, got %d (original %d)",
			len(req.Messages), len(msgs))
	}

	// Summary text must appear in the request.
	hasSummary := false
	for _, msg := range req.Messages {
		if strings.Contains(msg.Content, "[Earlier context compressed] from iter 2.") {
			hasSummary = true
			break
		}
	}
	if !hasSummary {
		t.Error("deferred summary text not found in LLM request")
	}

	// Most recent user message must survive.
	found := false
	for _, msg := range req.Messages {
		if msg.Content == lastUserContent {
			found = true
			break
		}
	}
	if !found {
		t.Error("most recent user message lost after deferred summary apply")
	}
}

// TestThinkDeferredSummaryNotAppliedBeforeDelay verifies that a pending
// deferred summary is NOT applied when fewer than delayedSummaryTurns (2)
// iterations have passed since generation.
func TestThinkDeferredSummaryNotAppliedBeforeDelay(t *testing.T) {
	mockLLM, getReq := captureLLMRequest()

	tokenLimit := 50000

	mockCtx := &mocks.MockContextManager{
		EstimateTokensFunc: contentAwareEstimate,
		ShouldCompressFunc: func(_ []ports.Message, _ int) bool { return false },
	}

	engine := NewReactEngine(ReactEngineConfig{
		MaxIterations: 10,
		Logger:        agent.NoopLogger{},
		Clock:         agent.SystemClock{},
		CompletionDefaults: CompletionDefaults{
			ContextTokenLimit: &tokenLimit,
		},
	})

	msgs := buildConversation(3, 50) // 7 messages
	originalCount := len(msgs)
	state := &agent.TaskState{
		Iterations:             3,                                         // Only 1 turn since generation
		Messages:               msgs,
		PendingSummary:         "[Earlier context compressed] too soon.",
		PendingSummaryAtIter:   2,                                         // delay requires iter >= 4
		PendingSummaryMsgCount: 5,
	}

	_, err := engine.think(context.Background(), state, agent.ServiceBundle{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      mockCtx,
	})
	if err != nil {
		t.Fatalf("think() error: %v", err)
	}

	if state.PendingSummary == "" {
		t.Error("PendingSummary should NOT be cleared before delay elapses")
	}

	req := getReq()
	if len(req.Messages) != originalCount {
		t.Errorf("expected all %d messages sent (no apply), got %d", originalCount, len(req.Messages))
	}
}

// TestThinkCascadeAutoCompactFailsThenAggressiveTrimSucceeds verifies that when
// AutoCompact returns messages unchanged (fails to help), the aggressive trim
// cascade activates and successfully brings tokens under budget with a trim
// marker rather than a compression summary.
func TestThinkCascadeAutoCompactFailsThenAggressiveTrimSucceeds(t *testing.T) {
	mockLLM, getReq := captureLLMRequest()

	// MessageLimit = 2524 - 1024 = 1500.
	// 15 turns × 200-char: 31 msgs × ~55 = ~1705 tokens → exceeds 1500.
	// AutoCompact fails. Aggressive trim (4 turns) → ~10 msgs ≈ ~560 → fits.
	tokenLimit := 2524

	mockCtx := &mocks.MockContextManager{
		EstimateTokensFunc: contentAwareEstimate,
		ShouldCompressFunc: func(_ []ports.Message, _ int) bool { return true },
		AutoCompactFunc:    func(msgs []ports.Message, _ int) ([]ports.Message, bool) { return msgs, false },
		BuildSummaryOnlyFunc: func(msgs []ports.Message) (string, int) {
			return "[Earlier context compressed] summary", len(msgs)
		},
	}

	engine := NewReactEngine(ReactEngineConfig{
		MaxIterations: 10,
		Logger:        agent.NoopLogger{},
		Clock:         agent.SystemClock{},
		CompletionDefaults: CompletionDefaults{
			ContextTokenLimit: &tokenLimit,
		},
	})

	msgs := buildConversation(15, 200)
	state := &agent.TaskState{Iterations: 10, Messages: msgs}
	lastUserContent := msgs[len(msgs)-2].Content

	_, err := engine.think(context.Background(), state, agent.ServiceBundle{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      mockCtx,
	})
	if err != nil {
		t.Fatalf("think() error: %v", err)
	}

	req := getReq()

	// Verify tokens fit.
	estimated := contentAwareEstimate(req.Messages)
	if estimated > tokenLimit {
		t.Errorf("estimated tokens %d exceed limit %d after aggressive trim", estimated, tokenLimit)
	}

	// Aggressive trim marker must be present.
	hasTrimMarker := false
	for _, msg := range req.Messages {
		if strings.HasPrefix(msg.Content, "[Context trimmed") {
			hasTrimMarker = true
			break
		}
	}
	if !hasTrimMarker {
		t.Error("expected aggressive trim marker — AutoCompact returned unchanged")
	}

	// No AutoCompact summary (it didn't fire).
	for _, msg := range req.Messages {
		if strings.HasPrefix(msg.Content, "[Earlier context compressed]") {
			t.Error("unexpected AutoCompact summary — it should have returned unchanged")
			break
		}
	}

	// Last user turn must survive.
	found := false
	for _, msg := range req.Messages {
		if msg.Content == lastUserContent {
			found = true
			break
		}
	}
	if !found {
		t.Error("most recent user message lost after aggressive trim")
	}
}

// TestThinkPreservesImportantAndCheckpointMessages verifies that messages
// marked as Important or Checkpoint survive compression when the budget allows.
func TestThinkPreservesImportantAndCheckpointMessages(t *testing.T) {
	mockLLM, getReq := captureLLMRequest()

	// Use content-aware estimation and a limit that aggressive trim with 1 turn
	// can satisfy while keeping preserved messages.
	tokenLimit := 5000

	mockCtx := &mocks.MockContextManager{
		EstimateTokensFunc: contentAwareEstimate,
		ShouldCompressFunc: func(_ []ports.Message, _ int) bool { return true },
		AutoCompactFunc:    autoCompactKeepLastTurn,
		BuildSummaryOnlyFunc: func(msgs []ports.Message) (string, int) {
			return "[Earlier context compressed] summary", len(msgs)
		},
	}

	engine := NewReactEngine(ReactEngineConfig{
		MaxIterations: 10,
		Logger:        agent.NoopLogger{},
		Clock:         agent.SystemClock{},
		CompletionDefaults: CompletionDefaults{
			ContextTokenLimit: &tokenLimit,
		},
	})

	msgs := []ports.Message{
		{Role: "system", Content: "System prompt", Source: ports.MessageSourceSystemPrompt},
		{Role: "system", Content: "IMPORTANT: never do X", Source: ports.MessageSourceImportant},
		{Role: "user", Content: "old question", Source: ports.MessageSourceUserInput},
		{Role: "assistant", Content: "old answer", Source: ports.MessageSourceAssistantReply},
		{Role: "user", Content: "checkpoint summary", Source: ports.MessageSourceCheckpoint},
		{Role: "user", Content: "mid question", Source: ports.MessageSourceUserInput},
		{Role: "assistant", Content: "mid answer", Source: ports.MessageSourceAssistantReply},
		{Role: "user", Content: "latest question", Source: ports.MessageSourceUserInput},
		{Role: "assistant", Content: "latest answer", Source: ports.MessageSourceAssistantReply},
	}

	state := &agent.TaskState{
		Iterations: 5,
		Messages:   msgs,
	}

	_, err := engine.think(context.Background(), state, agent.ServiceBundle{
		LLM:          mockLLM,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      mockCtx,
	})
	if err != nil {
		t.Fatalf("think() error: %v", err)
	}

	req := getReq()

	// AutoCompact preserves all IsPreservedSource messages (system prompt,
	// important, checkpoint), so they must be present in the LLM request.
	hasImportant := false
	hasCheckpoint := false
	for _, msg := range req.Messages {
		if msg.Source == ports.MessageSourceImportant {
			hasImportant = true
		}
		if msg.Source == ports.MessageSourceCheckpoint {
			hasCheckpoint = true
		}
	}
	if !hasImportant {
		t.Error("Important message lost after compression")
	}
	if !hasCheckpoint {
		t.Error("Checkpoint message lost after compression")
	}
}
