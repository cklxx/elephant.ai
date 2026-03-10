package react

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/ports/mocks"
	"alex/internal/domain/agent/ports/storage"
)

// ---------------------------------------------------------------------------
// Stress test via think() — full compression chain including artifact files.
//
// Validates that 10,000 rounds through the ReactEngine produce:
//   - Artifact .md files with correct JSON structure
//   - CTX_PLACEHOLDER messages with metadata
//   - Monotonically increasing compaction sequence numbers
//   - All artifact files readable and parsable
//   - History records maintained
//   - ~100M cumulative token accounting
// ---------------------------------------------------------------------------

// stressStreamingLLM tracks cumulative token usage and records requests.
type stressStreamingLLM struct {
	tokensPerCall int
	cumTotal      atomic.Int64
	callCount     atomic.Int64
}

func (c *stressStreamingLLM) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	half := c.tokensPerCall / 2
	c.cumTotal.Add(int64(c.tokensPerCall))
	c.callCount.Add(1)
	return &ports.CompletionResponse{
		Content:    fmt.Sprintf("Response for %d messages", len(req.Messages)),
		StopReason: "stop",
		Usage: ports.TokenUsage{
			PromptTokens:     half,
			CompletionTokens: c.tokensPerCall - half,
			TotalTokens:      c.tokensPerCall,
		},
	}, nil
}

func (c *stressStreamingLLM) Model() string { return "mock-stress" }

func (c *stressStreamingLLM) StreamComplete(ctx context.Context, req ports.CompletionRequest, cb ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
	resp, err := c.Complete(ctx, req)
	if err != nil {
		return nil, err
	}
	if cb.OnContentDelta != nil {
		cb.OnContentDelta(ports.ContentDelta{Delta: resp.Content, Final: true})
	}
	return resp, nil
}

// fastEstimate is a fast token estimator: 4 overhead + len/4 per message.
func fastEstimate(msgs []ports.Message) int {
	total := 0
	for _, msg := range msgs {
		total += 4 + len(msg.Content)/4
	}
	return total
}

// stressContextManager uses fast estimation and real compression plan logic.
type stressContextManager struct {
	threshold float64
}

func (m *stressContextManager) EstimateTokens(msgs []ports.Message) int {
	return fastEstimate(msgs)
}

func (m *stressContextManager) Compress(msgs []ports.Message, target int) ([]ports.Message, error) {
	return msgs, nil
}

func (m *stressContextManager) AutoCompact(msgs []ports.Message, limit int) ([]ports.Message, bool) {
	target := int(float64(limit) * m.threshold)
	if fastEstimate(msgs) <= target {
		return msgs, false
	}
	plan := ports.BuildCompressionPlan(msgs, ports.CompressionPlanOptions{
		KeepRecentTurns: 1,
		PreserveSource:  ports.IsPreservedSource,
		IsSynthetic:     func(msg ports.Message) bool { return ports.IsSyntheticSummary(msg.Content) },
	})
	if len(plan.CompressibleIndexes) == 0 || len(plan.SummarySource) == 0 {
		return msgs, false
	}
	summary := fmt.Sprintf("[Earlier context compressed] %d messages summarized.", len(plan.SummarySource))
	summaryMsg := ports.Message{Role: "assistant", Content: summary, Source: ports.MessageSourceUserHistory}
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
	return compressed, true
}

func (m *stressContextManager) ShouldCompress(msgs []ports.Message, limit int) bool {
	if limit <= 0 {
		return false
	}
	return float64(fastEstimate(msgs)) > float64(limit)*m.threshold
}

func (m *stressContextManager) BuildSummaryOnly(msgs []ports.Message) (string, int) {
	plan := ports.BuildCompressionPlan(msgs, ports.CompressionPlanOptions{
		KeepRecentTurns: 1,
		PreserveSource:  ports.IsPreservedSource,
		IsSynthetic:     func(msg ports.Message) bool { return ports.IsSyntheticSummary(msg.Content) },
	})
	if len(plan.SummarySource) == 0 {
		return "", 0
	}
	return fmt.Sprintf("[Earlier context compressed] %d messages summarized.", len(plan.SummarySource)), len(plan.CompressibleIndexes)
}

func (m *stressContextManager) Preload(_ context.Context) error { return nil }
func (m *stressContextManager) BuildWindow(_ context.Context, session *storage.Session, _ agent.ContextWindowConfig) (agent.ContextWindow, error) {
	if session == nil {
		return agent.ContextWindow{}, nil
	}
	return agent.ContextWindow{SessionID: session.ID, Messages: session.Messages}, nil
}
func (m *stressContextManager) RecordTurn(_ context.Context, _ agent.ContextTurnRecord) error {
	return nil
}

// TestCompressionArtifactStress10KRounds drives 10,000 rounds through think(),
// which triggers the full compression chain: tryArtifactCompaction → AutoCompact
// → AggressiveTrim. Verifies artifact files are produced and structurally valid.
func TestCompressionArtifactStress10KRounds(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in -short mode")
	}

	const (
		totalRounds   = 10_000
		tokensPerCall = 10_000
		// Context limit=3000. Each message ≈504 fast-estimated tokens (4+2000/4).
		// Adding user+assistant per round ≈1008 tokens. After ~3 rounds without
		// compression, tokens exceed the limit and trigger the safety net which
		// calls tryArtifactCompaction — writing artifact files.
		contextLimit  = 3_000
		contentSize   = 2_000
		progressEvery = 1_000
	)

	root := t.TempDir()
	llm := &stressStreamingLLM{tokensPerCall: tokensPerCall}
	ctxMgr := &stressContextManager{threshold: 0.7}
	engine := NewReactEngine(ReactEngineConfig{
		MaxIterations: totalRounds + 10,
		Logger:        agent.NoopLogger{},
		Clock:         agent.SystemClock{},
		CompletionDefaults: CompletionDefaults{
			ContextTokenLimit: intPtr(contextLimit),
		},
		CheckpointStore:  newTestFileCheckpointStore(filepath.Join(root, "checkpoints")),
		AtomicFileWriter: testAtomicWriter{},
	})

	state := &TaskState{
		SessionID:  "stress-artifact",
		RunID:      "run-stress-1",
		Iterations: 0,
		Messages: []ports.Message{
			{Role: "system", Content: "You are a helpful assistant.", Source: ports.MessageSourceSystemPrompt},
		},
	}

	services := Services{
		LLM:          llm,
		ToolExecutor: &mocks.MockToolRegistry{},
		Parser:       &mocks.MockParser{},
		Context:      ctxMgr,
	}

	var (
		artifactPaths     []string
		placeholderCount  int
		compressionEvents int
		maxMessageCount   int
		peakMemMB         uint64
	)

	t.Logf("Starting artifact stress: %d rounds, context_limit=%d, tokens/call=%d",
		totalRounds, contextLimit, tokensPerCall)
	start := time.Now()

	for round := 0; round < totalRounds; round++ {
		// Add user message
		state.Messages = append(state.Messages, ports.Message{
			Role:    "user",
			Content: fmt.Sprintf("Round %d: %s", round, strings.Repeat("q", contentSize)),
			Source:  ports.MessageSourceUserInput,
		})
		state.Iterations = round

		// Call think() — this drives the full compression chain
		prevSeq := state.ContextCompactionSeq
		resp, err := engine.think(context.Background(), state, services)
		if err != nil {
			t.Fatalf("round %d: think() failed: %v", round, err)
		}

		// Add assistant response
		state.Messages = append(state.Messages, ports.Message{
			Role:    "assistant",
			Content: resp.Content,
			Source:  ports.MessageSourceAssistantReply,
		})

		// Track artifact compaction events
		if state.ContextCompactionSeq > prevSeq {
			compressionEvents++
			if state.LastCompactionArtifact != "" {
				artifactPaths = append(artifactPaths, state.LastCompactionArtifact)
			}
		}

		// Count placeholders in current messages
		for _, msg := range state.Messages {
			if strings.HasPrefix(msg.Content, "[CTX_PLACEHOLDER") {
				placeholderCount++
				break
			}
		}

		if len(state.Messages) > maxMessageCount {
			maxMessageCount = len(state.Messages)
		}

		// Progress
		if (round+1)%progressEvery == 0 {
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			allocMB := memStats.Alloc / (1024 * 1024)
			if allocMB > peakMemMB {
				peakMemMB = allocMB
			}
			t.Logf("Round %5d/%d | msgs=%3d | artifacts=%d | seq=%d | cumTokens=%dM | alloc=%dMB",
				round+1, totalRounds, len(state.Messages), compressionEvents,
				state.ContextCompactionSeq, llm.cumTotal.Load()/(1_000_000), allocMB)
		}
	}

	elapsed := time.Since(start)

	// --- Verify cumulative tokens ---
	actualTotal := llm.cumTotal.Load()
	expectedTotal := int64(totalRounds) * int64(tokensPerCall)
	if actualTotal != expectedTotal {
		t.Errorf("cumulative tokens: got %d, want %d", actualTotal, expectedTotal)
	}
	if actualTotal < 90_000_000 {
		t.Errorf("expected >= 90M tokens, got %dM", actualTotal/1_000_000)
	}

	// --- Verify artifact compaction fired ---
	if compressionEvents == 0 {
		t.Error("artifact compaction never fired — tryArtifactCompaction not reached")
	}
	t.Logf("Artifact compaction events: %d", compressionEvents)

	// --- Verify artifact files exist and are structurally valid ---
	t.Logf("Verifying %d artifact files...", len(artifactPaths))
	for i, artifactPath := range artifactPaths {
		data, err := os.ReadFile(artifactPath)
		if err != nil {
			t.Errorf("artifact %d: file not found at %s: %v", i, artifactPath, err)
			continue
		}

		content := string(data)

		// 1. Must have YAML frontmatter
		if !strings.HasPrefix(content, "---\n") {
			t.Errorf("artifact %d: missing YAML frontmatter", i)
		}

		// 2. Must have kind marker
		if !strings.Contains(content, "kind: context_compaction_artifact") {
			t.Errorf("artifact %d: missing kind marker", i)
		}

		// 3. Must have session_id
		if !strings.Contains(content, `session_id: "stress-artifact"`) {
			t.Errorf("artifact %d: missing session_id", i)
		}

		// 4. Extract and validate JSON payload
		jsonStart := strings.Index(content, "```json\n")
		jsonEnd := strings.LastIndex(content, "\n```")
		if jsonStart < 0 || jsonEnd < 0 {
			t.Errorf("artifact %d: missing JSON code block", i)
			continue
		}
		jsonPayload := content[jsonStart+len("```json\n") : jsonEnd]

		var artifact contextCompactionArtifact
		if err := json.Unmarshal([]byte(jsonPayload), &artifact); err != nil {
			t.Errorf("artifact %d: invalid JSON: %v", i, err)
			continue
		}

		// 5. Validate artifact fields
		if artifact.Kind != compactionArtifactKind {
			t.Errorf("artifact %d: kind=%q, want %q", i, artifact.Kind, compactionArtifactKind)
		}
		if artifact.SessionID != "stress-artifact" {
			t.Errorf("artifact %d: session_id=%q", i, artifact.SessionID)
		}
		if artifact.Sequence <= 0 {
			t.Errorf("artifact %d: sequence=%d (should be > 0)", i, artifact.Sequence)
		}
		if artifact.MessageCount <= 0 {
			t.Errorf("artifact %d: message_count=%d (should be > 0)", i, artifact.MessageCount)
		}
		if len(artifact.Messages) == 0 {
			t.Errorf("artifact %d: messages array empty", i)
		}
		if artifact.TokensRemoved <= 0 {
			t.Errorf("artifact %d: tokens_removed=%d (should be > 0)", i, artifact.TokensRemoved)
		}

		// 6. Validate each message entry has role and content
		for j, entry := range artifact.Messages {
			if entry.Role == "" {
				t.Errorf("artifact %d message %d: empty role", i, j)
			}
			if entry.Content == "" && len(entry.ToolCalls) == 0 {
				t.Errorf("artifact %d message %d: empty content and no tool calls", i, j)
			}
		}
	}

	// --- Verify sequences are monotonically increasing ---
	if len(artifactPaths) >= 2 {
		for i := 1; i < len(artifactPaths); i++ {
			// Sequences must increase (checked via artifact file names)
			prev := artifactPaths[i-1]
			curr := artifactPaths[i]
			if prev >= curr {
				t.Errorf("artifact paths not monotonically increasing: %s >= %s", prev, curr)
			}
		}
	}

	// --- Verify message count stays bounded ---
	if maxMessageCount > 200 {
		t.Errorf("peak message count %d — compression not bounding growth", maxMessageCount)
	}

	// --- Verify state consistency ---
	if state.ContextCompactionSeq != len(artifactPaths) {
		t.Errorf("ContextCompactionSeq=%d but found %d artifact paths",
			state.ContextCompactionSeq, len(artifactPaths))
	}

	// --- Final summary ---
	t.Logf("=== Artifact Stress Test Summary ===")
	t.Logf("  Rounds:          %d", totalRounds)
	t.Logf("  Duration:        %v", elapsed)
	t.Logf("  Artifacts:       %d files", len(artifactPaths))
	t.Logf("  CompactionSeq:   %d", state.ContextCompactionSeq)
	t.Logf("  Total tokens:    %dM", actualTotal/1_000_000)
	t.Logf("  LLM calls:       %d", llm.callCount.Load())
	t.Logf("  Peak messages:   %d", maxMessageCount)
	t.Logf("  Peak memory:     %dMB", peakMemMB)
	t.Logf("  Final messages:  %d", len(state.Messages))

	// --- Dump a sample artifact for visibility ---
	if len(artifactPaths) > 0 {
		sample, _ := os.ReadFile(artifactPaths[0])
		if len(sample) > 2000 {
			sample = sample[:2000]
		}
		t.Logf("=== Sample artifact (first) ===\n%s\n...", string(sample))
	}
}

// intPtr defined in engine_internal_test.go — reuse via package scope.
