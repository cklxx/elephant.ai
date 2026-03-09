package context

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/ports/mocks"
	sessionstate "alex/internal/infra/session/state_store"
)

// ---------------------------------------------------------------------------
// Stress test: 10,000 rounds with mock LLM, verifying compression stability
// and cumulative ~100M token accounting across the full lifecycle.
//
// Performance note: these tests use a fast token estimator (len/4) instead of
// tiktoken BPE to avoid the O(n*m) cost of re-encoding all messages every
// round. The compression logic being tested is independent of the estimator.
// ---------------------------------------------------------------------------

// stressLLMClient is a mock LLM that tracks cumulative token usage.
type stressLLMClient struct {
	promptTokensPerCall     int
	completionTokensPerCall int
	cumulativePrompt        atomic.Int64
	cumulativeCompletion    atomic.Int64
	cumulativeTotal         atomic.Int64
	callCount               atomic.Int64
}

func (c *stressLLMClient) Complete(_ context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	prompt := c.promptTokensPerCall
	completion := c.completionTokensPerCall
	total := prompt + completion

	c.cumulativePrompt.Add(int64(prompt))
	c.cumulativeCompletion.Add(int64(completion))
	c.cumulativeTotal.Add(int64(total))
	c.callCount.Add(1)

	return &ports.CompletionResponse{
		Content:    fmt.Sprintf("Response for %d messages", len(req.Messages)),
		StopReason: "stop",
		Usage: ports.TokenUsage{
			PromptTokens:     prompt,
			CompletionTokens: completion,
			TotalTokens:      total,
		},
	}, nil
}

func (c *stressLLMClient) Model() string { return "mock-stress-model" }

func (c *stressLLMClient) StreamComplete(_ context.Context, req ports.CompletionRequest, cb ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
	resp, err := c.Complete(context.Background(), req)
	if err != nil {
		return nil, err
	}
	if cb.OnContentDelta != nil {
		cb.OnContentDelta(ports.ContentDelta{Delta: resp.Content, Final: true})
	}
	return resp, nil
}

// fastEstimateTokens is a fast O(1)-per-message token estimator: 4 overhead + len/4.
// This avoids tiktoken BPE which is the primary bottleneck in stress tests.
func fastEstimateTokens(msgs []ports.Message) int {
	total := 0
	for _, msg := range msgs {
		total += 4 + len(msg.Content)/4
		for _, tc := range msg.ToolCalls {
			total += 8 + len(tc.Name)/4
		}
	}
	return total
}

// fastCompress performs compression using the shared BuildCompressionPlan +
// buildCompressionSummary logic but with fast token estimation (no tiktoken).
func fastCompress(msgs []ports.Message, targetTokens int) ([]ports.Message, bool) {
	if targetTokens <= 0 || fastEstimateTokens(msgs) <= targetTokens {
		return msgs, false
	}

	plan := ports.BuildCompressionPlan(msgs, ports.CompressionPlanOptions{
		KeepRecentTurns: 1,
		PreserveSource:  ports.IsPreservedSource,
		IsSynthetic: func(msg ports.Message) bool {
			return ports.IsSyntheticSummary(msg.Content)
		},
	})
	if len(plan.CompressibleIndexes) == 0 || len(plan.SummarySource) == 0 {
		return msgs, false
	}

	summary := buildCompressionSummary(plan.SummarySource)
	if summary == "" {
		return msgs, false
	}

	summaryMsg := ports.Message{
		Role:    "assistant",
		Content: summary,
		Source:  ports.MessageSourceUserHistory,
	}

	compressed := make([]ports.Message, 0, len(msgs)-len(plan.CompressibleIndexes)+1)
	inserted := false
	for idx, msg := range msgs {
		if _, ok := plan.CompressibleIndexes[idx]; ok {
			if !inserted {
				compressed = append(compressed, summaryMsg)
				inserted = true
			}
			continue
		}
		compressed = append(compressed, msg)
	}
	if !inserted {
		compressed = append(compressed, summaryMsg)
	}
	return compressed, true
}

// newFastContextManager returns a mock context manager that uses fast token
// estimation (len/4) and the real compression plan logic — no tiktoken BPE.
func newFastContextManager(threshold float64) *mocks.MockContextManager {
	return &mocks.MockContextManager{
		EstimateTokensFunc: fastEstimateTokens,
		ShouldCompressFunc: func(msgs []ports.Message, limit int) bool {
			if limit <= 0 {
				return false
			}
			return float64(fastEstimateTokens(msgs)) > float64(limit)*threshold
		},
		AutoCompactFunc: func(msgs []ports.Message, limit int) ([]ports.Message, bool) {
			target := int(float64(limit) * threshold)
			return fastCompress(msgs, target)
		},
		BuildSummaryOnlyFunc: func(msgs []ports.Message) (string, int) {
			plan := ports.BuildCompressionPlan(msgs, ports.CompressionPlanOptions{
				KeepRecentTurns: 1,
				PreserveSource:  ports.IsPreservedSource,
				IsSynthetic: func(msg ports.Message) bool {
					return ports.IsSyntheticSummary(msg.Content)
				},
			})
			if len(plan.SummarySource) == 0 {
				return "", 0
			}
			return buildCompressionSummary(plan.SummarySource), len(plan.CompressibleIndexes)
		},
	}
}

// TestCompressionStress10KRounds drives 10,000 rounds of conversation through
// the compression pipeline, verifying:
//   - Compression fires when token budget is exceeded (no OOM/crash).
//   - History records are correctly maintained in the state store.
//   - Cumulative token accounting reaches ~100M without overflow or loss.
//   - Memory usage stays bounded (compression prevents unbounded growth).
func TestCompressionStress10KRounds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in -short mode")
	}

	const (
		totalRounds         = 10_000
		tokensPerPrompt     = 5_000
		tokensPerCompletion = 5_000
		contextWindowLimit  = 200_000 // Claude-class 200k window
		userContentSize     = 500     // ~125 fast-estimated tokens per message
		assistantContentSize = 500
		progressInterval    = 1_000
	)

	llmClient := &stressLLMClient{
		promptTokensPerCall:     tokensPerPrompt,
		completionTokensPerCall: tokensPerCompletion,
	}

	store := sessionstate.NewInMemoryStore()
	sessionID := "stress-test-session"
	histMgr := NewHistoryManager(store, agent.NoopLogger{}, agent.SystemClock{})

	mgr := newFastContextManager(defaultThreshold)

	messages := []ports.Message{
		{Role: "system", Content: "You are a helpful assistant.", Source: ports.MessageSourceSystemPrompt},
	}
	// Track previously-persisted messages to use AppendTurnWithExisting (avoids
	// re-loading all snapshots every round).
	var previousMessages []ports.Message

	var (
		compressionCount int
		maxMessageCount  int
		peakMemMB        uint64
		lastReplayLen    int
	)

	t.Logf("Starting stress test: %d rounds, %d tokens/call, %d context window",
		totalRounds, tokensPerPrompt+tokensPerCompletion, contextWindowLimit)
	start := time.Now()

	for round := 0; round < totalRounds; round++ {
		// 1. Add user message
		userMsg := ports.Message{
			Role:    "user",
			Content: fmt.Sprintf("Round %d question: %s", round, strings.Repeat("q", userContentSize)),
			Source:  ports.MessageSourceUserInput,
		}
		messages = append(messages, userMsg)

		// 2. Check compression and auto-compact if needed
		if mgr.ShouldCompress(messages, contextWindowLimit) {
			compacted, didCompact := mgr.AutoCompact(messages, contextWindowLimit)
			if didCompact {
				compressionCount++
				messages = compacted
			}
		}

		// 3. Call mock LLM
		_, err := llmClient.Complete(context.Background(), ports.CompletionRequest{
			Messages:  messages,
			MaxTokens: 4096,
		})
		if err != nil {
			t.Fatalf("round %d: LLM call failed: %v", round, err)
		}

		// 4. Add assistant response
		assistantMsg := ports.Message{
			Role:    "assistant",
			Content: fmt.Sprintf("Round %d: %s", round, strings.Repeat("a", assistantContentSize)),
			Source:  ports.MessageSourceAssistantReply,
		}
		messages = append(messages, assistantMsg)

		// 5. Persist turn to history (use WithExisting to avoid snapshot reload)
		if err := histMgr.AppendTurnWithExisting(context.Background(), sessionID, previousMessages, messages); err != nil {
			t.Fatalf("round %d: AppendTurnWithExisting failed: %v", round, err)
		}
		// Snapshot the current messages for next round's "existing" baseline.
		previousMessages = make([]ports.Message, len(messages))
		copy(previousMessages, messages)

		if len(messages) > maxMessageCount {
			maxMessageCount = len(messages)
		}

		// 6. Progress report
		if (round+1)%progressInterval == 0 {
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			allocMB := memStats.Alloc / (1024 * 1024)
			if allocMB > peakMemMB {
				peakMemMB = allocMB
			}
			t.Logf("Round %5d/%d | msgs=%3d | compressions=%d | cumTokens=%dM | alloc=%dMB",
				round+1, totalRounds, len(messages), compressionCount,
				llmClient.cumulativeTotal.Load()/(1_000_000), allocMB)
		}
	}

	elapsed := time.Since(start)

	// --- Verify cumulative token accounting ---
	expectedTotalTokens := int64(totalRounds) * int64(tokensPerPrompt+tokensPerCompletion)
	actualTotal := llmClient.cumulativeTotal.Load()
	if actualTotal != expectedTotalTokens {
		t.Errorf("cumulative tokens: got %d, want %d", actualTotal, expectedTotalTokens)
	}
	t.Logf("Cumulative tokens: %d (%dM)", actualTotal, actualTotal/1_000_000)

	if actualTotal < 90_000_000 {
		t.Errorf("expected cumulative tokens >= 90M, got %dM", actualTotal/1_000_000)
	}

	// --- Verify compression actually fired ---
	if compressionCount == 0 {
		t.Error("compression never fired — budget logic may be broken")
	}
	t.Logf("Compression fired %d times across %d rounds", compressionCount, totalRounds)

	// --- Verify message count stays bounded ---
	if maxMessageCount > 5000 {
		t.Errorf("max message count %d suggests compression isn't bounding growth", maxMessageCount)
	}
	t.Logf("Peak message count: %d", maxMessageCount)

	// --- Verify history replay ---
	replayed, err := histMgr.Replay(context.Background(), sessionID, 0)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}
	lastReplayLen = len(replayed)
	if lastReplayLen == 0 {
		t.Error("history replay returned empty — persistence is broken")
	}
	t.Logf("History replay: %d messages from store", lastReplayLen)

	// --- Verify call count ---
	calls := llmClient.callCount.Load()
	if calls != int64(totalRounds) {
		t.Errorf("LLM call count: got %d, want %d", calls, totalRounds)
	}

	// --- Final summary ---
	t.Logf("=== Stress Test Summary ===")
	t.Logf("  Rounds:        %d", totalRounds)
	t.Logf("  Duration:      %v", elapsed)
	t.Logf("  Compressions:  %d", compressionCount)
	t.Logf("  Total tokens:  %dM", actualTotal/1_000_000)
	t.Logf("  LLM calls:     %d", calls)
	t.Logf("  Peak messages: %d", maxMessageCount)
	t.Logf("  Peak memory:   %dMB", peakMemMB)
	t.Logf("  History turns: %d messages replayed", lastReplayLen)
}

// TestCompressionStressAggressiveTrim verifies that under extreme token pressure
// (tiny context window), aggressive trim kicks in and the system survives
// 10,000 rounds without panics or memory leaks.
func TestCompressionStressAggressiveTrim(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in -short mode")
	}

	const (
		totalRounds        = 10_000
		contextWindowLimit = 2_000 // Tiny window — forces aggressive trim almost every round
		userContentSize    = 200
	)

	llmClient := &stressLLMClient{
		promptTokensPerCall:     500,
		completionTokensPerCall: 500,
	}

	store := sessionstate.NewInMemoryStore()
	sessionID := "stress-aggressive-trim"
	histMgr := NewHistoryManager(store, agent.NoopLogger{}, agent.SystemClock{})

	mgr := newFastContextManager(defaultThreshold)

	messages := []ports.Message{
		{Role: "system", Content: "You are a helpful assistant.", Source: ports.MessageSourceSystemPrompt},
	}
	var previousMessages []ports.Message

	var compressionCount, trimCount int

	for round := 0; round < totalRounds; round++ {
		userMsg := ports.Message{
			Role:    "user",
			Content: fmt.Sprintf("R%d %s", round, strings.Repeat("x", userContentSize)),
			Source:  ports.MessageSourceUserInput,
		}
		messages = append(messages, userMsg)

		// Try auto-compact first
		if mgr.ShouldCompress(messages, contextWindowLimit) {
			compacted, didCompact := mgr.AutoCompact(messages, contextWindowLimit)
			if didCompact {
				compressionCount++
				messages = compacted
			}

			// If still over budget, use aggressive trim
			tokEst := fastEstimateTokens(messages)
			signal := BudgetCheck(tokEst, contextWindowLimit, defaultThreshold, 0.85)
			if signal == BudgetAggressiveTrim {
				messages = AggressiveTrim(messages, 2)
				trimCount++
			}
		}

		// Mock LLM call
		_, err := llmClient.Complete(context.Background(), ports.CompletionRequest{
			Messages:  messages,
			MaxTokens: 256,
		})
		if err != nil {
			t.Fatalf("round %d: LLM call failed: %v", round, err)
		}

		assistantMsg := ports.Message{
			Role:    "assistant",
			Content: fmt.Sprintf("R%d reply %s", round, strings.Repeat("r", userContentSize)),
			Source:  ports.MessageSourceAssistantReply,
		}
		messages = append(messages, assistantMsg)

		// Persist history
		if err := histMgr.AppendTurnWithExisting(context.Background(), sessionID, previousMessages, messages); err != nil {
			t.Fatalf("round %d: AppendTurnWithExisting failed: %v", round, err)
		}
		previousMessages = make([]ports.Message, len(messages))
		copy(previousMessages, messages)

		// Verify messages never grow unbounded
		if len(messages) > 100 {
			t.Fatalf("round %d: message count %d exceeds 100 — trim isn't working", round, len(messages))
		}
	}

	t.Logf("Aggressive trim stress: %d rounds, %d compressions, %d trims", totalRounds, compressionCount, trimCount)
	if trimCount == 0 && compressionCount == 0 {
		t.Error("neither compression nor trim fired — budget logic broken")
	}

	// Verify tokens
	actualTotal := llmClient.cumulativeTotal.Load()
	if actualTotal != int64(totalRounds)*1000 {
		t.Errorf("total tokens: got %d, want %d", actualTotal, int64(totalRounds)*1000)
	}
	t.Logf("Total tokens: %dM", actualTotal/1_000_000)
}

// TestCompressionStressHistoryIntegrity verifies that after many rounds of
// compression, the history manager's replay produces valid, ordered messages
// with no data corruption.
func TestCompressionStressHistoryIntegrity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in -short mode")
	}

	const (
		totalRounds        = 10_000
		contextWindowLimit = 50_000
		userContentSize    = 200
	)

	store := sessionstate.NewInMemoryStore()
	sessionID := "stress-history-integrity"
	histMgr := NewHistoryManager(store, agent.NoopLogger{}, agent.SystemClock{})

	mgr := newFastContextManager(defaultThreshold)

	messages := []ports.Message{
		{Role: "system", Content: "You are a helpful assistant.", Source: ports.MessageSourceSystemPrompt},
	}
	var previousMessages []ports.Message

	var lastUserContent, lastAssistantContent string

	for round := 0; round < totalRounds; round++ {
		userContent := fmt.Sprintf("Round-%d-user", round)
		assistantContent := fmt.Sprintf("Round-%d-assistant", round)
		lastUserContent = userContent
		lastAssistantContent = assistantContent

		messages = append(messages, ports.Message{
			Role:    "user",
			Content: userContent + strings.Repeat(".", userContentSize),
			Source:  ports.MessageSourceUserInput,
		})

		if mgr.ShouldCompress(messages, contextWindowLimit) {
			compacted, didCompact := mgr.AutoCompact(messages, contextWindowLimit)
			if didCompact {
				messages = compacted
			}
		}

		messages = append(messages, ports.Message{
			Role:    "assistant",
			Content: assistantContent + strings.Repeat(".", userContentSize),
			Source:  ports.MessageSourceAssistantReply,
		})

		if err := histMgr.AppendTurnWithExisting(context.Background(), sessionID, previousMessages, messages); err != nil {
			t.Fatalf("round %d: AppendTurnWithExisting failed: %v", round, err)
		}
		previousMessages = make([]ports.Message, len(messages))
		copy(previousMessages, messages)
	}

	// Replay full history
	replayed, err := histMgr.Replay(context.Background(), sessionID, 0)
	if err != nil {
		t.Fatalf("Replay failed: %v", err)
	}

	if len(replayed) == 0 {
		t.Fatal("replayed history is empty")
	}

	// The most recent messages should contain the last round's content.
	foundLastUser := false
	foundLastAssistant := false
	for _, msg := range replayed {
		if strings.HasPrefix(msg.Content, lastUserContent) {
			foundLastUser = true
		}
		if strings.HasPrefix(msg.Content, lastAssistantContent) {
			foundLastAssistant = true
		}
	}

	if !foundLastUser {
		t.Error("last user message not found in replayed history")
	}
	if !foundLastAssistant {
		t.Error("last assistant message not found in replayed history")
	}

	// Verify system prompt or summary appears at the start.
	if len(replayed) > 0 {
		first := replayed[0]
		if first.Source != ports.MessageSourceSystemPrompt &&
			first.Source != ports.MessageSourceUserHistory &&
			!strings.HasPrefix(first.Content, "[Earlier context compressed]") {
			t.Errorf("first replayed message has unexpected source=%q role=%q", first.Source, first.Role)
		}
	}

	t.Logf("History integrity: %d messages replayed from %d rounds", len(replayed), totalRounds)
}
